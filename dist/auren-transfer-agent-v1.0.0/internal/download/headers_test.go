package download

import (
	"context"
	"net/http"
	"testing"
)

func TestHeaderSetNormalizesAndAppliesHeaders(t *testing.T) {
	set, err := NewHeaderSet(map[string]string{"x-trace-id": " abc ", "accept": "*/*"})
	if err != nil {
		t.Fatalf("new header set: %v", err)
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.test/file", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err := set.Apply(request); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if request.Header.Get("X-Trace-Id") != "abc" {
		t.Fatalf("trace header = %q", request.Header.Get("X-Trace-Id"))
	}
	if set.Get("ACCEPT") != "*/*" {
		t.Fatalf("accept = %q", set.Get("ACCEPT"))
	}
}

func TestHeaderSetRejectsUnsafeHeaders(t *testing.T) {
	if _, err := NewHeaderSet(map[string]string{"bad header": "value"}); err == nil {
		t.Fatalf("expected invalid name error")
	}
	if _, err := NewHeaderSet(map[string]string{"X-Test": "ok\nno"}); err == nil {
		t.Fatalf("expected newline value error")
	}
}

func TestHeaderSetCloneIsDefensive(t *testing.T) {
	set, err := NewHeaderSet(map[string]string{"X-Test": "one"})
	if err != nil {
		t.Fatalf("new header set: %v", err)
	}
	clone := set.Clone()
	clone.Set("X-Test", "two")
	if set.Get("X-Test") != "one" {
		t.Fatalf("original header was mutated")
	}
}

func TestNewRequestAppliesHeadersAndDefaultsMethod(t *testing.T) {
	set, err := NewHeaderSet(map[string]string{"X-Test": "one"})
	if err != nil {
		t.Fatalf("new header set: %v", err)
	}
	request, err := NewRequest(context.Background(), RequestOptions{URL: "https://example.test/video.mp4", Headers: set})
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if request.Method != http.MethodGet {
		t.Fatalf("method = %s", request.Method)
	}
	if request.Header.Get("X-Test") != "one" {
		t.Fatalf("header = %q", request.Header.Get("X-Test"))
	}
}
