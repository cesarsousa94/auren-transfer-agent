package mediahub

import (
	"fmt"
	"strings"
	"time"
)

const (
	GatewayResolvePath          = "/api/internal/gateway/resolve"
	GatewaySessionHeartbeatPath = "/api/internal/gateway/sessions/heartbeat"
	GatewaySessionClosePath     = "/api/internal/gateway/sessions/close"
	GatewayEventsPath           = "/api/internal/gateway/events"
)

// GatewayResolveRequest is sent by the Agent before opening a public stream.
type GatewayResolveRequest struct {
	Token       string            `json:"token"`
	Kind        string            `json:"kind"`
	ID          string            `json:"id"`
	Extension   string            `json:"ext"`
	Method      string            `json:"method"`
	Range       string            `json:"range,omitempty"`
	UserAgent   string            `json:"user_agent,omitempty"`
	RemoteAddr  string            `json:"remote_addr,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// GatewayResolveResult is returned by Media Hub after validating the handoff token.
type GatewayResolveResult struct {
	Success          bool              `json:"success"`
	Mode             string            `json:"mode"`
	UpstreamURL      string            `json:"upstream_url"`
	RedirectURL      string            `json:"redirect_url,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
	SessionID        string            `json:"session_id,omitempty"`
	HeartbeatSeconds int               `json:"heartbeat_seconds,omitempty"`
	StatusCode       int               `json:"status_code,omitempty"`
	Raw              map[string]any    `json:"-"`
}

// Validate checks the minimal gateway resolve response contract.
func (result GatewayResolveResult) Validate() error {
	mode := result.NormalizedMode()
	if mode == "" {
		return fmt.Errorf("gateway resolve mode is required")
	}
	if mode != "proxy" && mode != "redirect" {
		return fmt.Errorf("unsupported gateway mode %q", result.Mode)
	}
	if strings.TrimSpace(result.UpstreamURL) == "" {
		return fmt.Errorf("gateway upstream_url is required")
	}
	return nil
}

// NormalizedMode returns proxy/redirect with safe default behavior.
func (result GatewayResolveResult) NormalizedMode() string {
	mode := strings.ToLower(strings.TrimSpace(result.Mode))
	if mode == "" {
		mode = "proxy"
	}
	return mode
}

// HeartbeatInterval returns the interval requested by Media Hub.
func (result GatewayResolveResult) HeartbeatInterval() time.Duration {
	if result.HeartbeatSeconds <= 0 {
		return 0
	}
	return time.Duration(result.HeartbeatSeconds) * time.Second
}

// SessionIDOr returns the Media Hub session id or the fallback value.
func (result GatewayResolveResult) SessionIDOr(fallback string) string {
	if strings.TrimSpace(result.SessionID) != "" {
		return strings.TrimSpace(result.SessionID)
	}
	return fallback
}

// GatewaySessionHeartbeatPayload updates a stream session while bytes are flowing.
type GatewaySessionHeartbeatPayload struct {
	SessionID        string    `json:"session_id"`
	Token            string    `json:"token,omitempty"`
	Kind             string    `json:"kind,omitempty"`
	ID               string    `json:"id,omitempty"`
	Extension        string    `json:"ext,omitempty"`
	Mode             string    `json:"mode"`
	Status           string    `json:"status"`
	HTTPStatus       int       `json:"http_status,omitempty"`
	BytesSent        int64     `json:"bytes_sent"`
	CurrentEgressBps int       `json:"current_egress_bps,omitempty"`
	Message          string    `json:"message,omitempty"`
	GeneratedAt      time.Time `json:"generated_at"`
}

// GatewaySessionClosePayload closes a Media Hub stream session.
type GatewaySessionClosePayload struct {
	SessionID       string    `json:"session_id"`
	Token           string    `json:"token,omitempty"`
	Kind            string    `json:"kind,omitempty"`
	ID              string    `json:"id,omitempty"`
	Extension       string    `json:"ext,omitempty"`
	Mode            string    `json:"mode"`
	Status          string    `json:"status"`
	HTTPStatus      int       `json:"http_status,omitempty"`
	BytesSent       int64     `json:"bytes_sent"`
	DurationSeconds float64   `json:"duration_seconds,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	GeneratedAt     time.Time `json:"generated_at"`
}

// GatewayEventsPayload batches gateway runtime events to Media Hub.
type GatewayEventsPayload struct {
	Events      []EventPayload `json:"events"`
	GeneratedAt time.Time      `json:"generated_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ParseGatewayResolveResult accepts flat, data and resolve response envelopes.
func ParseGatewayResolveResult(response map[string]any) (GatewayResolveResult, error) {
	if response == nil {
		return GatewayResolveResult{}, fmt.Errorf("gateway resolve response is empty")
	}
	encoded, err := marshalMap(response)
	if err != nil {
		return GatewayResolveResult{}, err
	}
	var result GatewayResolveResult
	if err := unmarshalBytes(encoded, &result); err != nil {
		return GatewayResolveResult{}, err
	}
	result.Raw = response
	if result.UpstreamURL == "" && result.RedirectURL != "" {
		result.UpstreamURL = result.RedirectURL
	}
	if result.UpstreamURL == "" {
		for _, key := range []string{"data", "resolve", "gateway"} {
			nested, ok := response[key].(map[string]any)
			if !ok {
				continue
			}
			nestedResult, err := ParseGatewayResolveResult(nested)
			if err != nil {
				continue
			}
			nestedResult.Raw = response
			return nestedResult, nil
		}
	}
	if result.Mode == "" {
		result.Mode = firstString(response, "delivery_mode", "proxy_mode")
	}
	if result.UpstreamURL == "" {
		result.UpstreamURL = firstString(response, "url", "source_url", "effective_url")
	}
	if result.SessionID == "" {
		result.SessionID = firstString(response, "session_uuid", "stream_session_id", "session")
	}
	return result, result.Validate()
}
