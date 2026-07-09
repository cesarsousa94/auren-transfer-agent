package mediahub

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
	"github.com/cesarsousa94/auren-transfer-agent/internal/identity"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
)

// SnapshotFunc lazily builds payload fragments for periodic connector loops.
type SnapshotFunc func() any

// EventsFunc lazily builds node event batches.
type EventsFunc func() []EventPayload

// CapacityFunc lazily builds live node capacity overrides.
type CapacityFunc func() Capacity

// ConnectorOptions wires the Media Hub connector without importing bootstrap internals.
type ConnectorOptions struct {
	Config            config.MediaHubConfig
	Identity          identity.Snapshot
	Runtime           runtime.VersionInfo
	Store             StateStore
	Client            *Client
	HeartbeatSnapshot SnapshotFunc
	QueueSnapshot     SnapshotFunc
	DownloadSnapshot  SnapshotFunc
	EventsSnapshot    EventsFunc
	CapacitySnapshot  CapacityFunc
	Now               func() time.Time
}

// Connector manages registration, config pull and telemetry flushes.
type Connector struct {
	cfg               config.MediaHubConfig
	identity          identity.Snapshot
	runtime           runtime.VersionInfo
	store             StateStore
	client            *Client
	heartbeatSnapshot SnapshotFunc
	queueSnapshot     SnapshotFunc
	downloadSnapshot  SnapshotFunc
	eventsSnapshot    EventsFunc
	capacitySnapshot  CapacityFunc
	now               func() time.Time
	mu                sync.RWMutex
	state             NodeState
	lastConfig        ConfigResult
}

// NewConnector creates a Media Hub connector.
func NewConnector(options ConnectorOptions) (*Connector, error) {
	if options.Client == nil {
		return nil, fmt.Errorf("media hub connector client cannot be nil")
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Connector{cfg: options.Config, identity: options.Identity, runtime: options.Runtime, store: options.Store, client: options.Client, heartbeatSnapshot: options.HeartbeatSnapshot, queueSnapshot: options.QueueSnapshot, downloadSnapshot: options.DownloadSnapshot, eventsSnapshot: options.EventsSnapshot, capacitySnapshot: options.CapacitySnapshot, now: now}, nil
}

// State returns the current node state.
func (connector *Connector) State() NodeState {
	connector.mu.RLock()
	defer connector.mu.RUnlock()
	return connector.state.Clone()
}

// LastConfig returns the latest config pull result.
func (connector *Connector) LastConfig() ConfigResult {
	connector.mu.RLock()
	defer connector.mu.RUnlock()
	return ConfigResult{ConfigVersion: connector.lastConfig.ConfigVersion, Raw: connector.lastConfig.Raw}
}

// Bootstrap ensures credentials exist, pulls config once and emits initial telemetry.
func (connector *Connector) Bootstrap(ctx context.Context) (NodeState, error) {
	if connector == nil {
		return NodeState{}, fmt.Errorf("media hub connector cannot be nil")
	}
	state, _, err := connector.store.LoadOrEmpty()
	if err != nil {
		return NodeState{}, err
	}
	if state.Empty() && strings.TrimSpace(connector.cfg.NodeUUID) != "" && strings.TrimSpace(connector.cfg.NodeSecret) != "" {
		state, err = NewNodeState(connector.cfg.NodeUUID, connector.cfg.NodeSecret, "", connector.now())
		if err != nil {
			return NodeState{}, err
		}
		if err := connector.store.Save(state); err != nil {
			return NodeState{}, err
		}
	}
	if state.Empty() {
		registration, err := connector.client.Register(ctx, connector.registrationPayload())
		if err != nil {
			return NodeState{}, err
		}
		state, err = NewNodeState(registration.NodeUUID, registration.NodeSecret, registration.ConfigVersion, connector.now())
		if err != nil {
			return NodeState{}, err
		}
		if err := connector.store.Save(state); err != nil {
			return NodeState{}, err
		}
	}
	connector.setState(state)
	if err := connector.PullConfig(ctx); err != nil {
		return state, err
	}
	if err := connector.FlushHeartbeat(ctx); err != nil {
		return state, err
	}
	if err := connector.FlushMetrics(ctx); err != nil {
		return state, err
	}
	if err := connector.FlushEvents(ctx); err != nil {
		return state, err
	}
	return connector.State(), nil
}

// PullConfig fetches Media Hub node config and stores the config version.
func (connector *Connector) PullConfig(ctx context.Context) error {
	state := connector.State()
	if state.Empty() {
		return fmt.Errorf("media hub node state is not initialized")
	}
	result, err := connector.client.FetchConfig(ctx, state)
	if err != nil {
		return err
	}
	state.LastConfigAt = connector.now().UTC()
	if strings.TrimSpace(result.ConfigVersion) != "" {
		state.ConfigVersion = strings.TrimSpace(result.ConfigVersion)
	}
	state.UpdatedAt = connector.now().UTC()
	if err := connector.store.Save(state); err != nil {
		return err
	}
	connector.mu.Lock()
	connector.state = state
	connector.lastConfig = result
	connector.mu.Unlock()
	return nil
}

// FlushHeartbeat sends one heartbeat payload.
func (connector *Connector) FlushHeartbeat(ctx context.Context) error {
	return connector.client.SendHeartbeat(ctx, connector.State(), connector.heartbeatPayload())
}

// FlushMetrics sends one metrics payload.
func (connector *Connector) FlushMetrics(ctx context.Context) error {
	return connector.client.SendMetrics(ctx, connector.State(), connector.metricsPayload())
}

// FlushEvents sends one events payload. Empty batches are intentionally accepted.
func (connector *Connector) FlushEvents(ctx context.Context) error {
	return connector.client.SendEvents(ctx, connector.State(), connector.eventsPayload())
}

// Start runs periodic config/heartbeat/metrics/events loops until ctx is done.
func (connector *Connector) Start(ctx context.Context) {
	if connector == nil {
		return
	}
	go connector.loop(ctx, parseDurationOr(connector.cfg.PollInterval, 2*time.Second), connector.cfg.PollEnabled, connector.PullConfig)
	go connector.loop(ctx, parseDurationOr(connector.cfg.HeartbeatInterval, 30*time.Second), true, connector.FlushHeartbeat)
	go connector.loop(ctx, parseDurationOr(connector.cfg.MetricsInterval, 60*time.Second), true, connector.FlushMetrics)
	go connector.loop(ctx, parseDurationOr(connector.cfg.EventsFlushInterval, 10*time.Second), true, connector.FlushEvents)
}

func (connector *Connector) loop(ctx context.Context, interval time.Duration, enabled bool, fn func(context.Context) error) {
	if !enabled || fn == nil {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = fn(ctx)
		}
	}
}

func (connector *Connector) setState(state NodeState) {
	connector.mu.Lock()
	connector.state = state
	connector.mu.Unlock()
}

func (connector *Connector) registrationPayload() RegistrationPayload {
	return RegistrationPayload{
		RegistrationToken: strings.TrimSpace(connector.cfg.RegistrationToken),
		AgentID:           connector.identity.AgentID,
		Fingerprint:       connector.identity.Fingerprint,
		Hostname:          connector.identity.Hostname,
		HostnameSource:    connector.identity.HostnameSource,
		Version:           connector.runtime.Version,
		RuntimeStatus:     connector.runtime.Status,
		Role:              connector.cfg.Role,
		Provider:          connector.cfg.Provider,
		Region:            connector.cfg.Region,
		AvailabilityZone:  connector.cfg.AvailabilityZone,
		PublicBaseURL:     connector.cfg.PublicBaseURL,
		HealthURL:         connector.cfg.HealthURL,
		MaxSessions:       connector.cfg.MaxSessions,
		MaxEgressMbps:     connector.cfg.MaxEgressMbps,
		Capabilities:      normalizedCapabilities(connector.cfg.Capabilities),
		Metadata: map[string]any{
			"connector": "media_hub_v1",
			"agent_id":  connector.identity.AgentID,
		},
	}
}

func (connector *Connector) heartbeatPayload() HeartbeatPayload {
	state := connector.State()
	return HeartbeatPayload{
		NodeUUID:         state.NodeUUID,
		AgentID:          connector.identity.AgentID,
		Fingerprint:      connector.identity.Fingerprint,
		Hostname:         connector.identity.Hostname,
		Version:          connector.runtime.Version,
		RuntimeStatus:    connector.runtime.Status,
		Status:           "ready",
		Role:             connector.cfg.Role,
		Provider:         connector.cfg.Provider,
		Region:           connector.cfg.Region,
		AvailabilityZone: connector.cfg.AvailabilityZone,
		PublicBaseURL:    connector.cfg.PublicBaseURL,
		HealthURL:        connector.cfg.HealthURL,
		Capabilities:     normalizedCapabilities(connector.cfg.Capabilities),
		Capacity:         connector.capacity(),
		Heartbeat:        snapshot(connector.heartbeatSnapshot),
		GeneratedAt:      connector.now().UTC(),
		Metadata:         map[string]any{"connector": "media_hub_v1"},
	}
}

func (connector *Connector) metricsPayload() MetricsPayload {
	state := connector.State()
	return MetricsPayload{
		NodeUUID:     state.NodeUUID,
		Runtime:      connector.runtime,
		Heartbeat:    snapshot(connector.heartbeatSnapshot),
		Queue:        snapshot(connector.queueSnapshot),
		Download:     snapshot(connector.downloadSnapshot),
		Capacity:     connector.capacity(),
		Capabilities: normalizedCapabilities(connector.cfg.Capabilities),
		GeneratedAt:  connector.now().UTC(),
		Metadata:     map[string]any{"connector": "media_hub_v1"},
	}
}

func (connector *Connector) eventsPayload() EventsPayload {
	state := connector.State()
	events := []EventPayload(nil)
	if connector.eventsSnapshot != nil {
		events = connector.eventsSnapshot()
	}
	if events == nil {
		events = []EventPayload{}
	}
	return EventsPayload{NodeUUID: state.NodeUUID, Events: events, GeneratedAt: connector.now().UTC(), Metadata: map[string]any{"connector": "media_hub_v1"}}
}

func (connector *Connector) capacity() Capacity {
	capacity := Capacity{MaxSessions: connector.cfg.MaxSessions, ActiveSessions: 0, MaxConcurrentJobs: connector.cfg.MaxConcurrentJobs, ActiveJobs: 0, MaxEgressMbps: connector.cfg.MaxEgressMbps, CurrentEgressMbps: 0}
	if connector.capacitySnapshot != nil {
		override := connector.capacitySnapshot()
		if override.MaxSessions > 0 {
			capacity.MaxSessions = override.MaxSessions
		}
		capacity.ActiveSessions = override.ActiveSessions
		if override.MaxConcurrentJobs > 0 {
			capacity.MaxConcurrentJobs = override.MaxConcurrentJobs
		}
		capacity.ActiveJobs = override.ActiveJobs
		if override.MaxEgressMbps > 0 {
			capacity.MaxEgressMbps = override.MaxEgressMbps
		}
		capacity.CurrentEgressMbps = override.CurrentEgressMbps
	}
	return capacity
}

func snapshot(fn SnapshotFunc) any {
	if fn == nil {
		return map[string]any{}
	}
	return fn()
}

func parseDurationOr(value string, fallback time.Duration) time.Duration {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}

func normalizedCapabilities(values string) []string {
	parts := strings.FieldsFunc(values, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t' })
	seen := map[string]struct{}{}
	output := make([]string, 0, len(parts))
	for _, value := range parts {
		capability := strings.ToLower(strings.TrimSpace(value))
		if capability == "" {
			continue
		}
		if _, ok := seen[capability]; ok {
			continue
		}
		seen[capability] = struct{}{}
		output = append(output, capability)
	}
	return output
}
