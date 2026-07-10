package transfer

import (
	"sort"
	"sync"
	"time"
)

// Stats exposes executor capacity for Media Hub claim/heartbeat payloads and the local Dev Console.
type Stats struct {
	MaxConcurrentJobs int         `json:"max_concurrent_jobs"`
	ActiveJobs        int         `json:"active_jobs"`
	CompletedJobs     int         `json:"completed_jobs"`
	FailedJobs        int         `json:"failed_jobs"`
	ActiveJobDetails  []ActiveJob `json:"active_job_details,omitempty"`
}

// ActiveJob describes one in-flight transfer for the local Dev Console.
type ActiveJob struct {
	UUID              string    `json:"uuid"`
	Operation         string    `json:"operation,omitempty"`
	Stage             string    `json:"stage,omitempty"`
	Message           string    `json:"message,omitempty"`
	SourceURL         string    `json:"source_url,omitempty"`
	DestinationDriver string    `json:"destination_driver,omitempty"`
	ObjectPath        string    `json:"object_path,omitempty"`
	CurrentBytes      int64     `json:"current_bytes"`
	TotalBytes        int64     `json:"total_bytes"`
	SpeedBps          int64     `json:"speed_bps"`
	Percent           float64   `json:"percent"`
	StartedAt         time.Time `json:"started_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Tracker tracks active and terminal jobs in memory.
type Tracker struct {
	mu                sync.Mutex
	maxConcurrentJobs int
	active            map[string]ActiveJob
	completed         int
	failed            int
}

// NewTracker creates an in-memory stats tracker.
func NewTracker(maxConcurrentJobs int) *Tracker {
	if maxConcurrentJobs <= 0 {
		maxConcurrentJobs = 1
	}
	return &Tracker{maxConcurrentJobs: maxConcurrentJobs, active: map[string]ActiveJob{}}
}

// TryStart marks a job active when capacity is available.
func (tracker *Tracker) TryStart(uuid string) bool {
	return tracker.TryStartJob(ActiveJob{UUID: uuid})
}

// TryStartJob marks a detailed job active when capacity is available.
func (tracker *Tracker) TryStartJob(job ActiveJob) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if len(tracker.active) >= tracker.maxConcurrentJobs {
		return false
	}
	now := time.Now().UTC()
	if job.StartedAt.IsZero() {
		job.StartedAt = now
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = job.StartedAt
	}
	tracker.active[job.UUID] = job
	return true
}

// UpdateJob updates the details of an active job.
func (tracker *Tracker) UpdateJob(uuid string, update ActiveJob) {
	if tracker == nil || uuid == "" {
		return
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	current, ok := tracker.active[uuid]
	if !ok {
		return
	}
	if update.Operation != "" {
		current.Operation = update.Operation
	}
	if update.Stage != "" {
		current.Stage = update.Stage
	}
	if update.Message != "" {
		current.Message = update.Message
	}
	if update.SourceURL != "" {
		current.SourceURL = update.SourceURL
	}
	if update.DestinationDriver != "" {
		current.DestinationDriver = update.DestinationDriver
	}
	if update.ObjectPath != "" {
		current.ObjectPath = update.ObjectPath
	}
	if update.CurrentBytes > 0 || update.TotalBytes > 0 || update.Percent > 0 || update.SpeedBps > 0 {
		current.CurrentBytes = update.CurrentBytes
		current.TotalBytes = update.TotalBytes
		current.Percent = update.Percent
		current.SpeedBps = update.SpeedBps
	}
	current.UpdatedAt = time.Now().UTC()
	tracker.active[uuid] = current
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
	active := make([]ActiveJob, 0, len(tracker.active))
	for _, job := range tracker.active {
		active = append(active, job)
	}
	sort.Slice(active, func(i, j int) bool { return active[i].StartedAt.Before(active[j].StartedAt) })
	return Stats{MaxConcurrentJobs: tracker.maxConcurrentJobs, ActiveJobs: len(tracker.active), CompletedJobs: tracker.completed, FailedJobs: tracker.failed, ActiveJobDetails: active}
}
