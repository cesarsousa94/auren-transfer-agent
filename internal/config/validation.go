package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ValidationIssue describes one invalid configuration field.
type ValidationIssue struct {
	Field   string
	Message string
}

// ValidationError groups all configuration validation issues found in one pass.
type ValidationError struct {
	Issues []ValidationIssue
}

// Error renders a stable, human-readable validation failure.
func (err ValidationError) Error() string {
	if len(err.Issues) == 0 {
		return "config validation failed"
	}

	parts := make([]string, 0, len(err.Issues))
	for _, issue := range err.Issues {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Field, issue.Message))
	}

	return "config validation failed: " + strings.Join(parts, "; ")
}

// Validate checks the structural configuration contract used by EPIC 1.
//
// The agent remains business-rule free: validation only verifies safe runtime
// primitives such as required fields, ports, durations, sizes and currently
// supported foundation drivers.
func Validate(cfg Config) error {
	validator := configValidator{}

	validator.requiredString("app.name", cfg.App.Name)
	validator.requiredString("app.description", cfg.App.Description)

	validator.oneOf("runtime.environment", cfg.Runtime.Environment, []string{"local", "development", "test", "staging", "production"})
	validator.requiredString("runtime.data_dir", cfg.Runtime.DataDir)
	validator.requiredString("runtime.temp_dir", cfg.Runtime.TempDir)

	validator.oneOf("logger.level", cfg.Logger.Level, []string{"trace", "debug", "info", "warn", "error", "fatal", "panic", "disabled"})
	validator.oneOf("logger.format", cfg.Logger.Format, []string{"json", "console"})
	validator.requiredString("logger.service", cfg.Logger.Service)

	validator.requiredString("server.host", cfg.Server.Host)
	validator.port("server.port", cfg.Server.Port)
	validator.duration("server.read_timeout", cfg.Server.ReadTimeout)
	validator.duration("server.write_timeout", cfg.Server.WriteTimeout)
	validator.duration("server.idle_timeout", cfg.Server.IdleTimeout)

	validator.positiveInt("worker.concurrency", cfg.Worker.Concurrency)
	validator.duration("worker.shutdown_timeout", cfg.Worker.ShutdownTimeout)

	validator.oneOf("queue.driver", cfg.Queue.Driver, []string{"memory", "redis_streams", "rabbitmq", "nats"})
	validator.positiveInt("queue.memory_capacity", cfg.Queue.MemoryCapacity)
	validator.duration("queue.poll_interval", cfg.Queue.PollInterval)
	if strings.EqualFold(strings.TrimSpace(cfg.Queue.Driver), "redis_streams") {
		validator.requiredString("queue.redis_address", cfg.Queue.RedisAddress)
		validator.requiredString("queue.redis_stream", cfg.Queue.RedisStream)
		validator.requiredString("queue.redis_consumer_group", cfg.Queue.RedisConsumerGroup)
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Queue.Driver), "rabbitmq") {
		validator.requiredString("queue.rabbitmq_url", cfg.Queue.RabbitMQURL)
		validator.requiredString("queue.rabbitmq_queue", cfg.Queue.RabbitMQQueue)
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Queue.Driver), "nats") {
		validator.requiredString("queue.nats_url", cfg.Queue.NATSURL)
		validator.requiredString("queue.nats_subject", cfg.Queue.NATSSubject)
		validator.requiredString("queue.nats_queue_group", cfg.Queue.NATSQueueGroup)
	}

	validator.requiredString("resolver.default_user_agent", cfg.Resolver.DefaultUserAgent)
	validator.nonNegativeInt("resolver.max_redirects", cfg.Resolver.MaxRedirects)

	validator.requiredString("download.temp_dir", cfg.Download.TempDir)
	validator.duration("download.connect_timeout", cfg.Download.ConnectTimeout)
	validator.duration("download.response_header_timeout", cfg.Download.ResponseHeaderTimeout)
	validator.duration("download.idle_timeout", cfg.Download.IdleTimeout)
	validator.nonNegativeInt("download.max_retries", cfg.Download.MaxRetries)
	validator.duration("download.retry_backoff", cfg.Download.RetryBackoff)
	validator.size("download.chunk_size", cfg.Download.ChunkSize)
	validator.oneOf("download.checksum", cfg.Download.Checksum, []string{"sha256", "none"})

	validator.oneOf("upload.driver", cfg.Upload.Driver, []string{"local"})
	validator.nonNegativeInt("upload.max_retries", cfg.Upload.MaxRetries)
	validator.duration("upload.retry_backoff", cfg.Upload.RetryBackoff)
	validator.size("upload.part_size", cfg.Upload.PartSize)

	validator.oneOf("storage.driver", cfg.Storage.Driver, []string{"local", "auren_storage"})
	validator.requiredString("storage.region", cfg.Storage.Region)
	if strings.EqualFold(strings.TrimSpace(cfg.Storage.Driver), "auren_storage") {
		validator.requiredString("storage.endpoint", cfg.Storage.Endpoint)
		validator.requiredString("storage.bucket", cfg.Storage.Bucket)
	} else {
		validator.requiredString("storage.local_path", cfg.Storage.LocalPath)
	}

	validator.requiredString("metrics.host", cfg.Metrics.Host)
	validator.port("metrics.port", cfg.Metrics.Port)
	validator.path("metrics.path", cfg.Metrics.Path)

	validator.duration("heartbeat.interval", cfg.Heartbeat.Interval)
	validator.duration("heartbeat.timeout", cfg.Heartbeat.Timeout)

	validator.requiredString("security.token_header", cfg.Security.TokenHeader)
	if strings.ContainsAny(cfg.Security.TokenHeader, "\r\n") {
		validator.add("security.token_header", "cannot contain newline characters")
	}
	if strings.TrimSpace(cfg.Security.APIKeyHash) != "" && len(strings.TrimSpace(cfg.Security.APIKeyHash)) != 64 {
		validator.add("security.api_key_hash", "must be a sha256 hex digest")
	}
	validator.duration("security.jwt_ttl", cfg.Security.JWTTTL)
	if cfg.Security.JWTEnabled && len(strings.TrimSpace(cfg.Security.JWTSecret)) < 16 {
		validator.add("security.jwt_secret", "must contain at least 16 characters when jwt is enabled")
	}
	if cfg.Security.RateLimitEnabled {
		validator.positiveInt("security.rate_limit_per_minute", cfg.Security.RateLimitPerMin)
	} else {
		validator.nonNegativeInt("security.rate_limit_per_minute", cfg.Security.RateLimitPerMin)
	}
	validator.oneOf("security.secrets_provider", cfg.Security.SecretsProvider, []string{"env", "file", "none"})
	if strings.EqualFold(strings.TrimSpace(cfg.Security.SecretsProvider), "file") {
		validator.requiredString("security.secrets_file", cfg.Security.SecretsFile)
	}

	if cfg.MediaHub.Enabled {
		validator.requiredString("media_hub.base_url", cfg.MediaHub.BaseURL)
		validator.duration("media_hub.poll_interval", cfg.MediaHub.PollInterval)
		validator.duration("media_hub.claim_interval", cfg.MediaHub.ClaimInterval)
		validator.duration("media_hub.progress_interval", cfg.MediaHub.ProgressInterval)
		validator.duration("media_hub.control_interval", cfg.MediaHub.ControlInterval)
		validator.nonNegativeInt("media_hub.max_concurrent_jobs", cfg.MediaHub.MaxConcurrentJobs)
		if cfg.MediaHub.TransferEnabled || cfg.MediaHub.ClaimEnabled {
			validator.requiredString("media_hub.work_dir", cfg.MediaHub.WorkDir)
			validator.requiredString("media_hub.accepted_operations", cfg.MediaHub.AcceptedOperations)
			validator.positiveInt("media_hub.max_concurrent_jobs", cfg.MediaHub.MaxConcurrentJobs)
		}
		if cfg.MediaHub.MinBytes < 0 {
			validator.add("media_hub.min_bytes", "must be zero or greater")
		}
		validator.duration("media_hub.gateway_heartbeat_interval", cfg.MediaHub.GatewayHeartbeatInterval)
		validator.duration("media_hub.gateway_token_ttl", cfg.MediaHub.GatewayTokenTTL)
		validator.duration("media_hub.lease_renewal_interval", cfg.MediaHub.LeaseRenewalInterval)
		validator.duration("media_hub.secret_rotation_interval", cfg.MediaHub.SecretRotationInterval)
		if cfg.MediaHub.DiskMinFreeBytes < 0 {
			validator.add("media_hub.disk_min_free_bytes", "must be zero or greater")
		}
		if cfg.MediaHub.DrainEnabled {
			validator.requiredString("media_hub.drain_file", cfg.MediaHub.DrainFile)
		}
		if cfg.MediaHub.DeadLetterEnabled {
			validator.requiredString("media_hub.dead_letter_dir", cfg.MediaHub.DeadLetterDir)
		}
		if cfg.MediaHub.GatewayEnabled {
			validator.requiredString("media_hub.public_base_url", cfg.MediaHub.PublicBaseURL)
		}
		validator.duration("media_hub.heartbeat_interval", cfg.MediaHub.HeartbeatInterval)
		validator.duration("media_hub.metrics_interval", cfg.MediaHub.MetricsInterval)
		validator.duration("media_hub.events_flush_interval", cfg.MediaHub.EventsFlushInterval)
		validator.oneOf("media_hub.role", cfg.MediaHub.Role, []string{"gateway", "worker", "edge", "hybrid"})
		validator.requiredString("media_hub.provider", cfg.MediaHub.Provider)
		validator.requiredString("media_hub.region", cfg.MediaHub.Region)
		validator.nonNegativeInt("media_hub.max_sessions", cfg.MediaHub.MaxSessions)
		validator.nonNegativeInt("media_hub.max_egress_mbps", cfg.MediaHub.MaxEgressMbps)
		if strings.TrimSpace(cfg.MediaHub.Capabilities) == "" {
			validator.add("media_hub.capabilities", "must include at least one capability when media hub connector is enabled")
		}
	}

	if len(validator.issues) > 0 {
		return ValidationError{Issues: validator.issues}
	}

	return nil
}

type configValidator struct {
	issues []ValidationIssue
}

func (validator *configValidator) add(field string, message string) {
	validator.issues = append(validator.issues, ValidationIssue{Field: field, Message: message})
}

func (validator *configValidator) requiredString(field string, value string) {
	if strings.TrimSpace(value) == "" {
		validator.add(field, "is required")
	}
}

func (validator *configValidator) positiveInt(field string, value int) {
	if value <= 0 {
		validator.add(field, "must be greater than zero")
	}
}

func (validator *configValidator) nonNegativeInt(field string, value int) {
	if value < 0 {
		validator.add(field, "must be zero or greater")
	}
}

func (validator *configValidator) port(field string, value int) {
	if value < 1 || value > 65535 {
		validator.add(field, "must be between 1 and 65535")
	}
}

func (validator *configValidator) duration(field string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		validator.add(field, "is required")
		return
	}

	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		validator.add(field, "must be a valid duration")
		return
	}
	if duration <= 0 {
		validator.add(field, "must be greater than zero")
	}
}

func (validator *configValidator) size(field string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		validator.add(field, "is required")
		return
	}

	if _, err := parseSizeBytes(trimmed); err != nil {
		validator.add(field, "must be a valid positive byte size")
	}
}

func (validator *configValidator) path(field string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		validator.add(field, "is required")
		return
	}
	if !strings.HasPrefix(trimmed, "/") {
		validator.add(field, "must start with /")
	}
}

func (validator *configValidator) oneOf(field string, value string, allowed []string) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		validator.add(field, "is required")
		return
	}
	for _, candidate := range allowed {
		if trimmed == strings.ToLower(candidate) {
			return
		}
	}
	validator.add(field, "must be one of "+strings.Join(allowed, ", "))
}

func parseSizeBytes(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("empty size")
	}

	lower := strings.ToLower(trimmed)
	units := []struct {
		suffix     string
		multiplier int64
	}{
		{suffix: "kib", multiplier: 1024},
		{suffix: "mib", multiplier: 1024 * 1024},
		{suffix: "gib", multiplier: 1024 * 1024 * 1024},
		{suffix: "kb", multiplier: 1000},
		{suffix: "mb", multiplier: 1000 * 1000},
		{suffix: "gb", multiplier: 1000 * 1000 * 1000},
		{suffix: "b", multiplier: 1},
	}

	multiplier := int64(1)
	number := lower
	for _, unit := range units {
		if strings.HasSuffix(lower, unit.suffix) {
			multiplier = unit.multiplier
			number = strings.TrimSpace(strings.TrimSuffix(lower, unit.suffix))
			break
		}
	}

	parsed, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("size must be positive")
	}

	return parsed * multiplier, nil
}
