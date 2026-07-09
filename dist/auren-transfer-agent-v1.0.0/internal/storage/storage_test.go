package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeObjectPath(t *testing.T) {
	path, err := NormalizeObjectPath("media//file.bin")
	if err != nil {
		t.Fatal(err)
	}
	if path != "media/file.bin" {
		t.Fatalf("unexpected path %q", path)
	}
	for _, invalid := range []string{"", "/absolute", "../escape", "a/../../escape"} {
		if _, err := NormalizeObjectPath(invalid); err == nil {
			t.Fatalf("expected invalid object path %q", invalid)
		}
	}
}

func TestLocalAdapterUpload(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.bin")
	if err := os.WriteFile(source, []byte("storage local"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewLocalAdapter(filepath.Join(tmp, "root"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Upload(context.Background(), UploadRequest{SourcePath: source, ObjectPath: "objects/source.bin", Metadata: map[string]string{"k": "v"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Adapter != DriverLocal || result.Driver != DriverLocal || result.ObjectPath != "objects/source.bin" || result.BytesWritten != int64(len("storage local")) {
		t.Fatalf("unexpected result: %+v", result)
	}
	stored, err := os.ReadFile(filepath.Join(tmp, "root", "objects", "source.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != "storage local" {
		t.Fatalf("unexpected stored payload %q", stored)
	}
}

func TestAurenStorageAdapterUpload(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.bin")
	payload := []byte("auren storage")
	if err := os.WriteFile(source, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	var sawPath string
	var sawAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/v1/buckets/media/objects") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		sawPath = r.URL.Query().Get("path")
		sawAuth = r.Header.Get("Authorization")
		if r.Header.Get("X-Auren-Object-Path") != "movies/file.bin" || r.Header.Get("X-Auren-Region") != "sa-east-1" {
			t.Fatalf("missing auren headers")
		}
		w.Header().Set("Location", "https://storage.example/objects/movies/file.bin")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	adapter, err := NewAurenStorageAdapter(AurenOptions{Endpoint: server.URL, Bucket: "media", Region: "sa-east-1", APIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Upload(context.Background(), UploadRequest{SourcePath: source, ObjectPath: "movies/file.bin"})
	if err != nil {
		t.Fatal(err)
	}
	if sawPath != "movies/file.bin" || sawAuth != "Bearer secret" {
		t.Fatalf("unexpected request path/auth %q %q", sawPath, sawAuth)
	}
	if result.Driver != DriverAurenStorage || result.BytesWritten != int64(len(payload)) || result.Location == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
