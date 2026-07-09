package download

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestBandwidthControllerUnlimitedHasNoDelay(t *testing.T) {
	controller, err := NewBandwidthController(0)
	if err != nil {
		t.Fatalf("controller: %v", err)
	}
	if controller.Enabled() {
		t.Fatalf("unlimited controller should be disabled")
	}
	if delay := controller.DelayFor(1024, 0); delay != 0 {
		t.Fatalf("delay = %s", delay)
	}
}

func TestBandwidthControllerCalculatesDelay(t *testing.T) {
	controller, err := NewBandwidthController(100)
	if err != nil {
		t.Fatalf("controller: %v", err)
	}
	if !controller.Enabled() || controller.LimitBytesPerSecond() != 100 {
		t.Fatalf("unexpected controller state")
	}
	if delay := controller.DelayFor(200, 500*time.Millisecond); delay != 1500*time.Millisecond {
		t.Fatalf("delay = %s", delay)
	}
}

func TestBandwidthControllerRejectsNegativeLimit(t *testing.T) {
	_, err := NewBandwidthController(-1)
	if err == nil {
		t.Fatalf("expected negative limit error")
	}
}

func TestBandwidthWriterPropagatesContextCancellation(t *testing.T) {
	controller, err := NewBandwidthController(1)
	if err != nil {
		t.Fatalf("controller: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	writer := controller.WrapWriter(ctx, &output)
	written, err := writer.Write([]byte("abc"))
	if written != 3 || err == nil {
		t.Fatalf("written=%d err=%v", written, err)
	}
}
