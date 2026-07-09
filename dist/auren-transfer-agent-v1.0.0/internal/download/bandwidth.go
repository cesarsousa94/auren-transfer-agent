package download

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	// BandwidthControllerName is the canonical name for download bandwidth control.
	BandwidthControllerName = "bandwidth"
)

// BandwidthOptions configures an optional mechanical bandwidth limiter.
type BandwidthOptions struct {
	// BytesPerSecond is the maximum transfer rate. Zero means unlimited.
	BytesPerSecond int64
}

// BandwidthController throttles writers using a byte-per-second ceiling.
type BandwidthController struct {
	bytesPerSecond int64
	now            func() time.Time
	sleep          func(time.Duration)
}

// NewBandwidthController validates and creates a bandwidth controller.
func NewBandwidthController(bytesPerSecond int64) (*BandwidthController, error) {
	return NewBandwidthControllerWithOptions(BandwidthOptions{BytesPerSecond: bytesPerSecond})
}

// NewBandwidthControllerWithOptions validates and creates a bandwidth controller.
func NewBandwidthControllerWithOptions(options BandwidthOptions) (*BandwidthController, error) {
	if options.BytesPerSecond < 0 {
		return nil, fmt.Errorf("download bandwidth limit must be zero or greater")
	}
	return &BandwidthController{bytesPerSecond: options.BytesPerSecond, now: time.Now, sleep: time.Sleep}, nil
}

// LimitBytesPerSecond returns the configured maximum rate. Zero means unlimited.
func (controller *BandwidthController) LimitBytesPerSecond() int64 {
	if controller == nil {
		return 0
	}
	return controller.bytesPerSecond
}

// Enabled reports whether throttling is active.
func (controller *BandwidthController) Enabled() bool {
	return controller != nil && controller.bytesPerSecond > 0
}

// DelayFor returns the additional delay needed to keep bytes within the configured rate.
func (controller *BandwidthController) DelayFor(bytes int64, elapsed time.Duration) time.Duration {
	if controller == nil || controller.bytesPerSecond <= 0 || bytes <= 0 {
		return 0
	}
	if elapsed < 0 {
		elapsed = 0
	}
	expected := time.Duration(float64(bytes) / float64(controller.bytesPerSecond) * float64(time.Second))
	if expected <= elapsed {
		return 0
	}
	return expected - elapsed
}

// Wait blocks until the configured rate allows the supplied byte count.
func (controller *BandwidthController) Wait(ctx context.Context, bytes int64, started time.Time) error {
	if controller == nil || !controller.Enabled() {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	delay := controller.DelayFor(bytes, controller.now().Sub(started))
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// WrapWriter returns a writer that applies throttling after successful writes.
func (controller *BandwidthController) WrapWriter(ctx context.Context, writer io.Writer) io.Writer {
	if controller == nil || !controller.Enabled() || writer == nil {
		return writer
	}
	return &bandwidthWriter{ctx: ctx, writer: writer, controller: controller, started: controller.now()}
}

type bandwidthWriter struct {
	ctx        context.Context
	writer     io.Writer
	controller *BandwidthController
	started    time.Time
	written    int64
	mu         sync.Mutex
}

func (writer *bandwidthWriter) Write(payload []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	written, err := writer.writer.Write(payload)
	if written > 0 {
		writer.written += int64(written)
	}
	if err != nil || written <= 0 {
		return written, err
	}
	if waitErr := writer.controller.Wait(writer.ctx, writer.written, writer.started); waitErr != nil {
		return written, waitErr
	}
	return written, nil
}
