package transfer

import "sync"

// Stats exposes executor capacity for Media Hub claim/heartbeat payloads.
type Stats struct {
	MaxConcurrentJobs int `json:"max_concurrent_jobs"`
	ActiveJobs        int `json:"active_jobs"`
	CompletedJobs     int `json:"completed_jobs"`
	FailedJobs        int `json:"failed_jobs"`
}

// Tracker tracks active and terminal jobs in memory.
type Tracker struct {
	mu                sync.Mutex
	maxConcurrentJobs int
	active            map[string]struct{}
	completed         int
	failed            int
}

// NewTracker creates an in-memory stats tracker.
func NewTracker(maxConcurrentJobs int) *Tracker {
	if maxConcurrentJobs <= 0 {
		maxConcurrentJobs = 1
	}
	return &Tracker{maxConcurrentJobs: maxConcurrentJobs, active: map[string]struct{}{}}
}

// TryStart marks a job active when capacity is available.
func (tracker *Tracker) TryStart(uuid string) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if len(tracker.active) >= tracker.maxConcurrentJobs {
		return false
	}
	tracker.active[uuid] = struct{}{}
	return true
}

// Finish marks a job inactive and increments success/failure counters.
func (tracker *Tracker) Finish(uuid string, success bool) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	delete(tracker.active, uuid)
	if success {
		tracker.completed++
	} else {
		tracker.failed++
	}
}

// Snapshot returns a defensive stats snapshot.
func (tracker *Tracker) Snapshot() Stats {
	if tracker == nil {
		return Stats{}
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	return Stats{MaxConcurrentJobs: tracker.maxConcurrentJobs, ActiveJobs: len(tracker.active), CompletedJobs: tracker.completed, FailedJobs: tracker.failed}
}
