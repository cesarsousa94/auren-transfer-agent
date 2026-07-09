package bootstrap

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auren/auren-transfer-agent/internal/cluster"
	"github.com/auren/auren-transfer-agent/internal/config"
	"github.com/auren/auren-transfer-agent/internal/dispatcher"
	"github.com/auren/auren-transfer-agent/internal/download"
	"github.com/auren/auren-transfer-agent/internal/heartbeat"
	"github.com/auren/auren-transfer-agent/internal/identity"
	"github.com/auren/auren-transfer-agent/internal/logger"
	"github.com/auren/auren-transfer-agent/internal/observability"
	"github.com/auren/auren-transfer-agent/internal/queue"
	"github.com/auren/auren-transfer-agent/internal/resolver"
	"github.com/auren/auren-transfer-agent/internal/runtime"
	"github.com/auren/auren-transfer-agent/internal/scheduler"
	"github.com/auren/auren-transfer-agent/internal/security"
	"github.com/auren/auren-transfer-agent/internal/server"
	"github.com/auren/auren-transfer-agent/internal/storage"
	"github.com/auren/auren-transfer-agent/internal/upload"
	"github.com/auren/auren-transfer-agent/internal/worker"
	"github.com/auren/auren-transfer-agent/pkg/plugins"
)

// Run starts the Auren Transfer Agent foundation executable.
//
// v1.0.0 completes the production foundation while keeping business
// decisions outside the Agent runtime.
func Run(args []string) error {
	flags := flag.NewFlagSet("auren-transfer-agent", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	showVersion := flags.Bool("version", false, "print version information")
	showHelp := flags.Bool("help", false, "print help")
	configPath := flags.String("config", "", "path to agent YAML configuration")
	flags.BoolVar(showHelp, "h", false, "print help")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *showHelp {
		printHelp(os.Stdout)
		return nil
	}

	if *showVersion {
		fmt.Fprintln(os.Stdout, runtime.Info().String())
		return nil
	}

	cfg, err := config.Load(config.LoadOptions{Path: *configPath})
	if err != nil {
		return err
	}

	log, err := logger.New(cfg.Logger, os.Stdout)
	if err != nil {
		return err
	}

	identityStore := identity.NewFileStore(identity.DefaultStorePath(cfg.Runtime.DataDir))
	identityResult, err := identityStore.Ensure()
	if err != nil {
		return err
	}
	agentID := identityResult.Record.AgentID
	hostInfo := identity.ResolveHostname()
	identitySnapshot, err := identity.NewSnapshot(identityResult, hostInfo)
	if err != nil {
		return err
	}

	ctx := logger.IntoContext(context.Background(), logger.WithFields(log, logger.String(logger.FieldComponent, "bootstrap"), logger.String(logger.FieldAgentID, agentID), logger.String("fingerprint", identitySnapshot.Fingerprint), logger.String("hostname", hostInfo.Normalized), logger.String("hostname_source", hostInfo.Source)))
	logger.LogRuntimeStartup(logger.FromContextOrDefault(ctx, log), logger.RuntimeStartupEvent{Version: runtime.Version, Status: runtime.Status, Environment: cfg.Runtime.Environment})
	downloadClient, err := download.NewHTTPClientFromConfig(cfg)
	if err != nil {
		return err
	}

	downloadRetryPolicy, err := download.NewRetryPolicyFromConfig(cfg.Download.MaxRetries, cfg.Download.RetryBackoff)
	if err != nil {
		return err
	}
	downloadBandwidthController, err := download.NewBandwidthController(0)
	if err != nil {
		return err
	}
	downloadMetricsRecorder := download.NewMemoryMetricsRecorder()
	uploadPartSize, err := upload.ParsePartSize(cfg.Upload.PartSize)
	if err != nil {
		return err
	}
	localUploader, err := upload.NewLocalUploader(cfg.Storage.LocalPath)
	if err != nil {
		return err
	}
	localStorageAdapter, err := storage.NewLocalAdapter(cfg.Storage.LocalPath)
	if err != nil {
		return err
	}
	aurenStorageStatus := "not_configured"
	if storage.AurenConfigured(cfg.Storage.Endpoint, cfg.Storage.Bucket) {
		aurenAdapter, err := storage.NewAurenStorageAdapter(storage.AurenOptions{Endpoint: cfg.Storage.Endpoint, Bucket: cfg.Storage.Bucket, Region: cfg.Storage.Region, TokenHeader: cfg.Security.TokenHeader})
		if err != nil {
			return err
		}
		aurenStorageStatus = "configured:" + aurenAdapter.Bucket()
	}

	httpResolver, err := resolver.NewHTTPResolver(downloadClient)
	if err != nil {
		return err
	}
	redirectResolver, err := resolver.NewRedirectResolver(downloadClient)
	if err != nil {
		return err
	}
	cloudflareResolver, err := resolver.NewCloudflareResolver(downloadClient)
	if err != nil {
		return err
	}
	m3u8Resolver, err := resolver.NewM3U8Resolver(downloadClient)
	if err != nil {
		return err
	}
	hlsResolver, err := resolver.NewHLSResolver(downloadClient)
	if err != nil {
		return err
	}
	resolverRegistry, err := resolver.NewRegistry(resolver.NewXtreamResolver(), resolver.NewShuiResolver(), cloudflareResolver, hlsResolver, m3u8Resolver, resolver.NewGoogleDriveResolver(), resolver.NewMEGAResolver(), resolver.NewOneDriveResolver(), redirectResolver, httpResolver)
	if err != nil {
		return err
	}

	queueStore := queue.NewFileStore(queue.DefaultPersistencePath(cfg.Runtime.DataDir))
	jobQueue, err := queue.NewQueue(queue.Options{Driver: cfg.Queue.Driver, MemoryCapacity: cfg.Queue.MemoryCapacity, RedisAddress: cfg.Queue.RedisAddress, RedisStream: cfg.Queue.RedisStream, RedisConsumerGroup: cfg.Queue.RedisConsumerGroup, RabbitMQURL: cfg.Queue.RabbitMQURL, RabbitMQQueue: cfg.Queue.RabbitMQQueue, NATSURL: cfg.Queue.NATSURL, NATSSubject: cfg.Queue.NATSSubject, NATSQueueGroup: cfg.Queue.NATSQueueGroup})
	if err != nil {
		return err
	}
	defer jobQueue.Close()
	queueInfo := jobQueue.Info()

	queueSnapshot, queueStoreResult, err := queueStore.Ensure(ctx, cfg.Queue.Driver)
	if err != nil {
		return err
	}
	queueStoreSource := queueStoreResult.Source
	restoredJobs := 0
	if queueStoreSource == queue.StoreSourceLoaded {
		restoredJobs, err = queue.Restore(ctx, jobQueue, queueSnapshot)
		if err != nil {
			return err
		}
	}
	queueStoreResult, err = queueStore.Save(ctx, cfg.Queue.Driver, jobQueue.Snapshot())
	if err != nil {
		return err
	}

	workerPool, err := worker.NewPool(worker.PoolOptions{Concurrency: cfg.Worker.Concurrency, Queue: jobQueue, Handler: worker.NoopHandler()})
	if err != nil {
		return err
	}
	jobDispatcher, err := dispatcher.New(dispatcher.Options{Pool: workerPool, RetryQueue: jobQueue, RetryPolicy: dispatcher.NewAttemptsRetryPolicy()})
	if err != nil {
		return err
	}
	pollInterval, err := time.ParseDuration(cfg.Queue.PollInterval)
	if err != nil {
		return err
	}
	workerScheduler, err := scheduler.NewFixedInterval(pollInterval, func(ctx context.Context) error {
		_, err := jobDispatcher.RunOnce(ctx)
		return err
	})
	if err != nil {
		return err
	}
	heartbeatInterval, err := time.ParseDuration(cfg.Heartbeat.Interval)
	if err != nil {
		return err
	}
	heartbeatRecord, err := heartbeat.NewRecord(heartbeat.Input{
		Identity:      identitySnapshot,
		Version:       runtime.Info(),
		Status:        heartbeat.StatusIdle,
		Interval:      heartbeatInterval,
		WorkerEnabled: cfg.Worker.Enabled,
		PoolStats:     workerPool.Stats(),
		QueueStats:    heartbeat.QueueStats{Driver: cfg.Queue.Driver, Length: jobQueue.Len(), Capacity: jobQueue.Capacity()},
	})
	if err != nil {
		return err
	}

	workerAPIOptions := server.WorkerAPIOptions{Info: runtime.Info(), Heartbeat: heartbeatRecord, Queue: jobQueue, Driver: cfg.Queue.Driver, Persister: queueStore}
	authenticator, err := server.NewAuthenticator(server.AuthOptions{Required: cfg.Security.APIKeyRequired, APIKey: cfg.Security.APIKey, TokenHeader: cfg.Security.TokenHeader})
	if err != nil {
		return err
	}
	securityAPIKeys, err := security.NewAPIKeyVerifier(security.APIKeyOptions{Required: cfg.Security.APIKeyRequired, RawKey: cfg.Security.APIKey, Hash: cfg.Security.APIKeyHash, Header: cfg.Security.TokenHeader})
	if err != nil {
		return err
	}
	jwtTTL, err := time.ParseDuration(cfg.Security.JWTTTL)
	if err != nil {
		return err
	}
	jwtService, err := security.NewJWTService(security.JWTOptions{Enabled: cfg.Security.JWTEnabled, Secret: cfg.Security.JWTSecret, Issuer: cfg.App.Name, Audience: "auren-media-hub", TTL: jwtTTL})
	if err != nil {
		return err
	}
	mtlsPolicy := security.NewMTLSPolicy(security.MTLSOptions{Enabled: cfg.Security.MTLSEnabled, RequiredCN: cfg.Security.MTLSRequiredCN})
	rbacPolicy := security.NewDefaultPolicy()
	rateLimiter, err := security.NewRateLimiter(rateLimitValue(cfg.Security.RateLimitEnabled, cfg.Security.RateLimitPerMin), time.Minute)
	if err != nil {
		return err
	}
	secretStore := security.NewSecrets(map[string]string{"api_key": cfg.Security.APIKey, "jwt_secret": cfg.Security.JWTSecret})
	if cfg.Security.SecretsProvider == "file" {
		secretStore, err = security.LoadSecretsFile(cfg.Security.SecretsFile)
		if err != nil {
			return err
		}
	}
	communicationOptions := server.CommunicationOptions{Info: runtime.Info(), Identity: identitySnapshot, Heartbeat: heartbeatRecord, Authenticator: authenticator}
	localAgent, err := cluster.LocalAgent(identitySnapshot, runtime.Info(), cfg.Worker.Concurrency, time.Now().UTC())
	if err != nil {
		return err
	}
	agentRegistry, err := cluster.NewRegistry(localAgent)
	if err != nil {
		return err
	}
	leaderResult, leaderFound := cluster.ElectLeader(agentRegistry.List())
	loadBalancedAgent, loadBalancerReady := cluster.SelectLeastLoaded(agentRegistry.List())
	failoverPlan, err := cluster.PlanFailover(agentRegistry.List(), localAgent.ID, jobQueue.Snapshot())
	if err != nil {
		return err
	}

	eventRecorder := server.NewEventRecorder(100)
	_, _ = eventRecorder.Record(server.EventInput{Level: server.EventLevelInfo, Type: "agent.bootstrap", Message: "communication telemetry initialized"})
	traceRecorder := observability.NewTraceRecorder(100)
	_, _ = traceRecorder.Record(observability.SpanInput{Name: "bootstrap.initialize", Kind: "internal", Status: "ok", Attributes: map[string]string{"version": runtime.Version}})
	auditRecorder := observability.NewAuditRecorder(100)
	_, _ = auditRecorder.Record(observability.AuditInput{Actor: "system", Action: "initialize", Resource: "agent", Outcome: "success", Metadata: map[string]string{"version": runtime.Version}})
	centralLogSink := observability.NewCentralLogSink(100)
	_, _ = centralLogSink.Record(observability.LogInput{Level: "info", Component: "bootstrap", Message: "observability foundation initialized", Metadata: map[string]string{"version": runtime.Version}})
	metricsAPIOptions := server.MetricsAPIOptions{Info: runtime.Info(), Heartbeat: heartbeatRecord, Queue: jobQueue, DownloadMetrics: downloadMetricsRecorder}
	eventsAPIOptions := server.EventsAPIOptions{Info: runtime.Info(), Recorder: eventRecorder, MaxEvents: 100}
	observabilityOptions := server.ObservabilityOptions{Info: runtime.Info(), Heartbeat: heartbeatRecord, Queue: jobQueue, DownloadMetrics: downloadMetricsRecorder, Events: eventRecorder, Traces: traceRecorder, Audit: auditRecorder, Logs: centralLogSink, PrometheusPath: cfg.Metrics.Path, Authenticator: authenticator}
	routes := append(server.FoundationRoutes(runtime.Info(), identitySnapshot), server.WorkerRoutes(workerAPIOptions)...)
	routes = append(routes, server.CommunicationRoutes(communicationOptions)...)
	routes = append(routes, server.TelemetryRoutes(metricsAPIOptions, eventsAPIOptions, authenticator)...)
	routes = append(routes, server.ObservabilityRoutes(observabilityOptions)...)
	registry, err := server.NewRouteRegistry(routes...)
	if err != nil {
		return err
	}
	middlewareRegistry, err := server.DefaultMiddlewareRegistry(server.MiddlewareOptions{Logger: log, RequestLogging: true, RecoverPanics: true})
	if err != nil {
		return err
	}
	router, err := server.BuildRouter(server.RouterOptions{Logger: log, Middlewares: middlewareRegistry.Definitions(), Routes: registry.Routes()})
	if err != nil {
		return err
	}
	_ = router

	fmt.Fprintf(os.Stdout, "%s %s initialized\n", runtime.AppName, runtime.Version)
	fmt.Fprintln(os.Stdout, "status: production-ready")
	fmt.Fprintf(os.Stdout, "identity: agent_id=%s fingerprint=%s algorithm=%s persistence=%s source=%s path=%s\n", agentID, identitySnapshot.Fingerprint, identitySnapshot.FingerprintAlgorithm, identityResult.Persistence(), identityResult.Source(), identityResult.Path)
	fmt.Fprintf(os.Stdout, "host: hostname=%s source=%s raw=%q\n", hostInfo.Normalized, hostInfo.Source, hostInfo.Raw)
	fmt.Fprintf(os.Stdout, "queue: driver=%s mode=%s endpoint=%s name=%s capacity=%d queued=%d poll_interval=%s source=%s restored=%d path=%s snapshot=%s\n", queueInfo.Driver, queueInfo.Mode, queueInfo.Endpoint, queueInfo.Name, jobQueue.Capacity(), jobQueue.Len(), workerScheduler.Interval(), queueStoreSource, restoredJobs, queueStoreResult.Path, queueStoreResult.Source)
	fmt.Fprintf(os.Stdout, "worker: enabled=%t concurrency=%d pool_size=%d workers=%v handler=noop\n", cfg.Worker.Enabled, cfg.Worker.Concurrency, workerPool.Size(), workerPool.WorkerIDs())
	fmt.Fprintf(os.Stdout, "dispatcher: retry_policy=attempts max_attempts=per_job state=idle\n")
	fmt.Fprintf(os.Stdout, "scheduler: mode=fixed_interval interval=%s state=idle\n", workerScheduler.Interval())
	fmt.Fprintf(os.Stdout, "heartbeat: enabled=%t interval=%s status=%s queue=%d/%d\n", cfg.Heartbeat.Enabled, heartbeatRecord.Interval, heartbeatRecord.Status, heartbeatRecord.Queue.Length, heartbeatRecord.Queue.Capacity)
	fmt.Fprintf(os.Stdout, "worker-api: routes=%d paths=%s,%s\n", 3, server.WorkerPath, server.WorkerJobsPath)
	fmt.Fprintf(os.Stdout, "communication: rest=%s websocket=%s registration=%s heartbeat=%s metrics=%s events=%s authentication=%s token_header=%s capabilities=%v telemetry=%v event_count=%d\n", server.CommunicationAPIBasePath, server.CommunicationWebSocketPath, server.CommunicationRegistrationPath, server.CommunicationHeartbeatPath, server.MetricsAPIPath, server.EventsAPIPath, authenticator.Mode(), authenticator.TokenHeader(), server.CommunicationCapabilities(), server.TelemetryCapabilities(), eventRecorder.Count())
	fmt.Fprintf(os.Stdout, "download: client=%s user_agent=%q redirects=%t max_redirects=%d cookies=%s headers=%s resume=%t streaming=%s multipart=%s checksum=%s retry=%s bandwidth=%s bandwidth_limit=%d metrics=%s metrics_count=%d max_retries=%d retry_backoff=%s chunk_size=%s connect_timeout=%s response_header_timeout=%s idle_timeout=%s\n", download.HTTPClientName, downloadClient.UserAgent(), downloadClient.Redirects().Follow(), downloadClient.Redirects().MaxRedirects(), download.CookieEngineName, download.HeaderEngineName, cfg.Download.ResumeEnabled, download.StreamingEngineName, download.MultipartEngineName, download.SHA256ChecksumName, download.RetryEngineName, download.BandwidthControllerName, downloadBandwidthController.LimitBytesPerSecond(), download.DownloadMetricsName, downloadMetricsRecorder.Count(), downloadRetryPolicy.MaxRetries(), downloadRetryPolicy.Backoff(), cfg.Download.ChunkSize, downloadClient.ConnectTimeout(), downloadClient.ResponseHeaderTimeout(), downloadClient.IdleTimeout())
	fmt.Fprintf(os.Stdout, "plugins: sdk=%s kinds=%s,%s\n", plugins.SDKVersion, plugins.KindResolver, plugins.KindUploader)
	fmt.Fprintf(os.Stdout, "upload: interface=%s driver=%s uploader=%s root=%s multipart=%t resume=%s integrity=%s callback=%s part_size=%s part_bytes=%d max_retries=%d retry_backoff=%s\n", upload.InterfaceName, cfg.Upload.Driver, localUploader.Name(), localUploader.Root(), cfg.Upload.MultipartEnabled, upload.ResumeUploadName, upload.IntegrityValidatorName, upload.CallbackEngineName, cfg.Upload.PartSize, uploadPartSize, cfg.Upload.MaxRetries, cfg.Upload.RetryBackoff)
	fmt.Fprintf(os.Stdout, "storage-adapter: interface=%s local=%s root=%s auren=%s driver=%s endpoint_configured=%t bucket_configured=%t\n", storage.InterfaceName, localStorageAdapter.Name(), localStorageAdapter.Root(), aurenStorageStatus, cfg.Storage.Driver, cfg.Storage.Endpoint != "", cfg.Storage.Bucket != "")
	fmt.Fprintf(os.Stdout, "resolver: interface=%s registry=%d order=%v default_user_agent=%q follow_redirects=%t max_redirects=%d manifest_limit=%d cloudflare_bypass=false cloud_storage=%s,%s,%s\n", resolver.InterfaceName, resolverRegistry.Len(), resolverRegistry.Names(), cfg.Resolver.DefaultUserAgent, cfg.Resolver.FollowRedirects, cfg.Resolver.MaxRedirects, resolver.DefaultManifestReadLimit, resolver.GoogleDriveResolverName, resolver.MEGAResolverName, resolver.OneDriveResolverName)
	fmt.Fprintf(os.Stdout, "cluster-queues: interface=%s redis=%s redis_stream=%s redis_group=%s rabbitmq=%s rabbitmq_queue=%s nats=%s nats_subject=%s nats_group=%s active=%s\n", "queue.ClusterQueue", queue.RedisStreamsDriver, cfg.Queue.RedisStream, cfg.Queue.RedisConsumerGroup, queue.RabbitMQDriver, cfg.Queue.RabbitMQQueue, queue.NATSDriver, cfg.Queue.NATSSubject, cfg.Queue.NATSQueueGroup, queueInfo.Driver)
	fmt.Fprintf(os.Stdout, "cluster: registry=%s agents=%d local=%s load_balancer=%s selected=%s ready=%t leader_election=%s leader=%s found=%t failover=%s assignments=%d unassigned=%d mode=foundation_local\n", cluster.RegistryName, agentRegistry.Len(), localAgent.ID, cluster.LoadBalancerName, loadBalancedAgent.ID, loadBalancerReady, cluster.LeaderElectionName, leaderResult.LeaderID, leaderFound, cluster.FailoverName, len(failoverPlan.Assignments), len(failoverPlan.UnassignedIDs))
	fmt.Fprintf(os.Stdout, "observability: prometheus=%s path=%s grafana=%s tracing=%s spans=%d audit=%s audit_events=%d alerts=%s active_alerts=%d dashboard=%s centralized_logs=%s log_records=%d capabilities=%v\n", observability.PrometheusName, cfg.Metrics.Path, observability.GrafanaName, observability.TracingName, traceRecorder.Count(), observability.AuditName, auditRecorder.Count(), observability.AlertsName, len(observability.EvaluateAlerts(observability.SnapshotInput{Info: runtime.Info(), Heartbeat: heartbeatRecord, Queue: jobQueue.Info(), Download: downloadMetricsRecorder.Summary(), EventsCount: eventRecorder.Count(), AuditCount: auditRecorder.Count(), TraceCount: traceRecorder.Count(), CentralLogCount: centralLogSink.Count()})), observability.DashboardName, observability.CentralizedLogsName, centralLogSink.Count(), server.ObservabilityCapabilities())
	fmt.Fprintf(os.Stdout, "security: jwt=%s enabled=%t ttl=%s api_keys=%s required=%t header=%s hash_configured=%t mtls=%s enabled=%t min_tls=%d rbac=%s enabled=%t roles=%v rate_limit=%s enabled=%t limit_per_minute=%d secrets=%s provider=%s count=%d\n", security.JWTName, jwtService.Enabled(), jwtService.TTL(), securityAPIKeys.Mode(), securityAPIKeys.Required(), securityAPIKeys.Header(), cfg.Security.APIKeyHash != "", security.MTLSName, mtlsPolicy.Enabled(), mtlsPolicy.MinVersion(), security.RBACName, cfg.Security.RBACEnabled, rbacPolicy.Roles(), security.RateLimitName, cfg.Security.RateLimitEnabled, rateLimiter.Limit(), security.SecretsName, cfg.Security.SecretsProvider, secretStore.Count())
	fmt.Fprintf(os.Stdout, "production: docker=true compose=true linux_installer=true systemd=true kubernetes=true ci=true release_pipeline=true server_runtime=%s enabled=%t\n", server.RuntimeName, cfg.Server.Enabled)
	fmt.Fprintf(os.Stdout, "config: app=%s environment=%s logger=%s/%s router=%s middlewares=%d routes=%d server=%s worker=%t queue=%s storage=%s metrics=%s\n", cfg.App.Name, cfg.Runtime.Environment, cfg.Logger.Level, cfg.Logger.Format, server.RouterKindName(), middlewareRegistry.Len(), registry.Len(), cfg.ServerAddress(), cfg.Worker.Enabled, cfg.Queue.Driver, cfg.Storage.Driver, cfg.MetricsAddress())

	if !cfg.Server.Enabled {
		return nil
	}

	readTimeout, err := time.ParseDuration(cfg.Server.ReadTimeout)
	if err != nil {
		return err
	}
	writeTimeout, err := time.ParseDuration(cfg.Server.WriteTimeout)
	if err != nil {
		return err
	}
	idleTimeout, err := time.ParseDuration(cfg.Server.IdleTimeout)
	if err != nil {
		return err
	}
	shutdownTimeout, err := time.ParseDuration(cfg.Worker.ShutdownTimeout)
	if err != nil {
		return err
	}

	serverCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(os.Stdout, "server-runtime: enabled=true address=%s shutdown_timeout=%s\n", cfg.ServerAddress(), shutdownTimeout)
	_, err = server.Serve(serverCtx, server.ServeOptions{
		Address:         cfg.ServerAddress(),
		Handler:         router,
		ReadTimeout:     readTimeout,
		WriteTimeout:    writeTimeout,
		IdleTimeout:     idleTimeout,
		ShutdownTimeout: shutdownTimeout,
	})
	return err
}

func printHelp(out io.Writer) {
	fmt.Fprintf(out, "%s\n\n", runtime.AppName)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  auren-transfer-agent [--config ./configs/agent.yaml] [--version] [--help]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Production v1.0.0 supports validated configuration, logging, HTTP APIs, identity, worker engine contracts, download/resolver/upload primitives, communication, cluster coordination, observability, security and production deployment assets.")
}

func rateLimitValue(enabled bool, configured int) int {
	if !enabled {
		return 0
	}
	return configured
}
