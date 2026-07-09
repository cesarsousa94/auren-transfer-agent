package download

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// DownloadMetricsName is the canonical name for download metrics support.
	DownloadMetricsName = "download_metrics"
)

// DownloadMetric describes one mechanical download operation observation.
type DownloadMetric struct {
	Engine              string        `json:"engine"`
	URL                 string        `json:"url"`
	StatusCode          int           `json:"status_code"`
	BytesWritten        int64         `json:"bytes_written"`
	ContentLength       int64         `json:"content_length"`
	Duration            time.Duration `json:"duration"`
	StartedAt           time.Time     `json:"started_at"`
	CompletedAt         time.Time     `json:"completed_at"`
	Resumed             bool          `json:"resumed"`
	RangeHeader         string        `json:"range_header,omitempty"`
	MultipartParts      int           `json:"multipart_parts,omitempty"`
	ChecksumAlgorithm   string        `json:"checksum_algorithm,omitempty"`
	BandwidthLimited    bool          `json:"bandwidth_limited"`
	BandwidthBytesLimit int64         `json:"bandwidth_bytes_limit"`
	Error               string        `json:"error,omitempty"`
}

// DownloadSummary aggregates metrics held by an in-memory recorder.
type DownloadSummary struct {
	Count        int   `json:"count"`
	Succeeded    int   `json:"succeeded"`
	Failed       int   `json:"failed"`
	BytesWritten int64 `json:"bytes_written"`
}

// MetricsRecorder receives download metrics.
type MetricsRecorder interface {
	RecordDownloadMetric(context.Context, DownloadMetric) error
}

// MemoryMetricsRecorder stores download metrics in memory for local diagnostics and tests.
type MemoryMetricsRecorder struct {
	mu      sync.Mutex
	metrics []DownloadMetric
}

// NewMemoryMetricsRecorder creates an empty in-memory metrics recorder.
func NewMemoryMetricsRecorder() *MemoryMetricsRecorder {
	return &MemoryMetricsRecorder{}
}

// RecordDownloadMetric stores a defensive metric copy.
func (recorder *MemoryMetricsRecorder) RecordDownloadMetric(_ context.Context, metric DownloadMetric) error {
	if recorder == nil {
		return fmt.Errorf("download metrics recorder cannot be nil")
	}
	if err := ValidateDownloadMetric(metric); err != nil {
		return err
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.metrics = append(recorder.metrics, metric)
	return nil
}

// Snapshot returns a defensive copy of recorded metrics.
func (recorder *MemoryMetricsRecorder) Snapshot() []DownloadMetric {
	if recorder == nil {
		return nil
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	metrics := make([]DownloadMetric, len(recorder.metrics))
	copy(metrics, recorder.metrics)
	return metrics
}

// Count returns the number of recorded metrics.
func (recorder *MemoryMetricsRecorder) Count() int {
	if recorder == nil {
		return 0
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return len(recorder.metrics)
}

// Summary returns aggregate counters for recorded metrics.
func (recorder *MemoryMetricsRecorder) Summary() DownloadSummary {
	snapshot := recorder.Snapshot()
	summary := DownloadSummary{Count: len(snapshot)}
	for _, metric := range snapshot {
		summary.BytesWritten += metric.BytesWritten
		if metric.Error == "" {
			summary.Succeeded++
		} else {
			summary.Failed++
		}
	}
	return summary
}

// ValidateDownloadMetric validates a metric without applying business rules.
func ValidateDownloadMetric(metric DownloadMetric) error {
	if metric.Engine == "" {
		return fmt.Errorf("download metric engine is required")
	}
	if metric.Duration < 0 {
		return fmt.Errorf("download metric duration must be zero or greater")
	}
	if metric.BytesWritten < 0 {
		return fmt.Errorf("download metric bytes written must be zero or greater")
	}
	if metric.ContentLength < -1 {
		return fmt.Errorf("download metric content length must be -1 or greater")
	}
	if metric.BandwidthBytesLimit < 0 {
		return fmt.Errorf("download metric bandwidth limit must be zero or greater")
	}
	return nil
}

func recordDownloadMetric(ctx context.Context, recorder MetricsRecorder, metric DownloadMetric) {
	if recorder == nil {
		return
	}
	if metric.CompletedAt.IsZero() {
		metric.CompletedAt = time.Now()
	}
	if metric.StartedAt.IsZero() {
		metric.StartedAt = metric.CompletedAt.Add(-metric.Duration)
	}
	_ = recorder.RecordDownloadMetric(ctx, metric)
}
