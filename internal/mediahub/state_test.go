package mediahub

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateStorePersistsCredentialsWithRestrictedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "media-hub", "node.json")
	store := NewStateStore(path, WithClock(func() time.Time { return time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC) }))
	state, err := NewNodeState("node-uuid", "node-secret", "cfg-1", time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, ok, err := store.LoadOrEmpty()
	if err != nil || !ok {
		t.Fatalf("LoadOrEmpty() = %#v, %t, %v", loaded, ok, err)
	}
	if loaded.NodeUUID != "node-uuid" || loaded.NodeSecret != "node-secret" || loaded.ConfigVersion != "cfg-1" {
		t.Fatalf("loaded state mismatch: %#v", loaded)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state mode = %v, want 0600", info.Mode().Perm())
	}
}
