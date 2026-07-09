package cluster

import (
	"testing"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/identity"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
	"github.com/cesarsousa94/auren-transfer-agent/internal/worker"
)

func testSnapshot(t *testing.T, agentID string, hostname string) identity.Snapshot {
	t.Helper()
	fingerprint, err := identity.NewFingerprint(agentID, hostname)
	if err != nil {
		t.Fatalf("NewFingerprint: %v", err)
	}
	return identity.Snapshot{AgentID: agentID, Fingerprint: fingerprint, FingerprintAlgorithm: identity.FingerprintAlgorithm, Hostname: hostname, HostnameSource: "test", Persistence: "persistent", StoreSource: "created", StorePath: "/tmp/agent.json"}
}

func TestRegistryRegistersLocalAgentAndReturnsCopies(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	snapshot := testSnapshot(t, "11111111-1111-4111-8111-111111111111", "agent-a")
	agent, err := LocalAgent(snapshot, runtime.VersionInfo{Name: runtime.AppName, Version: "v-test", Status: "foundation/test"}, 3, now)
	if err != nil {
		t.Fatalf("LocalAgent: %v", err)
	}
	registry, err := NewRegistry(agent)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	listed := registry.List()
	if len(listed) != 1 || listed[0].ID != agent.ID || listed[0].Capacity != 3 {
		t.Fatalf("unexpected registry list: %#v", listed)
	}
	listed[0].Metadata["runtime_status"] = "mutated"

	stored, ok := registry.Get(agent.ID)
	if !ok {
		t.Fatal("expected stored agent")
	}
	if stored.Metadata["runtime_status"] == "mutated" {
		t.Fatal("expected defensive copies from registry")
	}
}

func TestUpdateHeartbeatAndLoadBalancer(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	agentA, _ := LocalAgent(testSnapshot(t, "11111111-1111-4111-8111-111111111111", "agent-a"), runtime.Info(), 4, now)
	agentB, _ := LocalAgent(testSnapshot(t, "22222222-2222-4222-8222-222222222222", "agent-b"), runtime.Info(), 2, now)
	registry, err := NewRegistry(agentA, agentB)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if _, err := registry.UpdateHeartbeat(agentA.ID, 3, AgentStatusAvailable, now.Add(time.Second)); err != nil {
		t.Fatalf("UpdateHeartbeat: %v", err)
	}

	selected, ok := SelectLeastLoaded(registry.List())
	if !ok {
		t.Fatal("expected load balancer to select an agent")
	}
	if selected.ID != agentB.ID {
		t.Fatalf("expected agent-b with lower load ratio, got %#v", selected)
	}
}

func TestElectLeaderUsesLowestFingerprint(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	agentA, _ := LocalAgent(testSnapshot(t, "11111111-1111-4111-8111-111111111111", "agent-a"), runtime.Info(), 1, now)
	agentB, _ := LocalAgent(testSnapshot(t, "22222222-2222-4222-8222-222222222222", "agent-b"), runtime.Info(), 1, now)
	result, ok := ElectLeader([]Agent{agentA, agentB})
	if !ok {
		t.Fatal("expected leader election result")
	}
	expected := agentA
	if agentB.Fingerprint < agentA.Fingerprint {
		expected = agentB
	}
	if result.LeaderID != expected.ID || result.Candidates != 2 || result.Algorithm != "lowest_fingerprint" {
		t.Fatalf("unexpected leader result: %#v expected=%s", result, expected.ID)
	}
}

func TestPlanFailoverAssignsJobsToAvailableAgents(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	failed, _ := LocalAgent(testSnapshot(t, "11111111-1111-4111-8111-111111111111", "failed"), runtime.Info(), 1, now)
	target, _ := LocalAgent(testSnapshot(t, "22222222-2222-4222-8222-222222222222", "target"), runtime.Info(), 2, now)
	job, err := worker.NewJob(worker.JobInput{ID: "33333333-3333-4333-8333-333333333333", SourceURL: "https://example.test/media.mp4", DestinationKey: "media.mp4", Now: now})
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	plan, err := PlanFailover([]Agent{failed, target}, failed.ID, []worker.Job{job})
	if err != nil {
		t.Fatalf("PlanFailover: %v", err)
	}
	if len(plan.Assignments) != 1 || plan.Assignments[0].AgentID != target.ID || len(plan.UnassignedIDs) != 0 {
		t.Fatalf("unexpected failover plan: %#v", plan)
	}
}

func TestPlanFailoverTracksUnassignedJobs(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	failed, _ := LocalAgent(testSnapshot(t, "11111111-1111-4111-8111-111111111111", "failed"), runtime.Info(), 1, now)
	job, err := worker.NewJob(worker.JobInput{ID: "33333333-3333-4333-8333-333333333333", SourceURL: "https://example.test/media.mp4", DestinationKey: "media.mp4", Now: now})
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	plan, err := PlanFailover([]Agent{failed}, failed.ID, []worker.Job{job})
	if err != nil {
		t.Fatalf("PlanFailover: %v", err)
	}
	if len(plan.Assignments) != 0 || len(plan.UnassignedIDs) != 1 || plan.UnassignedIDs[0] != job.ID {
		t.Fatalf("unexpected unassigned plan: %#v", plan)
	}
}
