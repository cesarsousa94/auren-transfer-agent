package config

const (
	// DefaultEnvPrefix is the canonical environment variable prefix.
	DefaultEnvPrefix = "AUREN"
)

var defaultSearchPaths = []string{
	".",
	"./configs",
	"/etc/auren-transfer-agent",
}

// DefaultSearchPaths returns the canonical config discovery locations.
func DefaultSearchPaths() []string {
	paths := make([]string, len(defaultSearchPaths))
	copy(paths, defaultSearchPaths)
	return paths
}

// DefaultConfig returns the official built-in configuration baseline.
func DefaultConfig() Config {
	return Config{
		App: AppConfig{
			Name:        "auren-transfer-agent",
			Description: "High-reliability media transfer agent",
		},
		Runtime: RuntimeConfig{
			Environment: "local",
			DataDir:     "./data",
			TempDir:     "./tmp",
		},
		Logger: LoggerConfig{
			Level:     "info",
			Format:    "json",
			Timestamp: true,
			Service:   "auren-transfer-agent",
		},
		Server: ServerConfig{
			Enabled:      false,
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  "30s",
			WriteTimeout: "30s",
			IdleTimeout:  "60s",
		},
		Worker: WorkerConfig{
			Enabled:         false,
			Concurrency:     1,
			ShutdownTimeout: "30s",
		},
		Queue: QueueConfig{
			Driver:             "memory",
			MemoryCapacity:     100,
			PollInterval:       "1s",
			RedisAddress:       "redis://localhost:6379",
			RedisStream:        "auren.transfer.jobs",
			RedisConsumerGroup: "auren-transfer-agents",
			RabbitMQURL:        "amqp://guest:guest@localhost:5672/",
			RabbitMQQueue:      "auren.transfer.jobs",
			NATSURL:            "nats://localhost:4222",
			NATSSubject:        "auren.transfer.jobs",
			NATSQueueGroup:     "auren-transfer-agents",
		},
		Resolver: ResolverConfig{
			DefaultUserAgent: "AurenTransferAgent/1.0",
			FollowRedirects:  true,
			MaxRedirects:     10,
		},
		Download: DownloadConfig{
			TempDir:               "./tmp/downloads",
			ConnectTimeout:        "15s",
			ResponseHeaderTimeout: "30s",
			IdleTimeout:           "60s",
			MaxRetries:            3,
			RetryBackoff:          "2s",
			ChunkSize:             "8MiB",
			ResumeEnabled:         true,
			Checksum:              "sha256",
		},
		Upload: UploadConfig{
			Driver:           "local",
			MaxRetries:       3,
			RetryBackoff:     "2s",
			MultipartEnabled: true,
			PartSize:         "16MiB",
		},
		Storage: StorageConfig{
			Driver:       "local",
			Endpoint:     "",
			Bucket:       "",
			Region:       "us-east-1",
			LocalPath:    "./data/storage",
			UsePathStyle: true,
		},
		Metrics: MetricsConfig{
			Enabled: false,
			Host:    "0.0.0.0",
			Port:    9090,
			Path:    "/metrics",
		},
		Heartbeat: HeartbeatConfig{
			Enabled:  false,
			Interval: "30s",
			Timeout:  "10s",
		},
		Security: SecurityConfig{
			APIKeyRequired:    false,
			APIKey:            "",
			APIKeyHash:        "",
			TokenHeader:       "Authorization",
			AllowInsecureHTTP: true,
			JWTEnabled:        false,
			JWTSecret:         "",
			JWTTTL:            "15m",
			MTLSEnabled:       false,
			MTLSRequiredCN:    "",
			RBACEnabled:       false,
			RateLimitEnabled:  false,
			RateLimitPerMin:   60,
			SecretsProvider:   "env",
			SecretsFile:       "",
		},
	}
}

// DefaultValues returns the built-in defaults as dotted keys for Viper.
func DefaultValues() map[string]any {
	cfg := DefaultConfig()

	return map[string]any{
		"app.name":        cfg.App.Name,
		"app.description": cfg.App.Description,

		"runtime.environment": cfg.Runtime.Environment,
		"runtime.data_dir":    cfg.Runtime.DataDir,
		"runtime.temp_dir":    cfg.Runtime.TempDir,

		"logger.level":     cfg.Logger.Level,
		"logger.format":    cfg.Logger.Format,
		"logger.timestamp": cfg.Logger.Timestamp,
		"logger.service":   cfg.Logger.Service,

		"server.enabled":       cfg.Server.Enabled,
		"server.host":          cfg.Server.Host,
		"server.port":          cfg.Server.Port,
		"server.read_timeout":  cfg.Server.ReadTimeout,
		"server.write_timeout": cfg.Server.WriteTimeout,
		"server.idle_timeout":  cfg.Server.IdleTimeout,

		"worker.enabled":          cfg.Worker.Enabled,
		"worker.concurrency":      cfg.Worker.Concurrency,
		"worker.shutdown_timeout": cfg.Worker.ShutdownTimeout,

		"queue.driver":               cfg.Queue.Driver,
		"queue.memory_capacity":      cfg.Queue.MemoryCapacity,
		"queue.poll_interval":        cfg.Queue.PollInterval,
		"queue.redis_address":        cfg.Queue.RedisAddress,
		"queue.redis_stream":         cfg.Queue.RedisStream,
		"queue.redis_consumer_group": cfg.Queue.RedisConsumerGroup,
		"queue.rabbitmq_url":         cfg.Queue.RabbitMQURL,
		"queue.rabbitmq_queue":       cfg.Queue.RabbitMQQueue,
		"queue.nats_url":             cfg.Queue.NATSURL,
		"queue.nats_subject":         cfg.Queue.NATSSubject,
		"queue.nats_queue_group":     cfg.Queue.NATSQueueGroup,

		"resolver.default_user_agent": cfg.Resolver.DefaultUserAgent,
		"resolver.follow_redirects":   cfg.Resolver.FollowRedirects,
		"resolver.max_redirects":      cfg.Resolver.MaxRedirects,

		"download.temp_dir":                cfg.Download.TempDir,
		"download.connect_timeout":         cfg.Download.ConnectTimeout,
		"download.response_header_timeout": cfg.Download.ResponseHeaderTimeout,
		"download.idle_timeout":            cfg.Download.IdleTimeout,
		"download.max_retries":             cfg.Download.MaxRetries,
		"download.retry_backoff":           cfg.Download.RetryBackoff,
		"download.chunk_size":              cfg.Download.ChunkSize,
		"download.resume_enabled":          cfg.Download.ResumeEnabled,
		"download.checksum":                cfg.Download.Checksum,

		"upload.driver":            cfg.Upload.Driver,
		"upload.max_retries":       cfg.Upload.MaxRetries,
		"upload.retry_backoff":     cfg.Upload.RetryBackoff,
		"upload.multipart_enabled": cfg.Upload.MultipartEnabled,
		"upload.part_size":         cfg.Upload.PartSize,

		"storage.driver":         cfg.Storage.Driver,
		"storage.endpoint":       cfg.Storage.Endpoint,
		"storage.bucket":         cfg.Storage.Bucket,
		"storage.region":         cfg.Storage.Region,
		"storage.local_path":     cfg.Storage.LocalPath,
		"storage.use_path_style": cfg.Storage.UsePathStyle,

		"metrics.enabled": cfg.Metrics.Enabled,
		"metrics.host":    cfg.Metrics.Host,
		"metrics.port":    cfg.Metrics.Port,
		"metrics.path":    cfg.Metrics.Path,

		"heartbeat.enabled":  cfg.Heartbeat.Enabled,
		"heartbeat.interval": cfg.Heartbeat.Interval,
		"heartbeat.timeout":  cfg.Heartbeat.Timeout,

		"security.api_key_required":      cfg.Security.APIKeyRequired,
		"security.api_key":               cfg.Security.APIKey,
		"security.api_key_hash":          cfg.Security.APIKeyHash,
		"security.token_header":          cfg.Security.TokenHeader,
		"security.allow_insecure_http":   cfg.Security.AllowInsecureHTTP,
		"security.jwt_enabled":           cfg.Security.JWTEnabled,
		"security.jwt_secret":            cfg.Security.JWTSecret,
		"security.jwt_ttl":               cfg.Security.JWTTTL,
		"security.mtls_enabled":          cfg.Security.MTLSEnabled,
		"security.mtls_required_cn":      cfg.Security.MTLSRequiredCN,
		"security.rbac_enabled":          cfg.Security.RBACEnabled,
		"security.rate_limit_enabled":    cfg.Security.RateLimitEnabled,
		"security.rate_limit_per_minute": cfg.Security.RateLimitPerMin,
		"security.secrets_provider":      cfg.Security.SecretsProvider,
		"security.secrets_file":          cfg.Security.SecretsFile,
	}
}
