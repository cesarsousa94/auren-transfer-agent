// Package transfer executes Media Hub transfer-agent jobs.
package transfer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/auren/auren-transfer-agent/internal/mediahub"
)

const (
	StateStatusClaimed   = "claimed"
	StateStatusRunning   = "running"
	StateStatusCompleted = "completed"
	StateStatusFailed    = "failed"
)

// JobState is the local crash-recovery snapshot for a remote Media Hub job.
type JobState struct {
	UUID           string               `json:"uuid"`
	Operation      string               `json:"operation"`
	Status         string               `json:"status"`
	Stage          string               `json:"stage"`
	SourceURL      string               `json:"source_url"`
	ObjectPath     string               `json:"object_path"`
	TempPath       string               `json:"temp_path"`
	CurrentBytes   int64                `json:"current_bytes"`
	TotalBytes     int64                `json:"total_bytes"`
	ChecksumSHA256 string               `json:"checksum_sha256,omitempty"`
	LastError      string               `json:"last_error,omitempty"`
	StartedAt      time.Time            `json:"started_at,omitempty"`
	UpdatedAt      time.Time            `json:"updated_at"`
	CompletedAt    time.Time            `json:"completed_at,omitempty"`
	Job            mediahub.TransferJob `json:"job"`
}

// StateStore persists per-job state as JSON files.
type StateStore struct {
	root string
	mu   sync.Mutex
}

// NewStateStore creates a local transfer state store below workDir.
func NewStateStore(workDir string) (*StateStore, error) {
	trimmed := strings.TrimSpace(workDir)
	if trimmed == "" {
		return nil, fmt.Errorf("transfer work_dir is required")
	}
	root := filepath.Join(filepath.Clean(trimmed), "jobs")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &StateStore{root: root}, nil
}

// Path returns the state file path for uuid.
func (store *StateStore) Path(uuid string) string {
	return filepath.Join(store.root, safeName(uuid)+".json")
}

// Save writes state atomically enough for Agent crash recovery.
func (store *StateStore) Save(state JobState) error {
	if store == nil {
		return fmt.Errorf("transfer state store cannot be nil")
	}
	if strings.TrimSpace(state.UUID) == "" {
		return fmt.Errorf("transfer state uuid is required")
	}
	state.UpdatedAt = time.Now().UTC()
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	path := store.Path(state.UUID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600)
}

func safeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' {
			builder.WriteRune(char)
		} else {
			builder.WriteRune('_')
		}
	}
	return builder.String()
}
