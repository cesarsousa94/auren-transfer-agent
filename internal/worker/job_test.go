package worker

import (
	"strings"
	"testing"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/identity"
)

func TestNewJobAppliesDefaultsAndValidates(t *testing.T) {
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	job, err := NewJob(JobInput{
		SourceURL:      "https://example.com/movie.mp4",
		DestinationKey: "media/org/movie.mp4",
		Headers:        map[string]string{"User-Agent": "Auren"},
		Metadata:       map[string]string{"tenant": "demo"},
		Now:            now,
	})
	if err != nil {
		t.Fatalf("NewJob returned error: %v", err)
	}
	if !identity.IsUUID(job.ID) {
		t.Fatalf("expected generated uuid, got %q", job.ID)
	}
	if job.Type != JobTypeTransfer || job.Status != JobStatusPending {
		t.Fatalf("unexpected type/status: %s/%s", job.Type, job.Status)
	}
	if job.MaxAttempts != 1 || !job.CreatedAt.Equal(now) || !job.UpdatedAt.Equal(now) {
		t.Fatalf("defaults were not applied correctly: %#v", job)
	}
}

func TestNewJobNormalizesProvidedUUID(t *testing.T) {
	job, err := NewJob(JobInput{ID: "9D3A0E63-0DD0-4FC7-A229-5B15D3E9083D", SourceURL: "http://example.com/a.ts", DestinationKey: "a.ts"})
	if err != nil {
		t.Fatalf("NewJob returned error: %v", err)
	}
	if job.ID != "9d3a0e63-0dd0-4fc7-a229-5b15d3e9083d" {
		t.Fatalf("uuid was not normalized: %s", job.ID)
	}
}

func TestValidateJobRejectsInvalidSourceURL(t *testing.T) {
	_, err := NewJob(JobInput{SourceURL: "ftp://example.com/file", DestinationKey: "file"})
	if err == nil || !strings.Contains(err.Error(), "source_url") {
		t.Fatalf("expected source_url validation error, got %v", err)
	}
}

func TestJobCloneDefensivelyCopiesMaps(t *testing.T) {
	job, err := NewJob(JobInput{SourceURL: "https://example.com/file", DestinationKey: "file", Headers: map[string]string{"A": "B"}, Metadata: map[string]string{"x": "y"}})
	if err != nil {
		t.Fatalf("NewJob returned error: %v", err)
	}
	clone := job.Clone()
	clone.Headers["A"] = "changed"
	clone.Metadata["x"] = "changed"
	if job.Headers["A"] != "B" || job.Metadata["x"] != "y" {
		t.Fatalf("clone mutated original maps")
	}
}

func TestWithStatusReturnsQueuedCopy(t *testing.T) {
	job, err := NewJob(JobInput{SourceURL: "https://example.com/file", DestinationKey: "file"})
	if err != nil {
		t.Fatalf("NewJob returned error: %v", err)
	}
	queued, err := job.WithStatus(JobStatusQueued, time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("WithStatus returned error: %v", err)
	}
	if job.Status != JobStatusPending || queued.Status != JobStatusQueued {
		t.Fatalf("status copy contract failed: original=%s queued=%s", job.Status, queued.Status)
	}
}
