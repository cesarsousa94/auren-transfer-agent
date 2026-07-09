package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// LocalUploaderName is the canonical local upload implementation name.
	LocalUploaderName = "local"
)

// LocalUploader writes uploaded objects into a local root directory.
type LocalUploader struct {
	root string
}

// NewLocalUploader validates and creates a local uploader.
func NewLocalUploader(root string) (*LocalUploader, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return nil, fmt.Errorf("local upload root is required")
	}
	return &LocalUploader{root: filepath.Clean(trimmed)}, nil
}

// Name returns the canonical implementation name.
func (uploader *LocalUploader) Name() string { return LocalUploaderName }

// Driver returns the canonical driver name.
func (uploader *LocalUploader) Driver() string { return DriverLocal }

// Root returns the configured local root.
func (uploader *LocalUploader) Root() string {
	if uploader == nil {
		return ""
	}
	return uploader.root
}

// Upload copies SourcePath to a destination inside the configured local root.
func (uploader *LocalUploader) Upload(ctx context.Context, request Request) (Result, error) {
	if uploader == nil {
		return Result{}, fmt.Errorf("local uploader cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	request = request.Clone()
	if err := ValidateRequest(request); err != nil {
		return Result{}, err
	}
	destination, err := uploader.ResolveDestination(request.DestinationPath)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return Result{}, err
	}

	started := time.Now()
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	source, err := os.Open(request.SourcePath)
	if err != nil {
		return Result{}, err
	}
	defer source.Close()
	target, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return Result{}, err
	}
	written, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return Result{}, copyErr
	}
	if closeErr != nil {
		return Result{}, closeErr
	}
	return Result{Uploader: uploader.Name(), Driver: uploader.Driver(), SourcePath: request.SourcePath, DestinationPath: destination, BytesWritten: written, Duration: time.Since(started), Metadata: cloneStringMap(request.Metadata)}, nil
}

// ResolveDestination converts a relative object path into a safe local path.
func (uploader *LocalUploader) ResolveDestination(destination string) (string, error) {
	if uploader == nil {
		return "", fmt.Errorf("local uploader cannot be nil")
	}
	trimmed := strings.TrimSpace(destination)
	if trimmed == "" {
		return "", fmt.Errorf("upload destination path is required")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("upload destination must be relative")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("upload destination escapes local root")
	}
	return filepath.Join(uploader.root, cleaned), nil
}
