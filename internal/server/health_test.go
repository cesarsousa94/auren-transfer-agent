package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
)

func TestHealthHandlerReturnsFoundationPayload(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, HealthPath, nil)

	HealthHandler(runtime.Info()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var payload HealthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	want := HealthResponse{
		Status:        HealthResponseStatus,
		Name:          runtime.AppName,
		Version:       runtime.Version,
		RuntimeStatus: runtime.Status,
		Router:        RouterKind,
	}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("health payload = %#v, want %#v", payload, want)
	}
}

func TestHealthRouteBuildsCanonicalRouteDefinition(t *testing.T) {
	route := HealthRoute(runtime.Info())

	if route.Name != HealthRouteName {
		t.Fatalf("Name = %q, want %q", route.Name, HealthRouteName)
	}
	if route.Method != http.MethodGet {
		t.Fatalf("Method = %q, want %q", route.Method, http.MethodGet)
	}
	if route.Pattern != HealthPath {
		t.Fatalf("Pattern = %q, want %q", route.Pattern, HealthPath)
	}
	if route.Handler == nil {
		t.Fatal("Handler = nil, want handler")
	}
}

func TestBuildRouterServesHealthRoute(t *testing.T) {
	router, err := BuildRouter(RouterOptions{Routes: HealthRoutes(runtime.Info())})
	if err != nil {
		t.Fatalf("BuildRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, HealthPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %q, want status ok", recorder.Body.String())
	}
}

func TestHealthRoutesReturnsDefensiveSlice(t *testing.T) {
	routes := HealthRoutes(runtime.Info())
	if len(routes) != 1 {
		t.Fatalf("len(HealthRoutes()) = %d, want 1", len(routes))
	}

	routes[0].Name = "mutated"
	if HealthRoutes(runtime.Info())[0].Name != HealthRouteName {
		t.Fatal("HealthRoutes() did not return an independent slice")
	}
}
