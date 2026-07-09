package download

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunWithRetrySucceedsAfterFailure(t *testing.T) {
	policy, err := NewRetryPolicy(RetryOptions{MaxRetries: 2, Backoff: "0s"})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	attempts := 0
	result, err := RunWithRetry(context.Background(), policy, func(ctx context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", errors.New("temporary")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if result.Value != "ok" || result.Attempts != 2 || result.Retries != 1 || len(result.Failed) != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunWithRetryReturnsFinalError(t *testing.T) {
	policy, err := NewRetryPolicy(RetryOptions{MaxRetries: 1, Backoff: "0s"})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	result, err := RunWithRetry(context.Background(), policy, func(ctx context.Context) (int, error) {
		return 0, errors.New("boom")
	})
	if err == nil {
		t.Fatalf("expected final error")
	}
	if result.Attempts != 2 || result.Retries != 1 || len(result.Failed) != 2 || result.FinalError != "boom" {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunWithRetryHonorsContext(t *testing.T) {
	policy, err := NewRetryPolicy(RetryOptions{MaxRetries: 3, Backoff: "1h"})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := RunWithRetry(ctx, policy, func(ctx context.Context) (int, error) {
		return 0, errors.New("temporary")
	})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got result=%+v err=%v", result, err)
	}
}

func TestNewRetryPolicyValidation(t *testing.T) {
	if _, err := NewRetryPolicy(RetryOptions{MaxRetries: -1, Backoff: "0s"}); err == nil {
		t.Fatalf("expected max retries error")
	}
	if _, err := NewRetryPolicy(RetryOptions{MaxRetries: 1, Backoff: "bad"}); err == nil {
		t.Fatalf("expected backoff error")
	}
	policy, err := NewRetryPolicyFromConfig(3, "2s")
	if err != nil {
		t.Fatalf("from config: %v", err)
	}
	if policy.MaxRetries() != 3 || policy.Backoff() != 2*time.Second {
		t.Fatalf("policy = retries:%d backoff:%s", policy.MaxRetries(), policy.Backoff())
	}
}
