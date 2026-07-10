package devui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRecorderKeepsBoundedNewestSnapshot(t *testing.T) {
	recorder := NewRecorder(2)
	recorder.Record(RequestRecord{Direction: "inbound", Method: http.MethodGet, Path: "/one", Status: 200})
	recorder.Record(RequestRecord{Direction: "outbound", Method: http.MethodPost, Path: "/two", Status: 201})
	recorder.Record(RequestRecord{Direction: "outbound", Method: http.MethodPost, Path: "/three", Status: 500, Error: "boom"})

	snapshot := recorder.Snapshot(10)
	if len(snapshot) != 2 {
		t.Fatalf("len(snapshot) = %d, want 2", len(snapshot))
	}
	if snapshot[0].Path != "/three" || snapshot[1].Path != "/two" {
		t.Fatalf("snapshot order = %#v", snapshot)
	}
	counters := recorder.Counters()
	if counters.Total != 3 || counters.Inbound != 1 || counters.Outbound != 2 || counters.Errors != 1 {
		t.Fatalf("counters = %#v", counters)
	}
}

func TestMiddlewareRecordsInboundRequest(t *testing.T) {
	recorder := NewRecorder(10)
	handler := recorder.Middleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusAccepted)
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/health?verbose=1", nil)
	req.Header.Set("User-Agent", "agent-test")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	snapshot := recorder.Snapshot(1)
	if len(snapshot) != 1 {
		t.Fatalf("len(snapshot) = %d, want 1", len(snapshot))
	}
	if snapshot[0].Direction != "inbound" || snapshot[0].Status != http.StatusAccepted || snapshot[0].Path != "/health?verbose=1" || snapshot[0].Bytes == 0 {
		t.Fatalf("record = %#v", snapshot[0])
	}
}

func TestRecordOutbound(t *testing.T) {
	recorder := NewRecorder(10)
	recorder.RecordOutbound(http.MethodPost, "/api/internal/nodes/heartbeat", 200, 25*time.Millisecond, 42, nil)
	snapshot := recorder.Snapshot(1)
	if len(snapshot) != 1 || snapshot[0].Direction != "outbound" || snapshot[0].DurationMS != 25 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}
