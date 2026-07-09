// Package worker contains foundation job contracts for the Agent worker engine.
package worker

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/identity"
)

const (
	// JobTypeTransfer is the foundation technical job type accepted by the Agent.
	JobTypeTransfer JobType = "transfer"

	// JobStatusPending means the job has been created but not yet accepted by a queue.
	JobStatusPending JobStatus = "pending"

	// JobStatusQueued means the job is stored in a queue and waiting for a worker.
	JobStatusQueued JobStatus = "queued"

	// JobStatusRunning means a worker has claimed the job for execution.
	JobStatusRunning JobStatus = "running"

	// JobStatusSucceeded means the worker handler completed the mechanical job.
	JobStatusSucceeded JobStatus = "succeeded"

	// JobStatusFailed means the worker handler failed the mechanical job.
	JobStatusFailed JobStatus = "failed"
)

// JobType identifies the mechanical action a worker will eventually execute.
type JobType string

// JobStatus describes the foundation lifecycle state known by the worker engine.
type JobStatus string

// JobInput contains the minimum transfer job material accepted by the foundation model.
type JobInput struct {
	ID             string
	ExternalID     string
	Type           JobType
	Priority       int
	MaxAttempts    int
	SourceURL      string
	DestinationKey string
	Headers        map[string]string
	Metadata       map[string]string
	Now            time.Time
}

// Job is the canonical foundation worker job model.
//
// It is transport-agnostic and contains no Media Hub business decisions. The
// Agent only stores the mechanical transfer request that later worker phases
// will execute.
type Job struct {
	ID             string            `json:"id"`
	ExternalID     string            `json:"external_id,omitempty"`
	Type           JobType           `json:"type"`
	Status         JobStatus         `json:"status"`
	Priority       int               `json:"priority"`
	Attempt        int               `json:"attempt"`
	MaxAttempts    int               `json:"max_attempts"`
	SourceURL      string            `json:"source_url"`
	DestinationKey string            `json:"destination_key"`
	Headers        map[string]string `json:"headers,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// NewJob creates a validated foundation job with safe defaults.
func NewJob(input JobInput) (Job, error) {
	jobID := strings.TrimSpace(input.ID)
	if jobID == "" {
		generated, err := identity.NewUUID()
		if err != nil {
			return Job{}, err
		}
		jobID = generated
	} else {
		normalized, err := identity.NormalizeUUID(jobID)
		if err != nil {
			return Job{}, fmt.Errorf("job id: %w", err)
		}
		jobID = normalized
	}

	jobType := input.Type
	if strings.TrimSpace(string(jobType)) == "" {
		jobType = JobTypeTransfer
	}

	maxAttempts := input.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 1
	}

	now := input.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	job := Job{
		ID:             jobID,
		ExternalID:     strings.TrimSpace(input.ExternalID),
		Type:           jobType,
		Status:         JobStatusPending,
		Priority:       input.Priority,
		Attempt:        0,
		MaxAttempts:    maxAttempts,
		SourceURL:      strings.TrimSpace(input.SourceURL),
		DestinationKey: strings.TrimSpace(input.DestinationKey),
		Headers:        cloneStringMap(input.Headers),
		Metadata:       cloneStringMap(input.Metadata),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := ValidateJob(job); err != nil {
		return Job{}, err
	}
	return job, nil
}

// ValidateJob checks the structural job contract without applying business rules.
func ValidateJob(job Job) error {
	var issues []string

	if err := identity.ValidateUUID(job.ID); err != nil {
		issues = append(issues, "id must be a canonical UUID")
	}
	if !IsSupportedJobType(job.Type) {
		issues = append(issues, "type must be transfer")
	}
	if !IsSupportedJobStatus(job.Status) {
		issues = append(issues, "status must be pending, queued, running, succeeded or failed")
	}
	if job.Priority < 0 {
		issues = append(issues, "priority must be zero or greater")
	}
	if job.Attempt < 0 {
		issues = append(issues, "attempt must be zero or greater")
	}
	if job.MaxAttempts <= 0 {
		issues = append(issues, "max_attempts must be greater than zero")
	}
	if job.Attempt > job.MaxAttempts {
		issues = append(issues, "attempt cannot be greater than max_attempts")
	}
	if !isValidAbsoluteURL(job.SourceURL) {
		issues = append(issues, "source_url must be an absolute http or https URL")
	}
	if strings.TrimSpace(job.DestinationKey) == "" {
		issues = append(issues, "destination_key is required")
	}
	if job.CreatedAt.IsZero() {
		issues = append(issues, "created_at is required")
	}
	if job.UpdatedAt.IsZero() {
		issues = append(issues, "updated_at is required")
	}

	if len(issues) > 0 {
		return fmt.Errorf("job validation failed: %s", strings.Join(issues, "; "))
	}
	return nil
}

// Clone returns a defensive copy of the job and its maps.
func (job Job) Clone() Job {
	copy := job
	copy.Headers = cloneStringMap(job.Headers)
	copy.Metadata = cloneStringMap(job.Metadata)
	return copy
}

// WithStatus returns a copy of the job with a new lifecycle status.
func (job Job) WithStatus(status JobStatus, now time.Time) (Job, error) {
	updated := job.Clone()
	updated.Status = status
	if now.IsZero() {
		now = time.Now().UTC()
	}
	updated.UpdatedAt = now.UTC()
	if err := ValidateJob(updated); err != nil {
		return Job{}, err
	}
	return updated, nil
}

// WithAttemptStatus returns a copy with an incremented attempt and lifecycle status.
func (job Job) WithAttemptStatus(status JobStatus, now time.Time) (Job, error) {
	updated := job.Clone()
	updated.Attempt++
	updated.Status = status
	if now.IsZero() {
		now = time.Now().UTC()
	}
	updated.UpdatedAt = now.UTC()
	if err := ValidateJob(updated); err != nil {
		return Job{}, err
	}
	return updated, nil
}

// IsTerminalJobStatus reports whether the status ends a single worker execution.
func IsTerminalJobStatus(status JobStatus) bool {
	switch JobStatus(strings.TrimSpace(string(status))) {
	case JobStatusSucceeded, JobStatusFailed:
		return true
	default:
		return false
	}
}

// IsSupportedJobType reports whether the job type is part of the foundation contract.
func IsSupportedJobType(jobType JobType) bool {
	return JobType(strings.TrimSpace(string(jobType))) == JobTypeTransfer
}

// IsSupportedJobStatus reports whether the status is part of the foundation worker contract.
func IsSupportedJobStatus(status JobStatus) bool {
	switch JobStatus(strings.TrimSpace(string(status))) {
	case JobStatusPending, JobStatusQueued, JobStatusRunning, JobStatusSucceeded, JobStatusFailed:
		return true
	default:
		return false
	}
}

// SupportedJobStatuses returns a sorted defensive list of foundation statuses.
func SupportedJobStatuses() []JobStatus {
	statuses := []JobStatus{JobStatusPending, JobStatusQueued, JobStatusRunning, JobStatusSucceeded, JobStatusFailed}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i] < statuses[j] })
	return statuses
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		output[trimmedKey] = value
	}
	return output
}

func isValidAbsoluteURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil || parsed.Host == "" {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https"
}
