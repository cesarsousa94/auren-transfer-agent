package heartbeat

import (
	"testing"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/identity"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
	"github.com/cesarsousa94/auren-transfer-agent/internal/worker"
)

func heartbeatIdentity(t *testing.T) identity.Snapshot {
	t.Helper()
	agentID, err := identity.NewUUID()
	if err != nil {
		t.Fatalf("NewUUID returned error: %v", err)
	}
	fingerprint, err := identity.NewFingerprint(agentID, "agent.local")
	if err != nil {
		t.Fatalf("NewFingerprint returned error: %v", err)
	}
	return identity.Snapshot{AgentID: agentID, Fingerprint: fingerprint, FingerprintAlgorithm: identity.FingerprintAlgorithm, Hostname: "agent.local"}
}

func TestNewRecordBuildsCanonicalHeartbeat(t *testing.T) {
	generated := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	record, err := NewRecord(Input{
		Identity:      heartbeatIdentity(t),
		Version:       runtime.Info(),
		Status:        StatusReady,
		GeneratedAt:   generated,
		Interval:      30 * time.Second,
		WorkerEnabled: true,
		PoolStats:     worker.PoolStats{Concurrency: 2, WorkerIDs: []string{"worker-1", "worker-2"}},
		QueueStats:    QueueStats{Driver: "memory", Length: 1, Capacity: 10},
	})
	if err != nil {
		t.Fatalf("NewRecord returned error: %v", err)
	}
	if record.Status != StatusReady || record.Interval != "30s" || record.WorkerConcurrency != 2 || record.Queue.Length != 1 {
		t.Fatalf("unexpected record: %#v", record)
	}
	if err := ValidateRecord(record); err != nil {
		t.Fatalf("ValidateRecord returned error: %v", err)
	}
}

func TestRecordCloneIsDefensive(t *testing.T) {
	record, err := NewRecord(Input{Identity: heartbeatIdentity(t), PoolStats: worker.PoolStats{Concurrency: 1, WorkerIDs: []string{"worker-1"}}, QueueStats: QueueStats{Driver: "memory"}})
	if err != nil {
		t.Fatalf("NewRecord returned error: %v", err)
	}
	clone := record.Clone()
	clone.WorkerIDs[0] = "mutated"
	if record.WorkerIDs[0] == "mutated" {
		t.Fatalf("clone mutated original record")
	}
}

func TestValidateRecordRejectsInvalidFingerprint(t *testing.T) {
	record, err := NewRecord(Input{Identity: heartbeatIdentity(t)})
	if err != nil {
		t.Fatalf("NewRecord returned error: %v", err)
	}
	record.Fingerprint = "invalid"
	if err := ValidateRecord(record); err == nil {
		t.Fatalf("expected invalid fingerprint error")
	}
}
