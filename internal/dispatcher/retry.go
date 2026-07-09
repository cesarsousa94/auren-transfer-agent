// Package dispatcher coordinates worker-pool execution cycles.
package dispatcher

import (
	"fmt"
	"strings"
	"time"

	"github.com/auren/auren-transfer-agent/internal/worker"
)

const (
	// RetryReasonMaxAttemptsReached marks a failed job that cannot be retried again.
	RetryReasonMaxAttemptsReached = "max_attempts_reached"

	// RetryReasonJobNotFailed marks a non-failed job that is not eligible for retry.
	RetryReasonJobNotFailed = "job_not_failed"

	// RetryReasonEligible marks a failed job that can be requeued mechanically.
	RetryReasonEligible = "eligible"
)

// RetryDecision describes whether a terminal worker result should be requeued.
type RetryDecision struct {
	Retry  bool
	Reason string
	Job    worker.Job
}

// RetryPolicy decides purely mechanical retry eligibility.
type RetryPolicy interface {
	Decide(worker.Job, time.Time) (RetryDecision, error)
}

// AttemptsRetryPolicy retries failed jobs while attempt < max_attempts.
type AttemptsRetryPolicy struct{}

// NewAttemptsRetryPolicy returns the foundation retry policy.
func NewAttemptsRetryPolicy() AttemptsRetryPolicy {
	return AttemptsRetryPolicy{}
}

// Decide returns a retry decision without any Media Hub business rule.
func (AttemptsRetryPolicy) Decide(job worker.Job, now time.Time) (RetryDecision, error) {
	if err := worker.ValidateJob(job); err != nil {
		return RetryDecision{}, err
	}
	if job.Status != worker.JobStatusFailed {
		return RetryDecision{Retry: false, Reason: RetryReasonJobNotFailed, Job: job.Clone()}, nil
	}
	if job.Attempt >= job.MaxAttempts {
		return RetryDecision{Retry: false, Reason: RetryReasonMaxAttemptsReached, Job: job.Clone()}, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	retryJob := job.Clone()
	retryJob.Status = worker.JobStatusPending
	retryJob.UpdatedAt = now.UTC()
	if retryJob.Metadata == nil {
		retryJob.Metadata = map[string]string{}
	}
	retryJob.Metadata["retry_reason"] = RetryReasonEligible
	retryJob.Metadata["retry_attempt"] = fmt.Sprintf("%d", retryJob.Attempt+1)
	if err := worker.ValidateJob(retryJob); err != nil {
		return RetryDecision{}, err
	}
	return RetryDecision{Retry: true, Reason: RetryReasonEligible, Job: retryJob}, nil
}

// NormalizeRetryReason trims and canonicalizes retry reason values.
func NormalizeRetryReason(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
