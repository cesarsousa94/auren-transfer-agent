package identity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testUUID = "123e4567-e89b-42d3-a456-426614174000"

func TestDefaultStorePathUsesRuntimeDataDir(t *testing.T) {
	got := DefaultStorePath("/var/lib/auren-transfer-agent")
	want := filepath.Join("/var/lib/auren-transfer-agent", DefaultStoreDirName, DefaultStoreFileName)
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEnsureCreatesAndPersistsIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity", "agent.json")
	clock := fixedClock("2026-07-09T03:00:00Z")
	store := NewFileStore(path, WithClock(clock), WithGenerator(func() (string, error) { return testUUID, nil }))

	result, err := store.Ensure()
	if err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}
	if !result.Created {
		t.Fatal("expected identity to be created")
	}
	if result.Source() != "created" {
		t.Fatalf("expected created source, got %q", result.Source())
	}
	if result.Persistence() != StorePersistencePersistent {
		t.Fatalf("expected persistent identity, got %q", result.Persistence())
	}
	if result.Record.AgentID != testUUID {
		t.Fatalf("expected agent id %q, got %q", testUUID, result.Record.AgentID)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded != result.Record {
		t.Fatalf("expected loaded record to match saved record")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat identity file: %v", err)
	}
	if info.Mode().Perm() != storeFileMode {
		t.Fatalf("expected file mode %v, got %v", storeFileMode, info.Mode().Perm())
	}
}

func TestEnsureLoadsExistingIdentityWithoutRegenerating(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity", "agent.json")
	store := NewFileStore(path, WithClock(fixedClock("2026-07-09T03:00:00Z")), WithGenerator(func() (string, error) { return testUUID, nil }))

	created, err := store.Ensure()
	if err != nil {
		t.Fatalf("initial Ensure returned error: %v", err)
	}

	secondGeneratorCalled := false
	secondStore := NewFileStore(path, WithGenerator(func() (string, error) {
		secondGeneratorCalled = true
		return "123e4567-e89b-42d3-a456-426614174111", nil
	}))
	loaded, err := secondStore.Ensure()
	if err != nil {
		t.Fatalf("second Ensure returned error: %v", err)
	}

	if loaded.Created {
		t.Fatal("expected identity to be loaded, not created")
	}
	if loaded.Source() != "loaded" {
		t.Fatalf("expected loaded source, got %q", loaded.Source())
	}
	if secondGeneratorCalled {
		t.Fatal("generator should not run when identity already exists")
	}
	if loaded.Record != created.Record {
		t.Fatalf("expected persisted record to be reused")
	}
}

func TestNewRecordNormalizesUUIDAndTimestamps(t *testing.T) {
	record, err := NewRecord("  123E4567-E89B-42D3-A456-426614174000  ", fixedClock("2026-07-09T03:00:00Z")())
	if err != nil {
		t.Fatalf("NewRecord returned error: %v", err)
	}

	if record.SchemaVersion != StoreSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", StoreSchemaVersion, record.SchemaVersion)
	}
	if record.AgentID != testUUID {
		t.Fatalf("expected normalized UUID %q, got %q", testUUID, record.AgentID)
	}
	if record.CreatedAt != "2026-07-09T03:00:00Z" || record.UpdatedAt != "2026-07-09T03:00:00Z" {
		t.Fatalf("unexpected timestamps: %+v", record)
	}
}

func TestValidateRecordRejectsInvalidRecord(t *testing.T) {
	valid, err := NewRecord(testUUID, fixedClock("2026-07-09T03:00:00Z")())
	if err != nil {
		t.Fatalf("NewRecord returned error: %v", err)
	}

	cases := []Record{
		{SchemaVersion: 0, AgentID: valid.AgentID, CreatedAt: valid.CreatedAt, UpdatedAt: valid.UpdatedAt},
		{SchemaVersion: StoreSchemaVersion, AgentID: "not-a-uuid", CreatedAt: valid.CreatedAt, UpdatedAt: valid.UpdatedAt},
		{SchemaVersion: StoreSchemaVersion, AgentID: valid.AgentID, CreatedAt: "", UpdatedAt: valid.UpdatedAt},
		{SchemaVersion: StoreSchemaVersion, AgentID: valid.AgentID, CreatedAt: valid.CreatedAt, UpdatedAt: "not-time"},
		{SchemaVersion: StoreSchemaVersion, AgentID: valid.AgentID, CreatedAt: "2026-07-09T03:00:01Z", UpdatedAt: "2026-07-09T03:00:00Z"},
	}

	for _, record := range cases {
		if err := ValidateRecord(record); err == nil {
			t.Fatalf("expected invalid record to fail: %+v", record)
		}
	}
}

func TestLoadRejectsMalformedIdentityFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity", "agent.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"agent_id":`), 0o600); err != nil {
		t.Fatalf("write malformed file: %v", err)
	}

	store := NewFileStore(path)
	if _, err := store.Load(); err == nil {
		t.Fatal("expected malformed identity file to fail")
	}
}

func TestSaveWritesCanonicalJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity", "agent.json")
	store := NewFileStore(path)
	record, err := NewRecord(testUUID, fixedClock("2026-07-09T03:00:00Z")())
	if err != nil {
		t.Fatalf("NewRecord returned error: %v", err)
	}

	if err := store.Save(record); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	var decoded Record
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if decoded != record {
		t.Fatalf("expected decoded record to match original")
	}
}

func fixedClock(value string) func() time.Time {
	return func() time.Time {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			panic(err)
		}
		return parsed
	}
}
