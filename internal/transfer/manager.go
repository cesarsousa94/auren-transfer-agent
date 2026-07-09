package transfer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/auren/auren-transfer-agent/internal/config"
	"github.com/auren/auren-transfer-agent/internal/mediahub"
	"github.com/auren/auren-transfer-agent/internal/ops"
)

// ManagerOptions configures the Media Hub pull worker loop.
type ManagerOptions struct {
	Config       config.MediaHubConfig
	Client       *mediahub.Client
	NodeState    func() mediahub.NodeState
	Executor     *Executor
	Capabilities []string
	Operations   *ops.Runtime
	GatewayStats func() ops.ClaimSnapshot
	Now          func() time.Time
}

// Manager claims transfer jobs from Media Hub and executes them with bounded concurrency.
type Manager struct {
	cfg          config.MediaHubConfig
	client       *mediahub.Client
	nodeState    func() mediahub.NodeState
	executor     *Executor
	capabilities []string
	operations   *ops.Runtime
	gatewayStats func() ops.ClaimSnapshot
	now          func() time.Time
}

// NewManager creates a transfer claim manager.
func NewManager(options ManagerOptions) (*Manager, error) {
	if options.Client == nil {
		return nil, fmt.Errorf("transfer manager media hub client cannot be nil")
	}
	if options.NodeState == nil {
		return nil, fmt.Errorf("transfer manager node state callback cannot be nil")
	}
	if options.Executor == nil {
		return nil, fmt.Errorf("transfer manager executor cannot be nil")
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Manager{cfg: options.Config, client: options.Client, nodeState: options.NodeState, executor: options.Executor, capabilities: normalizedList(options.Capabilities), operations: options.Operations, gatewayStats: options.GatewayStats, now: now}, nil
}

// Start runs the claim loop until ctx is cancelled.
func (manager *Manager) Start(ctx context.Context) {
	if manager == nil || !manager.cfg.TransferEnabled || !manager.cfg.ClaimEnabled {
		return
	}
	go manager.loop(ctx)
}

func (manager *Manager) loop(ctx context.Context) {
	interval := parseDurationOr(manager.cfg.ClaimInterval, 2*time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			manager.claimOnce(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ClaimOnce claims and starts at most one job.
func (manager *Manager) ClaimOnce(ctx context.Context) error {
	return manager.claimOnce(ctx)
}

func (manager *Manager) claimOnce(ctx context.Context) error {
	state := manager.nodeState()
	if state.Empty() {
		return fmt.Errorf("media hub node state is not initialized")
	}
	stats := manager.executor.Stats()
	if stats.ActiveJobs >= stats.MaxConcurrentJobs {
		return nil
	}
	snapshot := ops.ClaimSnapshot{ActiveJobs: stats.ActiveJobs, MaxConcurrentJobs: stats.MaxConcurrentJobs, WorkDir: manager.cfg.WorkDir}
	if manager.gatewayStats != nil {
		gw := manager.gatewayStats()
		snapshot.ActiveSessions = gw.ActiveSessions
		snapshot.MaxSessions = gw.MaxSessions
		snapshot.CurrentEgressMbps = gw.CurrentEgressMbps
		snapshot.MaxEgressMbps = gw.MaxEgressMbps
	}
	decision := ops.Decision{Allowed: true, Reason: ops.DecisionAllowed}
	if manager.operations != nil {
		decision = manager.operations.CanClaim(snapshot)
		if !decision.Allowed {
			return nil
		}
	}
	request := mediahub.ClaimRequest{Capabilities: manager.capabilities, Capacity: mediahub.Capacity{MaxConcurrentJobs: stats.MaxConcurrentJobs, ActiveJobs: stats.ActiveJobs, MaxSessions: snapshot.MaxSessions, ActiveSessions: snapshot.ActiveSessions, MaxEgressMbps: snapshot.MaxEgressMbps, CurrentEgressMbps: snapshot.CurrentEgressMbps}, AcceptedOperations: normalizedCSV(manager.cfg.AcceptedOperations), Region: manager.cfg.Region, Metadata: map[string]any{"agent_claimer": "transfer_executor_v1", "hardening_decision": decision.Reason, "claimed_at": manager.now().Format(time.RFC3339Nano)}}
	response, err := manager.client.ClaimTransferJob(ctx, state, request)
	if err != nil {
		return err
	}
	if !response.Success || strings.TrimSpace(response.Job.UUID) == "" {
		return nil
	}
	go func(job mediahub.TransferJob) {
		_ = manager.executor.Execute(context.Background(), job)
	}(response.Job)
	return nil
}

func normalizedCSV(value string) []string {
	return normalizedList(strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t' }))
}

func normalizedList(values []string) []string {
	seen := map[string]struct{}{}
	output := make([]string, 0, len(values))
	for _, value := range values {
		item := strings.ToLower(strings.TrimSpace(value))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		output = append(output, item)
	}
	return output
}

func parseDurationOr(value string, fallback time.Duration) time.Duration {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}
