package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
	"github.com/cesarsousa94/auren-transfer-agent/internal/logger"
)

func TestDefaultMiddlewareRegistryBuildsCanonicalStack(t *testing.T) {
	buffer := &bytes.Buffer{}
	log, err := logger.New(config.DefaultConfig().Logger, buffer)
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}

	registry, err := DefaultMiddlewareRegistry(MiddlewareOptions{Logger: log, RequestLogging: true, RecoverPanics: true})
	if err != nil {
		t.Fatalf("DefaultMiddlewareRegistry() error = %v", err)
	}

	want := []MiddlewareInfo{{Name: MiddlewareNameRequestLogger}, {Name: MiddlewareNameRecoverer}}
	if !reflect.DeepEqual(registry.Snapshot(), want) {
		t.Fatalf("Snapshot() = %#v, want %#v", registry.Snapshot(), want)
	}
	if registry.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", registry.Len())
	}
}

func TestMiddlewareRegistryValidationAndDefensiveCopies(t *testing.T) {
	registry, err := NewMiddlewareRegistry(MiddlewareDefinition{Name: " Alpha ", Handler: passThroughMiddleware})
	if err != nil {
		t.Fatalf("NewMiddlewareRegistry() error = %v", err)
	}
	if err := registry.Add(MiddlewareDefinition{Name: "alpha", Handler: passThroughMiddleware}); err == nil {
		t.Fatal("Add() duplicate error = nil, want error")
	}

	definitions := registry.Definitions()
	definitions[0].Name = "mutated"
	if registry.Definitions()[0].Name != "alpha" {
		t.Fatal("Definitions() did not return a defensive copy")
	}

	snapshot := registry.Snapshot()
	snapshot[0].Name = "mutated"
	if registry.Snapshot()[0].Name != "alpha" {
		t.Fatal("Snapshot() did not return a defensive copy")
	}

	middlewares := registry.Middlewares()
	middlewares[0] = nil
	if registry.Middlewares()[0] == nil {
		t.Fatal("Middlewares() did not return a defensive copy")
	}
}

func TestNormalizeMiddlewareDefinitionRejectsInvalidDefinitions(t *testing.T) {
	cases := []MiddlewareDefinition{
		{Name: "", Handler: passThroughMiddleware},
		{Name: "nil", Handler: nil},
	}

	for _, candidate := range cases {
		if _, err := NormalizeMiddlewareDefinition(candidate); err == nil {
			t.Fatalf("NormalizeMiddlewareDefinition(%#v) error = nil, want error", candidate)
		}
	}
}

func TestBuildRouterAppliesMiddlewaresInOrder(t *testing.T) {
	order := []string{}
	first := MiddlewareDefinition{Name: "first", Handler: func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			order = append(order, "first-before")
			next.ServeHTTP(writer, request)
			order = append(order, "first-after")
		})
	}}
	second := MiddlewareDefinition{Name: "second", Handler: func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			order = append(order, "second-before")
			next.ServeHTTP(writer, request)
			order = append(order, "second-after")
		})
	}}

	router, err := BuildRouter(RouterOptions{
		Middlewares: []MiddlewareDefinition{first, second},
		Routes: []RouteDefinition{{Method: http.MethodGet, Pattern: "/ordered", Handler: func(writer http.ResponseWriter, _ *http.Request) {
			order = append(order, "handler")
			writer.WriteHeader(http.StatusNoContent)
		}}},
	})
	if err != nil {
		t.Fatalf("BuildRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ordered", nil))

	want := []string{"first-before", "second-before", "handler", "second-after", "first-after"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("middleware order = %#v, want %#v", order, want)
	}
}

func TestBuildRouterRequestLoggerAndRecoverer(t *testing.T) {
	buffer := &bytes.Buffer{}
	log, err := logger.New(config.DefaultConfig().Logger, buffer)
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}

	router, err := BuildRouter(RouterOptions{
		Logger:         log,
		RequestLogging: true,
		RecoverPanics:  true,
		Routes: []RouteDefinition{{Method: http.MethodGet, Pattern: "/panic", Handler: func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}}},
	})
	if err != nil {
		t.Fatalf("BuildRouter() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	request.Header.Set(logger.RequestIDHeader, "req_middleware_1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	lines := nonEmptyLines(buffer.String())
	if len(lines) != 2 {
		t.Fatalf("log lines = %d, want 2; payload=%q", len(lines), buffer.String())
	}

	recoveryEvent, err := logger.DecodeJSONLine(lines[0])
	if err != nil {
		t.Fatalf("recovery log should be json: %v", err)
	}
	if recoveryEvent[logger.FieldComponent] != MiddlewareComponent {
		t.Fatalf("recovery component = %#v, want %s", recoveryEvent[logger.FieldComponent], MiddlewareComponent)
	}

	requestEvent, err := logger.DecodeJSONLine(lines[1])
	if err != nil {
		t.Fatalf("request log should be json: %v", err)
	}
	if requestEvent[logger.FieldHTTPStatus] != float64(http.StatusInternalServerError) {
		t.Fatalf("http_status = %#v, want %d", requestEvent[logger.FieldHTTPStatus], http.StatusInternalServerError)
	}
	if requestEvent[logger.FieldRequestID] != "req_middleware_1" {
		t.Fatalf("request_id = %#v, want req_middleware_1", requestEvent[logger.FieldRequestID])
	}
}

func TestRegisterMiddlewaresNilRouterReturnsError(t *testing.T) {
	if err := RegisterMiddlewares(nil, MiddlewareDefinition{Name: "x", Handler: passThroughMiddleware}); err == nil {
		t.Fatal("RegisterMiddlewares(nil) error = nil, want error")
	}
}

func passThroughMiddleware(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(writer, request)
	})
}

func nonEmptyLines(payload string) []string {
	parts := strings.Split(payload, "\n")
	lines := []string{}
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			lines = append(lines, part)
		}
	}
	return lines
}
