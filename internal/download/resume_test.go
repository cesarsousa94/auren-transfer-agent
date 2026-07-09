package download

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestNewResumeStateBuildsRangeHeader(t *testing.T) {
	state, err := NewResumeState(ResumeOptions{Enabled: true, ExistingBytes: 1024})
	if err != nil {
		t.Fatalf("resume state: %v", err)
	}
	if state.RangeHeader != "bytes=1024-" || state.Complete {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestNewResumeStateMarksCompleteWhenExistingMatchesTotal(t *testing.T) {
	state, err := NewResumeState(ResumeOptions{Enabled: true, ExistingBytes: 2048, TotalBytes: 2048})
	if err != nil {
		t.Fatalf("resume state: %v", err)
	}
	if !state.Complete || state.RangeHeader != "" {
		t.Fatalf("unexpected complete state: %+v", state)
	}
}

func TestResumeFromFileUsesFileSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.bin")
	if err := os.WriteFile(path, []byte("partial"), 0o644); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	state, err := ResumeFromFile(path, true, 0)
	if err != nil {
		t.Fatalf("resume from file: %v", err)
	}
	if state.ExistingBytes != 7 || state.RangeHeader != "bytes=7-" {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestApplyResumeSetsRangeHeader(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://example.test/file", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if err := ApplyResume(request, ResumeState{RangeHeader: "bytes=10-"}); err != nil {
		t.Fatalf("apply resume: %v", err)
	}
	if request.Header.Get(HeaderRange) != "bytes=10-" {
		t.Fatalf("range = %q", request.Header.Get(HeaderRange))
	}
}
