package server

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/auren/auren-transfer-agent/internal/heartbeat"
	agentidentity "github.com/auren/auren-transfer-agent/internal/identity"
	"github.com/auren/auren-transfer-agent/internal/runtime"
)

const (
	// CommunicationAPIBasePath is the canonical foundation communication API prefix.
	CommunicationAPIBasePath = "/api/v1"

	// CommunicationRESTRouteName is the canonical route name for the REST API status endpoint.
	CommunicationRESTRouteName = "communication.rest"

	// CommunicationWebSocketRouteName is the canonical route name for the WebSocket foundation endpoint.
	CommunicationWebSocketRouteName = "communication.websocket"

	// CommunicationRegistrationRouteName is the canonical route name for Agent registration.
	CommunicationRegistrationRouteName = "communication.registration"

	// CommunicationHeartbeatGetRouteName is the canonical route name for heartbeat reads.
	CommunicationHeartbeatGetRouteName = "communication.heartbeat.show"

	// CommunicationHeartbeatPostRouteName is the canonical route name for heartbeat submissions.
	CommunicationHeartbeatPostRouteName = "communication.heartbeat.store"

	// CommunicationWebSocketPath is the canonical WebSocket foundation path.
	CommunicationWebSocketPath = CommunicationAPIBasePath + "/ws"

	// CommunicationRegistrationPath is the canonical registration path.
	CommunicationRegistrationPath = CommunicationAPIBasePath + "/registration"

	// CommunicationHeartbeatPath is the canonical heartbeat path.
	CommunicationHeartbeatPath = CommunicationAPIBasePath + "/heartbeat"

	// CommunicationStatusOK is the stable OK status value for communication endpoints.
	CommunicationStatusOK = "ok"

	// CommunicationStatusError is the stable error status value for communication endpoints.
	CommunicationStatusError = "error"

	webSocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

// AuthOptions configures optional API-key authentication for foundation communication routes.
type AuthOptions struct {
	Required    bool
	APIKey      string
	TokenHeader string
}

// Authenticator wraps HTTP handlers with optional API-key authentication.
type Authenticator struct {
	required    bool
	apiKey      string
	tokenHeader string
}

// CommunicationOptions configures the foundation communication REST/WebSocket routes.
type CommunicationOptions struct {
	Info          runtime.VersionInfo
	Identity      agentidentity.Snapshot
	Heartbeat     heartbeat.Record
	Authenticator Authenticator
}

// RESTStatusResponse is returned by GET /api/v1.
type RESTStatusResponse struct {
	Status         string    `json:"status"`
	Name           string    `json:"name"`
	Version        string    `json:"version"`
	RuntimeStatus  string    `json:"runtime_status"`
	Router         string    `json:"router"`
	RESTAPI        bool      `json:"rest_api"`
	WebSocket      bool      `json:"websocket"`
	Registration   bool      `json:"registration"`
	Heartbeat      bool      `json:"heartbeat"`
	Authentication string    `json:"authentication"`
	Routes         []string  `json:"routes"`
	GeneratedAt    time.Time `json:"generated_at"`
}

// RegistrationRequest is the foundation registration input accepted from a controller.
type RegistrationRequest struct {
	Controller string            `json:"controller"`
	Labels     map[string]string `json:"labels"`
	Metadata   map[string]string `json:"metadata"`
}

// RegistrationResponse is returned after a mechanical local registration acknowledgement.
type RegistrationResponse struct {
	Status       string                 `json:"status"`
	Registered   bool                   `json:"registered"`
	Controller   string                 `json:"controller,omitempty"`
	Agent        agentidentity.Snapshot `json:"agent"`
	Version      runtime.VersionInfo    `json:"version"`
	Heartbeat    heartbeat.Record       `json:"heartbeat"`
	Capabilities []string               `json:"capabilities"`
	Labels       map[string]string      `json:"labels,omitempty"`
	Metadata     map[string]string      `json:"metadata,omitempty"`
	GeneratedAt  time.Time              `json:"generated_at"`
}

// HeartbeatResponse is returned by communication heartbeat endpoints.
type HeartbeatResponse struct {
	Status      string           `json:"status"`
	Accepted    bool             `json:"accepted"`
	Heartbeat   heartbeat.Record `json:"heartbeat"`
	GeneratedAt time.Time        `json:"generated_at"`
}

// CommunicationErrorResponse is the stable JSON error payload for communication endpoints.
type CommunicationErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// NewAuthenticator creates the foundation API-key authenticator.
func NewAuthenticator(options AuthOptions) (Authenticator, error) {
	tokenHeader := strings.TrimSpace(options.TokenHeader)
	if tokenHeader == "" {
		tokenHeader = "Authorization"
	}
	if strings.ContainsAny(tokenHeader, "\r\n") {
		return Authenticator{}, fmt.Errorf("token header cannot contain newline characters")
	}
	apiKey := strings.TrimSpace(options.APIKey)
	if options.Required && apiKey == "" {
		return Authenticator{}, fmt.Errorf("api key is required when authentication is enabled")
	}
	return Authenticator{required: options.Required, apiKey: apiKey, tokenHeader: tokenHeader}, nil
}

// Required reports whether this authenticator enforces API keys.
func (auth Authenticator) Required() bool {
	return auth.required
}

// TokenHeader returns the canonical header used for API-key authentication.
func (auth Authenticator) TokenHeader() string {
	if strings.TrimSpace(auth.tokenHeader) == "" {
		return "Authorization"
	}
	return auth.tokenHeader
}

// Mode returns the stable authentication mode name used by diagnostics.
func (auth Authenticator) Mode() string {
	if auth.required {
		return "api_key"
	}
	return "disabled"
}

// AuthenticateRequest checks one request against the configured API key.
func (auth Authenticator) AuthenticateRequest(request *http.Request) bool {
	if !auth.required {
		return true
	}
	if request == nil {
		return false
	}
	value := strings.TrimSpace(request.Header.Get(auth.TokenHeader()))
	if value == "" {
		return false
	}
	return value == auth.apiKey || value == "Bearer "+auth.apiKey || value == "ApiKey "+auth.apiKey
}

// Wrap protects a handler with the configured authentication policy.
func (auth Authenticator) Wrap(handler http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !auth.AuthenticateRequest(request) {
			writer.Header().Set("WWW-Authenticate", `ApiKey realm="auren-transfer-agent"`)
			writeCommunicationError(writer, http.StatusUnauthorized, "authentication required")
			return
		}
		handler(writer, request)
	}
}

// RESTStatusHandler returns a foundation REST API status response.
func RESTStatusHandler(options CommunicationOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		payload := RESTStatusResponse{
			Status:         CommunicationStatusOK,
			Name:           options.Info.Name,
			Version:        options.Info.Version,
			RuntimeStatus:  options.Info.Status,
			Router:         RouterKindName(),
			RESTAPI:        true,
			WebSocket:      true,
			Registration:   true,
			Heartbeat:      true,
			Authentication: options.Authenticator.Mode(),
			Routes:         CommunicationRoutePatterns(),
			GeneratedAt:    time.Now().UTC(),
		}
		writeJSON(writer, http.StatusOK, payload)
		_ = request
	}
}

// WebSocketHandler performs the foundation WebSocket upgrade handshake.
func WebSocketHandler(options CommunicationOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !isWebSocketUpgrade(request) {
			writer.Header().Set("Upgrade", "websocket")
			writeCommunicationError(writer, http.StatusUpgradeRequired, "websocket upgrade required")
			return
		}
		key := strings.TrimSpace(request.Header.Get("Sec-WebSocket-Key"))
		if key == "" {
			writeCommunicationError(writer, http.StatusBadRequest, "sec-websocket-key is required")
			return
		}
		writer.Header().Set("Upgrade", "websocket")
		writer.Header().Set("Connection", "Upgrade")
		writer.Header().Set("Sec-WebSocket-Accept", WebSocketAcceptKey(key))
		writer.Header().Set("X-Auren-Agent-ID", options.Identity.AgentID)
		writer.WriteHeader(http.StatusSwitchingProtocols)
	}
}

// RegistrationHandler acknowledges a local Agent registration request.
func RegistrationHandler(options CommunicationOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		input := RegistrationRequest{}
		if request.Body != nil && request.ContentLength != 0 {
			decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&input); err != nil {
				writeCommunicationError(writer, http.StatusBadRequest, "invalid registration payload")
				return
			}
		}
		payload := RegistrationResponse{
			Status:       CommunicationStatusOK,
			Registered:   true,
			Controller:   strings.TrimSpace(input.Controller),
			Agent:        options.Identity,
			Version:      options.Info,
			Heartbeat:    options.Heartbeat.Clone(),
			Capabilities: CommunicationCapabilities(),
			Labels:       cloneStringMap(input.Labels),
			Metadata:     cloneStringMap(input.Metadata),
			GeneratedAt:  time.Now().UTC(),
		}
		writeJSON(writer, http.StatusAccepted, payload)
	}
}

// CommunicationHeartbeatHandler returns or accepts the current local heartbeat snapshot.
func CommunicationHeartbeatHandler(options CommunicationOptions, accepted bool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		status := http.StatusOK
		if accepted {
			status = http.StatusAccepted
		}
		payload := HeartbeatResponse{Status: CommunicationStatusOK, Accepted: accepted, Heartbeat: options.Heartbeat.Clone(), GeneratedAt: time.Now().UTC()}
		writeJSON(writer, status, payload)
		_ = request
	}
}

// CommunicationRoutes returns the foundation communication route set.
func CommunicationRoutes(options CommunicationOptions) []RouteDefinition {
	auth := options.Authenticator
	return []RouteDefinition{
		{Name: CommunicationRESTRouteName, Method: http.MethodGet, Pattern: CommunicationAPIBasePath, Handler: auth.Wrap(RESTStatusHandler(options))},
		{Name: CommunicationWebSocketRouteName, Method: http.MethodGet, Pattern: CommunicationWebSocketPath, Handler: auth.Wrap(WebSocketHandler(options))},
		{Name: CommunicationRegistrationRouteName, Method: http.MethodPost, Pattern: CommunicationRegistrationPath, Handler: auth.Wrap(RegistrationHandler(options))},
		{Name: CommunicationHeartbeatGetRouteName, Method: http.MethodGet, Pattern: CommunicationHeartbeatPath, Handler: auth.Wrap(CommunicationHeartbeatHandler(options, false))},
		{Name: CommunicationHeartbeatPostRouteName, Method: http.MethodPost, Pattern: CommunicationHeartbeatPath, Handler: auth.Wrap(CommunicationHeartbeatHandler(options, true))},
	}
}

// CommunicationCapabilities returns the stable foundation communication capability list.
func CommunicationCapabilities() []string {
	capabilities := []string{"rest_api", "websocket_handshake", "registration", "heartbeat", "api_key_auth"}
	sort.Strings(capabilities)
	return capabilities
}

// CommunicationRoutePatterns returns a defensive list of canonical communication route patterns.
func CommunicationRoutePatterns() []string {
	routes := []string{CommunicationAPIBasePath, CommunicationWebSocketPath, CommunicationRegistrationPath, CommunicationHeartbeatPath}
	sort.Strings(routes)
	return routes
}

// WebSocketAcceptKey returns the RFC 6455 accept value for a client key.
func WebSocketAcceptKey(key string) string {
	digest := sha1.Sum([]byte(strings.TrimSpace(key) + webSocketGUID))
	return base64.StdEncoding.EncodeToString(digest[:])
}

func isWebSocketUpgrade(request *http.Request) bool {
	if request == nil {
		return false
	}
	connection := strings.ToLower(request.Header.Get("Connection"))
	upgrade := strings.ToLower(request.Header.Get("Upgrade"))
	return strings.Contains(connection, "upgrade") && upgrade == "websocket"
}

func writeCommunicationError(writer http.ResponseWriter, status int, message string) {
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("communication api error: %d", status)
	}
	writeJSON(writer, status, CommunicationErrorResponse{Status: CommunicationStatusError, Message: message})
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
