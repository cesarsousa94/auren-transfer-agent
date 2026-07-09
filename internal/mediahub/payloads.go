package mediahub

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RegistrationPayload is sent to /api/internal/nodes/register.
type RegistrationPayload struct {
	RegistrationToken string         `json:"registration_token"`
	AgentID           string         `json:"agent_id"`
	Fingerprint       string         `json:"fingerprint"`
	Hostname          string         `json:"hostname"`
	HostnameSource    string         `json:"hostname_source"`
	Version           string         `json:"version"`
	RuntimeStatus     string         `json:"runtime_status"`
	Role              string         `json:"role"`
	Provider          string         `json:"provider"`
	Region            string         `json:"region"`
	AvailabilityZone  string         `json:"availability_zone,omitempty"`
	PublicBaseURL     string         `json:"base_url,omitempty"`
	HealthURL         string         `json:"health_url,omitempty"`
	MaxSessions       int            `json:"max_sessions"`
	MaxEgressMbps     int            `json:"max_egress_mbps"`
	Capabilities      []string       `json:"capabilities"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

// RegistrationResult contains Media Hub-issued node credentials.
type RegistrationResult struct {
	NodeUUID      string
	NodeSecret    string
	ConfigVersion string
	Raw           map[string]any
}

// ConfigResult contains a config pull response.
type ConfigResult struct {
	ConfigVersion string
	Raw           map[string]any
}

// HeartbeatPayload is sent to /api/internal/nodes/heartbeat.
type HeartbeatPayload struct {
	NodeUUID         string         `json:"node_uuid,omitempty"`
	AgentID          string         `json:"agent_id"`
	Fingerprint      string         `json:"fingerprint"`
	Hostname         string         `json:"hostname"`
	Version          string         `json:"version"`
	RuntimeStatus    string         `json:"runtime_status"`
	Status           string         `json:"status"`
	Role             string         `json:"role"`
	Provider         string         `json:"provider"`
	Region           string         `json:"region"`
	AvailabilityZone string         `json:"availability_zone,omitempty"`
	PublicBaseURL    string         `json:"base_url,omitempty"`
	HealthURL        string         `json:"health_url,omitempty"`
	Capabilities     []string       `json:"capabilities"`
	Capacity         Capacity       `json:"capacity"`
	Heartbeat        any            `json:"heartbeat"`
	GeneratedAt      time.Time      `json:"generated_at"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// Capacity describes node workload metrics consumed by Media Hub routing/policy.
type Capacity struct {
	MaxSessions       int `json:"max_sessions"`
	ActiveSessions    int `json:"active_sessions"`
	MaxConcurrentJobs int `json:"max_concurrent_jobs"`
	ActiveJobs        int `json:"active_jobs"`
	MaxEgressMbps     int `json:"max_egress_mbps"`
	CurrentEgressMbps int `json:"current_egress_mbps"`
}

// MetricsPayload is sent to /api/internal/nodes/metrics.
type MetricsPayload struct {
	NodeUUID     string         `json:"node_uuid,omitempty"`
	Runtime      any            `json:"runtime"`
	Heartbeat    any            `json:"heartbeat"`
	Queue        any            `json:"queue"`
	Download     any            `json:"download"`
	Capacity     Capacity       `json:"capacity"`
	Capabilities []string       `json:"capabilities"`
	GeneratedAt  time.Time      `json:"generated_at"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// EventsPayload is sent to /api/internal/nodes/events.
type EventsPayload struct {
	NodeUUID    string         `json:"node_uuid,omitempty"`
	Events      []EventPayload `json:"events"`
	GeneratedAt time.Time      `json:"generated_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// DrainPayload is sent when the Agent enters or completes graceful drain.
type DrainPayload struct {
	NodeUUID       string         `json:"node_uuid,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	ActiveJobs     int            `json:"active_jobs"`
	ActiveSessions int            `json:"active_sessions"`
	Forced         bool           `json:"forced,omitempty"`
	GeneratedAt    time.Time      `json:"generated_at"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// EventPayload is one operational event emitted to Media Hub.
type EventPayload struct {
	ID        string            `json:"id,omitempty"`
	Level     string            `json:"level"`
	Type      string            `json:"type"`
	Message   string            `json:"message"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// ParseRegistrationResult extracts node credentials from permissive Media Hub response shapes.
func ParseRegistrationResult(response map[string]any) (RegistrationResult, error) {
	if response == nil {
		return RegistrationResult{}, fmt.Errorf("media hub registration response is empty")
	}
	nodeUUID := firstString(response, "node_uuid", "uuid", "id")
	nodeSecret := firstString(response, "node_secret", "agent_secret", "secret")
	configVersion := firstString(response, "config_version")
	for _, containerKey := range []string{"node", "data", "edge_node"} {
		if nested, ok := response[containerKey].(map[string]any); ok {
			if nodeUUID == "" {
				nodeUUID = firstString(nested, "node_uuid", "uuid", "id")
			}
			if nodeSecret == "" {
				nodeSecret = firstString(nested, "node_secret", "agent_secret", "secret")
			}
			if configVersion == "" {
				configVersion = firstString(nested, "config_version")
			}
		}
	}
	if strings.TrimSpace(nodeUUID) == "" {
		return RegistrationResult{}, fmt.Errorf("media hub registration response did not include node_uuid")
	}
	if strings.TrimSpace(nodeSecret) == "" {
		return RegistrationResult{}, fmt.Errorf("media hub registration response did not include node_secret")
	}
	return RegistrationResult{NodeUUID: strings.TrimSpace(nodeUUID), NodeSecret: strings.TrimSpace(nodeSecret), ConfigVersion: strings.TrimSpace(configVersion), Raw: response}, nil
}

// ParseConfigResult extracts config version while preserving the full config response.
func ParseConfigResult(response map[string]any) ConfigResult {
	if response == nil {
		response = map[string]any{}
	}
	configVersion := firstString(response, "config_version", "version")
	if configVersion == "" {
		for _, containerKey := range []string{"config", "data"} {
			if nested, ok := response[containerKey].(map[string]any); ok {
				configVersion = firstString(nested, "config_version", "version")
				if configVersion != "" {
					break
				}
			}
		}
	}
	return ConfigResult{ConfigVersion: strings.TrimSpace(configVersion), Raw: response}
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case fmt.Stringer:
			if strings.TrimSpace(typed.String()) != "" {
				return strings.TrimSpace(typed.String())
			}
		case float64:
			return strconv.FormatInt(int64(typed), 10)
		case int:
			return strconv.Itoa(typed)
		}
	}
	return ""
}
