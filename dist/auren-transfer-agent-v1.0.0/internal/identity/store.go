package identity

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
	// StoreSchemaVersion is the current local identity document schema version.
	StoreSchemaVersion = 1

	// DefaultStoreDirName is the canonical directory under runtime.data_dir.
	DefaultStoreDirName = "identity"

	// DefaultStoreFileName is the canonical local identity document name.
	DefaultStoreFileName = "agent.json"

	// StorePersistencePersistent identifies durable local identity storage.
	StorePersistencePersistent = "persistent"
)

const (
	storeDirMode  os.FileMode = 0o700
	storeFileMode os.FileMode = 0o600
)

// Record is the durable local Agent identity payload.
type Record struct {
	SchemaVersion int    `json:"schema_version"`
	AgentID       string `json:"agent_id"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// StoreResult describes the outcome of ensuring local identity storage.
type StoreResult struct {
	Record  Record
	Path    string
	Created bool
}

// Source reports whether the identity was created in this boot or loaded from disk.
func (result StoreResult) Source() string {
	if result.Created {
		return "created"
	}
	return "loaded"
}

// Persistence reports the identity persistence mode used by this store result.
func (result StoreResult) Persistence() string {
	return StorePersistencePersistent
}

// FileStore stores the Agent identity in one local JSON file.
type FileStore struct {
	path     string
	now      func() time.Time
	generate func() (string, error)
}

// StoreOption customizes FileStore behavior for tests and future runtime wiring.
type StoreOption func(*FileStore)

// WithClock overrides the clock used by the identity store.
func WithClock(clock func() time.Time) StoreOption {
	return func(store *FileStore) {
		if clock != nil {
			store.now = clock
		}
	}
}

// WithGenerator overrides the UUID generator used for newly created identities.
func WithGenerator(generator func() (string, error)) StoreOption {
	return func(store *FileStore) {
		if generator != nil {
			store.generate = generator
		}
	}
}

// DefaultStorePath resolves the canonical identity file path for a runtime data directory.
func DefaultStorePath(dataDir string) string {
	root := strings.TrimSpace(dataDir)
	if root == "" {
		root = "./data"
	}
	return filepath.Join(root, DefaultStoreDirName, DefaultStoreFileName)
}

// NewFileStore builds a local identity JSON store.
func NewFileStore(path string, options ...StoreOption) FileStore {
	store := FileStore{
		path:     strings.TrimSpace(path),
		now:      time.Now,
		generate: NewUUID,
	}
	for _, option := range options {
		if option != nil {
			option(&store)
		}
	}
	return store
}

// Path returns the configured identity document path.
func (store FileStore) Path() string {
	return store.path
}

// Ensure loads the existing identity or creates it when it does not exist yet.
func (store FileStore) Ensure() (StoreResult, error) {
	if strings.TrimSpace(store.path) == "" {
		return StoreResult{}, fmt.Errorf("identity store path cannot be empty")
	}

	record, err := store.Load()
	if err == nil {
		return StoreResult{Record: record, Path: store.path, Created: false}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return StoreResult{}, err
	}

	agentID, err := store.generate()
	if err != nil {
		return StoreResult{}, fmt.Errorf("create identity: %w", err)
	}
	now := store.now().UTC()
	record, err = NewRecord(agentID, now)
	if err != nil {
		return StoreResult{}, err
	}
	if err := store.Save(record); err != nil {
		return StoreResult{}, err
	}

	return StoreResult{Record: record, Path: store.path, Created: true}, nil
}

// Load reads and validates an existing identity record.
func (store FileStore) Load() (Record, error) {
	if strings.TrimSpace(store.path) == "" {
		return Record{}, fmt.Errorf("identity store path cannot be empty")
	}

	payload, err := os.ReadFile(store.path)
	if err != nil {
		return Record{}, fmt.Errorf("load identity store %s: %w", store.path, err)
	}

	var record Record
	if err := json.Unmarshal(payload, &record); err != nil {
		return Record{}, fmt.Errorf("decode identity store %s: %w", store.path, err)
	}
	if err := ValidateRecord(record); err != nil {
		return Record{}, fmt.Errorf("validate identity store %s: %w", store.path, err)
	}

	return record, nil
}

// Save validates and atomically writes an identity record.
func (store FileStore) Save(record Record) error {
	if strings.TrimSpace(store.path) == "" {
		return fmt.Errorf("identity store path cannot be empty")
	}
	if err := ValidateRecord(record); err != nil {
		return err
	}

	directory := filepath.Dir(store.path)
	if err := os.MkdirAll(directory, storeDirMode); err != nil {
		return fmt.Errorf("create identity store directory %s: %w", directory, err)
	}

	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode identity store: %w", err)
	}
	payload = append(payload, '\n')

	temporary, err := os.CreateTemp(directory, ".agent-identity-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary identity store file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary identity store file: %w", err)
	}
	if err := temporary.Chmod(storeFileMode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("chmod temporary identity store file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary identity store file: %w", err)
	}
	if err := os.Rename(temporaryPath, store.path); err != nil {
		return fmt.Errorf("replace identity store file %s: %w", store.path, err)
	}

	return nil
}

// NewRecord builds a validated identity record from an Agent ID and timestamp.
func NewRecord(agentID string, timestamp time.Time) (Record, error) {
	normalized, err := NormalizeUUID(agentID)
	if err != nil {
		return Record{}, fmt.Errorf("identity record agent_id: %w", err)
	}

	stamp := timestamp.UTC().Format(time.RFC3339Nano)
	record := Record{
		SchemaVersion: StoreSchemaVersion,
		AgentID:       normalized,
		CreatedAt:     stamp,
		UpdatedAt:     stamp,
	}
	if err := ValidateRecord(record); err != nil {
		return Record{}, err
	}

	return record, nil
}

// ValidateRecord validates the durable identity document contract.
func ValidateRecord(record Record) error {
	if record.SchemaVersion != StoreSchemaVersion {
		return fmt.Errorf("identity record schema_version must be %d", StoreSchemaVersion)
	}
	if err := ValidateUUID(record.AgentID); err != nil {
		return fmt.Errorf("identity record agent_id: %w", err)
	}
	createdAt, err := parseRecordTime("created_at", record.CreatedAt)
	if err != nil {
		return err
	}
	updatedAt, err := parseRecordTime("updated_at", record.UpdatedAt)
	if err != nil {
		return err
	}
	if updatedAt.Before(createdAt) {
		return fmt.Errorf("identity record updated_at cannot be before created_at")
	}

	return nil
}

func parseRecordTime(field string, value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("identity record %s is required", field)
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("identity record %s must be RFC3339Nano: %w", field, err)
	}
	return parsed, nil
}
