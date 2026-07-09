package transfer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
	"github.com/cesarsousa94/auren-transfer-agent/internal/download"
	"github.com/cesarsousa94/auren-transfer-agent/internal/mediahub"
	"github.com/cesarsousa94/auren-transfer-agent/internal/ops"
	"github.com/cesarsousa94/auren-transfer-agent/internal/resolver"
	"github.com/cesarsousa94/auren-transfer-agent/internal/storage"
	"github.com/cesarsousa94/auren-transfer-agent/internal/upload"
)

// ExecutorOptions wires the real transfer executor.
type ExecutorOptions struct {
	Config          config.Config
	Client          *mediahub.Client
	NodeState       func() mediahub.NodeState
	HTTPClient      *download.HTTPClient
	Resolver        *resolver.Registry
	LocalAdapter    *storage.LocalAdapter
	StateStore      *StateStore
	DownloadMetrics download.MetricsRecorder
	Tracker         *Tracker
	Operations      *ops.Runtime
	Now             func() time.Time
}

// Executor executes one Media Hub transfer job end-to-end.
type Executor struct {
	cfg             config.Config
	client          *mediahub.Client
	nodeState       func() mediahub.NodeState
	httpClient      *download.HTTPClient
	resolver        *resolver.Registry
	localAdapter    *storage.LocalAdapter
	stateStore      *StateStore
	downloadMetrics download.MetricsRecorder
	tracker         *Tracker
	operations      *ops.Runtime
	now             func() time.Time
}

// NewExecutor creates a validated real transfer executor.
func NewExecutor(options ExecutorOptions) (*Executor, error) {
	if options.HTTPClient == nil {
		return nil, fmt.Errorf("transfer executor download client cannot be nil")
	}
	if options.LocalAdapter == nil {
		return nil, fmt.Errorf("transfer executor local adapter cannot be nil")
	}
	if options.StateStore == nil {
		return nil, fmt.Errorf("transfer executor state store cannot be nil")
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	tracker := options.Tracker
	if tracker == nil {
		tracker = NewTracker(options.Config.MediaHub.MaxConcurrentJobs)
	}
	return &Executor{cfg: options.Config, client: options.Client, nodeState: options.NodeState, httpClient: options.HTTPClient, resolver: options.Resolver, localAdapter: options.LocalAdapter, stateStore: options.StateStore, downloadMetrics: options.DownloadMetrics, tracker: tracker, operations: options.Operations, now: now}, nil
}

// Stats returns current executor workload counters.
func (executor *Executor) Stats() Stats {
	if executor == nil || executor.tracker == nil {
		return Stats{}
	}
	return executor.tracker.Snapshot()
}

// Execute runs one transfer job and sends Media Hub callbacks when a client/state is configured.
func (executor *Executor) Execute(ctx context.Context, job mediahub.TransferJob) error {
	if executor == nil {
		return fmt.Errorf("transfer executor cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := mediahub.ValidateTransferJob(job); err != nil {
		return err
	}
	if decision := executor.admissionDecision(); !decision.Allowed {
		_ = executor.release(ctx, job, decision.Reason, decision.Message)
		return fmt.Errorf("transfer job rejected by hardening policy: %s", firstNonEmpty(decision.Message, decision.Reason))
	}
	if !executor.tracker.TryStart(job.UUID) {
		return fmt.Errorf("transfer executor capacity exhausted")
	}
	success := false
	defer func() { executor.tracker.Finish(job.UUID, success) }()

	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	controlErr := executor.startControlWatcher(execCtx, job, cancel)
	state := JobState{UUID: job.UUID, Operation: job.Operation, Status: StateStatusClaimed, Stage: "claim", SourceURL: job.Source.URL, ObjectPath: objectPath(job.Destination), Job: job, StartedAt: executor.now(), UpdatedAt: executor.now()}
	_ = executor.stateStore.Save(state)
	leaseStop := executor.startLeaseRenewal(execCtx, job, &state)
	defer close(leaseStop)
	if err := executor.started(execCtx, job, "Transferência iniciada pelo Agent."); err != nil {
		// Callback failures should not block the mechanical transfer once a lease was granted.
		_ = err
	}

	downloadStarted := executor.now()
	downloadResult, err := executor.download(execCtx, job, &state)
	if err != nil {
		err = preferControlError(err, controlErr)
		state.Status = StateStatusFailed
		state.LastError = err.Error()
		_ = executor.stateStore.Save(state)
		executor.deadLetter(execCtx, job, "download", err, true, map[string]any{"temp_path": state.TempPath})
		_ = executor.failed(context.Background(), job, "download", err, true, map[string]any{"temp_path": state.TempPath})
		return err
	}
	uploadStarted := executor.now()
	uploadResult, err := executor.upload(execCtx, job, downloadResult)
	if err != nil {
		err = preferControlError(err, controlErr)
		state.Status = StateStatusFailed
		state.Stage = "upload"
		state.LastError = err.Error()
		_ = executor.stateStore.Save(state)
		executor.deadLetter(execCtx, job, "upload", err, true, map[string]any{"temp_path": downloadResult.Path})
		_ = executor.failed(context.Background(), job, "upload", err, true, map[string]any{"temp_path": downloadResult.Path})
		return err
	}
	state.Status = StateStatusCompleted
	state.Stage = "completed"
	state.CurrentBytes = downloadResult.Bytes
	state.TotalBytes = downloadResult.Bytes
	state.ChecksumSHA256 = downloadResult.ChecksumSHA256
	state.CompletedAt = executor.now()
	_ = executor.stateStore.Save(state)

	completedPayload := mediahub.TransferCompletedPayload{Status: "completed", Stage: "upload", Bytes: downloadResult.Bytes, ChecksumSHA256: downloadResult.ChecksumSHA256, Destination: uploadResult, Timing: map[string]float64{"download_seconds": uploadStarted.Sub(downloadStarted).Seconds(), "upload_seconds": executor.now().Sub(uploadStarted).Seconds(), "total_seconds": executor.now().Sub(downloadStarted).Seconds()}, Metrics: map[string]any{"effective_url": downloadResult.EffectiveURL, "http_status": downloadResult.StatusCode, "content_type": downloadResult.ContentType}, GeneratedAt: executor.now()}
	if err := executor.completed(execCtx, job, completedPayload); err != nil {
		return err
	}
	success = true
	return nil
}

func (executor *Executor) admissionDecision() ops.Decision {
	if executor == nil || executor.operations == nil {
		return ops.Decision{Allowed: true, Reason: ops.DecisionAllowed}
	}
	stats := executor.Stats()
	return executor.operations.CanClaim(ops.ClaimSnapshot{ActiveJobs: stats.ActiveJobs, MaxConcurrentJobs: stats.MaxConcurrentJobs, WorkDir: executor.cfg.MediaHub.WorkDir})
}

func (executor *Executor) startLeaseRenewal(ctx context.Context, job mediahub.TransferJob, state *JobState) chan struct{} {
	stop := make(chan struct{})
	if executor == nil || !executor.cfg.MediaHub.LeaseRenewalEnabled || !executor.hasCallbacks() {
		return stop
	}
	interval := leaseRenewalInterval(job, executor.cfg.MediaHub.LeaseRenewalInterval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				current, total, stage := int64(0), int64(0), "lease"
				if state != nil {
					current = state.CurrentBytes
					total = state.TotalBytes
					stage = firstNonEmpty(state.Stage, "lease")
				}
				_ = executor.progress(context.Background(), job, stage, current, total, 0, progressPercent(current, total), "Lease renovado pelo Agent.", map[string]any{"lease_renewal": true})
			}
		}
	}()
	return stop
}

func (executor *Executor) startControlWatcher(ctx context.Context, job mediahub.TransferJob, cancel context.CancelFunc) <-chan error {
	controlErr := make(chan error, 1)
	if executor == nil || !executor.hasCallbacks() || strings.TrimSpace(executor.cfg.MediaHub.ControlInterval) == "" {
		return controlErr
	}
	interval := parseDurationOr(executor.cfg.MediaHub.ControlInterval, 10*time.Second)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				control, err := executor.client.FetchTransferControl(ctx, executor.nodeState(), job.UUID)
				if err != nil {
					continue
				}
				action := strings.ToLower(strings.TrimSpace(control.Action))
				switch action {
				case "cancel", "cancelled", "canceled", "pause", "release", "drain":
					err := fmt.Errorf("transfer control requested %s: %s", action, firstNonEmpty(control.Reason, "operator request"))
					select {
					case controlErr <- err:
					default:
					}
					cancel()
					return
				}
			}
		}
	}()
	return controlErr
}

func (executor *Executor) deadLetter(ctx context.Context, job mediahub.TransferJob, stage string, err error, retryable bool, payload any) {
	if executor == nil || executor.operations == nil || err == nil {
		return
	}
	_, _ = executor.operations.StoreDeadLetter(ctx, ops.DeadLetter{JobUUID: job.UUID, Operation: job.Operation, Stage: stage, Error: err.Error(), Retryable: retryable, Payload: payload, CreatedAt: executor.now()})
}

func (executor *Executor) release(ctx context.Context, job mediahub.TransferJob, reason string, message string) error {
	if !executor.hasCallbacks() {
		return nil
	}
	return executor.client.ReleaseTransferJob(ctx, executor.nodeState(), job.UUID, map[string]any{"reason": reason, "message": message, "generated_at": executor.now()})
}

func preferControlError(err error, controlErr <-chan error) error {
	if err == nil {
		return nil
	}
	select {
	case control := <-controlErr:
		if control != nil {
			return control
		}
	default:
	}
	return err
}

func leaseRenewalInterval(job mediahub.TransferJob, fallback string) time.Duration {
	if job.Control.HeartbeatSeconds > 0 {
		d := time.Duration(job.Control.HeartbeatSeconds) * time.Second / 2
		if d > 0 {
			return d
		}
	}
	return parseDurationOr(fallback, 30*time.Second)
}

func progressPercent(current int64, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return (float64(current) / float64(total)) * 100
}

type downloadResult struct {
	Path           string
	EffectiveURL   string
	StatusCode     int
	ContentType    string
	Bytes          int64
	TotalBytes     int64
	ChecksumSHA256 string
}

func (executor *Executor) download(ctx context.Context, job mediahub.TransferJob, state *JobState) (downloadResult, error) {
	resolvedURL := strings.TrimSpace(job.Source.URL)
	if executor.resolver != nil {
		metadata := map[string]string{}
		if strings.TrimSpace(job.Source.ResolverStrategy) != "" {
			metadata["resolver"] = job.Source.ResolverStrategy
		}
		resolved, err := executor.resolver.Resolve(ctx, resolver.Request{URL: job.Source.URL, Method: http.MethodHead, Headers: job.Source.Headers, Metadata: metadata})
		if err == nil && strings.TrimSpace(resolved.ResolvedURL) != "" {
			resolvedURL = resolved.ResolvedURL
		}
	}

	jobDir := filepath.Join(executor.cfg.MediaHub.WorkDir, "downloads", safeName(job.UUID))
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return downloadResult{}, err
	}
	partialPath := filepath.Join(jobDir, "payload.part")
	finalPath := filepath.Join(jobDir, "payload.bin")
	state.Stage = "download"
	state.Status = StateStatusRunning
	state.TempPath = partialPath
	_ = executor.stateStore.Save(*state)

	resume, err := download.ResumeFromFile(partialPath, job.Source.ResumeEnabled || executor.cfg.Download.ResumeEnabled, 0)
	if err != nil {
		return downloadResult{}, err
	}
	headers, err := download.NewHeaderSet(job.Source.Headers)
	if err != nil {
		return downloadResult{}, err
	}
	request, err := download.NewRequest(ctx, download.RequestOptions{Method: http.MethodGet, URL: resolvedURL, Headers: headers})
	if err != nil {
		return downloadResult{}, err
	}
	if err := download.ApplyResume(request, resume); err != nil {
		return downloadResult{}, err
	}

	started := executor.now()
	response, err := executor.httpClient.Do(request)
	if err != nil {
		return downloadResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return downloadResult{}, fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}
	if resume.RangeHeader != "" && response.StatusCode != http.StatusPartialContent {
		return downloadResult{}, fmt.Errorf("resume requested but server returned HTTP %d", response.StatusCode)
	}
	contentType := response.Header.Get("Content-Type")
	if shouldBlockHTML(job, executor.cfg.MediaHub.BlockHTML, contentType) {
		return downloadResult{}, fmt.Errorf("download returned blocked HTML content type %q", contentType)
	}
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if resume.RangeHeader != "" {
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	file, err := os.OpenFile(partialPath, flag, 0o644)
	if err != nil {
		return downloadResult{}, err
	}
	writer := &progressWriter{writer: file, current: resume.ExistingBytes, total: contentTotal(response.ContentLength, resume.ExistingBytes), started: started, interval: progressInterval(job, executor.cfg.MediaHub.ProgressInterval), callback: func(current int64, total int64, speed int64, percent float64) {
		state.CurrentBytes = current
		state.TotalBytes = total
		_ = executor.stateStore.Save(*state)
		_ = executor.progress(ctx, job, "download", current, total, speed, percent, "Download em andamento.", map[string]any{"http_status": response.StatusCode, "content_type": contentType})
	}}
	written, copyErr := io.Copy(writer, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return downloadResult{}, copyErr
	}
	if closeErr != nil {
		return downloadResult{}, closeErr
	}
	bytesOnDisk := resume.ExistingBytes + written
	minBytes := minBytes(job, executor.cfg.MediaHub.MinBytes)
	if minBytes > 0 && bytesOnDisk < minBytes {
		return downloadResult{}, fmt.Errorf("downloaded file is too small: got %d bytes, minimum %d", bytesOnDisk, minBytes)
	}
	if err := os.Rename(partialPath, finalPath); err != nil {
		return downloadResult{}, err
	}
	checksum, err := download.SHA256File(finalPath)
	if err != nil {
		return downloadResult{}, err
	}
	if strings.TrimSpace(job.Source.ExpectedSHA256) != "" && !strings.EqualFold(checksum.Hex, strings.TrimSpace(job.Source.ExpectedSHA256)) {
		return downloadResult{}, fmt.Errorf("sha256 mismatch: expected %s got %s", job.Source.ExpectedSHA256, checksum.Hex)
	}
	if executor.downloadMetrics != nil {
		_ = executor.downloadMetrics.RecordDownloadMetric(ctx, download.DownloadMetric{Engine: "transfer_executor", URL: resolvedURL, StatusCode: response.StatusCode, BytesWritten: bytesOnDisk, ContentLength: response.ContentLength, Duration: executor.now().Sub(started), StartedAt: started, CompletedAt: executor.now(), Resumed: resume.RangeHeader != "", RangeHeader: resume.RangeHeader, ChecksumAlgorithm: download.SHA256ChecksumName})
	}
	state.CurrentBytes = bytesOnDisk
	state.TotalBytes = bytesOnDisk
	state.ChecksumSHA256 = checksum.Hex
	_ = executor.stateStore.Save(*state)
	return downloadResult{Path: finalPath, EffectiveURL: effectiveURL(response, resolvedURL), StatusCode: response.StatusCode, ContentType: contentType, Bytes: bytesOnDisk, TotalBytes: bytesOnDisk, ChecksumSHA256: checksum.Hex}, nil
}

func (executor *Executor) upload(ctx context.Context, job mediahub.TransferJob, downloadResult downloadResult) (mediahub.TransferObjectResult, error) {
	_ = executor.progress(ctx, job, "upload", 0, 0, 0, 0, "Upload iniciado.", nil)
	objectPath := objectPath(job.Destination)
	driver := strings.ToLower(strings.TrimSpace(job.Destination.Driver))
	if driver == "" || driver == storage.DriverLocal {
		result, err := executor.localAdapter.Upload(ctx, executor.storageUploadRequest(job, downloadResult, objectPath))
		if err != nil {
			return mediahub.TransferObjectResult{}, err
		}
		_ = executor.progress(ctx, job, "upload", result.BytesWritten, result.BytesWritten, 0, 100, "Upload local concluído.", nil)
		return mediahub.TransferObjectResult{Driver: result.Driver, BucketUUID: result.BucketUUID, Bucket: result.Bucket, ObjectUUID: result.ObjectUUID, Path: result.ObjectPath, URL: result.Location, Size: result.BytesWritten, ChecksumSHA256: result.ChecksumSHA256, Visibility: result.Visibility, MimeType: result.MimeType, Multipart: result.Multipart, Metadata: result.Metadata}, nil
	}
	if driver == storage.DriverAurenStorage {
		if strings.TrimSpace(job.Destination.UploadURL) != "" {
			result, err := executor.uploadToSignedURL(ctx, job, downloadResult, objectPath)
			if err != nil {
				return mediahub.TransferObjectResult{}, err
			}
			return result, nil
		}
		endpoint := firstNonEmpty(job.Destination.Endpoint, executor.cfg.Storage.Endpoint)
		bucket := firstNonEmpty(job.Destination.BucketUUID, job.Destination.Bucket, executor.cfg.Storage.Bucket)
		adapter, err := storage.NewAurenStorageAdapter(storage.AurenOptions{Endpoint: endpoint, Bucket: bucket, Region: executor.cfg.Storage.Region, TokenHeader: firstNonEmpty(job.Destination.TokenHeader, executor.cfg.Security.TokenHeader), APIKey: firstNonEmpty(job.Destination.Token, executor.cfg.Storage.APIKey), HTTPClient: executor.httpClient.StandardClient(), MultipartEnabled: executor.cfg.Upload.MultipartEnabled, PartSize: uploadPartSizeBytes(executor.cfg.Upload.PartSize)})
		if err != nil {
			return mediahub.TransferObjectResult{}, err
		}
		result, err := adapter.Upload(ctx, executor.storageUploadRequest(job, downloadResult, objectPath))
		if err != nil {
			return mediahub.TransferObjectResult{}, err
		}
		_ = executor.progress(ctx, job, "upload", result.BytesWritten, result.BytesWritten, 0, 100, "Upload para Auren Storage concluído.", nil)
		return mediahub.TransferObjectResult{Driver: result.Driver, BucketUUID: firstNonEmpty(result.BucketUUID, job.Destination.BucketUUID), Bucket: firstNonEmpty(result.Bucket, bucket), ObjectUUID: result.ObjectUUID, Path: result.ObjectPath, URL: result.Location, Size: result.BytesWritten, ChecksumSHA256: result.ChecksumSHA256, Visibility: result.Visibility, MimeType: result.MimeType, Multipart: result.Multipart, Metadata: result.Metadata}, nil
	}
	return mediahub.TransferObjectResult{}, fmt.Errorf("unsupported destination driver %q", job.Destination.Driver)
}

func (executor *Executor) storageUploadRequest(job mediahub.TransferJob, downloadResult downloadResult, objectPath string) storage.UploadRequest {
	return storage.UploadRequest{
		SourcePath:        downloadResult.Path,
		ObjectPath:        objectPath,
		BucketUUID:        job.Destination.BucketUUID,
		Bucket:            job.Destination.Bucket,
		DirectoryPath:     job.Destination.DirectoryPath,
		RelativePath:      job.Destination.RelativePath,
		Visibility:        job.Destination.Visibility,
		MimeType:          firstNonEmpty(job.Destination.MimeType, downloadResult.ContentType, "application/octet-stream"),
		ChecksumAlgorithm: firstNonEmpty(job.Destination.ChecksumAlgorithm, "sha256"),
		ChecksumSHA256:    downloadResult.ChecksumSHA256,
		Metadata:          job.Destination.Metadata,
		Progress: func(callbackCtx context.Context, progress storage.UploadProgress) error {
			metrics := progress.Metrics
			if metrics == nil {
				metrics = map[string]any{}
			}
			metrics["checksum_sha256"] = downloadResult.ChecksumSHA256
			return executor.progress(callbackCtx, job, firstNonEmpty(progress.Stage, "upload"), progress.CurrentBytes, progress.TotalBytes, progress.SpeedBps, progress.Percent, firstNonEmpty(progress.Message, "Upload em andamento."), metrics)
		},
	}
}

func uploadPartSizeBytes(value string) int64 {
	parsed, err := upload.ParsePartSize(value)
	if err != nil || parsed <= 0 {
		return 16 * 1024 * 1024
	}
	return parsed
}

func (executor *Executor) uploadToSignedURL(ctx context.Context, job mediahub.TransferJob, downloadResult downloadResult, objectPath string) (mediahub.TransferObjectResult, error) {
	file, err := os.Open(downloadResult.Path)
	if err != nil {
		return mediahub.TransferObjectResult{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return mediahub.TransferObjectResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, job.Destination.UploadURL, file)
	if err != nil {
		return mediahub.TransferObjectResult{}, err
	}
	req.ContentLength = info.Size()
	if strings.TrimSpace(job.Destination.MimeType) != "" {
		req.Header.Set("Content-Type", job.Destination.MimeType)
	} else {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	if strings.TrimSpace(job.Destination.Token) != "" {
		header := firstNonEmpty(job.Destination.TokenHeader, executor.cfg.Security.TokenHeader)
		value := job.Destination.Token
		if strings.EqualFold(header, "Authorization") && !strings.HasPrefix(strings.ToLower(value), "bearer ") {
			value = "Bearer " + value
		}
		req.Header.Set(header, value)
	}
	resp, err := executor.httpClient.StandardClient().Do(req)
	if err != nil {
		return mediahub.TransferObjectResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mediahub.TransferObjectResult{}, fmt.Errorf("signed upload returned HTTP %d", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	if location == "" {
		location = job.Destination.UploadURL
	}
	_ = executor.progress(ctx, job, "upload", info.Size(), info.Size(), 0, 100, "Upload assinado concluído.", nil)
	return mediahub.TransferObjectResult{Driver: storage.DriverAurenStorage, BucketUUID: job.Destination.BucketUUID, Bucket: firstNonEmpty(job.Destination.Bucket, job.Destination.BucketUUID), Path: objectPath, URL: location, Size: info.Size(), ChecksumSHA256: downloadResult.ChecksumSHA256, Visibility: job.Destination.Visibility, MimeType: firstNonEmpty(job.Destination.MimeType, "application/octet-stream"), Metadata: job.Destination.Metadata}, nil
}

func (executor *Executor) started(ctx context.Context, job mediahub.TransferJob, message string) error {
	if !executor.hasCallbacks() {
		return nil
	}
	return executor.client.SendTransferStarted(ctx, executor.nodeState(), job.UUID, mediahub.TransferProgressPayload{Status: "running", Stage: "started", Message: message, GeneratedAt: executor.now()})
}

func (executor *Executor) progress(ctx context.Context, job mediahub.TransferJob, stage string, current int64, total int64, speed int64, percent float64, message string, metrics map[string]any) error {
	if !executor.hasCallbacks() {
		return nil
	}
	return executor.client.SendTransferProgress(ctx, executor.nodeState(), job.UUID, mediahub.TransferProgressPayload{Status: "running", Stage: stage, CurrentBytes: current, TotalBytes: total, SpeedBps: speed, Percent: percent, Message: message, Metrics: metrics, GeneratedAt: executor.now()})
}

func (executor *Executor) completed(ctx context.Context, job mediahub.TransferJob, payload mediahub.TransferCompletedPayload) error {
	if !executor.hasCallbacks() {
		return nil
	}
	return executor.client.SendTransferCompleted(ctx, executor.nodeState(), job.UUID, payload)
}

func (executor *Executor) failed(ctx context.Context, job mediahub.TransferJob, stage string, err error, retryable bool, metrics map[string]any) error {
	if !executor.hasCallbacks() {
		return nil
	}
	return executor.client.SendTransferFailed(ctx, executor.nodeState(), job.UUID, mediahub.TransferFailedPayload{Status: "failed", Stage: stage, Message: "Transferência falhou no Agent.", Error: err.Error(), Retryable: retryable, Metrics: metrics, GeneratedAt: executor.now()})
}

func (executor *Executor) hasCallbacks() bool {
	return executor != nil && executor.client != nil && executor.nodeState != nil && !executor.nodeState().Empty()
}

type progressWriter struct {
	writer   io.Writer
	current  int64
	total    int64
	started  time.Time
	interval time.Duration
	last     atomic.Int64
	callback func(current int64, total int64, speed int64, percent float64)
}

func (writer *progressWriter) Write(p []byte) (int, error) {
	n, err := writer.writer.Write(p)
	if n > 0 {
		current := atomic.AddInt64(&writer.current, int64(n))
		now := time.Now().UnixNano()
		last := writer.last.Load()
		if last == 0 || time.Duration(now-last) >= writer.interval {
			if writer.last.CompareAndSwap(last, now) && writer.callback != nil {
				elapsed := time.Since(writer.started).Seconds()
				speed := int64(0)
				if elapsed > 0 {
					speed = int64(float64(current) / elapsed)
				}
				percent := float64(0)
				if writer.total > 0 {
					percent = (float64(current) / float64(writer.total)) * 100
				}
				writer.callback(current, writer.total, speed, percent)
			}
		}
	}
	return n, err
}

func objectPath(destination mediahub.TransferDestination) string {
	if strings.TrimSpace(destination.ObjectPath) != "" {
		return strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(destination.ObjectPath), "\\", "/"), "/")
	}
	dir := strings.Trim(strings.ReplaceAll(destination.DirectoryPath, "\\", "/"), "/")
	relative := strings.Trim(strings.ReplaceAll(destination.RelativePath, "\\", "/"), "/")
	if relative == "" {
		relative = "payload.bin"
	}
	if dir == "" {
		return relative
	}
	return path.Join(dir, relative)
}

func shouldBlockHTML(job mediahub.TransferJob, defaultBlock bool, contentType string) bool {
	block := defaultBlock || job.Source.BlockHTML
	if !block {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.ToLower(strings.TrimSpace(contentType))
	}
	return mediaType == "text/html" || mediaType == "application/xhtml+xml"
}

func minBytes(job mediahub.TransferJob, fallback int64) int64 {
	if job.Source.MinBytes > 0 {
		return job.Source.MinBytes
	}
	return fallback
}

func progressInterval(job mediahub.TransferJob, fallback string) time.Duration {
	if job.Control.ProgressEventSeconds > 0 {
		return time.Duration(job.Control.ProgressEventSeconds) * time.Second
	}
	duration, err := time.ParseDuration(strings.TrimSpace(fallback))
	if err != nil || duration <= 0 {
		return 5 * time.Second
	}
	return duration
}

func contentTotal(contentLength int64, existing int64) int64 {
	if contentLength <= 0 {
		return 0
	}
	return contentLength + existing
}

func effectiveURL(response *http.Response, fallback string) string {
	if response != nil && response.Request != nil && response.Request.URL != nil {
		return response.Request.URL.String()
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func randomHex(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func sanitizeURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return ""
	}
	parsed.User = nil
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "key") || strings.Contains(lower, "password") || strings.Contains(lower, "pass") {
			query.Set(key, "redacted")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
