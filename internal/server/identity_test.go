package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	agentidentity "github.com/auren/auren-transfer-agent/internal/identity"
	"github.com/auren/auren-transfer-agent/internal/runtime"
)

func TestIdentityHandlerReturnsFoundationPayload(t *testing.T) {
	snapshot := testIdentitySnapshot(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, IdentityPath, nil)

	IdentityHandler(runtime.Info(), snapshot).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var payload IdentityResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode identity response: %v", err)
	}

	want := NewIdentityResponse(runtime.Info(), snapshot)
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("identity payload = %#v, want %#v", payload, want)
	}
}

func TestIdentityRouteBuildsCanonicalRouteDefinition(t *testing.T) {
	route := IdentityRoute(runtime.Info(), testIdentitySnapshot(t))

	if route.Name != IdentityRouteName {
		t.Fatalf("Name = %q, want %q", route.Name, IdentityRouteName)
	}
	if route.Method != http.MethodGet {
		t.Fatalf("Method = %q, want %q", route.Method, http.MethodGet)
	}
	if route.Pattern != IdentityPath {
		t.Fatalf("Pattern = %q, want %q", route.Pattern, IdentityPath)
	}
	if route.Handler == nil {
		t.Fatal("Handler = nil, want handler")
	}
}

func TestBuildRouterServesIdentityRoute(t *testing.T) {
	snapshot := testIdentitySnapshot(t)
	router, err := BuildRouter(RouterOptions{Routes: IdentityRoutes(runtime.Info(), snapshot)})
	if err != nil {
		t.Fatalf("BuildRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, IdentityPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"agent_id":"`+snapshot.AgentID+`"`) {
		t.Fatalf("body = %q, want agent_id", recorder.Body.String())
	}
}

func TestIdentityRoutesReturnsDefensiveSlice(t *testing.T) {
	routes := IdentityRoutes(runtime.Info(), testIdentitySnapshot(t))
	if len(routes) != 1 {
		t.Fatalf("len(IdentityRoutes()) = %d, want 1", len(routes))
	}

	routes[0].Name = "mutated"
	if IdentityRoutes(runtime.Info(), testIdentitySnapshot(t))[0].Name != IdentityRouteName {
		t.Fatal("IdentityRoutes() did not return an independent slice")
	}
}

func TestFoundationRoutesIncludesIdentityWhenSnapshotProvided(t *testing.T) {
	routes := FoundationRoutes(runtime.Info(), testIdentitySnapshot(t))
	if len(routes) != 4 {
		t.Fatalf("len(FoundationRoutes(identity)) = %d, want 4", len(routes))
	}
	if routes[3].Name != IdentityRouteName {
		t.Fatalf("last route = %q, want %q", routes[3].Name, IdentityRouteName)
	}
}

func testIdentitySnapshot(t *testing.T) agentidentity.Snapshot {
	t.Helper()
	fingerprint, err := agentidentity.NewFingerprint("123e4567-e89b-42d3-a456-426614174000", "auren-node-01")
	if err != nil {
		t.Fatalf("NewFingerprint returned error: %v", err)
	}
	return agentidentity.Snapshot{
		AgentID:              "123e4567-e89b-42d3-a456-426614174000",
		Fingerprint:          fingerprint,
		FingerprintAlgorithm: agentidentity.FingerprintAlgorithm,
		Hostname:             "auren-node-01",
		HostnameSource:       agentidentity.HostnameSourceOS,
		Persistence:          agentidentity.StorePersistencePersistent,
		StoreSource:          "loaded",
		StorePath:            "/var/lib/auren-transfer-agent/identity/agent.json",
	}
}
