package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFixedIntervalRunOnceExecutesTask(t *testing.T) {
	called := 0
	scheduler, err := NewFixedInterval(time.Millisecond, func(context.Context) error {
		called++
		return nil
	})
	if err != nil {
		t.Fatalf("NewFixedInterval returned error: %v", err)
	}
	result := scheduler.RunOnce(context.Background())
	if result.Error != "" || called != 1 || result.Duration < 0 {
		t.Fatalf("unexpected run result: %#v called=%d", result, called)
	}
}

func TestFixedIntervalRunOnceCapturesError(t *testing.T) {
	expected := errors.New("tick failed")
	scheduler, err := NewFixedInterval(time.Millisecond, func(context.Context) error { return expected })
	if err != nil {
		t.Fatalf("NewFixedInterval returned error: %v", err)
	}
	result := scheduler.RunOnce(context.Background())
	if result.Error != expected.Error() {
		t.Fatalf("expected captured error, got %#v", result)
	}
}

func TestFixedIntervalStartStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	scheduler, err := NewFixedInterval(time.Millisecond, func(context.Context) error {
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("NewFixedInterval returned error: %v", err)
	}
	results, err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	select {
	case _, ok := <-results:
		if !ok {
			t.Fatalf("results channel closed before first tick")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out waiting for scheduler tick")
	}
}

func TestNewFixedIntervalRejectsInvalidOptions(t *testing.T) {
	if _, err := NewFixedInterval(0, func(context.Context) error { return nil }); err == nil {
		t.Fatalf("expected invalid interval error")
	}
	if _, err := NewFixedInterval(time.Second, nil); err == nil {
		t.Fatalf("expected nil task error")
	}
}
