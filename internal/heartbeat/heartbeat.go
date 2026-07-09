// Package heartbeat builds local Agent heartbeat snapshots.
package heartbeat

import (
	"fmt"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/identity"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
	"github.com/cesarsousa94/auren-transfer-agent/internal/worker"
)

const (
	// StatusIdle indicates the foundation Agent is initialized but not running a loop.
	StatusIdle = "idle"

	// StatusReady indicates the Agent is structurally ready.
	StatusReady = "ready"
)

// QueueStats is a small queue snapshot embedded in heartbeat records.
type QueueStats struct {
	Driver   string `json:"driver"`
	Length   int    `json:"length"`
	Capacity int    `json:"capacity"`
}

// Input describes the local state used to build a heartbeat.
type Input struct {
	Identity      identity.Snapshot
	Version       runtime.VersionInfo
	Status        string
	GeneratedAt   time.Time
	Interval      time.Duration
	WorkerEnabled bool
	PoolStats     worker.PoolStats
	QueueStats    QueueStats
}

// Record is the canonical foundation heartbeat payload.
type Record struct {
	AgentID              string     `json:"agent_id"`
	Fingerprint          string     `json:"fingerprint"`
	FingerprintAlgorithm string     `json:"fingerprint_algorithm"`
	Hostname             string     `json:"hostname"`
	Version              string     `json:"version"`
	RuntimeStatus        string     `json:"runtime_status"`
	Status               string     `json:"status"`
	GeneratedAt          time.Time  `json:"generated_at"`
	Interval             string     `json:"interval"`
	WorkerEnabled        bool       `json:"worker_enabled"`
	WorkerConcurrency    int        `json:"worker_concurrency"`
	WorkerIDs            []string   `json:"worker_ids"`
	Queue                QueueStats `json:"queue"`
}

// NewRecord creates a validated heartbeat record from local Agent state.
func NewRecord(input Input) (Record, error) {
	generatedAt := input.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	status := input.Status
	if status == "" {
		status = StatusIdle
	}
	version := input.Version
	if version.Version == "" {
		version = runtime.Info()
	}
	record := Record{
		AgentID:              input.Identity.AgentID,
		Fingerprint:          input.Identity.Fingerprint,
		FingerprintAlgorithm: input.Identity.FingerprintAlgorithm,
		Hostname:             input.Identity.Hostname,
		Version:              version.Version,
		RuntimeStatus:        version.Status,
		Status:               status,
		GeneratedAt:          generatedAt,
		Interval:             input.Interval.String(),
		WorkerEnabled:        input.WorkerEnabled,
		WorkerConcurrency:    input.PoolStats.Concurrency,
		WorkerIDs:            cloneStrings(input.PoolStats.WorkerIDs),
		Queue:                input.QueueStats,
	}
	if err := ValidateRecord(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

// ValidateRecord verifies the structural heartbeat contract.
func ValidateRecord(record Record) error {
	if err := identity.ValidateUUID(record.AgentID); err != nil {
		return fmt.Errorf("heartbeat agent_id: %w", err)
	}
	if err := identity.ValidateFingerprint(record.Fingerprint); err != nil {
		return fmt.Errorf("heartbeat fingerprint: %w", err)
	}
	if record.FingerprintAlgorithm != identity.FingerprintAlgorithm {
		return fmt.Errorf("heartbeat fingerprint_algorithm must be %s", identity.FingerprintAlgorithm)
	}
	if record.Hostname == "" {
		return fmt.Errorf("heartbeat hostname is required")
	}
	if record.Version == "" {
		return fmt.Errorf("heartbeat version is required")
	}
	if record.RuntimeStatus == "" {
		return fmt.Errorf("heartbeat runtime_status is required")
	}
	if record.Status == "" {
		return fmt.Errorf("heartbeat status is required")
	}
	if record.GeneratedAt.IsZero() {
		return fmt.Errorf("heartbeat generated_at is required")
	}
	if record.WorkerConcurrency < 0 {
		return fmt.Errorf("heartbeat worker_concurrency must be zero or greater")
	}
	if record.Queue.Length < 0 || record.Queue.Capacity < 0 {
		return fmt.Errorf("heartbeat queue length and capacity must be zero or greater")
	}
	return nil
}

// Clone returns a defensive heartbeat copy.
func (record Record) Clone() Record {
	copy := record
	copy.WorkerIDs = cloneStrings(record.WorkerIDs)
	return copy
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
