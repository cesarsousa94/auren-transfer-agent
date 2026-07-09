package dispatcher

import (
	"testing"
	"time"

	"github.com/auren/auren-transfer-agent/internal/worker"
)

func retryTestJob(t *testing.T, maxAttempts int) worker.Job {
	t.Helper()
	job, err := worker.NewJob(worker.JobInput{SourceURL: "https://example.com/retry.ts", DestinationKey: "retry.ts", MaxAttempts: maxAttempts})
	if err != nil {
		t.Fatalf("NewJob returned error: %v", err)
	}
	return job
}

func TestAttemptsRetryPolicyRetriesFailedJobBelowMaxAttempts(t *testing.T) {
	failed, err := retryTestJob(t, 3).WithAttemptStatus(worker.JobStatusFailed, time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("WithAttemptStatus returned error: %v", err)
	}
	decision, err := NewAttemptsRetryPolicy().Decide(failed, time.Date(2026, 7, 9, 10, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if !decision.Retry || decision.Reason != RetryReasonEligible {
		t.Fatalf("unexpected decision: %#v", decision)
	}
	if decision.Job.Status != worker.JobStatusPending || decision.Job.Attempt != 1 || decision.Job.Metadata["retry_attempt"] != "2" {
		t.Fatalf("retry job not prepared correctly: %#v", decision.Job)
	}
}

func TestAttemptsRetryPolicyStopsAtMaxAttempts(t *testing.T) {
	failed, err := retryTestJob(t, 1).WithAttemptStatus(worker.JobStatusFailed, time.Now().UTC())
	if err != nil {
		t.Fatalf("WithAttemptStatus returned error: %v", err)
	}
	decision, err := NewAttemptsRetryPolicy().Decide(failed, time.Now().UTC())
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if decision.Retry || decision.Reason != RetryReasonMaxAttemptsReached {
		t.Fatalf("expected max attempts stop, got %#v", decision)
	}
}

func TestAttemptsRetryPolicyIgnoresNonFailedJob(t *testing.T) {
	job := retryTestJob(t, 2)
	decision, err := NewAttemptsRetryPolicy().Decide(job, time.Now().UTC())
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if decision.Retry || decision.Reason != RetryReasonJobNotFailed {
		t.Fatalf("expected non-failed decision, got %#v", decision)
	}
}
