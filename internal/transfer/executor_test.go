package transfer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/auren/auren-transfer-agent/internal/config"
	"github.com/auren/auren-transfer-agent/internal/download"
	"github.com/auren/auren-transfer-agent/internal/mediahub"
	"github.com/auren/auren-transfer-agent/internal/storage"
)

func TestExecutorDownloadsAndUploadsLocalDestination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("0123456789abcdef0123456789abcdef"))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	root := t.TempDir()
	cfg.MediaHub.WorkDir = filepath.Join(root, "work")
	cfg.MediaHub.MinBytes = 1
	cfg.Storage.LocalPath = filepath.Join(root, "storage")
	httpClient, err := download.NewHTTPClientFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	localAdapter, err := storage.NewLocalAdapter(cfg.Storage.LocalPath)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStateStore(cfg.MediaHub.WorkDir)
	if err != nil {
		t.Fatal(err)
	}
	executor, err := NewExecutor(ExecutorOptions{Config: cfg, HTTPClient: httpClient, LocalAdapter: localAdapter, StateStore: store})
	if err != nil {
		t.Fatal(err)
	}

	job := mediahub.TransferJob{UUID: "job-local-1", Operation: "remote_download", Source: mediahub.TransferSource{URL: server.URL + "/movie.mp4", ResumeEnabled: true, MinBytes: 1, BlockHTML: true}, Destination: mediahub.TransferDestination{Driver: storage.DriverLocal, ObjectPath: "media/movie.mp4"}}
	if err := executor.Execute(context.Background(), job); err != nil {
		t.Fatalf("execute transfer: %v", err)
	}
	stored := filepath.Join(cfg.Storage.LocalPath, "media", "movie.mp4")
	info, err := os.Stat(stored)
	if err != nil {
		t.Fatalf("expected uploaded object: %v", err)
	}
	if info.Size() != 32 {
		t.Fatalf("unexpected object size: %d", info.Size())
	}
	statePath := store.Path(job.UUID)
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected state file: %v", err)
	}
}

func TestExecutorBlocksHTMLTinyProviderResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>login</html>"))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	root := t.TempDir()
	cfg.MediaHub.WorkDir = filepath.Join(root, "work")
	cfg.MediaHub.MinBytes = 1
	cfg.Storage.LocalPath = filepath.Join(root, "storage")
	httpClient, err := download.NewHTTPClientFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	localAdapter, err := storage.NewLocalAdapter(cfg.Storage.LocalPath)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStateStore(cfg.MediaHub.WorkDir)
	if err != nil {
		t.Fatal(err)
	}
	executor, err := NewExecutor(ExecutorOptions{Config: cfg, HTTPClient: httpClient, LocalAdapter: localAdapter, StateStore: store})
	if err != nil {
		t.Fatal(err)
	}
	job := mediahub.TransferJob{UUID: "job-html-1", Operation: "remote_download", Source: mediahub.TransferSource{URL: server.URL + "/blocked", BlockHTML: true}, Destination: mediahub.TransferDestination{Driver: storage.DriverLocal, ObjectPath: "media/blocked.bin"}}
	if err := executor.Execute(context.Background(), job); err == nil {
		t.Fatal("expected html response to be blocked")
	}
}
