package bootstrap

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
	"github.com/cesarsousa94/auren-transfer-agent/internal/identity"
	"github.com/cesarsousa94/auren-transfer-agent/internal/mediahub"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
)

const (
	linuxDefaultConfigPath = "/etc/auren-transfer-agent/agent.yaml"
	linuxDefaultDataDir    = "/var/lib/auren-transfer-agent"
	linuxDefaultLogDir     = "/var/log/auren-transfer-agent"
	linuxDefaultTempDir    = "/var/tmp/auren-transfer-agent"
	linuxDefaultUnit       = "auren-transfer-agent.service"
)

type bootstrapOptions struct {
	ConfigPath        string
	MediaHubURL       string
	RegistrationToken string
	Role              string
	Region            string
	AvailabilityZone  string
	PublicBaseURL     string
	HealthURL         string
	DataDir           string
	WorkDir           string
	LogDir            string
	ServerHost        string
	ServerPort        int
	MaxConcurrentJobs int
	MaxSessions       int
	MaxEgressMbps     int
	EnableGateway     bool
	DisableTransfer   bool
	SkipRegister      bool
	StartService      bool
	SystemdUnit       string
	DryRun            bool
}

func runBootstrap(args []string) error {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	options := bootstrapOptions{}
	flags.StringVar(&options.ConfigPath, "config", linuxDefaultConfigPath, "config file to write")
	flags.StringVar(&options.MediaHubURL, "media-hub", "", "Auren Media Hub base URL")
	flags.StringVar(&options.RegistrationToken, "token", "", "one-time Media Hub registration token")
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
	if *showHelp {
		printBootstrapHelp(os.Stdout)
		return nil
	}
	if err := validateBootstrapOptions(options); err != nil {
		return err
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
	if err := writeAgentConfig(options.ConfigPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "config: written %s\n", options.ConfigPath)
	if !options.SkipRegister {
		state, err := registerBootstrapNode(cfg)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "media-hub: registered node_uuid=%s state=%s\n", state.NodeUUID, mediahub.DefaultStatePath(cfg.Runtime.DataDir))
	} else {
		fmt.Fprintln(os.Stdout, "media-hub: registration skipped")
	}
	if options.StartService {
		if err := systemctl("daemon-reload"); err != nil {
			return err
		}
		if err := systemctl("enable", "--now", options.SystemdUnit); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "systemd: enabled and started %s\n", options.SystemdUnit)
	}
	fmt.Fprintln(os.Stdout, "bootstrap: complete")
	return nil
}

func runDoctor(args []string) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", linuxDefaultConfigPath, "config file to validate")
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
	cfg, err := config.Load(config.LoadOptions{Path: *configPath})
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
		}
	} else {
		checks = append(checks, warnLine("media_hub", "disabled"))
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
	showHelp := flags.Bool("help", false, "print help")
	flags.BoolVar(showHelp, "h", false, "print help")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showHelp {
		fmt.Fprintln(os.Stdout, "Usage: auren-transfer-agent status [--config /etc/auren-transfer-agent/agent.yaml]")
		return nil
	}
	cfg, err := config.Load(config.LoadOptions{Path: *configPath})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s %s\n", runtime.AppName, runtime.Version)
	fmt.Fprintf(os.Stdout, "config: %s\n", *configPath)
	fmt.Fprintf(os.Stdout, "server: enabled=%t address=%s\n", cfg.Server.Enabled, cfg.ServerAddress())
	fmt.Fprintf(os.Stdout, "media-hub: enabled=%t base_url=%s role=%s transfer=%t gateway=%t\n", cfg.MediaHub.Enabled, cfg.MediaHub.BaseURL, cfg.MediaHub.Role, cfg.MediaHub.TransferEnabled, cfg.MediaHub.GatewayEnabled)
	statePath := mediahub.DefaultStatePath(cfg.Runtime.DataDir)
	if state, err := mediahub.NewStateStore(statePath).Load(); err == nil {
		fmt.Fprintf(os.Stdout, "node: uuid=%s config_version=%s registered_at=%s state=%s\n", state.NodeUUID, state.ConfigVersion, state.RegisteredAt.Format(time.RFC3339), statePath)
	} else {
		fmt.Fprintf(os.Stdout, "node: not registered state=%s\n", statePath)
	}
	if systemctlAvailable() {
		_ = runSystemctlStatus(linuxDefaultUnit)
	}
	return nil
}

func validateBootstrapOptions(options bootstrapOptions) error {
	if strings.TrimSpace(options.ConfigPath) == "" {
		return fmt.Errorf("--config is required")
	}
	if strings.TrimSpace(options.MediaHubURL) == "" {
		return fmt.Errorf("--media-hub is required")
	}
	if strings.TrimSpace(options.RegistrationToken) == "" && !options.SkipRegister {
		return fmt.Errorf("--token is required unless --skip-register is set")
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
	cfg.Worker.Enabled = true
	cfg.Worker.Concurrency = maxInt(1, options.MaxConcurrentJobs)
	cfg.Download.TempDir = filepath.Join(linuxDefaultTempDir, "downloads")
	cfg.Storage.LocalPath = filepath.Join(options.DataDir, "storage")
	cfg.MediaHub.Enabled = true
	cfg.MediaHub.BaseURL = strings.TrimRight(strings.TrimSpace(options.MediaHubURL), "/")
	cfg.MediaHub.RegistrationToken = strings.TrimSpace(options.RegistrationToken)
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
	client, err := mediahub.NewClient(mediahub.ClientOptions{BaseURL: cfg.MediaHub.BaseURL, HMACEnabled: cfg.MediaHub.HMACEnabled, UserAgent: runtime.AppName + "/" + runtime.Version})
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
	line("media_hub:")
	line("  enabled: %t", cfg.MediaHub.Enabled)
	line("  base_url: %q", cfg.MediaHub.BaseURL)
	line("  registration_token: %q", cfg.MediaHub.RegistrationToken)
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
	fmt.Fprintln(out, "  auren-transfer-agent bootstrap --media-hub https://media.example.com --token TOKEN [options]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Common options:")
	fmt.Fprintln(out, "  --config /etc/auren-transfer-agent/agent.yaml")
	fmt.Fprintln(out, "  --role worker|gateway|hybrid")
	fmt.Fprintln(out, "  --region sa-east-1")
	fmt.Fprintln(out, "  --enable-gateway --public-base-url https://node.example.com")
	fmt.Fprintln(out, "  --max-concurrent-jobs 2")
	fmt.Fprintln(out, "  --start-service")
}
