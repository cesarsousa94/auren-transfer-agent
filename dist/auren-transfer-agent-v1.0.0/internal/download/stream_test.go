package download

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHTTPClientStreamWritesResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Test") != "stream" {
			t.Fatalf("missing stream header: %q", request.Header.Get("X-Test"))
		}
		_, _ = writer.Write([]byte("hello world"))
	}))
	defer server.Close()

	client := newTestHTTPClient(t)
	headers, err := NewHeaderSet(map[string]string{"X-Test": "stream"})
	if err != nil {
		t.Fatalf("headers: %v", err)
	}
	var output bytes.Buffer
	result, err := client.Stream(context.Background(), StreamOptions{URL: server.URL, Headers: headers, Writer: &output})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if output.String() != "hello world" {
		t.Fatalf("output = %q", output.String())
	}
	if result.StatusCode != http.StatusOK || result.BytesWritten != 11 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestHTTPClientStreamRequiresPartialContentForResume(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("full"))
	}))
	defer server.Close()

	client := newTestHTTPClient(t)
	var output bytes.Buffer
	_, err := client.Stream(context.Background(), StreamOptions{URL: server.URL, Resume: ResumeState{RangeHeader: "bytes=2-"}, Writer: &output})
	if err == nil {
		t.Fatalf("expected resume status error")
	}
}

func TestHTTPClientStreamToFileAppendsResumeContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get(HeaderRange) != "bytes=4-" {
			t.Fatalf("range = %q", request.Header.Get(HeaderRange))
		}
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write([]byte("tail"))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "nested", "file.bin")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("head"), 0o644); err != nil {
		t.Fatalf("write head: %v", err)
	}
	client := newTestHTTPClient(t)
	result, err := client.StreamToFile(context.Background(), StreamToFileOptions{URL: server.URL, Path: path, Resume: ResumeState{RangeHeader: "bytes=4-"}, Append: true, CreateDirs: true})
	if err != nil {
		t.Fatalf("stream to file: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "headtail" || result.BytesWritten != 4 || !result.Resumed {
		t.Fatalf("content=%q result=%+v", string(content), result)
	}
}

func newTestHTTPClient(t *testing.T) *HTTPClient {
	t.Helper()
	client, err := NewHTTPClient(HTTPClientOptions{UserAgent: "AurenTransferAgent/Test", ConnectTimeout: "1s", ResponseHeaderTimeout: "1s", IdleTimeout: "1s", FollowRedirects: true, MaxRedirects: 2})
	if err != nil {
		t.Fatalf("new http client: %v", err)
	}
	return client
}
