// Package mediahub implements the Auren Media Hub node-agent connector.
package mediahub

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// StateSchemaVersion is the current durable Media Hub node credential schema.
	StateSchemaVersion = 1

	// DefaultStateDirName is the canonical subdirectory under runtime.data_dir.
	DefaultStateDirName = "media-hub"

	// DefaultStateFileName is the canonical persisted node credential document.
	DefaultStateFileName = "node.json"

	stateDirMode  os.FileMode = 0o700
	stateFileMode os.FileMode = 0o600
)

// NodeState stores Media Hub-issued node credentials locally.
type NodeState struct {
	SchemaVersion int       `json:"schema_version"`
	NodeUUID      string    `json:"node_uuid"`
	NodeSecret    string    `json:"node_secret"`
	ConfigVersion string    `json:"config_version,omitempty"`
	RegisteredAt  time.Time `json:"registered_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	LastConfigAt  time.Time `json:"last_config_at,omitempty"`
}

// Empty reports whether the state lacks usable node credentials.
func (state NodeState) Empty() bool {
	return strings.TrimSpace(state.NodeUUID) == "" || strings.TrimSpace(state.NodeSecret) == ""
}

// Clone returns a defensive copy of the state.
func (state NodeState) Clone() NodeState {
	return state
}

// Safe returns a redacted state snapshot for diagnostics.
func (state NodeState) Safe() map[string]any {
	return map[string]any{
		"schema_version": state.SchemaVersion,
		"node_uuid":      strings.TrimSpace(state.NodeUUID),
		"secret_present": strings.TrimSpace(state.NodeSecret) != "",
		"config_version": strings.TrimSpace(state.ConfigVersion),
		"registered_at":  state.RegisteredAt,
		"updated_at":     state.UpdatedAt,
		"last_config_at": state.LastConfigAt,
	}
}

// Validate verifies the durable state contract.
func (state NodeState) Validate() error {
	if state.SchemaVersion != StateSchemaVersion {
		return fmt.Errorf("media hub state schema_version must be %d", StateSchemaVersion)
	}
	if strings.TrimSpace(state.NodeUUID) == "" {
		return fmt.Errorf("media hub state node_uuid is required")
	}
	if strings.TrimSpace(state.NodeSecret) == "" {
		return fmt.Errorf("media hub state node_secret is required")
	}
	if state.RegisteredAt.IsZero() {
		return fmt.Errorf("media hub state registered_at is required")
	}
	if state.UpdatedAt.IsZero() {
		return fmt.Errorf("media hub state updated_at is required")
	}
	if state.UpdatedAt.Before(state.RegisteredAt) {
		return fmt.Errorf("media hub state updated_at cannot be before registered_at")
	}
	return nil
}

// NewNodeState creates a validated state record from registration response values.
func NewNodeState(nodeUUID string, nodeSecret string, configVersion string, now time.Time) (NodeState, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	state := NodeState{
		SchemaVersion: StateSchemaVersion,
		NodeUUID:      strings.TrimSpace(nodeUUID),
		NodeSecret:    strings.TrimSpace(nodeSecret),
		ConfigVersion: strings.TrimSpace(configVersion),
		RegisteredAt:  now.UTC(),
		UpdatedAt:     now.UTC(),
	}
	if err := state.Validate(); err != nil {
		return NodeState{}, err
	}
	return state, nil
}

// StateStore stores Media Hub-issued credentials in one local JSON file.
type StateStore struct {
	path string
	now  func() time.Time
}

// StateStoreOption customizes StateStore behavior for tests.
type StateStoreOption func(*StateStore)

// WithClock overrides the state store clock.
func WithClock(clock func() time.Time) StateStoreOption {
	return func(store *StateStore) {
		if clock != nil {
			store.now = clock
		}
	}
}

// DefaultStatePath resolves the canonical Media Hub state path.
func DefaultStatePath(dataDir string) string {
	root := strings.TrimSpace(dataDir)
	if root == "" {
		root = "./data"
	}
	return filepath.Join(root, DefaultStateDirName, DefaultStateFileName)
}

// NewStateStore creates a durable Media Hub state store.
func NewStateStore(path string, options ...StateStoreOption) StateStore {
	store := StateStore{path: strings.TrimSpace(path), now: time.Now}
	for _, option := range options {
		if option != nil {
			option(&store)
		}
	}
	return store
}

// Path returns the configured state path.
func (store StateStore) Path() string {
	return store.path
}

// Load reads a stored node credential document.
func (store StateStore) Load() (NodeState, error) {
	if strings.TrimSpace(store.path) == "" {
		return NodeState{}, fmt.Errorf("media hub state path cannot be empty")
	}
	payload, err := os.ReadFile(store.path)
	if err != nil {
		return NodeState{}, fmt.Errorf("load media hub state %s: %w", store.path, err)
	}
	var state NodeState
	if err := json.Unmarshal(payload, &state); err != nil {
		return NodeState{}, fmt.Errorf("decode media hub state %s: %w", store.path, err)
	}
	if err := state.Validate(); err != nil {
		return NodeState{}, fmt.Errorf("validate media hub state %s: %w", store.path, err)
	}
	return state, nil
}

// LoadOrEmpty loads state when available and returns an empty state when absent.
func (store StateStore) LoadOrEmpty() (NodeState, bool, error) {
	state, err := store.Load()
	if err == nil {
		return state, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return NodeState{}, false, nil
	}
	return NodeState{}, false, err
}

// Save validates and atomically writes a node state document.
func (store StateStore) Save(state NodeState) error {
	if strings.TrimSpace(store.path) == "" {
		return fmt.Errorf("media hub state path cannot be empty")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = store.now().UTC()
	}
	if err := state.Validate(); err != nil {
		return err
	}
	directory := filepath.Dir(store.path)
	if err := os.MkdirAll(directory, stateDirMode); err != nil {
		return fmt.Errorf("create media hub state directory %s: %w", directory, err)
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode media hub state: %w", err)
	}
	payload = append(payload, '\n')
	temporary, err := os.CreateTemp(directory, ".media-hub-node-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary media hub state file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary media hub state file: %w", err)
	}
	if err := temporary.Chmod(stateFileMode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("chmod temporary media hub state file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary media hub state file: %w", err)
	}
	if err := os.Rename(temporaryPath, store.path); err != nil {
		return fmt.Errorf("replace media hub state file %s: %w", store.path, err)
	}
	return nil
}
