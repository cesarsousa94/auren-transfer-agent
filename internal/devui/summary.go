package devui

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Summary groups recent activity for a navigable operational console.
type Summary struct {
	GeneratedAt     time.Time         `json:"generated_at"`
	WindowRecords   int               `json:"window_records"`
	Endpoints       []EndpointSummary `json:"endpoints"`
	Functions       []FunctionSummary `json:"functions"`
	Events          []EventSummary    `json:"events"`
	Jobs            []JobSummary      `json:"jobs"`
	ActiveDownloads int               `json:"active_downloads"`
	ActiveUploads   int               `json:"active_uploads"`
	ActiveTransfers int               `json:"active_transfers"`
	QueueLikeCalls  int               `json:"queue_like_calls"`
	NoisyCalls      int               `json:"noisy_calls"`
	Errors          []RequestRecord   `json:"errors"`
}

type EndpointSummary struct {
	Key            string    `json:"key"`
	Direction      string    `json:"direction"`
	Method         string    `json:"method"`
	Path           string    `json:"path"`
	Count          int64     `json:"count"`
	Errors         int64     `json:"errors"`
	LastStatus     int       `json:"last_status"`
	TotalBytes     int64     `json:"total_bytes"`
	AvgDurationMS  int64     `json:"avg_duration_ms"`
	MaxDurationMS  int64     `json:"max_duration_ms"`
	LastFinishedAt time.Time `json:"last_finished_at"`
}

type FunctionSummary struct {
	Name           string    `json:"name"`
	Count          int64     `json:"count"`
	Errors         int64     `json:"errors"`
	LastStatus     int       `json:"last_status"`
	LastPath       string    `json:"last_path"`
	LastFinishedAt time.Time `json:"last_finished_at"`
}

type EventSummary struct {
	Stage          string    `json:"stage"`
	Operation      string    `json:"operation"`
	Count          int64     `json:"count"`
	Errors         int64     `json:"errors"`
	LastJobUUID    string    `json:"last_job_uuid"`
	LastMessage    string    `json:"last_message"`
	LastFinishedAt time.Time `json:"last_finished_at"`
}

type JobSummary struct {
	UUID             string          `json:"uuid"`
	Operation        string          `json:"operation"`
	CurrentStage     string          `json:"current_stage"`
	SourceURL        string          `json:"source_url"`
	Destination      string          `json:"destination"`
	ObjectPath       string          `json:"object_path"`
	Events           int64           `json:"events"`
	Requests         int64           `json:"requests"`
	Errors           int64           `json:"errors"`
	CurrentBytes     int64           `json:"current_bytes"`
	TotalBytes       int64           `json:"total_bytes"`
	SpeedBps         int64           `json:"speed_bps"`
	Percent          float64         `json:"percent"`
	LastMessage      string          `json:"last_message"`
	LastFinishedAt   time.Time       `json:"last_finished_at"`
	RecentActivities []RequestRecord `json:"recent_activities"`
}

// Summary returns grouped recent activity from newest retained records.
func (recorder *Recorder) Summary(limit int) Summary {
	records := recorder.Snapshot(limit)
	summary := Summary{GeneratedAt: time.Now().UTC(), WindowRecords: len(records)}
	endpointMap := map[string]*EndpointSummary{}
	functionMap := map[string]*FunctionSummary{}
	eventMap := map[string]*EventSummary{}
	jobMap := map[string]*JobSummary{}

	for _, record := range records {
		if isNoisyRecord(record) {
			summary.NoisyCalls++
		}
		if strings.Contains(classifyFunction(record), "claim") || strings.Contains(record.Path, "/jobs/claim") {
			summary.QueueLikeCalls++
		}
		if record.Status >= 400 || record.Error != "" {
			if len(summary.Errors) < 25 {
				summary.Errors = append(summary.Errors, record)
			}
		}
		key := strings.ToLower(record.Direction) + " " + strings.ToUpper(record.Method) + " " + normalizePathForGroup(firstNonEmpty(record.Path, record.URL))
		ep := endpointMap[key]
		if ep == nil {
			ep = &EndpointSummary{Key: key, Direction: record.Direction, Method: record.Method, Path: normalizePathForGroup(firstNonEmpty(record.Path, record.URL))}
			endpointMap[key] = ep
		}
		ep.Count++
		ep.TotalBytes += record.Bytes
		ep.AvgDurationMS += record.DurationMS
		if record.DurationMS > ep.MaxDurationMS {
			ep.MaxDurationMS = record.DurationMS
		}
		if record.Status >= 400 || record.Error != "" {
			ep.Errors++
		}
		if record.FinishedAt.After(ep.LastFinishedAt) {
			ep.LastFinishedAt = record.FinishedAt
			ep.LastStatus = record.Status
		}

		fnName := classifyFunction(record)
		fn := functionMap[fnName]
		if fn == nil {
			fn = &FunctionSummary{Name: fnName}
			functionMap[fnName] = fn
		}
		fn.Count++
		if record.Status >= 400 || record.Error != "" {
			fn.Errors++
		}
		if record.FinishedAt.After(fn.LastFinishedAt) {
			fn.LastFinishedAt = record.FinishedAt
			fn.LastStatus = record.Status
			fn.LastPath = firstNonEmpty(record.Path, record.URL)
		}

		stage := firstNonEmpty(record.Stage, record.Path)
		op := record.Operation
		if record.Kind == "transfer" || record.JobUUID != "" || stage != "" {
			evKey := op + "::" + stage
			ev := eventMap[evKey]
			if ev == nil {
				ev = &EventSummary{Operation: op, Stage: stage}
				eventMap[evKey] = ev
			}
			ev.Count++
			if record.Status >= 400 || record.Error != "" {
				ev.Errors++
			}
			if record.FinishedAt.After(ev.LastFinishedAt) {
				ev.LastFinishedAt = record.FinishedAt
				ev.LastJobUUID = record.JobUUID
				ev.LastMessage = firstNonEmpty(record.Message, record.Error)
			}
		}

		if record.JobUUID != "" {
			job := jobMap[record.JobUUID]
			if job == nil {
				job = &JobSummary{UUID: record.JobUUID}
				jobMap[record.JobUUID] = job
			}
			if record.Kind == "transfer" {
				job.Events++
			} else {
				job.Requests++
			}
			if record.Status >= 400 || record.Error != "" {
				job.Errors++
			}
			if record.Operation != "" {
				job.Operation = record.Operation
			}
			if record.Stage != "" {
				job.CurrentStage = record.Stage
			}
			if record.SourceURL != "" {
				job.SourceURL = record.SourceURL
			}
			if record.Destination != "" {
				job.Destination = record.Destination
			}
			if record.ObjectPath != "" {
				job.ObjectPath = record.ObjectPath
			}
			if record.Message != "" || record.Error != "" {
				job.LastMessage = firstNonEmpty(record.Message, record.Error)
			}
			mergeProgressFromRecord(job, record)
			if record.FinishedAt.After(job.LastFinishedAt) {
				job.LastFinishedAt = record.FinishedAt
			}
			if len(job.RecentActivities) < 10 {
				job.RecentActivities = append(job.RecentActivities, record)
			}
		}
	}

	for _, ep := range endpointMap {
		if ep.Count > 0 {
			ep.AvgDurationMS = ep.AvgDurationMS / ep.Count
		}
		summary.Endpoints = append(summary.Endpoints, *ep)
	}
	for _, fn := range functionMap {
		summary.Functions = append(summary.Functions, *fn)
	}
	for _, ev := range eventMap {
		summary.Events = append(summary.Events, *ev)
		if strings.Contains(strings.ToLower(ev.Stage), "download") && !strings.Contains(strings.ToLower(ev.Stage), "completed") {
			summary.ActiveDownloads++
		}
		if strings.Contains(strings.ToLower(ev.Stage), "upload") && !strings.Contains(strings.ToLower(ev.Stage), "completed") {
			summary.ActiveUploads++
		}
	}
	for _, job := range jobMap {
		summary.Jobs = append(summary.Jobs, *job)
	}

	sort.Slice(summary.Endpoints, func(i, j int) bool { return summary.Endpoints[i].Count > summary.Endpoints[j].Count })
	sort.Slice(summary.Functions, func(i, j int) bool { return summary.Functions[i].Count > summary.Functions[j].Count })
	sort.Slice(summary.Events, func(i, j int) bool { return summary.Events[i].LastFinishedAt.After(summary.Events[j].LastFinishedAt) })
	sort.Slice(summary.Jobs, func(i, j int) bool { return summary.Jobs[i].LastFinishedAt.After(summary.Jobs[j].LastFinishedAt) })
	summary.ActiveTransfers = len(summary.Jobs)
	if len(summary.Endpoints) > 30 {
		summary.Endpoints = summary.Endpoints[:30]
	}
	if len(summary.Functions) > 30 {
		summary.Functions = summary.Functions[:30]
	}
	if len(summary.Events) > 40 {
		summary.Events = summary.Events[:40]
	}
	if len(summary.Jobs) > 25 {
		summary.Jobs = summary.Jobs[:25]
	}
	return summary
}

func classifyFunction(record RequestRecord) string {
	p := strings.ToLower(firstNonEmpty(record.Path, record.URL))
	stage := strings.ToLower(record.Stage)
	if record.Kind == "transfer" {
		if stage != "" {
			return "transfer." + strings.Trim(stage, "/")
		}
		return "transfer.event"
	}
	switch {
	case strings.Contains(p, "/nodes/register"):
		return "media_hub.node.register"
	case strings.Contains(p, "/nodes/config"):
		return "media_hub.node.config"
	case strings.Contains(p, "/nodes/heartbeat"):
		return "media_hub.node.heartbeat"
	case strings.Contains(p, "/nodes/metrics"):
		return "media_hub.node.metrics"
	case strings.Contains(p, "/nodes/events"):
		return "media_hub.node.events"
	case strings.Contains(p, "/transfer-agent/jobs/claim"):
		return "transfer.claim"
	case strings.Contains(p, "/transfer-agent/jobs/") && strings.Contains(p, "/started"):
		return "transfer.started"
	case strings.Contains(p, "/transfer-agent/jobs/") && strings.Contains(p, "/progress"):
		return "transfer.progress"
	case strings.Contains(p, "/transfer-agent/jobs/") && strings.Contains(p, "/completed"):
		return "transfer.completed"
	case strings.Contains(p, "/transfer-agent/jobs/") && strings.Contains(p, "/failed"):
		return "transfer.failed"
	case strings.Contains(p, "/transfer-agent/jobs/") && strings.Contains(p, "/control"):
		return "transfer.control"
	case strings.Contains(p, "/gateway/resolve"):
		return "gateway.resolve"
	case strings.Contains(p, "/gateway/sessions/heartbeat"):
		return "gateway.session.heartbeat"
	case strings.Contains(p, "/gateway/sessions/close"):
		return "gateway.session.close"
	case strings.Contains(p, "s3") || strings.Contains(p, "amazonaws.com"):
		return "storage.s3"
	case strings.Contains(p, "/_auren/dev"):
		return "dev_console"
	default:
		return "http." + strings.ToLower(firstNonEmpty(record.Method, "event"))
	}
}

func isNoisyRecord(record RequestRecord) bool {
	fn := classifyFunction(record)
	return fn == "media_hub.node.heartbeat" || fn == "media_hub.node.metrics" || fn == "media_hub.node.events" || fn == "transfer.control" || fn == "dev_console"
}

var uuidPathPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func normalizePathForGroup(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	path := raw
	if err == nil && parsed.Path != "" {
		path = parsed.Path
	}
	path = uuidPathPattern.ReplaceAllString(path, "{uuid}")
	return path
}

func mergeProgressFromRecord(job *JobSummary, record RequestRecord) {
	candidates := []any{record.Metadata}
	if record.RequestBody != nil && record.RequestBody.JSON != nil {
		candidates = append(candidates, record.RequestBody.JSON)
	}
	for _, candidate := range candidates {
		m, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := int64FromAny(m["current_bytes"]); ok {
			job.CurrentBytes = v
		}
		if v, ok := int64FromAny(m["total_bytes"]); ok {
			job.TotalBytes = v
		}
		if v, ok := int64FromAny(m["speed_bps"]); ok {
			job.SpeedBps = v
		}
		if v, ok := floatFromAny(m["percent"]); ok {
			job.Percent = v
		}
	}
}

func int64FromAny(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case jsonNumber:
		v, err := typed.Int64()
		return v, err == nil
	default:
		return 0, false
	}
}

func floatFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int:
		return float64(typed), true
	case jsonNumber:
		v, err := typed.Float64()
		return v, err == nil
	default:
		return 0, false
	}
}

type jsonNumber interface {
	Int64() (int64, error)
	Float64() (float64, error)
}
