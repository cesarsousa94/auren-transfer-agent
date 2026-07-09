package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// SessionInput describes one accepted gateway session.
type SessionInput struct {
	SessionID string
	Token     string
	Kind      string
	ID        string
	Extension string
	Mode      string
}

// SessionRecord is a point-in-time gateway session snapshot.
type SessionRecord struct {
	SessionID  string
	Token      string
	Kind       string
	ID         string
	Extension  string
	Mode       string
	StartedAt  time.Time
	LastSeenAt time.Time
	BytesSent  int64
}

// Stats describes gateway capacity and egress telemetry.
type Stats struct {
	ActiveSessions   int
	BytesSent        int64
	CurrentEgressBps int
}

// Tracker tracks active gateway sessions and aggregate byte counters.
type Tracker struct {
	mu              sync.RWMutex
	sessions        map[string]SessionRecord
	bytesSent       int64
	windowBytes     int64
	windowStartedAt time.Time
}

// NewTracker creates an empty gateway tracker.
func NewTracker() *Tracker {
	return &Tracker{sessions: map[string]SessionRecord{}, windowStartedAt: time.Now().UTC()}
}

// Start records a new active session.
func (tracker *Tracker) Start(input SessionInput) SessionRecord {
	if tracker == nil {
		return SessionRecord{}
	}
	sessionID := input.SessionID
	if sessionID == "" {
		sessionID = randomID()
	}
	now := time.Now().UTC()
	record := SessionRecord{SessionID: sessionID, Token: input.Token, Kind: input.Kind, ID: input.ID, Extension: input.Extension, Mode: input.Mode, StartedAt: now, LastSeenAt: now}
	tracker.mu.Lock()
	tracker.sessions[sessionID] = record
	tracker.mu.Unlock()
	return record
}

// AddBytes increments a session and aggregate byte counters.
func (tracker *Tracker) AddBytes(sessionID string, amount int64) {
	if tracker == nil || amount <= 0 {
		return
	}
	tracker.mu.Lock()
	record := tracker.sessions[sessionID]
	record.BytesSent += amount
	record.LastSeenAt = time.Now().UTC()
	tracker.sessions[sessionID] = record
	tracker.bytesSent += amount
	tracker.windowBytes += amount
	tracker.mu.Unlock()
}

// Snapshot returns one session record.
func (tracker *Tracker) Snapshot(sessionID string) SessionRecord {
	if tracker == nil {
		return SessionRecord{}
	}
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()
	return tracker.sessions[sessionID]
}

// Close removes a session and returns its latest snapshot.
func (tracker *Tracker) Close(sessionID string) SessionRecord {
	if tracker == nil {
		return SessionRecord{}
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	record := tracker.sessions[sessionID]
	delete(tracker.sessions, sessionID)
	return record
}

// Stats returns aggregate gateway counters.
func (tracker *Tracker) Stats() Stats {
	if tracker == nil {
		return Stats{}
	}
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()
	return Stats{ActiveSessions: len(tracker.sessions), BytesSent: tracker.bytesSent, CurrentEgressBps: tracker.currentEgressBpsLocked()}
}

// CurrentEgressBps returns the current approximate egress speed in bytes per second.
func (tracker *Tracker) CurrentEgressBps() int {
	if tracker == nil {
		return 0
	}
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()
	return tracker.currentEgressBpsLocked()
}

func (tracker *Tracker) currentEgressBpsLocked() int {
	elapsed := time.Since(tracker.windowStartedAt).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return int(float64(tracker.windowBytes) / elapsed)
}

func randomID() string {
	payload := make([]byte, 16)
	if _, err := rand.Read(payload); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(payload)
}
