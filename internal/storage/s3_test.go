package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestS3AdapterUploadsWithSigV4(t *testing.T) {
	var gotPath, gotAuth, gotHash string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotHash = r.Header.Get("X-Amz-Content-Sha256")
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s", r.Method)
		}
		payload, _ := io.ReadAll(r.Body)
		if string(payload) != "hello" {
			t.Fatalf("payload = %q", payload)
		}
		w.Header().Set("ETag", `"etag-1"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	file, err := os.CreateTemp(t.TempDir(), "s3-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("hello"); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	adapter, err := NewS3Adapter(S3Options{Endpoint: server.URL, Bucket: "bucket-a", Region: "us-east-1", AccessKeyID: "AKIA_TEST", SecretAccessKey: "secret", ForcePathStyle: true, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Upload(context.Background(), UploadRequest{SourcePath: file.Name(), ObjectPath: "dir/file.txt", MimeType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/bucket-a/dir/file.txt" {
		t.Fatalf("path = %s", gotPath)
	}
	if !strings.Contains(gotAuth, "AWS4-HMAC-SHA256") || !strings.Contains(gotAuth, "Credential=AKIA_TEST/") {
		t.Fatalf("auth = %s", gotAuth)
	}
	if gotHash == "" || result.ChecksumSHA256 != gotHash {
		t.Fatalf("hash/result mismatch got=%s result=%s", gotHash, result.ChecksumSHA256)
	}
	if result.Driver != DriverS3 || result.Bucket != "bucket-a" || result.ObjectPath != "dir/file.txt" {
		t.Fatalf("result = %#v", result)
	}
}

func TestS3Configured(t *testing.T) {
	if !S3Configured("b", "a", "s") {
		t.Fatal("expected configured")
	}
	if S3Configured("", "a", "s") {
		t.Fatal("expected not configured")
	}
}
