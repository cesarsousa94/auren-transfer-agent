package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func clearAurenEnvironment(t *testing.T) {
	t.Helper()

	previous := make(map[string]string)
	for _, pair := range os.Environ() {
		key, value, found := strings.Cut(pair, "=")
		if !found || !strings.HasPrefix(key, "AUREN_") {
			continue
		}
		previous[key] = value
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}

	t.Cleanup(func() {
		for key := range previous {
			_ = os.Unsetenv(key)
		}
		for key, value := range previous {
			_ = os.Setenv(key, value)
		}
	})
}

func TestDefaultConfigMatchesLoadWithoutFiles(t *testing.T) {
	clearAurenEnvironment(t)

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("expected defaults without config file, got error: %v", err)
	}

	if !reflect.DeepEqual(cfg, DefaultConfig()) {
		t.Fatalf("loaded defaults do not match DefaultConfig(): loaded=%#v defaults=%#v", cfg, DefaultConfig())
	}
}

func TestDefaultContractsAreDefensiveCopies(t *testing.T) {
	paths := DefaultSearchPaths()
	if len(paths) != 3 {
		t.Fatalf("unexpected default search path count: %d", len(paths))
	}
	paths[0] = "/tmp/mutated"
	if DefaultSearchPaths()[0] != "." {
		t.Fatalf("default search paths must be returned as a defensive copy")
	}

	values := DefaultValues()
	if values["server.port"] != 8080 {
		t.Fatalf("unexpected default server port: %#v", values["server.port"])
	}
	values["server.port"] = 9999
	if DefaultValues()["server.port"] != 8080 {
		t.Fatalf("default values must be returned as a defensive copy")
	}
}

func TestLoadUsesDefaultsWhenNoConfigFileExists(t *testing.T) {
	clearAurenEnvironment(t)

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("expected defaults without config file, got error: %v", err)
	}

	if cfg.App.Name != "auren-transfer-agent" {
		t.Fatalf("unexpected app name: %q", cfg.App.Name)
	}
	if cfg.Runtime.Environment != "local" {
		t.Fatalf("unexpected runtime environment: %q", cfg.Runtime.Environment)
	}
	if cfg.Logger.Level != "info" || cfg.Logger.Format != "json" {
		t.Fatalf("unexpected logger config: %#v", cfg.Logger)
	}
	if cfg.ServerAddress() != "0.0.0.0:8080" {
		t.Fatalf("unexpected server address: %q", cfg.ServerAddress())
	}
	if cfg.Worker.Concurrency != 1 {
		t.Fatalf("unexpected worker concurrency: %d", cfg.Worker.Concurrency)
	}
	if cfg.Queue.Driver != "memory" {
		t.Fatalf("unexpected queue driver: %q", cfg.Queue.Driver)
	}
	if !cfg.Resolver.FollowRedirects {
		t.Fatalf("expected resolver redirects to be enabled by default")
	}
	if cfg.Download.Checksum != "sha256" {
		t.Fatalf("unexpected checksum: %q", cfg.Download.Checksum)
	}
	if cfg.Upload.PartSize != "16MiB" {
		t.Fatalf("unexpected upload part size: %q", cfg.Upload.PartSize)
	}
	if cfg.Storage.LocalPath != "./data/storage" {
		t.Fatalf("unexpected storage local path: %q", cfg.Storage.LocalPath)
	}
	if cfg.MetricsAddress() != "0.0.0.0:9090" {
		t.Fatalf("unexpected metrics address: %q", cfg.MetricsAddress())
	}
	if cfg.Security.TokenHeader != "Authorization" {
		t.Fatalf("unexpected token header: %q", cfg.Security.TokenHeader)
	}
}

func TestLoadReadsExplicitYAMLConfig(t *testing.T) {
	clearAurenEnvironment(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")

	content := []byte(`app:
  name: custom-agent
  description: Custom transfer node
runtime:
  environment: test
  data_dir: /var/lib/auren-transfer-agent
  temp_dir: /tmp/auren-transfer-agent
logger:
  level: debug
  format: json
  timestamp: false
  service: custom-agent
server:
  enabled: true
  host: 127.0.0.1
  port: 9090
  read_timeout: 10s
  write_timeout: 11s
  idle_timeout: 12s
worker:
  enabled: true
  concurrency: 4
  shutdown_timeout: 45s
queue:
  driver: memory
  memory_capacity: 250
  poll_interval: 500ms
resolver:
  default_user_agent: CustomAgent/1.0
  follow_redirects: false
  max_redirects: 3
download:
  temp_dir: /tmp/downloads
  connect_timeout: 5s
  response_header_timeout: 6s
  idle_timeout: 7s
  max_retries: 5
  retry_backoff: 3s
  chunk_size: 4MiB
  resume_enabled: false
  checksum: sha256
upload:
  driver: local
  max_retries: 6
  retry_backoff: 4s
  multipart_enabled: false
  part_size: 8MiB
storage:
  driver: local
  endpoint: http://storage.test
  bucket: media
  region: sa-east-1
  local_path: /var/lib/storage
  use_path_style: false
metrics:
  enabled: true
  host: 127.0.0.1
  port: 9191
  path: /internal/metrics
heartbeat:
  enabled: true
  interval: 15s
  timeout: 5s
security:
  api_key_required: true
  token_header: X-Auren-Agent-Token
  allow_insecure_http: false
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := Load(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("expected explicit config to load, got error: %v", err)
	}

	if cfg.App.Name != "custom-agent" {
		t.Fatalf("unexpected app name: %q", cfg.App.Name)
	}
	if cfg.Runtime.DataDir != "/var/lib/auren-transfer-agent" {
		t.Fatalf("unexpected data dir: %q", cfg.Runtime.DataDir)
	}
	if cfg.Logger.Level != "debug" || cfg.Logger.Timestamp {
		t.Fatalf("unexpected logger config: %#v", cfg.Logger)
	}
	if !cfg.Server.Enabled {
		t.Fatalf("expected server enabled")
	}
	if cfg.ServerAddress() != "127.0.0.1:9090" {
		t.Fatalf("unexpected server address: %q", cfg.ServerAddress())
	}
	if cfg.Worker.Concurrency != 4 {
		t.Fatalf("unexpected worker concurrency: %d", cfg.Worker.Concurrency)
	}
	if cfg.Queue.MemoryCapacity != 250 {
		t.Fatalf("unexpected queue memory capacity: %d", cfg.Queue.MemoryCapacity)
	}
	if cfg.Resolver.FollowRedirects {
		t.Fatalf("expected redirects disabled")
	}
	if cfg.Download.ResumeEnabled {
		t.Fatalf("expected resume disabled")
	}
	if cfg.Upload.MultipartEnabled {
		t.Fatalf("expected multipart upload disabled")
	}
	if cfg.Storage.Region != "sa-east-1" {
		t.Fatalf("unexpected storage region: %q", cfg.Storage.Region)
	}
	if cfg.MetricsAddress() != "127.0.0.1:9191" {
		t.Fatalf("unexpected metrics address: %q", cfg.MetricsAddress())
	}
	if !cfg.Heartbeat.Enabled {
		t.Fatalf("expected heartbeat enabled")
	}
	if cfg.Security.AllowInsecureHTTP {
		t.Fatalf("expected insecure HTTP disabled")
	}
}

func TestLoadMergesPartialYAMLWithDefaults(t *testing.T) {
	clearAurenEnvironment(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")

	content := []byte(`server:
  port: 8181
download:
  max_retries: 9
security:
  api_key_required: true
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := Load(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("expected partial config to load, got error: %v", err)
	}

	if cfg.ServerAddress() != "0.0.0.0:8181" {
		t.Fatalf("expected merged server address, got %q", cfg.ServerAddress())
	}
	if cfg.Download.MaxRetries != 9 {
		t.Fatalf("expected overridden download retries, got %d", cfg.Download.MaxRetries)
	}
	if !cfg.Security.APIKeyRequired {
		t.Fatalf("expected security api key requirement to be true")
	}
	if cfg.Upload.PartSize != "16MiB" {
		t.Fatalf("expected default upload part size to remain, got %q", cfg.Upload.PartSize)
	}
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	clearAurenEnvironment(t)

	t.Setenv("AUREN_RUNTIME_ENVIRONMENT", "production")
	t.Setenv("AUREN_LOGGER_LEVEL", "debug")
	t.Setenv("AUREN_LOGGER_FORMAT", "console")
	t.Setenv("AUREN_SERVER_HOST", "127.0.0.1")
	t.Setenv("AUREN_SERVER_PORT", "8188")
	t.Setenv("AUREN_WORKER_ENABLED", "true")
	t.Setenv("AUREN_WORKER_CONCURRENCY", "6")
	t.Setenv("AUREN_STORAGE_ENDPOINT", "https://storage.example.test")
	t.Setenv("AUREN_SECURITY_ALLOW_INSECURE_HTTP", "false")

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("expected config with env overrides to load, got error: %v", err)
	}

	if cfg.Runtime.Environment != "production" {
		t.Fatalf("unexpected environment: %q", cfg.Runtime.Environment)
	}
	if cfg.Logger.Level != "debug" {
		t.Fatalf("unexpected logger level: %q", cfg.Logger.Level)
	}
	if cfg.Logger.Format != "console" {
		t.Fatalf("unexpected logger format: %q", cfg.Logger.Format)
	}
	if cfg.ServerAddress() != "127.0.0.1:8188" {
		t.Fatalf("unexpected server address: %q", cfg.ServerAddress())
	}
	if !cfg.Worker.Enabled {
		t.Fatalf("expected worker enabled by env override")
	}
	if cfg.Worker.Concurrency != 6 {
		t.Fatalf("unexpected worker concurrency: %d", cfg.Worker.Concurrency)
	}
	if cfg.Storage.Endpoint != "https://storage.example.test" {
		t.Fatalf("unexpected storage endpoint: %q", cfg.Storage.Endpoint)
	}
	if cfg.Security.AllowInsecureHTTP {
		t.Fatalf("expected insecure HTTP disabled by env override")
	}
}

func TestEnvironmentOverridesYAMLValues(t *testing.T) {
	clearAurenEnvironment(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")

	content := []byte(`runtime:
  environment: staging
server:
  host: 0.0.0.0
  port: 8081
worker:
  enabled: false
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	t.Setenv("AUREN_RUNTIME_ENVIRONMENT", "production")
	t.Setenv("AUREN_SERVER_PORT", "9199")
	t.Setenv("AUREN_WORKER_ENABLED", "true")

	cfg, err := Load(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("expected config with yaml and env overrides to load, got error: %v", err)
	}

	if cfg.Runtime.Environment != "production" {
		t.Fatalf("expected env to override yaml environment, got %q", cfg.Runtime.Environment)
	}
	if cfg.ServerAddress() != "0.0.0.0:9199" {
		t.Fatalf("expected env to override yaml port, got %q", cfg.ServerAddress())
	}
	if !cfg.Worker.Enabled {
		t.Fatalf("expected env to override yaml worker flag")
	}
}

func TestValidateAcceptsDefaultConfig(t *testing.T) {
	if err := Validate(DefaultConfig()); err != nil {
		t.Fatalf("expected default config to validate, got error: %v", err)
	}
}

func TestLoadRejectsInvalidYAMLConfig(t *testing.T) {
	clearAurenEnvironment(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")

	content := []byte(`app:
  name: ""
runtime:
  environment: invalid
logger:
  level: verbose
server:
  port: 70000
worker:
  concurrency: 0
queue:
  driver: redis
resolver:
  max_redirects: -1
download:
  chunk_size: tiny
upload:
  part_size: 0MiB
metrics:
  path: metrics
security:
  token_header: ""
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	_, err := Load(LoadOptions{Path: path})
	if err == nil {
		t.Fatalf("expected invalid config to fail validation")
	}

	message := err.Error()
	for _, expected := range []string{
		"app.name",
		"runtime.environment",
		"logger.level",
		"server.port",
		"worker.concurrency",
		"queue.driver",
		"resolver.max_redirects",
		"download.chunk_size",
		"upload.part_size",
		"metrics.path",
		"security.token_header",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("expected validation message to contain %q, got: %s", expected, message)
		}
	}
}

func TestEnvironmentOverridesAreValidated(t *testing.T) {
	clearAurenEnvironment(t)

	t.Setenv("AUREN_SERVER_PORT", "0")

	_, err := Load(LoadOptions{})
	if err == nil {
		t.Fatalf("expected invalid environment override to fail validation")
	}
	if !strings.Contains(err.Error(), "server.port") {
		t.Fatalf("expected server.port validation error, got: %v", err)
	}
}

func TestParseSizeBytesSupportsOfficialUnits(t *testing.T) {
	cases := map[string]int64{
		"1B":   1,
		"2KB":  2_000,
		"3MB":  3_000_000,
		"4GB":  4_000_000_000,
		"5KiB": 5 * 1024,
		"6MiB": 6 * 1024 * 1024,
		"7GiB": 7 * 1024 * 1024 * 1024,
		"8192": 8192,
	}

	for input, expected := range cases {
		actual, err := parseSizeBytes(input)
		if err != nil {
			t.Fatalf("expected %q to parse, got error: %v", input, err)
		}
		if actual != expected {
			t.Fatalf("expected %q to parse as %d bytes, got %d", input, expected, actual)
		}
	}
}
