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

func TestVersionHandlerReturnsFoundationPayload(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, VersionPath, nil)

	VersionHandler(runtime.Info()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var payload VersionResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode version response: %v", err)
	}

	want := VersionResponse{
		Name:          runtime.AppName,
		Version:       runtime.Version,
		RuntimeStatus: runtime.Status,
		Router:        RouterKind,
	}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("version payload = %#v, want %#v", payload, want)
	}
}

func TestVersionRouteBuildsCanonicalRouteDefinition(t *testing.T) {
	route := VersionRoute(runtime.Info())

	if route.Name != VersionRouteName {
		t.Fatalf("Name = %q, want %q", route.Name, VersionRouteName)
	}
	if route.Method != http.MethodGet {
		t.Fatalf("Method = %q, want %q", route.Method, http.MethodGet)
	}
	if route.Pattern != VersionPath {
		t.Fatalf("Pattern = %q, want %q", route.Pattern, VersionPath)
	}
	if route.Handler == nil {
		t.Fatal("Handler = nil, want handler")
	}
}

func TestBuildRouterServesVersionRoute(t *testing.T) {
	router, err := BuildRouter(RouterOptions{Routes: VersionRoutes(runtime.Info())})
	if err != nil {
		t.Fatalf("BuildRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, VersionPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"version":"`+runtime.Version+`"`) {
		t.Fatalf("body = %q, want runtime version", recorder.Body.String())
	}
}

func TestFoundationRoutesIncludeHealthVersionAndReady(t *testing.T) {
	routes := FoundationRoutes(runtime.Info())
	if len(routes) != 3 {
		t.Fatalf("len(FoundationRoutes()) = %d, want 3", len(routes))
	}
	if routes[0].Name != HealthRouteName {
		t.Fatalf("routes[0].Name = %q, want %q", routes[0].Name, HealthRouteName)
	}
	if routes[1].Name != VersionRouteName {
		t.Fatalf("routes[1].Name = %q, want %q", routes[1].Name, VersionRouteName)
	}
	if routes[2].Name != ReadyRouteName {
		t.Fatalf("routes[2].Name = %q, want %q", routes[2].Name, ReadyRouteName)
	}
}

func TestVersionRoutesAndFoundationRoutesReturnDefensiveSlices(t *testing.T) {
	versionRoutes := VersionRoutes(runtime.Info())
	if len(versionRoutes) != 1 {
		t.Fatalf("len(VersionRoutes()) = %d, want 1", len(versionRoutes))
	}
	versionRoutes[0].Name = "mutated"
	if VersionRoutes(runtime.Info())[0].Name != VersionRouteName {
		t.Fatal("VersionRoutes() did not return an independent slice")
	}

	foundationRoutes := FoundationRoutes(runtime.Info())
	foundationRoutes[0].Name = "mutated"
	foundationRoutes[1].Name = "mutated"
	foundationRoutes[2].Name = "mutated"
	next := FoundationRoutes(runtime.Info())
	if next[0].Name != HealthRouteName || next[1].Name != VersionRouteName || next[2].Name != ReadyRouteName {
		t.Fatal("FoundationRoutes() did not return an independent slice")
	}
}
