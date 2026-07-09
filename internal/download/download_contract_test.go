package download

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadEngineRecordsStreamingMetricsWithBandwidthMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("contract"))
	}))
	defer server.Close()

	client := newTestHTTPClient(t)
	bandwidth, err := NewBandwidthController(1024 * 1024)
	if err != nil {
		t.Fatalf("bandwidth: %v", err)
	}
	metrics := NewMemoryMetricsRecorder()
	var output bytes.Buffer
	result, err := client.Stream(context.Background(), StreamOptions{URL: server.URL, Writer: &output, Bandwidth: bandwidth, Metrics: metrics})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if output.String() != "contract" || result.BytesWritten != 8 {
		t.Fatalf("output=%q result=%+v", output.String(), result)
	}
	snapshot := metrics.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("metrics count = %d", len(snapshot))
	}
	metric := snapshot[0]
	if metric.Engine != StreamingEngineName || !metric.BandwidthLimited || metric.BandwidthBytesLimit != 1024*1024 || metric.BytesWritten != 8 {
		t.Fatalf("metric = %+v", metric)
	}
}

func TestDownloadEngineRecordsStreamingFailureMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestHTTPClient(t)
	metrics := NewMemoryMetricsRecorder()
	_, err := client.Stream(context.Background(), StreamOptions{URL: server.URL, Writer: &bytes.Buffer{}, Metrics: metrics})
	if err == nil {
		t.Fatalf("expected stream error")
	}
	snapshot := metrics.Snapshot()
	if len(snapshot) != 1 || snapshot[0].Error == "" || snapshot[0].StatusCode != http.StatusInternalServerError {
		t.Fatalf("metrics = %+v", snapshot)
	}
}
