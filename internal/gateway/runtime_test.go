package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auren/auren-transfer-agent/internal/config"
	"github.com/auren/auren-transfer-agent/internal/mediahub"
	"github.com/auren/auren-transfer-agent/internal/server"
)

type fakeGatewayClient struct {
	resolve    mediahub.GatewayResolveResult
	heartbeats int
	closes     int
	events     int
}

func (client *fakeGatewayClient) ResolveGateway(ctx context.Context, state mediahub.NodeState, payload mediahub.GatewayResolveRequest) (mediahub.GatewayResolveResult, error) {
	return client.resolve, nil
}
func (client *fakeGatewayClient) SendGatewaySessionHeartbeat(ctx context.Context, state mediahub.NodeState, payload mediahub.GatewaySessionHeartbeatPayload) error {
	client.heartbeats++
	return nil
}
func (client *fakeGatewayClient) CloseGatewaySession(ctx context.Context, state mediahub.NodeState, payload mediahub.GatewaySessionClosePayload) error {
	client.closes++
	return nil
}
func (client *fakeGatewayClient) SendGatewayEvents(ctx context.Context, state mediahub.NodeState, payload mediahub.GatewayEventsPayload) error {
	client.events++
	return nil
}

func TestParseHandoffPath(t *testing.T) {
	token, kind, id, ext := parseHandoffPath("/_auren/gateway/tok/movie/123.mp4")
	if token != "tok" || kind != "movie" || id != "123" || ext != "mp4" {
		t.Fatalf("parsed = %q %q %q %q", token, kind, id, ext)
	}
}

func TestGatewayProxyCopiesBodyAndReportsClose(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Range") != "bytes=0-3" {
			t.Fatalf("Range = %q", request.Header.Get("Range"))
		}
		writer.Header().Set("Content-Type", "video/mp4")
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write([]byte("DATA"))
	}))
	defer upstream.Close()

	client := &fakeGatewayClient{resolve: mediahub.GatewayResolveResult{Success: true, Mode: "proxy", UpstreamURL: upstream.URL, SessionID: "sess_1", HeartbeatSeconds: 1}}
	state, err := mediahub.NewNodeState("node-1", "secret", "", time.Now())
	if err != nil {
		t.Fatalf("NewNodeState() error = %v", err)
	}
	runtime, err := NewRuntime(RuntimeOptions{Config: config.MediaHubConfig{GatewayEnabled: true, GatewayProxyEnabled: true, GatewayRedirectEnabled: true, GatewayHeartbeatInterval: "1s"}, Client: client, NodeState: func() mediahub.NodeState { return state }})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	routes := runtime.Routes()
	router, err := server.BuildRouter(server.RouterOptions{Routes: routes})
	if err != nil {
		t.Fatalf("BuildRouter() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/_auren/gateway/tok/movie/123.mp4", nil)
	request.Header.Set("Range", "bytes=0-3")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Body.String() != "DATA" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
	if client.closes != 1 {
		t.Fatalf("closes = %d, want 1", client.closes)
	}
	if stats := runtime.Stats(); stats.BytesSent != 4 || stats.ActiveSessions != 0 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestGatewayRedirectDoesNotProxyBody(t *testing.T) {
	client := &fakeGatewayClient{resolve: mediahub.GatewayResolveResult{Success: true, Mode: "redirect", UpstreamURL: "https://cdn.example.test/stream.ts", SessionID: "sess_redirect"}}
	state, err := mediahub.NewNodeState("node-1", "secret", "", time.Now())
	if err != nil {
		t.Fatalf("NewNodeState() error = %v", err)
	}
	runtime, err := NewRuntime(RuntimeOptions{Config: config.MediaHubConfig{GatewayEnabled: true, GatewayProxyEnabled: true, GatewayRedirectEnabled: true}, Client: client, NodeState: func() mediahub.NodeState { return state }})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	router, err := server.BuildRouter(server.RouterOptions{Routes: runtime.Routes()})
	if err != nil {
		t.Fatalf("BuildRouter() error = %v", err)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/_auren/gateway/tok/live/55.ts", nil))
	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "https://cdn.example.test/stream.ts" {
		t.Fatalf("Location = %q", location)
	}
	if client.heartbeats != 1 || client.closes != 1 {
		t.Fatalf("heartbeats=%d closes=%d", client.heartbeats, client.closes)
	}
}
