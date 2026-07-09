// Package observability contains local foundation observability contracts.
package observability

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/download"
	"github.com/cesarsousa94/auren-transfer-agent/internal/heartbeat"
	"github.com/cesarsousa94/auren-transfer-agent/internal/queue"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
)

const (
	// PrometheusName is the canonical capability name for Prometheus exposition.
	PrometheusName = "prometheus"

	// PrometheusContentType is the text exposition content type used by /metrics.
	PrometheusContentType = "text/plain; version=0.0.4; charset=utf-8"
)

// SnapshotInput is the local state rendered by observability endpoints.
type SnapshotInput struct {
	Info            runtime.VersionInfo
	Heartbeat       heartbeat.Record
	Queue           queue.Info
	Download        download.DownloadSummary
	EventsCount     int
	AuditCount      int
	TraceCount      int
	AlertCount      int
	CentralLogCount int
	GeneratedAt     time.Time
}

// Prometheus renders foundation metrics in Prometheus text format.
func Prometheus(input SnapshotInput) string {
	if input.GeneratedAt.IsZero() {
		input.GeneratedAt = time.Now().UTC()
	}
	var builder strings.Builder
	writeMetricHeader(&builder, "auren_agent_info", "Auren Transfer Agent build and runtime information.", "gauge")
	builder.WriteString(fmt.Sprintf("auren_agent_info%s 1\n", labels(map[string]string{"name": input.Info.Name, "version": input.Info.Version, "status": input.Info.Status})))
	writeMetricHeader(&builder, "auren_heartbeat_info", "Auren Transfer Agent heartbeat state.", "gauge")
	builder.WriteString(fmt.Sprintf("auren_heartbeat_info%s 1\n", labels(map[string]string{"agent_id": input.Heartbeat.AgentID, "hostname": input.Heartbeat.Hostname, "status": input.Heartbeat.Status})))
	writeMetricHeader(&builder, "auren_queue_length", "Current local queue length.", "gauge")
	builder.WriteString(fmt.Sprintf("auren_queue_length%s %d\n", labels(map[string]string{"driver": input.Queue.Driver, "mode": input.Queue.Mode}), input.Queue.Length))
	writeMetricHeader(&builder, "auren_queue_capacity", "Current local queue capacity.", "gauge")
	builder.WriteString(fmt.Sprintf("auren_queue_capacity%s %d\n", labels(map[string]string{"driver": input.Queue.Driver, "mode": input.Queue.Mode}), input.Queue.Capacity))
	writeMetricHeader(&builder, "auren_download_total", "Download observations retained by the local metrics recorder.", "counter")
	builder.WriteString(fmt.Sprintf("auren_download_total{result=\"succeeded\"} %d\n", input.Download.Succeeded))
	builder.WriteString(fmt.Sprintf("auren_download_total{result=\"failed\"} %d\n", input.Download.Failed))
	writeMetricHeader(&builder, "auren_download_bytes_written_total", "Bytes written by retained download observations.", "counter")
	builder.WriteString(fmt.Sprintf("auren_download_bytes_written_total %d\n", input.Download.BytesWritten))
	writeMetricHeader(&builder, "auren_events_total", "Retained local communication events.", "gauge")
	builder.WriteString(fmt.Sprintf("auren_events_total %d\n", input.EventsCount))
	writeMetricHeader(&builder, "auren_audit_events_total", "Retained local audit events.", "gauge")
	builder.WriteString(fmt.Sprintf("auren_audit_events_total %d\n", input.AuditCount))
	writeMetricHeader(&builder, "auren_traces_total", "Retained local trace spans.", "gauge")
	builder.WriteString(fmt.Sprintf("auren_traces_total %d\n", input.TraceCount))
	writeMetricHeader(&builder, "auren_alerts_active", "Currently active foundation alerts.", "gauge")
	builder.WriteString(fmt.Sprintf("auren_alerts_active %d\n", input.AlertCount))
	writeMetricHeader(&builder, "auren_central_logs_total", "Retained centralized log records.", "gauge")
	builder.WriteString(fmt.Sprintf("auren_central_logs_total %d\n", input.CentralLogCount))
	writeMetricHeader(&builder, "auren_observability_snapshot_timestamp_seconds", "Unix timestamp for the rendered observability snapshot.", "gauge")
	builder.WriteString(fmt.Sprintf("auren_observability_snapshot_timestamp_seconds %d\n", input.GeneratedAt.Unix()))
	return builder.String()
}

func writeMetricHeader(builder *strings.Builder, name string, help string, metricType string) {
	builder.WriteString("# HELP ")
	builder.WriteString(name)
	builder.WriteByte(' ')
	builder.WriteString(help)
	builder.WriteByte('\n')
	builder.WriteString("# TYPE ")
	builder.WriteString(name)
	builder.WriteByte(' ')
	builder.WriteString(metricType)
	builder.WriteByte('\n')
}

func labels(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", key, escapeLabel(values[key])))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escapeLabel(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\"", "\\\"")
	return replacer.Replace(value)
}
