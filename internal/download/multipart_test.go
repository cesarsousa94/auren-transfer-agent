package download

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewMultipartPlanBuildsDeterministicRanges(t *testing.T) {
	plan, err := NewMultipartPlan(10, 4)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Parts) != 3 {
		t.Fatalf("parts = %d", len(plan.Parts))
	}
	expected := []string{"bytes=0-3", "bytes=4-7", "bytes=8-9"}
	for index, part := range plan.Parts {
		if part.Index != index || part.RangeHeader != expected[index] {
			t.Fatalf("part %d = %+v", index, part)
		}
	}
}

func TestMultipartPlanPartsSnapshotIsDefensive(t *testing.T) {
	plan, err := NewMultipartPlan(8, 4)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	snapshot := plan.PartsSnapshot()
	snapshot[0].RangeHeader = "changed"
	if plan.Parts[0].RangeHeader == "changed" {
		t.Fatalf("snapshot modified plan")
	}
}

func TestHTTPClientMultipartToFileDownloadsAndMergesParts(t *testing.T) {
	payload := []byte("abcdefghijkl")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var start, end int
		if _, err := fmt.Sscanf(request.Header.Get(HeaderRange), "bytes=%d-%d", &start, &end); err != nil {
			t.Fatalf("range parse: %v header=%q", err, request.Header.Get(HeaderRange))
		}
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write(payload[start : end+1])
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "out", "media.bin")
	client := newTestHTTPClient(t)
	result, err := client.MultipartToFile(context.Background(), MultipartOptions{URL: server.URL, Path: path, TotalSize: int64(len(payload)), PartSize: 5, CreateDirs: true})
	if err != nil {
		t.Fatalf("multipart: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(content) != string(payload) {
		t.Fatalf("content = %q", string(content))
	}
	if result.BytesWritten != int64(len(payload)) || len(result.Parts) != 3 {
		t.Fatalf("result = %+v", result)
	}
}

func TestHTTPClientMultipartToFileRequiresPartialContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("full"))
	}))
	defer server.Close()

	client := newTestHTTPClient(t)
	_, err := client.MultipartToFile(context.Background(), MultipartOptions{URL: server.URL, Path: filepath.Join(t.TempDir(), "media.bin"), TotalSize: 4, PartSize: 2})
	if err == nil {
		t.Fatalf("expected partial content error")
	}
}
