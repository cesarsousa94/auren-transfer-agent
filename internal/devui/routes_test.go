package devui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auren/auren-transfer-agent/internal/server"
	"github.com/rs/zerolog"
)

func TestDevUIJSONRoutesReturnJSONForCanonicalAndTrailingPaths(t *testing.T) {
	recorder := NewRecorder(10)
	router, err := server.BuildRouter(server.RouterOptions{
		Logger: zerolog.New(io.Discard),
		Routes: Routes(Options{Config: Config{Enabled: true, Path: "/_auren/dev"}, Recorder: recorder, Snapshot: func() MetricsSnapshot {
			return MetricsSnapshot{MediaHub: map[string]any{"node_uuid": "node-1"}}
		}}),
	})
	if err != nil {
		t.Fatalf("BuildRouter: %v", err)
	}

	for _, path := range []string{"/_auren/dev/api/snapshot", "/_auren/dev/api/snapshot/", "/_auren/dev/api/requests", "/_auren/dev/api/requests/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("%s content-type = %q", path, ct)
		}
		var decoded any
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("%s invalid JSON: %v body=%s", path, err, rec.Body.String())
		}
	}
}

func TestDevUIAPIFallbackReturnsJSONNotPlain404(t *testing.T) {
	router, err := server.BuildRouter(server.RouterOptions{
		Logger: zerolog.New(io.Discard),
		Routes: Routes(Options{Config: Config{Enabled: true, Path: "/_auren/dev"}, Recorder: NewRecorder(10)}),
	})
	if err != nil {
		t.Fatalf("BuildRouter: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/_auren/dev/api/missing", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, rec.Body.String())
	}
	if decoded["error"] != "dev_ui_api_route_not_found" {
		t.Fatalf("decoded = %#v", decoded)
	}
}
