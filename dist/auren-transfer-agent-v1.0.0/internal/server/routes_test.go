package server

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestBuildRouterRegistersRouteDefinitions(t *testing.T) {
	router, err := BuildRouter(RouterOptions{Routes: []RouteDefinition{
		{Method: " get ", Pattern: "router", Handler: func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte("registered"))
		}},
	}})
	if err != nil {
		t.Fatalf("BuildRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/router", nil))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if recorder.Body.String() != "registered" {
		t.Fatalf("body = %q, want registered", recorder.Body.String())
	}
}

func TestRouteRegistrySnapshotAndDuplicateValidation(t *testing.T) {
	registry, err := NewRouteRegistry(RouteDefinition{Method: http.MethodGet, Pattern: "/alpha", Handler: okHandler})
	if err != nil {
		t.Fatalf("NewRouteRegistry() error = %v", err)
	}
	if err := registry.Add(RouteDefinition{Name: "beta", Method: http.MethodPost, Pattern: "/beta", Handler: okHandler}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := registry.Add(RouteDefinition{Method: http.MethodGet, Pattern: "/alpha", Handler: okHandler}); err == nil {
		t.Fatal("Add() duplicate error = nil, want error")
	}

	snapshot := registry.Snapshot()
	want := []RouteInfo{
		{Name: "GET /alpha", Method: http.MethodGet, Pattern: "/alpha"},
		{Name: "beta", Method: http.MethodPost, Pattern: "/beta"},
	}
	if !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", snapshot, want)
	}
	if registry.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", registry.Len())
	}

	snapshot[0].Name = "mutated"
	if registry.Snapshot()[0].Name != "GET /alpha" {
		t.Fatal("Snapshot() did not return a defensive copy")
	}
}

func TestNormalizeRouteDefinitionRejectsInvalidRoutes(t *testing.T) {
	cases := []RouteDefinition{
		{Method: "", Pattern: "/x", Handler: okHandler},
		{Method: "BREW", Pattern: "/x", Handler: okHandler},
		{Method: http.MethodGet, Pattern: "", Handler: okHandler},
		{Method: http.MethodGet, Pattern: "/x", Handler: nil},
	}

	for _, candidate := range cases {
		if _, err := NormalizeRouteDefinition(candidate); err == nil {
			t.Fatalf("NormalizeRouteDefinition(%#v) error = nil, want error", candidate)
		}
	}
}

func TestSupportedHTTPMethodsReturnsSortedDefensiveCopy(t *testing.T) {
	methods := SupportedHTTPMethods()
	want := []string{"CONNECT", "DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT", "TRACE"}
	if !reflect.DeepEqual(methods, want) {
		t.Fatalf("SupportedHTTPMethods() = %#v, want %#v", methods, want)
	}

	methods[0] = "MUTATED"
	if SupportedHTTPMethods()[0] != "CONNECT" {
		t.Fatal("SupportedHTTPMethods() did not return a defensive copy")
	}
}

func TestRegistryRegisterNilRouterReturnsError(t *testing.T) {
	registry, err := NewRouteRegistry(RouteDefinition{Method: http.MethodGet, Pattern: "/alpha", Handler: okHandler})
	if err != nil {
		t.Fatalf("NewRouteRegistry() error = %v", err)
	}
	if err := registry.Register(nil); err == nil {
		t.Fatal("Register(nil) error = nil, want error")
	}
}

func okHandler(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusOK)
}
