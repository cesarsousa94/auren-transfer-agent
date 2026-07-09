// Package cluster contains foundation-only cluster coordination contracts.
package cluster

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/identity"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
	"github.com/cesarsousa94/auren-transfer-agent/internal/worker"
)

const (
	// RegistryName identifies the foundation agent registry component.
	RegistryName = "agent_registry"

	// LoadBalancerName identifies the foundation load-balancing component.
	LoadBalancerName = "load_balancer"

	// LeaderElectionName identifies the foundation leader-election component.
	LeaderElectionName = "leader_election"

	// FailoverName identifies the foundation failover planner component.
	FailoverName = "failover"

	// AgentStatusAvailable means the agent can receive mechanical work.
	AgentStatusAvailable AgentStatus = "available"

	// AgentStatusDraining means the agent is alive but should not receive new work.
	AgentStatusDraining AgentStatus = "draining"

	// AgentStatusUnhealthy means the agent should be skipped by cluster planners.
	AgentStatusUnhealthy AgentStatus = "unhealthy"
)

// AgentStatus represents the foundation cluster status for an Agent.
type AgentStatus string

// Agent is a safe local snapshot of a Transfer Agent known by the registry.
type Agent struct {
	ID          string            `json:"id"`
	Fingerprint string            `json:"fingerprint"`
	Hostname    string            `json:"hostname"`
	Version     string            `json:"version"`
	Status      AgentStatus       `json:"status"`
	Capacity    int               `json:"capacity"`
	ActiveJobs  int               `json:"active_jobs"`
	LastSeenAt  time.Time         `json:"last_seen_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Registry is an in-memory foundation registry for Agent snapshots.
type Registry struct {
	mu     sync.Mutex
	agents map[string]Agent
}

// NewRegistry creates a registry and optionally registers initial agents.
func NewRegistry(agents ...Agent) (*Registry, error) {
	registry := &Registry{agents: make(map[string]Agent)}
	for _, agent := range agents {
		if err := registry.Register(agent); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// LocalAgent builds the local registry record from identity and runtime data.
func LocalAgent(snapshot identity.Snapshot, info runtime.VersionInfo, capacity int, now time.Time) (Agent, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	agent := Agent{
		ID:          snapshot.AgentID,
		Fingerprint: snapshot.Fingerprint,
		Hostname:    snapshot.Hostname,
		Version:     info.Version,
		Status:      AgentStatusAvailable,
		Capacity:    capacity,
		LastSeenAt:  now.UTC(),
		Metadata: map[string]string{
			"runtime_status": info.Status,
			"leader_source":  "local_registry",
		},
	}
	if err := ValidateAgent(agent); err != nil {
		return Agent{}, err
	}
	return agent, nil
}

// Register stores or replaces an agent snapshot by ID.
func (registry *Registry) Register(agent Agent) error {
	if registry == nil {
		return fmt.Errorf("registry cannot be nil")
	}
	if err := ValidateAgent(agent); err != nil {
		return err
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.agents[agent.ID] = agent.Clone()
	return nil
}

// UpdateHeartbeat updates liveness/load fields for an existing agent.
func (registry *Registry) UpdateHeartbeat(agentID string, activeJobs int, status AgentStatus, now time.Time) (Agent, error) {
	if registry == nil {
		return Agent{}, fmt.Errorf("registry cannot be nil")
	}
	normalizedID, err := identity.NormalizeUUID(agentID)
	if err != nil {
		return Agent{}, fmt.Errorf("agent id: %w", err)
	}
	if activeJobs < 0 {
		return Agent{}, fmt.Errorf("active jobs must be zero or greater")
	}
	if strings.TrimSpace(string(status)) == "" {
		status = AgentStatusAvailable
	}
	if !IsSupportedAgentStatus(status) {
		return Agent{}, fmt.Errorf("unsupported agent status %q", status)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()
	agent, ok := registry.agents[normalizedID]
	if !ok {
		return Agent{}, fmt.Errorf("agent %s not found", normalizedID)
	}
	agent.ActiveJobs = activeJobs
	agent.Status = status
	agent.LastSeenAt = now.UTC()
	registry.agents[normalizedID] = agent.Clone()
	return agent.Clone(), nil
}

// Get returns a defensive copy of a registered agent.
func (registry *Registry) Get(agentID string) (Agent, bool) {
	if registry == nil {
		return Agent{}, false
	}
	normalizedID, err := identity.NormalizeUUID(agentID)
	if err != nil {
		return Agent{}, false
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	agent, ok := registry.agents[normalizedID]
	if !ok {
		return Agent{}, false
	}
	return agent.Clone(), true
}

// List returns registered agents in deterministic ID order.
func (registry *Registry) List() []Agent {
	if registry == nil {
		return nil
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()

	ids := make([]string, 0, len(registry.agents))
	for id := range registry.agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	agents := make([]Agent, 0, len(ids))
	for _, id := range ids {
		agents = append(agents, registry.agents[id].Clone())
	}
	return agents
}

// Len returns the number of registered agents.
func (registry *Registry) Len() int {
	if registry == nil {
		return 0
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return len(registry.agents)
}

// Clone returns a defensive copy of the agent.
func (agent Agent) Clone() Agent {
	copy := agent
	if agent.Metadata != nil {
		copy.Metadata = make(map[string]string, len(agent.Metadata))
		for key, value := range agent.Metadata {
			copy.Metadata[key] = value
		}
	}
	return copy
}

// ValidateAgent checks the structural foundation Agent record.
func ValidateAgent(agent Agent) error {
	var issues []string
	if err := identity.ValidateUUID(agent.ID); err != nil {
		issues = append(issues, "id must be a canonical UUID")
	}
	if err := identity.ValidateFingerprint(agent.Fingerprint); err != nil {
		issues = append(issues, "fingerprint must be a sha256 hex string")
	}
	if err := identity.ValidateHostname(agent.Hostname); err != nil {
		issues = append(issues, "hostname must be canonical")
	}
	if strings.TrimSpace(agent.Version) == "" {
		issues = append(issues, "version is required")
	}
	if !IsSupportedAgentStatus(agent.Status) {
		issues = append(issues, "status must be available, draining or unhealthy")
	}
	if agent.Capacity <= 0 {
		issues = append(issues, "capacity must be greater than zero")
	}
	if agent.ActiveJobs < 0 {
		issues = append(issues, "active_jobs must be zero or greater")
	}
	if agent.ActiveJobs > agent.Capacity {
		issues = append(issues, "active_jobs cannot be greater than capacity")
	}
	if agent.LastSeenAt.IsZero() {
		issues = append(issues, "last_seen_at is required")
	}
	if len(issues) > 0 {
		return fmt.Errorf("agent validation failed: %s", strings.Join(issues, "; "))
	}
	return nil
}

// IsSupportedAgentStatus reports whether status is known by the foundation registry.
func IsSupportedAgentStatus(status AgentStatus) bool {
	switch AgentStatus(strings.TrimSpace(string(status))) {
	case AgentStatusAvailable, AgentStatusDraining, AgentStatusUnhealthy:
		return true
	default:
		return false
	}
}

// SelectLeastLoaded selects the available agent with the lowest load ratio.
func SelectLeastLoaded(agents []Agent) (Agent, bool) {
	candidates := make([]Agent, 0, len(agents))
	for _, agent := range agents {
		if agent.Status != AgentStatusAvailable || agent.Capacity <= 0 || agent.ActiveJobs >= agent.Capacity {
			continue
		}
		candidates = append(candidates, agent.Clone())
	}
	if len(candidates) == 0 {
		return Agent{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := float64(candidates[i].ActiveJobs) / float64(candidates[i].Capacity)
		right := float64(candidates[j].ActiveJobs) / float64(candidates[j].Capacity)
		if left == right {
			return candidates[i].ID < candidates[j].ID
		}
		return left < right
	})
	return candidates[0].Clone(), true
}

// ElectionResult describes deterministic foundation leader election output.
type ElectionResult struct {
	LeaderID    string `json:"leader_id"`
	Fingerprint string `json:"fingerprint"`
	Candidates  int    `json:"candidates"`
	Algorithm   string `json:"algorithm"`
}

// ElectLeader chooses the available agent with the smallest fingerprint.
func ElectLeader(agents []Agent) (ElectionResult, bool) {
	candidates := make([]Agent, 0, len(agents))
	for _, agent := range agents {
		if agent.Status == AgentStatusAvailable {
			candidates = append(candidates, agent.Clone())
		}
	}
	if len(candidates) == 0 {
		return ElectionResult{Algorithm: "lowest_fingerprint"}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Fingerprint == candidates[j].Fingerprint {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Fingerprint < candidates[j].Fingerprint
	})
	leader := candidates[0]
	return ElectionResult{LeaderID: leader.ID, Fingerprint: leader.Fingerprint, Candidates: len(candidates), Algorithm: "lowest_fingerprint"}, true
}

// Assignment is a mechanical failover mapping from one job to one target agent.
type Assignment struct {
	JobID   string `json:"job_id"`
	AgentID string `json:"agent_id"`
}

// FailoverPlan describes deterministic local failover planning output.
type FailoverPlan struct {
	FailedAgentID string       `json:"failed_agent_id"`
	Assignments   []Assignment `json:"assignments"`
	UnassignedIDs []string     `json:"unassigned_ids"`
}

// PlanFailover assigns jobs to available agents without mutating the registry.
func PlanFailover(agents []Agent, failedAgentID string, jobs []worker.Job) (FailoverPlan, error) {
	failedID, err := identity.NormalizeUUID(failedAgentID)
	if err != nil {
		return FailoverPlan{}, fmt.Errorf("failed agent id: %w", err)
	}
	available := make([]Agent, 0, len(agents))
	for _, agent := range agents {
		if agent.ID == failedID || agent.Status != AgentStatusAvailable || agent.ActiveJobs >= agent.Capacity {
			continue
		}
		available = append(available, agent.Clone())
	}

	plan := FailoverPlan{FailedAgentID: failedID}
	for _, job := range jobs {
		if err := worker.ValidateJob(job); err != nil {
			return FailoverPlan{}, err
		}
		target, ok := SelectLeastLoaded(available)
		if !ok {
			plan.UnassignedIDs = append(plan.UnassignedIDs, job.ID)
			continue
		}
		plan.Assignments = append(plan.Assignments, Assignment{JobID: job.ID, AgentID: target.ID})
		for index := range available {
			if available[index].ID == target.ID {
				available[index].ActiveJobs++
				break
			}
		}
	}
	sort.Slice(plan.Assignments, func(i, j int) bool { return plan.Assignments[i].JobID < plan.Assignments[j].JobID })
	sort.Strings(plan.UnassignedIDs)
	return plan, nil
}
