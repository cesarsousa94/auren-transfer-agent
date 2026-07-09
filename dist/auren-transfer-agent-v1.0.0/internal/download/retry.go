package download

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// RetryEngineName is the canonical name for download retry support.
	RetryEngineName = "retry"
)

// RetryOptions configures mechanical retry behavior for download attempts.
type RetryOptions struct {
	MaxRetries int
	Backoff    string
}

// RetryPolicy decides how many attempts a download operation can make.
type RetryPolicy struct {
	maxRetries int
	backoff    time.Duration
}

// RetryAttempt describes one failed attempt.
type RetryAttempt struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

// RetryResult describes the final result of a retry loop.
type RetryResult[T any] struct {
	Value      T              `json:"value"`
	Attempts   int            `json:"attempts"`
	Retries    int            `json:"retries"`
	Failed     []RetryAttempt `json:"failed,omitempty"`
	Duration   time.Duration  `json:"duration"`
	FinalError string         `json:"final_error,omitempty"`
}

// Operation is a retryable download operation.
type Operation[T any] func(context.Context) (T, error)

// NewRetryPolicy validates and builds a retry policy.
func NewRetryPolicy(options RetryOptions) (RetryPolicy, error) {
	if options.MaxRetries < 0 {
		return RetryPolicy{}, fmt.Errorf("download max retries must be zero or greater")
	}
	backoff, err := parseNonNegativeDuration("download retry backoff", options.Backoff)
	if err != nil {
		return RetryPolicy{}, err
	}
	return RetryPolicy{maxRetries: options.MaxRetries, backoff: backoff}, nil
}

// NewRetryPolicyFromConfig derives retry behavior from download configuration values.
func NewRetryPolicyFromConfig(maxRetries int, backoff string) (RetryPolicy, error) {
	return NewRetryPolicy(RetryOptions{MaxRetries: maxRetries, Backoff: backoff})
}

// MaxRetries returns the configured retry count.
func (policy RetryPolicy) MaxRetries() int {
	return policy.maxRetries
}

// Backoff returns the configured delay between attempts.
func (policy RetryPolicy) Backoff() time.Duration {
	return policy.backoff
}

// Run executes operation until it succeeds, context is cancelled or retries are exhausted.
func RunWithRetry[T any](ctx context.Context, policy RetryPolicy, operation Operation[T]) (RetryResult[T], error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	if operation == nil {
		return RetryResult[T]{Value: zero}, fmt.Errorf("retry operation is required")
	}
	started := time.Now()
	result := RetryResult[T]{Value: zero}
	maxAttempts := policy.maxRetries + 1
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		value, err := operation(ctx)
		result.Attempts = attempt
		result.Value = value
		if err == nil {
			result.Retries = attempt - 1
			result.Duration = time.Since(started)
			return result, nil
		}
		result.Failed = append(result.Failed, RetryAttempt{Index: attempt, Error: err.Error()})
		if attempt == maxAttempts {
			result.Retries = attempt - 1
			result.Duration = time.Since(started)
			result.FinalError = err.Error()
			return result, err
		}
		if err := sleepWithContext(ctx, policy.backoff); err != nil {
			result.Retries = attempt - 1
			result.Duration = time.Since(started)
			result.FinalError = err.Error()
			return result, err
		}
	}
	return result, nil
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseNonNegativeDuration(field string, value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("%s is required", field)
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration", field)
	}
	if duration < 0 {
		return 0, fmt.Errorf("%s must be zero or greater", field)
	}
	return duration, nil
}
