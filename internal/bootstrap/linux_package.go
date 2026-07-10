package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/auren/auren-transfer-agent/internal/config"
	"github.com/auren/auren-transfer-agent/internal/identity"
	"github.com/auren/auren-transfer-agent/internal/mediahub"
	"github.com/auren/auren-transfer-agent/internal/runtime"
)

const (
	linuxDefaultConfigPath = "/etc/auren-transfer-agent/agent.yaml"
	linuxDefaultDataDir    = "/var/lib/auren-transfer-agent"
	linuxDefaultLogDir     = "/var/log/auren-transfer-agent"
	linuxDefaultTempDir    = "/var/tmp/auren-transfer-agent"
	linuxDefaultUnit       = "auren-transfer-agent.service"
)

type bootstrapOptions struct {
	ConfigPath             string
	EnvFile                string
	MediaHubURL            string
	RegistrationToken      string
	BootstrapTokenEndpoint string
	BootstrapTokenSecret   string
	Role                   string
	Region                 string
	AvailabilityZone       string
	PublicBaseURL          string
	HealthURL              string
	DataDir                string
	WorkDir                string
	LogDir                 string
	ServerHost             string
	ServerPort             int
	MaxConcurrentJobs      int
	MaxSessions            int
	MaxEgressMbps          int
	EnableGateway          bool
	DisableTransfer        bool
	SkipRegister           bool
	StartService           bool
	SystemdUnit            string
	DryRun                 bool
}

func runBootstrap(args []string) error {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	options := bootstrapOptions{}
	flags.StringVar(&options.ConfigPath, "config", linuxDefaultConfigPath, "config file to write")
	flags.StringVar(&options.EnvFile, "env-file", ".env", "dotenv file with bootstrap/runtime variables")
	flags.StringVar(&options.MediaHubURL, "media-hub", "", "Auren Media Hub base URL")
	flags.StringVar(&options.RegistrationToken, "token", "", "one-time Media Hub registration token")
	flags.StringVar(&options.BootstrapTokenEndpoint, "token-endpoint", "", "optional endpoint that returns a one-time registration token")
	flags.StringVar(&options.BootstrapTokenSecret, "token-secret", "", "optional shared secret used when requesting a registration token")
	flags.StringVar(&options.Role, "role", "hybrid", "node role: worker, gateway, edge or hybrid")
	flags.StringVar(&options.Region, "region", "sa-east-1", "node region")
	flags.StringVar(&options.AvailabilityZone, "availability-zone", "", "availability zone")
	flags.StringVar(&options.PublicBaseURL, "public-base-url", "", "public base URL for Gateway Runtime")
	flags.StringVar(&options.HealthURL, "health-url", "", "public health URL")
	flags.StringVar(&options.DataDir, "data-dir", linuxDefaultDataDir, "durable data directory")
	flags.StringVar(&options.WorkDir, "work-dir", filepath.Join(linuxDefaultDataDir, "transfer"), "transfer working directory")
	flags.StringVar(&options.LogDir, "log-dir", linuxDefaultLogDir, "log directory")
	flags.StringVar(&options.ServerHost, "server-host", "0.0.0.0", "HTTP bind host")
	flags.IntVar(&options.ServerPort, "server-port", 8080, "HTTP bind port")
	flags.IntVar(&options.MaxConcurrentJobs, "max-concurrent-jobs", 2, "max concurrent transfer jobs")
	flags.IntVar(&options.MaxSessions, "max-sessions", 500, "max gateway sessions")
	flags.IntVar(&options.MaxEgressMbps, "max-egress-mbps", 1000, "max egress Mbps")
	flags.BoolVar(&options.EnableGateway, "enable-gateway", false, "enable public Gateway Runtime")
	flags.BoolVar(&options.DisableTransfer, "disable-transfer", false, "disable transfer claim loop")
	flags.BoolVar(&options.SkipRegister, "skip-register", false, "write config without registering immediately")
	flags.BoolVar(&options.StartService, "start-service", false, "enable and start the systemd service after bootstrap")
	flags.StringVar(&options.SystemdUnit, "systemd-unit", linuxDefaultUnit, "systemd unit name")
	flags.BoolVar(&options.DryRun, "dry-run", false, "print intended actions without writing files")
	showHelp := flags.Bool("help", false, "print help")
	flags.BoolVar(showHelp, "h", false, "print help")
	if err := flags.Parse(args); err != nil {
		return err
	}
	visited := map[string]bool{}
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	if err := config.LoadEnvFiles([]string{options.EnvFile}); err != nil {
		return err
	}
	applyBootstrapEnv(&options, visited)
	if *showHelp {
		printBootstrapHelp(os.Stdout)
		return nil
	}
	if err := validateBootstrapOptions(options); err != nil {
		return err
	}
	if strings.TrimSpace(options.RegistrationToken) == "" && !options.SkipRegister && strings.TrimSpace(options.BootstrapTokenEndpoint) != "" {
		token, err := requestBootstrapRegistrationToken(options)
		if err != nil {
			return err
		}
		options.RegistrationToken = token
		fmt.Fprintln(os.Stdout, "media-hub: registration token acquired from token endpoint")
	}
	cfg, err := bootstrapConfig(options)
	if err != nil {
		return err
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}
	if options.DryRun {
		fmt.Fprintf(os.Stdout, "bootstrap dry-run: config=%s media_hub=%s role=%s gateway=%t transfer=%t\n", options.ConfigPath, cfg.MediaHub.BaseURL, cfg.MediaHub.Role, cfg.MediaHub.GatewayEnabled, cfg.MediaHub.TransferEnabled)
		return nil
	}
	if err := ensureLinuxRuntimeDirs(cfg, options); err != nil {
		return err
	}
	if !options.SkipRegister {
		state, err := registerBootstrapNode(cfg)
		if err != nil {
			return err
		}
		cfg.MediaHub.NodeUUID = state.NodeUUID
		cfg.MediaHub.NodeSecret = ""
		cfg.MediaHub.RegistrationToken = ""
		cfg.MediaHub.BootstrapTokenSecret = ""
		fmt.Fprintf(os.Stdout, "media-hub: registered node_uuid=%s state=%s\n", state.NodeUUID, mediahub.DefaultStatePath(cfg.Runtime.DataDir))
	} else {
		fmt.Fprintln(os.Stdout, "media-hub: registration skipped")
	}
	if err := writeAgentConfig(options.ConfigPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "config: written %s\n", options.ConfigPath)
	if options.StartService {
		if !systemdManageable() {
			fmt.Fprintln(os.Stdout, "systemd: not available as PID 1; service start skipped")
			fmt.Fprintf(os.Stdout, "systemd: run manually with: sudo auren-transfer-agent serve --config %s\n", options.ConfigPath)
		} else {
			if err := systemctl("daemon-reload"); err != nil {
				return err
			}
			if err := systemctl("enable", "--now", options.SystemdUnit); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "systemd: enabled and started %s\n", options.SystemdUnit)
		}
	}
	fmt.Fprintln(os.Stdout, "bootstrap: complete")
	return nil
}

func runDoctor(args []string) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", linuxDefaultConfigPath, "config file to validate")
	envFile := flags.String("env-file", ".env", "dotenv file with runtime variables")
	online := flags.Bool("online", false, "also test HTTP connectivity to Media Hub and local health")
	showHelp := flags.Bool("help", false, "print help")
	flags.BoolVar(showHelp, "h", false, "print help")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showHelp {
		fmt.Fprintln(os.Stdout, "Usage: auren-transfer-agent doctor [--config /etc/auren-transfer-agent/agent.yaml] [--online]")
		return nil
	}
	cfg, err := config.Load(config.LoadOptions{Path: *configPath, EnvFiles: []string{*envFile}})
	if err != nil {
		return err
	}
	checks := []string{}
	checks = append(checks, okLine("config", *configPath))
	for _, dir := range []struct{ name, path string }{{"data_dir", cfg.Runtime.DataDir}, {"temp_dir", cfg.Runtime.TempDir}, {"download_temp_dir", cfg.Download.TempDir}, {"storage_local_path", cfg.Storage.LocalPath}, {"media_hub_work_dir", cfg.MediaHub.WorkDir}} {
		if strings.TrimSpace(dir.path) == "" {
			checks = append(checks, warnLine(dir.name, "empty"))
			continue
		}
		if _, err := os.Stat(dir.path); err == nil {
			checks = append(checks, okLine(dir.name, dir.path))
		} else if errors.Is(err, os.ErrNotExist) {
			checks = append(checks, warnLine(dir.name, dir.path+" missing"))
		} else {
			checks = append(checks, failLine(dir.name, err.Error()))
		}
	}
	statePath := mediahub.DefaultStatePath(cfg.Runtime.DataDir)
	if cfg.MediaHub.Enabled {
		if state, err := mediahub.NewStateStore(statePath).Load(); err == nil {
			checks = append(checks, okLine("media_hub_state", state.NodeUUID))
		} else if errors.Is(err, os.ErrNotExist) {
			checks = append(checks, warnLine("media_hub_state", statePath+" missing; run bootstrap"))
		} else {
			checks = append(checks, failLine("media_hub_state", err.Error()))
		}
		if *online {
			checks = append(checks, httpCheck("media_hub", strings.TrimRight(cfg.MediaHub.BaseURL, "/")+"/health"))
			if cfg.Server.Enabled && cfg.DevUI.Enabled {
				checks = append(checks, httpCheck("dev_ui", "http://127.0.0.1:"+strconv.Itoa(cfg.Server.Port)+strings.TrimRight(cfg.DevUI.Path, "/")+"/api/snapshot"))
			}
		}
	} else {
		checks = append(checks, warnLine("media_hub", "disabled"))
	}
	if systemdManageable() {
		checks = append(checks, okLine("systemd", "manageable"))
	} else if systemctlAvailable() {
		checks = append(checks, warnLine("systemd", "systemctl exists but PID 1 is not systemd"))
	} else {
		checks = append(checks, warnLine("systemd", "systemctl not found"))
	}
	for _, check := range checks {
		fmt.Fprintln(os.Stdout, check)
	}
	for _, check := range checks {
		if strings.HasPrefix(check, "FAIL") {
			return fmt.Errorf("doctor found failing checks")
		}
	}
	return nil
}

func runStatus(args []string) error {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", linuxDefaultConfigPath, "config file to inspect")
	envFile := flags.String("env-file", ".env", "dotenv file with runtime variables")
	showHelp := flags.Bool("help", false, "print help")
	flags.BoolVar(showHelp, "h", false, "print help")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showHelp {
		fmt.Fprintln(os.Stdout, "Usage: auren-transfer-agent status [--config /etc/auren-transfer-agent/agent.yaml]")
		return nil
	}
	cfg, err := config.Load(config.LoadOptions{Path: *configPath, EnvFiles: []string{*envFile}})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s %s\n", runtime.AppName, runtime.Version)
	fmt.Fprintf(os.Stdout, "config: %s\n", *configPath)
	fmt.Fprintf(os.Stdout, "server: enabled=%t address=%s\n", cfg.Server.Enabled, cfg.ServerAddress())
	fmt.Fprintf(os.Stdout, "media-hub: enabled=%t base_url=%s role=%s transfer=%t gateway=%t\n", cfg.MediaHub.Enabled, cfg.MediaHub.BaseURL, cfg.MediaHub.Role, cfg.MediaHub.TransferEnabled, cfg.MediaHub.GatewayEnabled)
	fmt.Fprintf(os.Stdout, "dev-ui: enabled=%t metrics=http://127.0.0.1:%d%s/metrics requests=http://127.0.0.1:%d%s/requests\n", cfg.DevUI.Enabled, cfg.Server.Port, strings.TrimRight(cfg.DevUI.Path, "/"), cfg.Server.Port, strings.TrimRight(cfg.DevUI.Path, "/"))
	statePath := mediahub.DefaultStatePath(cfg.Runtime.DataDir)
	if state, err := mediahub.NewStateStore(statePath).Load(); err == nil {
		fmt.Fprintf(os.Stdout, "node: uuid=%s config_version=%s registered_at=%s state=%s\n", state.NodeUUID, state.ConfigVersion, state.RegisteredAt.Format(time.RFC3339), statePath)
	} else {
		fmt.Fprintf(os.Stdout, "node: not registered state=%s\n", statePath)
	}
	if systemdManageable() {
		_ = runSystemctlStatus(linuxDefaultUnit)
	} else if systemctlAvailable() {
		fmt.Fprintln(os.Stdout, "systemd: not manageable; PID 1 is not systemd")
	}
	return nil
}

func applyBootstrapEnv(options *bootstrapOptions, explicit map[string]bool) {
	if options == nil {
		return
	}
	setString := func(flagName string, target *string, names ...string) {
		if explicit[flagName] {
			return
		}
		for _, name := range names {
			if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
				*target = strings.TrimSpace(value)
				return
			}
		}
	}
	setInt := func(flagName string, target *int, names ...string) {
		if explicit[flagName] {
			return
		}
		for _, name := range names {
			value := strings.TrimSpace(os.Getenv(name))
			if value == "" {
				continue
			}
			if parsed, err := strconv.Atoi(value); err == nil {
				*target = parsed
				return
			}
		}
	}
	setBool := func(flagName string, target *bool, names ...string) {
		if explicit[flagName] {
			return
		}
		for _, name := range names {
			value := strings.TrimSpace(os.Getenv(name))
			if value == "" {
				continue
			}
			if parsed, err := strconv.ParseBool(value); err == nil {
				*target = parsed
				return
			}
		}
	}

	setString("media-hub", &options.MediaHubURL, "AUREN_MEDIA_HUB_BASE_URL", "AUREN_AGENT_MEDIA_HUB_BASE_URL", "AUREN_MEDIA_HUB_URL", "MEDIA_HUB_URL")
	setString("token", &options.RegistrationToken, "AUREN_MEDIA_HUB_REGISTRATION_TOKEN", "AUREN_NODE_REGISTRATION_TOKEN", "AUREN_AGENT_REGISTRATION_TOKEN", "AUREN_REGISTRATION_TOKEN", "REGISTRATION_TOKEN")
	setString("token-endpoint", &options.BootstrapTokenEndpoint, "AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_ENDPOINT", "AUREN_BOOTSTRAP_TOKEN_ENDPOINT", "AUREN_NODE_TOKEN_ENDPOINT", "MEDIA_HUB_NODE_TOKEN_ENDPOINT")
	setString("token-secret", &options.BootstrapTokenSecret, "AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_SECRET", "AUREN_BOOTSTRAP_TOKEN_SECRET", "AUREN_NODE_TOKEN_SECRET", "MEDIA_HUB_NODE_TOKEN_SECRET")
	setString("role", &options.Role, "AUREN_AGENT_ROLE", "AUREN_NODE_ROLE")
	setString("region", &options.Region, "AUREN_AGENT_REGION", "AUREN_NODE_REGION", "AWS_REGION", "AWS_DEFAULT_REGION")
	setString("availability-zone", &options.AvailabilityZone, "AUREN_AGENT_AVAILABILITY_ZONE", "AUREN_NODE_AVAILABILITY_ZONE")
	setString("public-base-url", &options.PublicBaseURL, "AUREN_AGENT_PUBLIC_BASE_URL", "AUREN_NODE_PUBLIC_BASE_URL")
	setString("health-url", &options.HealthURL, "AUREN_AGENT_HEALTH_URL", "AUREN_NODE_HEALTH_URL")
	setString("data-dir", &options.DataDir, "AUREN_RUNTIME_DATA_DIR", "AUREN_AGENT_DATA_DIR")
	setString("work-dir", &options.WorkDir, "AUREN_MEDIA_HUB_WORK_DIR", "AUREN_AGENT_WORK_DIR")
	setString("log-dir", &options.LogDir, "AUREN_AGENT_LOG_DIR", "AUREN_LOG_DIR")
	setString("server-host", &options.ServerHost, "AUREN_SERVER_HOST", "AUREN_AGENT_SERVER_HOST")
	setInt("server-port", &options.ServerPort, "AUREN_SERVER_PORT", "AUREN_AGENT_SERVER_PORT")
	setInt("max-concurrent-jobs", &options.MaxConcurrentJobs, "AUREN_MEDIA_HUB_MAX_CONCURRENT_JOBS", "AUREN_AGENT_MAX_CONCURRENT_JOBS")
	setInt("max-sessions", &options.MaxSessions, "AUREN_MEDIA_HUB_MAX_SESSIONS", "AUREN_AGENT_MAX_SESSIONS")
	setInt("max-egress-mbps", &options.MaxEgressMbps, "AUREN_MEDIA_HUB_MAX_EGRESS_MBPS", "AUREN_AGENT_MAX_EGRESS_MBPS")
	setBool("enable-gateway", &options.EnableGateway, "AUREN_MEDIA_HUB_GATEWAY_ENABLED", "AUREN_AGENT_ENABLE_GATEWAY")
	setBool("disable-transfer", &options.DisableTransfer, "AUREN_AGENT_DISABLE_TRANSFER")
	setBool("start-service", &options.StartService, "AUREN_AGENT_START_SERVICE")
}

func requestBootstrapRegistrationToken(options bootstrapOptions) (string, error) {
	endpoint, err := resolveTokenEndpoint(options.MediaHubURL, options.BootstrapTokenEndpoint)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"role":              normalizeRole(options.Role, options.EnableGateway, !options.DisableTransfer),
		"region":            strings.TrimSpace(options.Region),
		"availability_zone": strings.TrimSpace(options.AvailabilityZone),
		"base_url":          strings.TrimRight(strings.TrimSpace(options.PublicBaseURL), "/"),
		"health_url":        strings.TrimSpace(options.HealthURL),
		"max_sessions":      options.MaxSessions,
		"max_egress_mbps":   options.MaxEgressMbps,
		"capabilities":      strings.Split(bootstrapCapabilities(options.EnableGateway, !options.DisableTransfer), ","),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", runtime.AppName+"/"+runtime.Version)
	if strings.TrimSpace(options.BootstrapTokenSecret) != "" {
		req.Header.Set("X-Auren-Bootstrap-Secret", strings.TrimSpace(options.BootstrapTokenSecret))
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("request bootstrap registration token: %w", err)
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read bootstrap token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("bootstrap token endpoint returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(response)))
	}
	var decoded map[string]any
	if err := json.Unmarshal(response, &decoded); err != nil {
		return "", fmt.Errorf("decode bootstrap token response: %w", err)
	}
	token := firstTokenString(decoded, "registration_token", "token", "node_registration_token")
	for _, container := range []string{"data", "node", "registration"} {
		if token != "" {
			break
		}
		if nested, ok := decoded[container].(map[string]any); ok {
			token = firstTokenString(nested, "registration_token", "token", "node_registration_token")
		}
	}
	if strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("bootstrap token response did not include registration_token")
	}
	return strings.TrimSpace(token), nil
}

func resolveTokenEndpoint(baseURL string, endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("bootstrap token endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() {
		return parsed.String(), nil
	}
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return "", err
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("media hub base URL is required to resolve relative token endpoint")
	}
	path := "/" + strings.TrimLeft(endpoint, "/")
	base.Path = strings.TrimRight(base.Path, "/") + path
	base.RawQuery = ""
	return base.String(), nil
}

func firstTokenString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := values[key]; ok {
			if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func validateBootstrapOptions(options bootstrapOptions) error {
	if strings.TrimSpace(options.ConfigPath) == "" {
		return fmt.Errorf("--config is required")
	}
	if strings.TrimSpace(options.MediaHubURL) == "" {
		return fmt.Errorf("--media-hub is required")
	}
	if strings.TrimSpace(options.RegistrationToken) == "" && strings.TrimSpace(options.BootstrapTokenEndpoint) == "" && !options.SkipRegister {
		return fmt.Errorf("--token or --token-endpoint is required unless --skip-register is set")
	}
	if options.EnableGateway && strings.TrimSpace(options.PublicBaseURL) == "" {
		return fmt.Errorf("--public-base-url is required when --enable-gateway is set")
	}
	if options.ServerPort < 1 || options.ServerPort > 65535 {
		return fmt.Errorf("--server-port must be between 1 and 65535")
	}
	if options.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("--max-concurrent-jobs must be greater than zero")
	}
	return nil
}

func bootstrapConfig(options bootstrapOptions) (config.Config, error) {
	cfg := config.DefaultConfig()
	if _, err := os.Stat(options.ConfigPath); err == nil {
		loaded, err := config.Load(config.LoadOptions{Path: options.ConfigPath})
		if err != nil {
			return config.Config{}, err
		}
		cfg = loaded
	} else if !errors.Is(err, os.ErrNotExist) {
		return config.Config{}, err
	}
	cfg.Runtime.Environment = "production"
	cfg.Runtime.DataDir = options.DataDir
	cfg.Runtime.TempDir = linuxDefaultTempDir
	cfg.Logger.Format = "json"
	cfg.Server.Enabled = true
	cfg.Server.Host = options.ServerHost
	cfg.Server.Port = options.ServerPort
	cfg.DevUI.Enabled = true
	cfg.DevUI.Path = "/_auren/dev"
	cfg.DevUI.Retention = 1000
	cfg.DevUI.RefreshInterval = "2s"
	cfg.DevUI.CaptureBodies = true
	cfg.DevUI.BodyLimitBytes = 8192
	cfg.Worker.Enabled = true
	cfg.Worker.Concurrency = maxInt(1, options.MaxConcurrentJobs)
	cfg.Download.TempDir = filepath.Join(linuxDefaultTempDir, "downloads")
	cfg.Storage.LocalPath = filepath.Join(options.DataDir, "storage")
	cfg.MediaHub.Enabled = true
	cfg.MediaHub.BaseURL = strings.TrimRight(strings.TrimSpace(options.MediaHubURL), "/")
	cfg.MediaHub.RegistrationToken = strings.TrimSpace(options.RegistrationToken)
	cfg.MediaHub.BootstrapTokenEndpoint = strings.TrimSpace(options.BootstrapTokenEndpoint)
	cfg.MediaHub.BootstrapTokenSecret = strings.TrimSpace(options.BootstrapTokenSecret)
	cfg.MediaHub.HMACEnabled = true
	cfg.MediaHub.PollEnabled = true
	cfg.MediaHub.TransferEnabled = !options.DisableTransfer
	cfg.MediaHub.ClaimEnabled = !options.DisableTransfer
	cfg.MediaHub.MaxConcurrentJobs = maxInt(1, options.MaxConcurrentJobs)
	cfg.MediaHub.WorkDir = options.WorkDir
	cfg.MediaHub.GatewayEnabled = options.EnableGateway
	cfg.MediaHub.GatewayProxyEnabled = true
	cfg.MediaHub.GatewayRedirectEnabled = true
	cfg.MediaHub.DrainEnabled = true
	cfg.MediaHub.DrainFile = filepath.Join(options.WorkDir, "drain")
	cfg.MediaHub.BackpressureEnabled = true
	cfg.MediaHub.DiskGuardEnabled = true
	cfg.MediaHub.DiskMinFreeBytes = 1073741824
	cfg.MediaHub.DeadLetterEnabled = true
	cfg.MediaHub.DeadLetterDir = filepath.Join(options.WorkDir, "dead-letter")
	cfg.MediaHub.LeaseRenewalEnabled = true
	cfg.MediaHub.Role = normalizeRole(options.Role, options.EnableGateway, !options.DisableTransfer)
	cfg.MediaHub.Region = strings.TrimSpace(options.Region)
	cfg.MediaHub.AvailabilityZone = strings.TrimSpace(options.AvailabilityZone)
	cfg.MediaHub.PublicBaseURL = strings.TrimRight(strings.TrimSpace(options.PublicBaseURL), "/")
	if strings.TrimSpace(options.HealthURL) != "" {
		cfg.MediaHub.HealthURL = strings.TrimSpace(options.HealthURL)
	} else if cfg.MediaHub.PublicBaseURL != "" {
		cfg.MediaHub.HealthURL = strings.TrimRight(cfg.MediaHub.PublicBaseURL, "/") + "/health"
	}
	cfg.MediaHub.MaxSessions = options.MaxSessions
	cfg.MediaHub.MaxEgressMbps = options.MaxEgressMbps
	cfg.MediaHub.Capabilities = bootstrapCapabilities(options.EnableGateway, !options.DisableTransfer)
	return cfg, nil
}

func ensureLinuxRuntimeDirs(cfg config.Config, options bootstrapOptions) error {
	for _, dir := range []string{filepath.Dir(options.ConfigPath), cfg.Runtime.DataDir, cfg.Runtime.TempDir, cfg.Download.TempDir, cfg.Storage.LocalPath, cfg.MediaHub.WorkDir, cfg.MediaHub.DeadLetterDir, options.LogDir} {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}

func registerBootstrapNode(cfg config.Config) (mediahub.NodeState, error) {
	store := identity.NewFileStore(identity.DefaultStorePath(cfg.Runtime.DataDir))
	identityResult, err := store.Ensure()
	if err != nil {
		return mediahub.NodeState{}, err
	}
	snapshot, err := identity.NewSnapshot(identityResult, identity.ResolveHostname())
	if err != nil {
		return mediahub.NodeState{}, err
	}
	client, err := mediahub.NewClient(mediahub.ClientOptions{BaseURL: cfg.MediaHub.BaseURL, HMACEnabled: cfg.MediaHub.HMACEnabled, UserAgent: runtime.AppName + "/" + runtime.Version, Paths: mediahub.EndpointPaths{Register: cfg.MediaHub.RegisterPath, Config: cfg.MediaHub.ConfigPath, Heartbeat: cfg.MediaHub.HeartbeatPath, Metrics: cfg.MediaHub.MetricsPath, Events: cfg.MediaHub.EventsPath}})
	if err != nil {
		return mediahub.NodeState{}, err
	}
	connector, err := mediahub.NewConnector(mediahub.ConnectorOptions{
		Config:            cfg.MediaHub,
		Identity:          snapshot,
		Runtime:           runtime.Info(),
		Store:             mediahub.NewStateStore(mediahub.DefaultStatePath(cfg.Runtime.DataDir)),
		Client:            client,
		HeartbeatSnapshot: func() any { return map[string]any{"status": "bootstrap"} },
		QueueSnapshot:     func() any { return map[string]any{"driver": cfg.Queue.Driver} },
		DownloadSnapshot:  func() any { return map[string]any{} },
		EventsSnapshot: func() []mediahub.EventPayload {
			return []mediahub.EventPayload{{Level: "info", Type: "agent.bootstrap", Message: "Linux package bootstrap completed", CreatedAt: time.Now().UTC()}}
		},
		CapacitySnapshot: func() mediahub.Capacity {
			return mediahub.Capacity{MaxSessions: cfg.MediaHub.MaxSessions, MaxConcurrentJobs: cfg.MediaHub.MaxConcurrentJobs, MaxEgressMbps: cfg.MediaHub.MaxEgressMbps}
		},
	})
	if err != nil {
		return mediahub.NodeState{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return connector.Bootstrap(ctx)
}

func writeAgentConfig(path string, cfg config.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	payload := renderAgentYAML(cfg)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agent-*.yaml.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(payload); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o640); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func renderAgentYAML(cfg config.Config) string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("app:")
	line("  name: %q", cfg.App.Name)
	line("  description: %q", cfg.App.Description)
	line("runtime:")
	line("  environment: %q", cfg.Runtime.Environment)
	line("  data_dir: %q", cfg.Runtime.DataDir)
	line("  temp_dir: %q", cfg.Runtime.TempDir)
	line("logger:")
	line("  level: %q", cfg.Logger.Level)
	line("  format: %q", cfg.Logger.Format)
	line("  timestamp: %t", cfg.Logger.Timestamp)
	line("  service: %q", cfg.Logger.Service)
	line("server:")
	line("  enabled: %t", cfg.Server.Enabled)
	line("  host: %q", cfg.Server.Host)
	line("  port: %d", cfg.Server.Port)
	line("  read_timeout: %q", cfg.Server.ReadTimeout)
	line("  write_timeout: %q", cfg.Server.WriteTimeout)
	line("  idle_timeout: %q", cfg.Server.IdleTimeout)
	line("worker:")
	line("  enabled: %t", cfg.Worker.Enabled)
	line("  concurrency: %d", cfg.Worker.Concurrency)
	line("  shutdown_timeout: %q", cfg.Worker.ShutdownTimeout)
	line("queue:")
	line("  driver: %q", cfg.Queue.Driver)
	line("  memory_capacity: %d", cfg.Queue.MemoryCapacity)
	line("  poll_interval: %q", cfg.Queue.PollInterval)
	line("  redis_address: %q", cfg.Queue.RedisAddress)
	line("  redis_stream: %q", cfg.Queue.RedisStream)
	line("  redis_consumer_group: %q", cfg.Queue.RedisConsumerGroup)
	line("  rabbitmq_url: %q", cfg.Queue.RabbitMQURL)
	line("  rabbitmq_queue: %q", cfg.Queue.RabbitMQQueue)
	line("  nats_url: %q", cfg.Queue.NATSURL)
	line("  nats_subject: %q", cfg.Queue.NATSSubject)
	line("  nats_queue_group: %q", cfg.Queue.NATSQueueGroup)
	line("resolver:")
	line("  default_user_agent: %q", cfg.Resolver.DefaultUserAgent)
	line("  follow_redirects: %t", cfg.Resolver.FollowRedirects)
	line("  max_redirects: %d", cfg.Resolver.MaxRedirects)
	line("download:")
	line("  temp_dir: %q", cfg.Download.TempDir)
	line("  connect_timeout: %q", cfg.Download.ConnectTimeout)
	line("  response_header_timeout: %q", cfg.Download.ResponseHeaderTimeout)
	line("  idle_timeout: %q", cfg.Download.IdleTimeout)
	line("  max_retries: %d", cfg.Download.MaxRetries)
	line("  retry_backoff: %q", cfg.Download.RetryBackoff)
	line("  chunk_size: %q", cfg.Download.ChunkSize)
	line("  resume_enabled: %t", cfg.Download.ResumeEnabled)
	line("  checksum: %q", cfg.Download.Checksum)
	line("upload:")
	line("  driver: %q", cfg.Upload.Driver)
	line("  max_retries: %d", cfg.Upload.MaxRetries)
	line("  retry_backoff: %q", cfg.Upload.RetryBackoff)
	line("  multipart_enabled: %t", cfg.Upload.MultipartEnabled)
	line("  part_size: %q", cfg.Upload.PartSize)
	line("storage:")
	line("  driver: %q", cfg.Storage.Driver)
	line("  endpoint: %q", cfg.Storage.Endpoint)
	line("  bucket: %q", cfg.Storage.Bucket)
	line("  api_key: %q", cfg.Storage.APIKey)
	line("  region: %q", cfg.Storage.Region)
	line("  local_path: %q", cfg.Storage.LocalPath)
	line("  use_path_style: %t", cfg.Storage.UsePathStyle)
	line("  access_key_id: %q", cfg.Storage.AccessKeyID)
	line("  secret_access_key: %q", cfg.Storage.SecretAccessKey)
	line("  session_token: %q", cfg.Storage.SessionToken)
	line("  s3_force_path_style: %t", cfg.Storage.S3ForcePathStyle)
	line("metrics:")
	line("  enabled: %t", cfg.Metrics.Enabled)
	line("  host: %q", cfg.Metrics.Host)
	line("  port: %d", cfg.Metrics.Port)
	line("  path: %q", cfg.Metrics.Path)
	line("heartbeat:")
	line("  enabled: %t", cfg.Heartbeat.Enabled)
	line("  interval: %q", cfg.Heartbeat.Interval)
	line("  timeout: %q", cfg.Heartbeat.Timeout)
	line("security:")
	line("  api_key_required: %t", cfg.Security.APIKeyRequired)
	line("  api_key: %q", cfg.Security.APIKey)
	line("  api_key_hash: %q", cfg.Security.APIKeyHash)
	line("  token_header: %q", cfg.Security.TokenHeader)
	line("  allow_insecure_http: %t", cfg.Security.AllowInsecureHTTP)
	line("  jwt_enabled: %t", cfg.Security.JWTEnabled)
	line("  jwt_secret: %q", cfg.Security.JWTSecret)
	line("  jwt_ttl: %q", cfg.Security.JWTTTL)
	line("  mtls_enabled: %t", cfg.Security.MTLSEnabled)
	line("  mtls_required_cn: %q", cfg.Security.MTLSRequiredCN)
	line("  rbac_enabled: %t", cfg.Security.RBACEnabled)
	line("  rate_limit_enabled: %t", cfg.Security.RateLimitEnabled)
	line("  rate_limit_per_minute: %d", cfg.Security.RateLimitPerMin)
	line("  secrets_provider: %q", cfg.Security.SecretsProvider)
	line("  secrets_file: %q", cfg.Security.SecretsFile)
	line("dev_ui:")
	line("  enabled: %t", cfg.DevUI.Enabled)
	line("  path: %q", cfg.DevUI.Path)
	line("  retention: %d", cfg.DevUI.Retention)
	line("  refresh_interval: %q", cfg.DevUI.RefreshInterval)
	line("  capture_bodies: %t", cfg.DevUI.CaptureBodies)
	line("  body_limit_bytes: %d", cfg.DevUI.BodyLimitBytes)
	line("media_hub:")
	line("  enabled: %t", cfg.MediaHub.Enabled)
	line("  base_url: %q", cfg.MediaHub.BaseURL)
	line("  registration_token: %q", cfg.MediaHub.RegistrationToken)
	line("  bootstrap_token_endpoint: %q", cfg.MediaHub.BootstrapTokenEndpoint)
	line("  bootstrap_token_secret: %q", cfg.MediaHub.BootstrapTokenSecret)
	line("  register_path: %q", cfg.MediaHub.RegisterPath)
	line("  config_path: %q", cfg.MediaHub.ConfigPath)
	line("  heartbeat_path: %q", cfg.MediaHub.HeartbeatPath)
	line("  metrics_path: %q", cfg.MediaHub.MetricsPath)
	line("  events_path: %q", cfg.MediaHub.EventsPath)
	line("  node_uuid: %q", cfg.MediaHub.NodeUUID)
	line("  node_secret: %q", cfg.MediaHub.NodeSecret)
	line("  hmac_enabled: %t", cfg.MediaHub.HMACEnabled)
	line("  poll_enabled: %t", cfg.MediaHub.PollEnabled)
	line("  poll_interval: %q", cfg.MediaHub.PollInterval)
	line("  transfer_enabled: %t", cfg.MediaHub.TransferEnabled)
	line("  claim_enabled: %t", cfg.MediaHub.ClaimEnabled)
	line("  claim_interval: %q", cfg.MediaHub.ClaimInterval)
	line("  progress_interval: %q", cfg.MediaHub.ProgressInterval)
	line("  control_interval: %q", cfg.MediaHub.ControlInterval)
	line("  max_concurrent_jobs: %d", cfg.MediaHub.MaxConcurrentJobs)
	line("  accepted_operations: %q", cfg.MediaHub.AcceptedOperations)
	line("  work_dir: %q", cfg.MediaHub.WorkDir)
	line("  min_bytes: %d", cfg.MediaHub.MinBytes)
	line("  block_html: %t", cfg.MediaHub.BlockHTML)
	line("  gateway_enabled: %t", cfg.MediaHub.GatewayEnabled)
	line("  gateway_proxy_enabled: %t", cfg.MediaHub.GatewayProxyEnabled)
	line("  gateway_redirect_enabled: %t", cfg.MediaHub.GatewayRedirectEnabled)
	line("  gateway_heartbeat_interval: %q", cfg.MediaHub.GatewayHeartbeatInterval)
	line("  gateway_token_ttl: %q", cfg.MediaHub.GatewayTokenTTL)
	line("  drain_enabled: %t", cfg.MediaHub.DrainEnabled)
	line("  drain_file: %q", cfg.MediaHub.DrainFile)
	line("  backpressure_enabled: %t", cfg.MediaHub.BackpressureEnabled)
	line("  disk_guard_enabled: %t", cfg.MediaHub.DiskGuardEnabled)
	line("  disk_min_free_bytes: %d", cfg.MediaHub.DiskMinFreeBytes)
	line("  dead_letter_enabled: %t", cfg.MediaHub.DeadLetterEnabled)
	line("  dead_letter_dir: %q", cfg.MediaHub.DeadLetterDir)
	line("  lease_renewal_enabled: %t", cfg.MediaHub.LeaseRenewalEnabled)
	line("  lease_renewal_interval: %q", cfg.MediaHub.LeaseRenewalInterval)
	line("  secret_rotation_enabled: %t", cfg.MediaHub.SecretRotationEnabled)
	line("  secret_rotation_interval: %q", cfg.MediaHub.SecretRotationInterval)
	line("  heartbeat_interval: %q", cfg.MediaHub.HeartbeatInterval)
	line("  metrics_interval: %q", cfg.MediaHub.MetricsInterval)
	line("  events_flush_interval: %q", cfg.MediaHub.EventsFlushInterval)
	line("  role: %q", cfg.MediaHub.Role)
	line("  provider: %q", cfg.MediaHub.Provider)
	line("  region: %q", cfg.MediaHub.Region)
	line("  availability_zone: %q", cfg.MediaHub.AvailabilityZone)
	line("  public_base_url: %q", cfg.MediaHub.PublicBaseURL)
	line("  health_url: %q", cfg.MediaHub.HealthURL)
	line("  max_sessions: %d", cfg.MediaHub.MaxSessions)
	line("  max_egress_mbps: %d", cfg.MediaHub.MaxEgressMbps)
	line("  capabilities: %q", cfg.MediaHub.Capabilities)
	return b.String()
}

func normalizeRole(role string, gatewayEnabled bool, transferEnabled bool) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" || role == "worker,gateway" || role == "gateway,worker" {
		role = "hybrid"
	}
	switch role {
	case "worker", "gateway", "edge", "hybrid":
		return role
	}
	if gatewayEnabled && transferEnabled {
		return "hybrid"
	}
	if gatewayEnabled {
		return "gateway"
	}
	return "worker"
}

func bootstrapCapabilities(gatewayEnabled bool, transferEnabled bool) string {
	caps := []string{"download", "upload", "auren_storage", "xtream", "shui", "m3u8", "hls"}
	if transferEnabled {
		caps = append([]string{"transfer"}, caps...)
	}
	if gatewayEnabled {
		caps = append(caps, "gateway", "live", "movie", "series")
	}
	return strings.Join(caps, ",")
}

func systemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func systemctlAvailable() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func systemdManageable() bool {
	if !systemctlAvailable() {
		return false
	}
	data, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "systemd"
}

func runSystemctlStatus(unit string) error {
	cmd := exec.Command("systemctl", "is-active", unit)
	output, err := cmd.Output()
	status := strings.TrimSpace(string(output))
	if status == "" {
		status = "unknown"
	}
	fmt.Fprintf(os.Stdout, "systemd: unit=%s active=%s\n", unit, status)
	return err
}

func httpCheck(name string, target string) string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(target)
	if err != nil {
		return warnLine(name, err.Error())
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return okLine(name, target+" status="+strconv.Itoa(resp.StatusCode))
	}
	return warnLine(name, target+" status="+strconv.Itoa(resp.StatusCode))
}

func okLine(name string, value string) string   { return "OK   " + name + ": " + value }
func warnLine(name string, value string) string { return "WARN " + name + ": " + value }
func failLine(name string, value string) string { return "FAIL " + name + ": " + value }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func printBootstrapHelp(out io.Writer) {
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  auren-transfer-agent bootstrap --env-file .env [--media-hub https://media.example.com --token TOKEN] [options]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Common options:")
	fmt.Fprintln(out, "  --config /etc/auren-transfer-agent/agent.yaml")
	fmt.Fprintln(out, "  --role worker|gateway|hybrid")
	fmt.Fprintln(out, "  --region sa-east-1")
	fmt.Fprintln(out, "  --enable-gateway --public-base-url https://node.example.com")
	fmt.Fprintln(out, "  --max-concurrent-jobs 2")
	fmt.Fprintln(out, "  --start-service")
}
