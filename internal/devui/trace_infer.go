package devui

import (
	"regexp"
	"strings"
)

var transferJobPathPattern = regexp.MustCompile(`/transfer-agent/jobs/([^/]+)/([^/?#]+)`)

func inferOutboundTraceFields(trace OutboundTrace) (jobUUID string, operation string, stage string, message string, sourceURL string, destination string, objectPath string) {
	jobUUID = strings.TrimSpace(trace.JobUUID)
	operation = strings.TrimSpace(trace.Operation)
	stage = strings.TrimSpace(trace.Stage)
	message = strings.TrimSpace(trace.Message)
	sourceURL = strings.TrimSpace(trace.SourceURL)
	destination = strings.TrimSpace(trace.Destination)
	objectPath = strings.TrimSpace(trace.ObjectPath)

	if match := transferJobPathPattern.FindStringSubmatch(strings.ToLower(firstNonEmpty(trace.Path, trace.URL))); len(match) == 3 {
		if jobUUID == "" {
			jobUUID = match[1]
		}
		if stage == "" {
			stage = match[2]
		}
	}
	if strings.Contains(strings.ToLower(trace.Path), "/jobs/claim") && stage == "" {
		stage = "claim.poll"
	}
	enrichFromPayload := func(payload any) {
		m, ok := payload.(map[string]any)
		if !ok {
			return
		}
		if jobUUID == "" {
			jobUUID = firstStringFromAnyMap(m, "job_uuid", "uuid")
		}
		if operation == "" {
			operation = firstStringFromAnyMap(m, "operation")
		}
		if stage == "" {
			stage = firstStringFromAnyMap(m, "stage", "status")
		}
		if message == "" {
			message = firstStringFromAnyMap(m, "message", "error")
		}
		if sourceURL == "" {
			sourceURL = firstStringFromNestedMap(m, "source", "url")
		}
		if destination == "" {
			destination = firstStringFromNestedMap(m, "destination", "driver")
		}
		if objectPath == "" {
			objectPath = firstNonEmpty(firstStringFromNestedMap(m, "destination", "object_path"), firstStringFromNestedMap(m, "destination", "path"), firstStringFromAnyMap(m, "object_path", "path"))
		}
	}
	enrichFromPayload(trace.RequestPayload)
	if response, ok := trace.ResponsePayload.(map[string]any); ok {
		if job, ok := response["job"].(map[string]any); ok {
			enrichFromPayload(job)
		}
	}
	return sanitizeURL(jobUUID), operation, stage, message, sanitizeURL(sourceURL), destination, objectPath
}

func firstStringFromAnyMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			s := strings.TrimSpace(toString(value))
			s = strings.Trim(s, `"`)
			if s != "" && s != "null" {
				return s
			}
		}
	}
	return ""
}

func firstStringFromNestedMap(m map[string]any, outer string, inner string) string {
	nested, ok := m[outer].(map[string]any)
	if !ok {
		return ""
	}
	return firstStringFromAnyMap(nested, inner)
}
