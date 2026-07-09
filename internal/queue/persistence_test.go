package queue

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/auren/auren-transfer-agent/internal/worker"
)

func TestFileStoreEnsureCreatesAndLoadsEmptySnapshot(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "worker", "queue.json"))
	snapshot, result, err := store.Ensure(context.Background(), MemoryDriver)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if result.Source != StoreSourceCreated || result.Jobs != 0 {
		t.Fatalf("result = %#v, want created empty", result)
	}
	if snapshot.Version != PersistenceVersion || snapshot.Driver != MemoryDriver || len(snapshot.Jobs) != 0 {
		t.Fatalf("snapshot = %#v", snapshot)
	}

	loaded, result, err := store.Ensure(context.Background(), MemoryDriver)
	if err != nil {
		t.Fatalf("Ensure() second error = %v", err)
	}
	if result.Source != StoreSourceLoaded || loaded.Version != PersistenceVersion {
		t.Fatalf("second result=%#v snapshot=%#v", result, loaded)
	}
}

func TestFileStoreSaveLoadAndRestore(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "queue.json"))
	job := testJob(t, "persisted.ts")
	if _, err := store.Save(context.Background(), MemoryDriver, []worker.Job{job}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	snapshot, ok, err := store.Load(context.Background())
	if err != nil || !ok {
		t.Fatalf("Load() ok=%t err=%v", ok, err)
	}
	if len(snapshot.Jobs) != 1 || snapshot.Jobs[0].ID != job.ID {
		t.Fatalf("loaded snapshot = %#v", snapshot)
	}

	snapshot.Jobs[0].Metadata = map[string]string{"mutated": "true"}
	again, _, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() again error = %v", err)
	}
	if _, ok := again.Jobs[0].Metadata["mutated"]; ok {
		t.Fatal("Load() did not return a defensive copy")
	}

	queue, err := NewMemoryQueue(2)
	if err != nil {
		t.Fatalf("NewMemoryQueue() error = %v", err)
	}
	restored, err := Restore(context.Background(), queue, again)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored != 1 || queue.Len() != 1 {
		t.Fatalf("restored=%d queue.Len=%d, want 1/1", restored, queue.Len())
	}
}

func TestLoadRejectsMalformedPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	if err := os.WriteFile(path, []byte(`{"version":"bad"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, _, err := NewFileStore(path).Load(context.Background())
	if err == nil {
		t.Fatal("Load() error = nil, want malformed persistence error")
	}
}

func TestValidateSnapshotRejectsRunningJob(t *testing.T) {
	job := testJob(t, "running.ts")
	running, err := job.WithAttemptStatus(worker.JobStatusRunning, time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("WithAttemptStatus() error = %v", err)
	}
	snapshot := NewSnapshot(MemoryDriver, []worker.Job{running}, time.Now().UTC())
	if err := ValidateSnapshot(snapshot); err == nil {
		t.Fatal("ValidateSnapshot() error = nil, want running job rejection")
	}
}
