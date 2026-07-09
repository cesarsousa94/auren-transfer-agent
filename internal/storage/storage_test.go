package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeObjectPath(t *testing.T) {
	objectPath, err := NormalizeObjectPath("media//file.bin")
	if err != nil {
		t.Fatal(err)
	}
	if objectPath != "media/file.bin" {
		t.Fatalf("unexpected path %q", objectPath)
	}
	for _, invalid := range []string{"", "/absolute", "../escape", "a/../../escape"} {
		if _, err := NormalizeObjectPath(invalid); err == nil {
			t.Fatalf("expected invalid object path %q", invalid)
		}
	}
}

func TestCanonicalObjectPathFromDirectoryAndRelativePath(t *testing.T) {
	objectPath, err := CanonicalObjectPath(UploadRequest{DirectoryPath: "media-hub/org/originals", RelativePath: "movie.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	if objectPath != "media-hub/org/originals/movie.mp4" {
		t.Fatalf("unexpected object path %q", objectPath)
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

func TestAurenStorageAdapterDirectMultipartFormUpload(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.bin")
	payload := []byte("auren storage production form")
	if err := os.WriteFile(source, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	var sawAuth string
	var sawPath string
	var sawVisibility string
	var sawPayload string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/v1/buckets/bucket-uuid/objects") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		sawAuth = r.Header.Get("Authorization")
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		sawPath = r.FormValue("path")
		sawVisibility = r.FormValue("visibility")
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("missing file part: %v", err)
		}
		body, _ := io.ReadAll(file)
		sawPayload = string(body)
		w.Header().Set("Location", "https://storage.example/objects/movies/file.bin")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"object":{"uuid":"object-1","bucket_uuid":"bucket-uuid","path":"movies/file.bin","size":29,"checksum_sha256":"server-sha","visibility":"private","mime_type":"video/mp4"}}`))
	}))
	defer server.Close()
	adapter, err := NewAurenStorageAdapter(AurenOptions{Endpoint: server.URL, BucketUUID: "bucket-uuid", Region: "sa-east-1", APIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Upload(context.Background(), UploadRequest{SourcePath: source, DirectoryPath: "movies", RelativePath: "file.bin", Visibility: "private", MimeType: "video/mp4", Metadata: map[string]string{"tenant": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	if sawAuth != "Bearer secret" || sawPath != "movies/file.bin" || sawVisibility != "private" || sawPayload != string(payload) {
		t.Fatalf("unexpected request auth=%q path=%q visibility=%q payload=%q", sawAuth, sawPath, sawVisibility, sawPayload)
	}
	if result.Driver != DriverAurenStorage || result.BucketUUID != "bucket-uuid" || result.ObjectUUID != "object-1" || result.ObjectPath != "movies/file.bin" || result.Location == "" || result.BytesWritten != int64(len(payload)) || result.ChecksumSHA256 == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAurenStorageAdapterMultipartUploadLifecycle(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "large.bin")
	payload := []byte("0123456789abcdefghijklmnopqrstuv")
	if err := os.WriteFile(source, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	seen := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/multipart-uploads"):
			_, _ = w.Write([]byte(`{"success":true,"upload_id":"upload-1"}`))
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/multipart-uploads/upload-1/parts/"):
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = w.Write([]byte(`{"success":true,"etag":"etag-part"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/multipart-uploads/upload-1/complete"):
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode complete: %v", err)
			}
			if _, ok := body["parts"].([]any); !ok {
				t.Fatalf("missing parts in completion payload: %#v", body)
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"object_uuid":"object-large","bucket_uuid":"bucket-uuid","path":"large.bin","size":32,"checksum_sha256":"server-large-sha"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	adapter, err := NewAurenStorageAdapter(AurenOptions{Endpoint: server.URL, BucketUUID: "bucket-uuid", APIKey: "secret", MultipartEnabled: true, PartSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Upload(context.Background(), UploadRequest{SourcePath: source, ObjectPath: "large.bin"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Multipart || len(result.Parts) != 4 || result.ObjectUUID != "object-large" || result.ObjectPath != "large.bin" {
		t.Fatalf("unexpected multipart result: %+v", result)
	}
	if len(seen) != 6 { // initiate + 4 parts + complete
		t.Fatalf("unexpected lifecycle requests: %#v", seen)
	}
}
