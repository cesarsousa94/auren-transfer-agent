package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auren/auren-transfer-agent/internal/config"
	"github.com/auren/auren-transfer-agent/internal/logger"
)

func TestNewRouterRegistersChiRoute(t *testing.T) {
	buffer := &bytes.Buffer{}
	log, err := logger.New(config.DefaultConfig().Logger, buffer)
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}

	router := NewRouter(RouterOptions{Logger: log, RequestLogging: true})
	RegisterRoute(router, RouteInfo{Method: http.MethodGet, Pattern: "/foundation"}, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusAccepted)
		_, _ = writer.Write([]byte("chi-ready"))
	})

	request := httptest.NewRequest(http.MethodGet, "/foundation", nil)
	request.Header.Set(logger.RequestIDHeader, "req_router_1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if body := recorder.Body.String(); body != "chi-ready" {
		t.Fatalf("body = %q, want chi-ready", body)
	}

	event, err := logger.DecodeJSONLine(buffer.String())
	if err != nil {
		t.Fatalf("request log should be json: %v", err)
	}
	if event[logger.FieldHTTPStatus] != float64(http.StatusAccepted) {
		t.Fatalf("http_status = %#v, want %d", event[logger.FieldHTTPStatus], http.StatusAccepted)
	}
	if event[logger.FieldRequestID] != "req_router_1" {
		t.Fatalf("request_id = %#v, want req_router_1", event[logger.FieldRequestID])
	}
}

func TestRouterKindName(t *testing.T) {
	if RouterKindName() != "chi" {
		t.Fatalf("RouterKindName() = %q, want chi", RouterKindName())
	}
}

func TestRegisterRouteIgnoresNilInputs(t *testing.T) {
	RegisterRoute(nil, RouteInfo{Method: http.MethodGet, Pattern: "/nil"}, nil)

	router := NewRouter(RouterOptions{})
	RegisterRoute(router, RouteInfo{Method: http.MethodGet, Pattern: "/nil"}, nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/nil", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
