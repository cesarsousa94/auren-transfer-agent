package upload

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalUploaderUploadCopiesFile(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.bin")
	if err := os.WriteFile(source, []byte("hello upload"), 0o644); err != nil {
		t.Fatal(err)
	}
	uploader, err := NewLocalUploader(filepath.Join(tmp, "storage"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := uploader.Upload(context.Background(), Request{SourcePath: source, DestinationPath: "media/file.bin", Metadata: map[string]string{"job": "1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BytesWritten != int64(len("hello upload")) || result.Multipart {
		t.Fatalf("unexpected result: %+v", result)
	}
	payload, err := os.ReadFile(filepath.Join(tmp, "storage", "media", "file.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "hello upload" {
		t.Fatalf("unexpected payload: %q", payload)
	}
	clone := result.Clone()
	clone.Metadata["job"] = "2"
	if result.Metadata["job"] != "1" {
		t.Fatalf("clone mutated result metadata")
	}
}

func TestLocalUploaderRejectsEscapingDestination(t *testing.T) {
	uploader, err := NewLocalUploader(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uploader.ResolveDestination("../outside.bin"); err == nil {
		t.Fatal("expected path traversal rejection")
	}
	if _, err := uploader.ResolveDestination(filepath.Join(string(os.PathSeparator), "absolute.bin")); err == nil {
		t.Fatal("expected absolute path rejection")
	}
}

func TestMultipartUploadCopiesInParts(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source.bin")
	payload := []byte(strings.Repeat("abc", 11))
	if err := os.WriteFile(source, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	uploader, err := NewLocalUploader(filepath.Join(tmp, "storage"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := uploader.MultipartUpload(context.Background(), MultipartOptions{Request: Request{SourcePath: source, DestinationPath: "objects/source.bin"}, PartSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Multipart || result.BytesWritten != int64(len(payload)) || len(result.Parts) != 4 {
		t.Fatalf("unexpected multipart result: %+v", result)
	}
	stored, err := os.ReadFile(filepath.Join(tmp, "storage", "objects", "source.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(payload) {
		t.Fatalf("stored payload mismatch")
	}
	clone := result.Clone()
	clone.Parts[0].BytesWritten = 999
	if result.Parts[0].BytesWritten == 999 {
		t.Fatal("clone mutated source parts")
	}
}

func TestNewPlanAndPartSize(t *testing.T) {
	plan, err := NewPlan(25, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Parts) != 3 || plan.Parts[2].Start != 20 || plan.Parts[2].End != 24 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	bytes, err := ParsePartSize("16MiB")
	if err != nil {
		t.Fatal(err)
	}
	if bytes != 16*1024*1024 {
		t.Fatalf("unexpected parsed size: %d", bytes)
	}
	if _, err := ParsePartSize("0"); err == nil {
		t.Fatal("expected invalid zero size")
	}
}
