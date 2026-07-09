package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
)

func TestRuntimeRejectsClaimsDuringDrain(t *testing.T) {
	dir := t.TempDir()
	runtime := NewRuntime(Options{Config: config.MediaHubConfig{DrainEnabled: true, DrainFile: filepath.Join(dir, "drain"), BackpressureEnabled: true, WorkDir: dir}})
	runtime.RequestDrain("scale-in")
	decision := runtime.CanClaim(ClaimSnapshot{ActiveJobs: 0, MaxConcurrentJobs: 2, WorkDir: dir})
	if decision.Allowed {
		t.Fatalf("expected drain to reject claims")
	}
	if decision.Reason != DecisionDrain {
		t.Fatalf("expected drain reason, got %s", decision.Reason)
	}
}

func TestRuntimeRejectsDiskPressure(t *testing.T) {
	dir := t.TempDir()
	runtime := NewRuntime(Options{Config: config.MediaHubConfig{DiskGuardEnabled: true, DiskMinFreeBytes: 9223372036854775807, WorkDir: dir}})
	decision := runtime.CanClaim(ClaimSnapshot{ActiveJobs: 0, MaxConcurrentJobs: 2, WorkDir: dir})
	if decision.Allowed {
		t.Fatalf("expected disk pressure to reject claims")
	}
	if decision.Reason != DecisionDisk {
		t.Fatalf("expected disk pressure reason, got %s", decision.Reason)
	}
}

func TestRuntimeRejectsSessionAndEgressBackpressure(t *testing.T) {
	runtime := NewRuntime(Options{Config: config.MediaHubConfig{BackpressureEnabled: true, MaxSessions: 1, MaxEgressMbps: 100}})
	decision := runtime.CanAcceptGatewaySession(ClaimSnapshot{ActiveSessions: 1, MaxSessions: 1})
	if decision.Allowed || decision.Reason != DecisionSessions {
		t.Fatalf("expected session pressure, got %+v", decision)
	}
	decision = runtime.CanAcceptGatewaySession(ClaimSnapshot{ActiveSessions: 0, MaxSessions: 1, CurrentEgressMbps: 100, MaxEgressMbps: 100})
	if decision.Allowed || decision.Reason != DecisionEgress {
		t.Fatalf("expected egress pressure, got %+v", decision)
	}
}

func TestRuntimeStoresDeadLetter(t *testing.T) {
	dir := t.TempDir()
	runtime := NewRuntime(Options{Config: config.MediaHubConfig{DeadLetterEnabled: true, DeadLetterDir: dir}, Now: func() time.Time { return time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC) }})
	path, err := runtime.StoreDeadLetter(context.Background(), DeadLetter{JobUUID: "job-1", Operation: "remote_download", Stage: "download", Error: "boom", Retryable: true})
	if err != nil {
		t.Fatalf("store dead letter: %v", err)
	}
	if path == "" {
		t.Fatalf("expected path")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dead letter: %v", err)
	}
	if !strings.Contains(string(payload), "job-1") || !strings.Contains(string(payload), "boom") {
		t.Fatalf("unexpected payload: %s", string(payload))
	}
}

func contains(value, needle string) bool {
	return len(needle) == 0 || (len(value) >= len(needle) && filepath.Base(value) != "__never__" && stringIndex(value, needle) >= 0)
}

func stringIndex(value, needle string) int {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
