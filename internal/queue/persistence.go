// Package queue contains foundation queue contracts for worker jobs.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/auren/auren-transfer-agent/internal/worker"
)

const (
	// PersistenceVersion is the foundation queue persistence payload version.
	PersistenceVersion = "queue.snapshot.v1"

	// StoreSourceCreated means the store file did not exist and was initialized.
	StoreSourceCreated = "created"

	// StoreSourceLoaded means jobs were restored from an existing store file.
	StoreSourceLoaded = "loaded"

	// StoreSourceSaved means a snapshot was written to disk.
	StoreSourceSaved = "saved"
)

// Snapshot is the canonical local queue persistence payload.
type Snapshot struct {
	Version     string       `json:"version"`
	Driver      string       `json:"driver"`
	GeneratedAt time.Time    `json:"generated_at"`
	Jobs        []worker.Job `json:"jobs"`
}

// StoreResult summarizes a persistence store operation.
type StoreResult struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	Jobs   int    `json:"jobs"`
}

// FileStore stores queue snapshots as local JSON.
type FileStore struct {
	path string
	now  func() time.Time
}

// DefaultPersistencePath returns the canonical queue persistence file path.
func DefaultPersistencePath(dataDir string) string {
	base := strings.TrimSpace(dataDir)
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "worker", "queue.json")
}

// NewFileStore creates a queue persistence store.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: strings.TrimSpace(path), now: func() time.Time { return time.Now().UTC() }}
}

// Path returns the configured persistence file path.
func (store *FileStore) Path() string {
	if store == nil {
		return ""
	}
	return store.path
}

// Load reads a queue snapshot. Missing files are reported without error.
func (store *FileStore) Load(ctx context.Context) (Snapshot, bool, error) {
	if store == nil {
		return Snapshot{}, false, fmt.Errorf("queue file store cannot be nil")
	}
	if err := ctxErr(ctx); err != nil {
		return Snapshot{}, false, err
	}
	if strings.TrimSpace(store.path) == "" {
		return Snapshot{}, false, fmt.Errorf("queue persistence path cannot be empty")
	}

	content, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}

	var snapshot Snapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return Snapshot{}, false, fmt.Errorf("decode queue persistence: %w", err)
	}
	if err := ValidateSnapshot(snapshot); err != nil {
		return Snapshot{}, false, err
	}
	return snapshot.Clone(), true, nil
}

// Save writes a queue snapshot atomically.
func (store *FileStore) Save(ctx context.Context, driver string, jobs []worker.Job) (StoreResult, error) {
	if store == nil {
		return StoreResult{}, fmt.Errorf("queue file store cannot be nil")
	}
	if err := ctxErr(ctx); err != nil {
		return StoreResult{}, err
	}
	if strings.TrimSpace(store.path) == "" {
		return StoreResult{}, fmt.Errorf("queue persistence path cannot be empty")
	}

	snapshot := NewSnapshot(driver, jobs, store.now())
	if err := ValidateSnapshot(snapshot); err != nil {
		return StoreResult{}, err
	}

	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return StoreResult{}, err
	}
	content, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return StoreResult{}, err
	}
	content = append(content, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(store.path), ".queue-*.tmp")
	if err != nil {
		return StoreResult{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return StoreResult{}, err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return StoreResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return StoreResult{}, err
	}
	if err := os.Rename(tmpName, store.path); err != nil {
		return StoreResult{}, err
	}

	return StoreResult{Path: store.path, Source: StoreSourceSaved, Jobs: len(snapshot.Jobs)}, nil
}

// Ensure loads an existing snapshot or creates an empty one.
func (store *FileStore) Ensure(ctx context.Context, driver string) (Snapshot, StoreResult, error) {
	snapshot, ok, err := store.Load(ctx)
	if err != nil {
		return Snapshot{}, StoreResult{}, err
	}
	if ok {
		return snapshot.Clone(), StoreResult{Path: store.Path(), Source: StoreSourceLoaded, Jobs: len(snapshot.Jobs)}, nil
	}
	result, err := store.Save(ctx, driver, nil)
	if err != nil {
		return Snapshot{}, StoreResult{}, err
	}
	snapshot, ok, err = store.Load(ctx)
	if err != nil {
		return Snapshot{}, StoreResult{}, err
	}
	if !ok {
		return Snapshot{}, StoreResult{}, fmt.Errorf("queue persistence was not created")
	}
	result.Source = StoreSourceCreated
	return snapshot.Clone(), result, nil
}

// Restore enqueues jobs from a persisted snapshot into a queue.
func Restore(ctx context.Context, target Queue, snapshot Snapshot) (int, error) {
	if target == nil {
		return 0, fmt.Errorf("queue restore target cannot be nil")
	}
	if err := ValidateSnapshot(snapshot); err != nil {
		return 0, err
	}
	restored := 0
	for _, job := range snapshot.Jobs {
		if err := target.Enqueue(ctx, job); err != nil {
			return restored, err
		}
		restored++
	}
	return restored, nil
}

// NewSnapshot creates a defensive queue persistence snapshot.
func NewSnapshot(driver string, jobs []worker.Job, now time.Time) Snapshot {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Snapshot{Version: PersistenceVersion, Driver: strings.TrimSpace(driver), GeneratedAt: now.UTC(), Jobs: cloneJobs(jobs)}
}

// ValidateSnapshot checks the queue persistence contract.
func ValidateSnapshot(snapshot Snapshot) error {
	if snapshot.Version != PersistenceVersion {
		return fmt.Errorf("queue persistence version must be %s", PersistenceVersion)
	}
	if strings.TrimSpace(snapshot.Driver) == "" {
		return fmt.Errorf("queue persistence driver is required")
	}
	if snapshot.GeneratedAt.IsZero() {
		return fmt.Errorf("queue persistence generated_at is required")
	}
	for index, job := range snapshot.Jobs {
		if err := worker.ValidateJob(job); err != nil {
			return fmt.Errorf("queue persistence job %d: %w", index, err)
		}
		if job.Status != worker.JobStatusQueued && job.Status != worker.JobStatusPending {
			return fmt.Errorf("queue persistence job %d must be pending or queued", index)
		}
	}
	return nil
}

// Clone returns a defensive snapshot copy.
func (snapshot Snapshot) Clone() Snapshot {
	copy := snapshot
	copy.Jobs = cloneJobs(snapshot.Jobs)
	return copy
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func cloneJobs(jobs []worker.Job) []worker.Job {
	if jobs == nil {
		return nil
	}
	cloned := make([]worker.Job, len(jobs))
	for index, job := range jobs {
		cloned[index] = job.Clone()
	}
	return cloned
}
