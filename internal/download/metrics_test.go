package download

import (
	"context"
	"testing"
	"time"
)

func TestMemoryMetricsRecorderRecordsDefensiveSnapshot(t *testing.T) {
	recorder := NewMemoryMetricsRecorder()
	metric := DownloadMetric{Engine: StreamingEngineName, URL: "https://example.test/file", StatusCode: 200, BytesWritten: 10, ContentLength: 10, Duration: time.Millisecond}
	if err := recorder.RecordDownloadMetric(context.Background(), metric); err != nil {
		t.Fatalf("record: %v", err)
	}
	snapshot := recorder.Snapshot()
	if len(snapshot) != 1 || snapshot[0].BytesWritten != 10 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	snapshot[0].BytesWritten = 999
	if recorder.Snapshot()[0].BytesWritten == 999 {
		t.Fatalf("snapshot mutated recorder")
	}
}

func TestMemoryMetricsRecorderSummary(t *testing.T) {
	recorder := NewMemoryMetricsRecorder()
	_ = recorder.RecordDownloadMetric(context.Background(), DownloadMetric{Engine: StreamingEngineName, BytesWritten: 10})
	_ = recorder.RecordDownloadMetric(context.Background(), DownloadMetric{Engine: MultipartEngineName, BytesWritten: 5, Error: "boom"})
	summary := recorder.Summary()
	if summary.Count != 2 || summary.Succeeded != 1 || summary.Failed != 1 || summary.BytesWritten != 15 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestValidateDownloadMetricRejectsInvalidPayload(t *testing.T) {
	if err := ValidateDownloadMetric(DownloadMetric{}); err == nil {
		t.Fatalf("expected engine error")
	}
	if err := ValidateDownloadMetric(DownloadMetric{Engine: StreamingEngineName, BytesWritten: -1}); err == nil {
		t.Fatalf("expected bytes error")
	}
}
