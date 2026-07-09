package transfer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/auren/auren-transfer-agent/internal/mediahub"
	"github.com/auren/auren-transfer-agent/internal/storage"
	"github.com/auren/auren-transfer-agent/internal/worker"
)

// Handler executes legacy/foundation worker jobs through the real transfer executor.
type Handler struct {
	executor *Executor
}

// NewHandler creates a worker.Handler backed by the v1.2 transfer executor.
func NewHandler(executor *Executor) (*Handler, error) {
	if executor == nil {
		return nil, fmt.Errorf("transfer handler executor cannot be nil")
	}
	return &Handler{executor: executor}, nil
}

// HandleJob converts a foundation worker.Job into the rich transfer job contract and executes it.
func (handler *Handler) HandleJob(ctx context.Context, job worker.Job) (worker.HandlerResult, error) {
	if handler == nil || handler.executor == nil {
		return worker.HandlerResult{}, fmt.Errorf("transfer handler cannot be nil")
	}
	transferJob := mediahub.TransferJob{UUID: job.ID, Operation: firstNonEmpty(job.ExternalID, "remote_download"), Source: mediahub.TransferSource{URL: job.SourceURL, Headers: job.Headers, ResumeEnabled: true, BlockHTML: true}, Destination: mediahub.TransferDestination{Driver: storage.DriverLocal, ObjectPath: job.DestinationKey, Metadata: job.Metadata}, Control: mediahub.TransferControl{ProgressEventSeconds: 5, MaxAttempts: job.MaxAttempts}, Metadata: map[string]any{"foundation_worker_job": true}}
	if strings.TrimSpace(transferJob.Operation) == "" {
		transferJob.Operation = "remote_download"
	}
	if err := handler.executor.Execute(ctx, transferJob); err != nil {
		return worker.HandlerResult{Status: worker.JobStatusFailed, Message: err.Error(), Now: time.Now().UTC()}, err
	}
	return worker.HandlerResult{Status: worker.JobStatusSucceeded, Message: "transfer completed", Now: time.Now().UTC()}, nil
}
