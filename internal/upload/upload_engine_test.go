package upload

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResumeUploadAppendsMissingBytes(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.bin")
	payload := []byte("abcdefghijklmnopqrstuvwxyz")
	if err := os.WriteFile(source, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	uploader, err := NewLocalUploader(filepath.Join(tmp, "storage"))
	if err != nil {
		t.Fatal(err)
	}
	destination, err := uploader.ResolveDestination("objects/file.bin")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, payload[:10], 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := uploader.ResumeFromLocalState(Request{SourcePath: source, DestinationPath: "objects/file.bin"})
	if err != nil {
		t.Fatal(err)
	}
	if state.Offset != 10 || state.Complete {
		t.Fatalf("unexpected state: %+v", state)
	}
	result, err := uploader.ResumeUpload(context.Background(), Request{SourcePath: source, DestinationPath: "objects/file.bin"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Resumed || result.BytesWritten != int64(len(payload)-10) || result.Metadata["resume_offset"] != "10" {
		t.Fatalf("unexpected result: %+v", result)
	}
	stored, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(payload) {
		t.Fatalf("resume payload mismatch: %q", stored)
	}
}

func TestResumeUploadAlreadyComplete(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.bin")
	if err := os.WriteFile(source, []byte("complete"), 0o644); err != nil {
		t.Fatal(err)
	}
	uploader, err := NewLocalUploader(filepath.Join(tmp, "storage"))
	if err != nil {
		t.Fatal(err)
	}
	destination, err := uploader.ResolveDestination("file.bin")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("complete"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := uploader.ResumeUpload(context.Background(), Request{SourcePath: source, DestinationPath: "file.bin"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.AlreadyComplete || result.BytesWritten != 0 || result.Metadata["resume_complete"] != "true" {
		t.Fatalf("unexpected complete result: %+v", result)
	}
}

func TestValidateIntegrityDetectsMatchAndMismatch(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.bin")
	destination := filepath.Join(tmp, "destination.bin")
	if err := os.WriteFile(source, []byte(strings.Repeat("x", 64)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte(strings.Repeat("x", 64)), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ValidateIntegrity(context.Background(), source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.Algorithm != SHA256Algorithm || result.SourceChecksum == "" {
		t.Fatalf("unexpected integrity result: %+v", result)
	}
	if err := os.WriteFile(destination, []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err = ValidateIntegrity(context.Background(), source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid || result.ChecksumMatch || result.SizeMatch {
		t.Fatalf("expected mismatch: %+v", result)
	}
}

func TestHTTPCallbackSenderPostsJSON(t *testing.T) {
	var received CallbackPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("X-Test") != "yes" {
			t.Fatalf("unexpected callback request")
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	sender, err := NewHTTPCallbackSender(HTTPCallbackOptions{Endpoint: server.URL, Headers: map[string]string{"X-Test": "yes"}, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.Send(context.Background(), CallbackPayload{Event: CallbackEventUploadCompleted, Status: "succeeded", JobID: "job-1", Metadata: map[string]string{"a": "b"}}); err != nil {
		t.Fatal(err)
	}
	if received.Event != CallbackEventUploadCompleted || received.JobID != "job-1" || received.Metadata["a"] != "b" || received.Timestamp.IsZero() {
		t.Fatalf("unexpected payload: %+v", received)
	}
}
