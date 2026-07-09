package identity

import (
	"strings"
	"testing"
)

func TestNewFingerprintIsDeterministicAndCanonical(t *testing.T) {
	first, err := NewFingerprint("  123E4567-E89B-42D3-A456-426614174000  ", " AUREN-NODE-01. ")
	if err != nil {
		t.Fatalf("NewFingerprint returned error: %v", err)
	}
	second, err := NewFingerprint(testUUID, "auren-node-01")
	if err != nil {
		t.Fatalf("NewFingerprint returned error: %v", err)
	}

	if first != second {
		t.Fatalf("fingerprint should be deterministic, got %q and %q", first, second)
	}
	if len(first) != FingerprintLength {
		t.Fatalf("fingerprint length = %d, want %d", len(first), FingerprintLength)
	}
	if strings.ToLower(first) != first {
		t.Fatalf("fingerprint should be lowercase, got %q", first)
	}
	if !IsFingerprint(first) {
		t.Fatalf("expected %q to be a valid fingerprint", first)
	}
}

func TestNewFingerprintChangesWhenHostnameChanges(t *testing.T) {
	first, err := NewFingerprint(testUUID, "agent-a")
	if err != nil {
		t.Fatalf("NewFingerprint returned error: %v", err)
	}
	second, err := NewFingerprint(testUUID, "agent-b")
	if err != nil {
		t.Fatalf("NewFingerprint returned error: %v", err)
	}

	if first == second {
		t.Fatal("fingerprint should change when hostname changes")
	}
}

func TestValidateFingerprintRejectsInvalidValues(t *testing.T) {
	valid, err := NewFingerprint(testUUID, "agent-a")
	if err != nil {
		t.Fatalf("NewFingerprint returned error: %v", err)
	}

	cases := []string{
		"",
		" ",
		valid[:FingerprintLength-1],
		valid + "0",
		strings.ToUpper(valid),
		strings.Repeat("g", FingerprintLength),
	}

	for _, value := range cases {
		if err := ValidateFingerprint(value); err == nil {
			t.Fatalf("expected invalid fingerprint %q to fail", value)
		}
	}
}

func TestNewSnapshotBuildsCanonicalPayload(t *testing.T) {
	record, err := NewRecord(testUUID, fixedClock("2026-07-09T03:00:00Z")())
	if err != nil {
		t.Fatalf("NewRecord returned error: %v", err)
	}
	result := StoreResult{Record: record, Path: "/var/lib/auren-transfer-agent/identity/agent.json", Created: true}
	host := HostInfo{Raw: "AUREN-NODE-01", Normalized: "auren-node-01", Source: HostnameSourceOS}

	snapshot, err := NewSnapshot(result, host)
	if err != nil {
		t.Fatalf("NewSnapshot returned error: %v", err)
	}

	if snapshot.AgentID != testUUID {
		t.Fatalf("AgentID = %q, want %q", snapshot.AgentID, testUUID)
	}
	if !IsFingerprint(snapshot.Fingerprint) {
		t.Fatalf("invalid snapshot fingerprint %q", snapshot.Fingerprint)
	}
	if snapshot.FingerprintAlgorithm != FingerprintAlgorithm {
		t.Fatalf("FingerprintAlgorithm = %q, want %q", snapshot.FingerprintAlgorithm, FingerprintAlgorithm)
	}
	if snapshot.Hostname != "auren-node-01" || snapshot.HostnameSource != HostnameSourceOS {
		t.Fatalf("unexpected hostname fields: %+v", snapshot)
	}
	if snapshot.Persistence != StorePersistencePersistent || snapshot.StoreSource != "created" || snapshot.StorePath == "" {
		t.Fatalf("unexpected store fields: %+v", snapshot)
	}
}
