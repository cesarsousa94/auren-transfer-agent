// Package ops contains operational hardening primitives for production Agent nodes.
package ops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/auren/auren-transfer-agent/internal/config"
)

const (
	// RuntimeName identifies the operational hardening subsystem in diagnostics.
	RuntimeName = "operational_hardening_v1"

	DecisionAllowed  = "allowed"
	DecisionDrain    = "drain"
	DecisionCapacity = "capacity"
	DecisionDisk     = "disk_pressure"
	DecisionSessions = "session_pressure"
	DecisionEgress   = "egress_pressure"
)

// Options configures the runtime hardening helper.
type Options struct {
	Config config.MediaHubConfig
	Now    func() time.Time
}

// Runtime centralizes drain, backpressure, disk pressure and dead-letter behavior.
type Runtime struct {
	cfg         config.MediaHubConfig
	now         func() time.Time
	mu          sync.RWMutex
	manualDrain bool
	drainReason string
}

// ClaimSnapshot describes the node state used to decide whether a new job/session is safe.
type ClaimSnapshot struct {
	ActiveJobs        int
	MaxConcurrentJobs int
	ActiveSessions    int
	MaxSessions       int
	CurrentEgressMbps int
	MaxEgressMbps     int
	WorkDir           string
}

// Decision is returned by admission control checks.
type Decision struct {
	Allowed      bool   `json:"allowed"`
	Reason       string `json:"reason"`
	Message      string `json:"message,omitempty"`
	FreeBytes    uint64 `json:"free_bytes,omitempty"`
	MinFreeBytes int64  `json:"min_free_bytes,omitempty"`
}

// DeadLetter captures one failed job attempt for local recovery/audit.
type DeadLetter struct {
	ID        string    `json:"id"`
	JobUUID   string    `json:"job_uuid"`
	Operation string    `json:"operation,omitempty"`
	Stage     string    `json:"stage"`
	Error     string    `json:"error"`
	Retryable bool      `json:"retryable"`
	Payload   any       `json:"payload,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// NewRuntime creates a hardening runtime.
func NewRuntime(options Options) *Runtime {
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Runtime{cfg: options.Config, now: now}
}

// Name returns the stable subsystem name.
func (runtime *Runtime) Name() string { return RuntimeName }

// Enabled reports whether any operational hardening feature is enabled.
func (runtime *Runtime) Enabled() bool {
	if runtime == nil {
		return false
	}
	return runtime.cfg.DrainEnabled || runtime.cfg.BackpressureEnabled || runtime.cfg.DiskGuardEnabled || runtime.cfg.DeadLetterEnabled || runtime.cfg.LeaseRenewalEnabled || runtime.cfg.SecretRotationEnabled
}

// RequestDrain switches the runtime into drain mode without requiring a marker file.
func (runtime *Runtime) RequestDrain(reason string) {
	if runtime == nil {
		return
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.manualDrain = true
	runtime.drainReason = strings.TrimSpace(reason)
	if runtime.drainReason == "" {
		runtime.drainReason = "manual"
	}
}

// ClearDrain clears the in-process drain flag. A drain marker file still takes precedence.
func (runtime *Runtime) ClearDrain() {
	if runtime == nil {
		return
	}
	runtime.mu.Lock()
	runtime.manualDrain = false
	runtime.drainReason = ""
	runtime.mu.Unlock()
}

// DrainRequested reports whether the node must stop accepting new jobs/sessions.
func (runtime *Runtime) DrainRequested() (bool, string) {
	if runtime == nil || !runtime.cfg.DrainEnabled {
		return false, ""
	}
	runtime.mu.RLock()
	manual := runtime.manualDrain
	reason := runtime.drainReason
	runtime.mu.RUnlock()
	if manual {
		return true, firstNonEmpty(reason, "manual")
	}
	marker := strings.TrimSpace(runtime.cfg.DrainFile)
	if marker != "" {
		if payload, err := os.ReadFile(marker); err == nil {
			text := strings.TrimSpace(string(payload))
			return true, firstNonEmpty(text, "marker_file")
		}
	}
	return false, ""
}

// CanClaim applies drain, disk and capacity backpressure before a Media Hub job claim.
func (runtime *Runtime) CanClaim(snapshot ClaimSnapshot) Decision {
	if runtime == nil {
		return Decision{Allowed: true, Reason: DecisionAllowed}
	}
	if drain, reason := runtime.DrainRequested(); drain {
		return Decision{Allowed: false, Reason: DecisionDrain, Message: firstNonEmpty(reason, "node is draining")}
	}
	if snapshot.MaxConcurrentJobs > 0 && snapshot.ActiveJobs >= snapshot.MaxConcurrentJobs {
		return Decision{Allowed: false, Reason: DecisionCapacity, Message: "max concurrent jobs reached"}
	}
	if runtime.cfg.BackpressureEnabled {
		maxSessions := firstPositive(snapshot.MaxSessions, runtime.cfg.MaxSessions)
		if maxSessions > 0 && snapshot.ActiveSessions >= maxSessions {
			return Decision{Allowed: false, Reason: DecisionSessions, Message: "max active sessions reached"}
		}
		maxEgress := firstPositive(snapshot.MaxEgressMbps, runtime.cfg.MaxEgressMbps)
		if maxEgress > 0 && snapshot.CurrentEgressMbps >= maxEgress {
			return Decision{Allowed: false, Reason: DecisionEgress, Message: "max egress mbps reached"}
		}
	}
	if runtime.cfg.DiskGuardEnabled {
		dir := firstNonEmpty(snapshot.WorkDir, runtime.cfg.WorkDir, ".")
		disk, err := DiskFreeBytes(dir)
		if err != nil {
			return Decision{Allowed: false, Reason: DecisionDisk, Message: err.Error()}
		}
		if runtime.cfg.DiskMinFreeBytes > 0 && int64(disk) < runtime.cfg.DiskMinFreeBytes {
			return Decision{Allowed: false, Reason: DecisionDisk, Message: "free disk below configured minimum", FreeBytes: disk, MinFreeBytes: runtime.cfg.DiskMinFreeBytes}
		}
	}
	return Decision{Allowed: true, Reason: DecisionAllowed}
}

// CanAcceptGatewaySession applies drain/session/egress checks before a public stream is accepted.
func (runtime *Runtime) CanAcceptGatewaySession(snapshot ClaimSnapshot) Decision {
	if runtime == nil {
		return Decision{Allowed: true, Reason: DecisionAllowed}
	}
	if drain, reason := runtime.DrainRequested(); drain {
		return Decision{Allowed: false, Reason: DecisionDrain, Message: firstNonEmpty(reason, "node is draining")}
	}
	if runtime.cfg.BackpressureEnabled {
		maxSessions := firstPositive(snapshot.MaxSessions, runtime.cfg.MaxSessions)
		if maxSessions > 0 && snapshot.ActiveSessions >= maxSessions {
			return Decision{Allowed: false, Reason: DecisionSessions, Message: "max active sessions reached"}
		}
		maxEgress := firstPositive(snapshot.MaxEgressMbps, runtime.cfg.MaxEgressMbps)
		if maxEgress > 0 && snapshot.CurrentEgressMbps >= maxEgress {
			return Decision{Allowed: false, Reason: DecisionEgress, Message: "max egress mbps reached"}
		}
	}
	return Decision{Allowed: true, Reason: DecisionAllowed}
}

// StoreDeadLetter persists a failed attempt locally when configured.
func (runtime *Runtime) StoreDeadLetter(ctx context.Context, letter DeadLetter) (string, error) {
	_ = ctx
	if runtime == nil || !runtime.cfg.DeadLetterEnabled {
		return "", nil
	}
	dir := strings.TrimSpace(runtime.cfg.DeadLetterDir)
	if dir == "" {
		dir = filepath.Join(firstNonEmpty(runtime.cfg.WorkDir, "./data/transfer"), "dead-letter")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if strings.TrimSpace(letter.ID) == "" {
		letter.ID = randomHex(12)
	}
	if letter.CreatedAt.IsZero() {
		letter.CreatedAt = runtime.now().UTC()
	}
	name := fmt.Sprintf("%s-%s.json", letter.CreatedAt.UTC().Format("20060102T150405.000000000Z"), safeFileName(firstNonEmpty(letter.JobUUID, letter.ID)))
	path := filepath.Join(dir, name)
	payload, err := json.MarshalIndent(letter, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// DiskFreeBytes returns available bytes for the filesystem containing path.
func DiskFreeBytes(path string) (uint64, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		target = "."
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return 0, err
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(target, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

// Snapshot returns a diagnostic map for heartbeat/event metadata.
func (runtime *Runtime) Snapshot(snapshot ClaimSnapshot) map[string]any {
	decision := runtime.CanClaim(snapshot)
	drain, drainReason := runtime.DrainRequested()
	return map[string]any{
		"runtime":                 RuntimeName,
		"enabled":                 runtime.Enabled(),
		"drain_requested":         drain,
		"drain_reason":            drainReason,
		"claim_allowed":           decision.Allowed,
		"claim_reason":            decision.Reason,
		"disk_guard_enabled":      runtime.cfg.DiskGuardEnabled,
		"dead_letter_enabled":     runtime.cfg.DeadLetterEnabled,
		"lease_renewal_enabled":   runtime.cfg.LeaseRenewalEnabled,
		"backpressure_enabled":    runtime.cfg.BackpressureEnabled,
		"secret_rotation_enabled": runtime.cfg.SecretRotationEnabled,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func randomHex(size int) string {
	payload := make([]byte, size)
	if _, err := rand.Read(payload); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(payload)
}

func safeFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return replacer.Replace(value)
}
