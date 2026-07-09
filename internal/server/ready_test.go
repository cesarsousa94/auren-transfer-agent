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

func TestReadyHandlerReturnsFoundationPayload(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, ReadyPath, nil)

	ReadyHandler(runtime.Info()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var payload ReadyResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode ready response: %v", err)
	}

	want := ReadyResponse{
		Status:        ReadyResponseStatus,
		Ready:         true,
		Name:          runtime.AppName,
		Version:       runtime.Version,
		RuntimeStatus: runtime.Status,
		Router:        RouterKind,
	}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("ready payload = %#v, want %#v", payload, want)
	}
}

func TestReadyRouteBuildsCanonicalRouteDefinition(t *testing.T) {
	route := ReadyRoute(runtime.Info())

	if route.Name != ReadyRouteName {
		t.Fatalf("Name = %q, want %q", route.Name, ReadyRouteName)
	}
	if route.Method != http.MethodGet {
		t.Fatalf("Method = %q, want %q", route.Method, http.MethodGet)
	}
	if route.Pattern != ReadyPath {
		t.Fatalf("Pattern = %q, want %q", route.Pattern, ReadyPath)
	}
	if route.Handler == nil {
		t.Fatal("Handler = nil, want handler")
	}
}

func TestBuildRouterServesReadyRoute(t *testing.T) {
	router, err := BuildRouter(RouterOptions{Routes: ReadyRoutes(runtime.Info())})
	if err != nil {
		t.Fatalf("BuildRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, ReadyPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"ready":true`) {
		t.Fatalf("body = %q, want ready true", recorder.Body.String())
	}
}

func TestReadyRoutesReturnsDefensiveSlice(t *testing.T) {
	routes := ReadyRoutes(runtime.Info())
	if len(routes) != 1 {
		t.Fatalf("len(ReadyRoutes()) = %d, want 1", len(routes))
	}

	routes[0].Name = "mutated"
	if ReadyRoutes(runtime.Info())[0].Name != ReadyRouteName {
		t.Fatal("ReadyRoutes() did not return an independent slice")
	}
}
