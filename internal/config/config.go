package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const (
	// DefaultConfigName is the canonical base name for the agent config file.
	DefaultConfigName = "agent"

	// DefaultConfigType is the canonical config format for the agent.
	DefaultConfigType = "yaml"
)

// Config represents the runtime configuration known by the v0.1.x foundation.
//
// v0.1.6 starts EPIC 2 by adding a logger section while preserving
// YAML, environment overrides, centralized defaults and validation.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Runtime   RuntimeConfig   `mapstructure:"runtime"`
	Logger    LoggerConfig    `mapstructure:"logger"`
	Server    ServerConfig    `mapstructure:"server"`
	Worker    WorkerConfig    `mapstructure:"worker"`
	Queue     QueueConfig     `mapstructure:"queue"`
	Resolver  ResolverConfig  `mapstructure:"resolver"`
	Download  DownloadConfig  `mapstructure:"download"`
	Upload    UploadConfig    `mapstructure:"upload"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
	Heartbeat HeartbeatConfig `mapstructure:"heartbeat"`
	Security  SecurityConfig  `mapstructure:"security"`
	MediaHub  MediaHubConfig  `mapstructure:"media_hub"`
	DevUI    DevUIConfig    `mapstructure:"dev_ui"`
}

// AppConfig contains process identity settings used by diagnostics.
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
}

// RuntimeConfig contains local execution settings.
type RuntimeConfig struct {
	Environment string `mapstructure:"environment"`
	DataDir     string `mapstructure:"data_dir"`
	TempDir     string `mapstructure:"temp_dir"`
}

// LoggerConfig contains structured logger settings used by EPIC 2.
type LoggerConfig struct {
	Level     string `mapstructure:"level"`
	Format    string `mapstructure:"format"`
	Timestamp bool   `mapstructure:"timestamp"`
	Service   string `mapstructure:"service"`
}

// ServerConfig contains the future HTTP server bind settings.
type ServerConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  string `mapstructure:"read_timeout"`
	WriteTimeout string `mapstructure:"write_timeout"`
	IdleTimeout  string `mapstructure:"idle_timeout"`
}

// WorkerConfig contains worker engine sizing placeholders.
type WorkerConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Concurrency     int    `mapstructure:"concurrency"`
	ShutdownTimeout string `mapstructure:"shutdown_timeout"`
}

// QueueConfig contains local queue placeholders for the worker roadmap.
type QueueConfig struct {
	Driver             string `mapstructure:"driver"`
	MemoryCapacity     int    `mapstructure:"memory_capacity"`
	PollInterval       string `mapstructure:"poll_interval"`
	RedisAddress       string `mapstructure:"redis_address"`
	RedisStream        string `mapstructure:"redis_stream"`
	RedisConsumerGroup string `mapstructure:"redis_consumer_group"`
	RabbitMQURL        string `mapstructure:"rabbitmq_url"`
	RabbitMQQueue      string `mapstructure:"rabbitmq_queue"`
	NATSURL            string `mapstructure:"nats_url"`
	NATSSubject        string `mapstructure:"nats_subject"`
	NATSQueueGroup     string `mapstructure:"nats_queue_group"`
}

// ResolverConfig contains URL resolver behavior placeholders.
type ResolverConfig struct {
	DefaultUserAgent string `mapstructure:"default_user_agent"`
	FollowRedirects  bool   `mapstructure:"follow_redirects"`
	MaxRedirects     int    `mapstructure:"max_redirects"`
}

// DownloadConfig contains download engine behavior placeholders.
type DownloadConfig struct {
	TempDir               string `mapstructure:"temp_dir"`
	ConnectTimeout        string `mapstructure:"connect_timeout"`
	ResponseHeaderTimeout string `mapstructure:"response_header_timeout"`
	IdleTimeout           string `mapstructure:"idle_timeout"`
	MaxRetries            int    `mapstructure:"max_retries"`
	RetryBackoff          string `mapstructure:"retry_backoff"`
	ChunkSize             string `mapstructure:"chunk_size"`
	ResumeEnabled         bool   `mapstructure:"resume_enabled"`
	Checksum              string `mapstructure:"checksum"`
}

// UploadConfig contains upload engine behavior placeholders.
type UploadConfig struct {
	Driver           string `mapstructure:"driver"`
	MaxRetries       int    `mapstructure:"max_retries"`
	RetryBackoff     string `mapstructure:"retry_backoff"`
	MultipartEnabled bool   `mapstructure:"multipart_enabled"`
	PartSize         string `mapstructure:"part_size"`
}

// StorageConfig contains the future Auren Storage adapter settings.
type StorageConfig struct {
	Driver       string `mapstructure:"driver"`
	Endpoint     string `mapstructure:"endpoint"`
	Bucket       string `mapstructure:"bucket"`
	APIKey       string `mapstructure:"api_key"`
	Region       string `mapstructure:"region"`
	LocalPath    string `mapstructure:"local_path"`
	UsePathStyle bool   `mapstructure:"use_path_style"`
}

// MetricsConfig contains metrics endpoint placeholders.
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

// HeartbeatConfig contains heartbeat loop placeholders.
type HeartbeatConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Interval string `mapstructure:"interval"`
	Timeout  string `mapstructure:"timeout"`
}

// SecurityConfig contains the first security-related configuration keys.
type SecurityConfig struct {
	APIKeyRequired    bool   `mapstructure:"api_key_required"`
	APIKey            string `mapstructure:"api_key"`
	APIKeyHash        string `mapstructure:"api_key_hash"`
	TokenHeader       string `mapstructure:"token_header"`
	AllowInsecureHTTP bool   `mapstructure:"allow_insecure_http"`
	JWTEnabled        bool   `mapstructure:"jwt_enabled"`
	JWTSecret         string `mapstructure:"jwt_secret"`
	JWTTTL            string `mapstructure:"jwt_ttl"`
	MTLSEnabled       bool   `mapstructure:"mtls_enabled"`
	MTLSRequiredCN    string `mapstructure:"mtls_required_cn"`
	RBACEnabled       bool   `mapstructure:"rbac_enabled"`
	RateLimitEnabled  bool   `mapstructure:"rate_limit_enabled"`
	RateLimitPerMin   int    `mapstructure:"rate_limit_per_minute"`
	SecretsProvider   string `mapstructure:"secrets_provider"`
	SecretsFile       string `mapstructure:"secrets_file"`
}

// MediaHubConfig contains Auren Media Hub control-plane connector settings.
type MediaHubConfig struct {
	Enabled                  bool   `mapstructure:"enabled"`
	BaseURL                  string `mapstructure:"base_url"`
	RegistrationToken        string `mapstructure:"registration_token"`
	NodeUUID                 string `mapstructure:"node_uuid"`
	NodeSecret               string `mapstructure:"node_secret"`
	HMACEnabled              bool   `mapstructure:"hmac_enabled"`
	PollEnabled              bool   `mapstructure:"poll_enabled"`
	PollInterval             string `mapstructure:"poll_interval"`
	TransferEnabled          bool   `mapstructure:"transfer_enabled"`
	ClaimEnabled             bool   `mapstructure:"claim_enabled"`
	ClaimInterval            string `mapstructure:"claim_interval"`
	ProgressInterval         string `mapstructure:"progress_interval"`
	ControlInterval          string `mapstructure:"control_interval"`
	MaxConcurrentJobs        int    `mapstructure:"max_concurrent_jobs"`
	AcceptedOperations       string `mapstructure:"accepted_operations"`
	WorkDir                  string `mapstructure:"work_dir"`
	MinBytes                 int64  `mapstructure:"min_bytes"`
	BlockHTML                bool   `mapstructure:"block_html"`
	GatewayEnabled           bool   `mapstructure:"gateway_enabled"`
	GatewayProxyEnabled      bool   `mapstructure:"gateway_proxy_enabled"`
	GatewayRedirectEnabled   bool   `mapstructure:"gateway_redirect_enabled"`
	GatewayHeartbeatInterval string `mapstructure:"gateway_heartbeat_interval"`
	GatewayTokenTTL          string `mapstructure:"gateway_token_ttl"`
	DrainEnabled             bool   `mapstructure:"drain_enabled"`
	DrainFile                string `mapstructure:"drain_file"`
	BackpressureEnabled      bool   `mapstructure:"backpressure_enabled"`
	DiskGuardEnabled         bool   `mapstructure:"disk_guard_enabled"`
	DiskMinFreeBytes         int64  `mapstructure:"disk_min_free_bytes"`
	DeadLetterEnabled        bool   `mapstructure:"dead_letter_enabled"`
	DeadLetterDir            string `mapstructure:"dead_letter_dir"`
	LeaseRenewalEnabled      bool   `mapstructure:"lease_renewal_enabled"`
	LeaseRenewalInterval     string `mapstructure:"lease_renewal_interval"`
	SecretRotationEnabled    bool   `mapstructure:"secret_rotation_enabled"`
	SecretRotationInterval   string `mapstructure:"secret_rotation_interval"`
	HeartbeatInterval        string `mapstructure:"heartbeat_interval"`
	MetricsInterval          string `mapstructure:"metrics_interval"`
	EventsFlushInterval      string `mapstructure:"events_flush_interval"`
	Role                     string `mapstructure:"role"`
	Provider                 string `mapstructure:"provider"`
	Region                   string `mapstructure:"region"`
	AvailabilityZone         string `mapstructure:"availability_zone"`
	PublicBaseURL            string `mapstructure:"public_base_url"`
	HealthURL                string `mapstructure:"health_url"`
	MaxSessions              int    `mapstructure:"max_sessions"`
	MaxEgressMbps            int    `mapstructure:"max_egress_mbps"`
	Capabilities             string `mapstructure:"capabilities"`
}

// LoadOptions controls how Viper discovers and reads configuration.
type LoadOptions struct {
	// Path points to an explicit config file. When empty, Viper searches the
	// standard project config locations without failing if no file is present.
	Path string
}

// Load reads the agent configuration through Viper.
func Load(options LoadOptions) (Config, error) {
	reader := viper.New()
	reader.SetConfigType(DefaultConfigType)
	registerDefaults(reader)
	registerEnvironmentOverrides(reader)

	if strings.TrimSpace(options.Path) != "" {
		reader.SetConfigFile(options.Path)
	} else {
		reader.SetConfigName(DefaultConfigName)
		for _, path := range DefaultSearchPaths() {
			reader.AddConfigPath(path)
		}
	}

	if err := reader.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) || strings.TrimSpace(options.Path) != "" {
			return Config{}, fmt.Errorf("load config: %w", err)
		}
	}

	var cfg Config
	if err := reader.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if err := Validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// ServerAddress renders the configured HTTP server bind address.
func (cfg Config) ServerAddress() string {
	return fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
}

// MetricsAddress renders the configured metrics bind address.
func (cfg Config) MetricsAddress() string {
	return fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.Port)
}

func registerDefaults(reader *viper.Viper) {
	for key, value := range DefaultValues() {
		reader.SetDefault(key, value)
	}
}

func registerEnvironmentOverrides(reader *viper.Viper) {
	reader.SetEnvPrefix(DefaultEnvPrefix)
	reader.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	reader.AutomaticEnv()
}


// DevUIConfig controls the lightweight local development console exposed by the Agent.
type DevUIConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Path            string `mapstructure:"path"`
	Retention        int    `mapstructure:"retention"`
	RefreshInterval string `mapstructure:"refresh_interval"`
}
