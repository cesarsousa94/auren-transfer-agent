package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auren/auren-transfer-agent/internal/heartbeat"
	"github.com/auren/auren-transfer-agent/internal/identity"
	"github.com/auren/auren-transfer-agent/internal/runtime"
	"github.com/auren/auren-transfer-agent/internal/worker"
)

func TestCommunicationRoutesExposeRESTRegistrationHeartbeatAndWebSocket(t *testing.T) {
	options := testCommunicationOptions(t, false)
	router, err := BuildRouter(RouterOptions{Routes: CommunicationRoutes(options)})
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	statusRecorder := httptest.NewRecorder()
	router.ServeHTTP(statusRecorder, httptest.NewRequest(http.MethodGet, CommunicationAPIBasePath, nil))
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("expected REST status 200, got %d: %s", statusRecorder.Code, statusRecorder.Body.String())
	}
	var status RESTStatusResponse
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !status.RESTAPI || !status.WebSocket || !status.Registration || !status.Heartbeat {
		t.Fatalf("expected all communication capabilities enabled: %+v", status)
	}

	registrationRecorder := httptest.NewRecorder()
	router.ServeHTTP(registrationRecorder, httptest.NewRequest(http.MethodPost, CommunicationRegistrationPath, nil))
	if registrationRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected registration 202, got %d: %s", registrationRecorder.Code, registrationRecorder.Body.String())
	}

	heartbeatRecorder := httptest.NewRecorder()
	router.ServeHTTP(heartbeatRecorder, httptest.NewRequest(http.MethodPost, CommunicationHeartbeatPath, nil))
	if heartbeatRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected heartbeat post 202, got %d: %s", heartbeatRecorder.Code, heartbeatRecorder.Body.String())
	}

	websocketRecorder := httptest.NewRecorder()
	websocketRequest := httptest.NewRequest(http.MethodGet, CommunicationWebSocketPath, nil)
	websocketRequest.Header.Set("Connection", "Upgrade")
	websocketRequest.Header.Set("Upgrade", "websocket")
	websocketRequest.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	router.ServeHTTP(websocketRecorder, websocketRequest)
	if websocketRecorder.Code != http.StatusSwitchingProtocols {
		t.Fatalf("expected websocket 101, got %d: %s", websocketRecorder.Code, websocketRecorder.Body.String())
	}
	if websocketRecorder.Header().Get("Sec-WebSocket-Accept") != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("unexpected websocket accept key: %s", websocketRecorder.Header().Get("Sec-WebSocket-Accept"))
	}
}

func TestCommunicationAuthenticationRequiresAPIKey(t *testing.T) {
	options := testCommunicationOptions(t, true)
	router, err := BuildRouter(RouterOptions{Routes: CommunicationRoutes(options)})
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, CommunicationAPIBasePath, nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized without api key, got %d", unauthorized.Code)
	}

	authorized := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, CommunicationAPIBasePath, nil)
	request.Header.Set("X-Auren-Agent-Key", "Bearer test-key")
	router.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusOK {
		t.Fatalf("expected authorized status 200, got %d: %s", authorized.Code, authorized.Body.String())
	}
}

func TestWebSocketUpgradeRequired(t *testing.T) {
	options := testCommunicationOptions(t, false)
	recorder := httptest.NewRecorder()
	WebSocketHandler(options)(recorder, httptest.NewRequest(http.MethodGet, CommunicationWebSocketPath, nil))
	if recorder.Code != http.StatusUpgradeRequired {
		t.Fatalf("expected upgrade required, got %d", recorder.Code)
	}
}

func TestCommunicationDefensiveSlices(t *testing.T) {
	capabilities := CommunicationCapabilities()
	capabilities[0] = "mutated"
	if CommunicationCapabilities()[0] == "mutated" {
		t.Fatalf("capabilities must be defensive")
	}

	routes := CommunicationRoutePatterns()
	routes[0] = "mutated"
	if CommunicationRoutePatterns()[0] == "mutated" {
		t.Fatalf("route patterns must be defensive")
	}
}

func testCommunicationOptions(t *testing.T, authRequired bool) CommunicationOptions {
	t.Helper()
	agentID := "123e4567-e89b-42d3-a456-426614174000"
	record, err := identity.NewRecord(agentID, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("identity record: %v", err)
	}
	storeResult := identity.StoreResult{Record: record, Path: "/tmp/agent.json", Created: false}
	snapshot, err := identity.NewSnapshot(storeResult, identity.HostInfo{Raw: "Node-01", Normalized: "node-01", Source: identity.HostnameSourceOS})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	heartbeatRecord, err := heartbeat.NewRecord(heartbeat.Input{Identity: snapshot, Version: runtime.Info(), Status: heartbeat.StatusReady, Interval: time.Second, WorkerEnabled: true, PoolStats: worker.PoolStats{Concurrency: 1, WorkerIDs: []string{"worker-1"}}, QueueStats: heartbeat.QueueStats{Driver: "memory", Capacity: 10}})
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	authenticator, err := NewAuthenticator(AuthOptions{Required: authRequired, APIKey: "test-key", TokenHeader: "X-Auren-Agent-Key"})
	if err != nil {
		t.Fatalf("authenticator: %v", err)
	}
	return CommunicationOptions{Info: runtime.Info(), Identity: snapshot, Heartbeat: heartbeatRecord, Authenticator: authenticator}
}
