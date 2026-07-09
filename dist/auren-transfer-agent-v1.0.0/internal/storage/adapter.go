// Package storage contains storage adapter contracts for upload targets.
package storage

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/auren/auren-transfer-agent/internal/upload"
)

const (
	// InterfaceName is the canonical storage adapter interface name.
	InterfaceName = "storage_adapter"

	// DriverLocal is the local storage adapter driver.
	DriverLocal = "local"

	// DriverAurenStorage is the Auren Storage HTTP adapter driver.
	DriverAurenStorage = "auren_storage"
)

// UploadRequest describes one object upload through a storage adapter.
type UploadRequest struct {
	SourcePath string            `json:"source_path"`
	ObjectPath string            `json:"object_path"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// UploadResult describes one completed storage upload.
type UploadResult struct {
	Adapter      string            `json:"adapter"`
	Driver       string            `json:"driver"`
	ObjectPath   string            `json:"object_path"`
	Location     string            `json:"location"`
	BytesWritten int64             `json:"bytes_written"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Adapter is the mechanical storage upload contract.
type Adapter interface {
	Name() string
	Driver() string
	Upload(context.Context, UploadRequest) (UploadResult, error)
}

// LocalAdapter stores objects through the local upload implementation.
type LocalAdapter struct {
	uploader *upload.LocalUploader
}

// NewLocalAdapter creates a local storage adapter rooted at localPath.
func NewLocalAdapter(localPath string) (*LocalAdapter, error) {
	uploader, err := upload.NewLocalUploader(localPath)
	if err != nil {
		return nil, err
	}
	return &LocalAdapter{uploader: uploader}, nil
}

// Name returns the adapter implementation name.
func (adapter *LocalAdapter) Name() string { return DriverLocal }

// Driver returns the adapter driver name.
func (adapter *LocalAdapter) Driver() string { return DriverLocal }

// Root returns the local adapter root path.
func (adapter *LocalAdapter) Root() string {
	if adapter == nil || adapter.uploader == nil {
		return ""
	}
	return adapter.uploader.Root()
}

// Upload stores the source file below the local root.
func (adapter *LocalAdapter) Upload(ctx context.Context, request UploadRequest) (UploadResult, error) {
	if adapter == nil || adapter.uploader == nil {
		return UploadResult{}, fmt.Errorf("local storage adapter cannot be nil")
	}
	objectPath, err := NormalizeObjectPath(request.ObjectPath)
	if err != nil {
		return UploadResult{}, err
	}
	result, err := adapter.uploader.Upload(ctx, upload.Request{SourcePath: request.SourcePath, DestinationPath: objectPath, Metadata: cloneStringMap(request.Metadata)})
	if err != nil {
		return UploadResult{}, err
	}
	return UploadResult{Adapter: adapter.Name(), Driver: adapter.Driver(), ObjectPath: objectPath, Location: result.DestinationPath, BytesWritten: result.BytesWritten, Metadata: cloneStringMap(request.Metadata)}, nil
}

// NormalizeObjectPath validates and canonicalizes an object path.
func NormalizeObjectPath(objectPath string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(objectPath, "\\", "/"))
	if trimmed == "" {
		return "", fmt.Errorf("storage object path is required")
	}
	if strings.HasPrefix(trimmed, "/") {
		return "", fmt.Errorf("storage object path must be relative")
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("storage object path escapes root")
	}
	return cleaned, nil
}

// ValidateUploadRequest checks mechanical storage request requirements.
func ValidateUploadRequest(request UploadRequest) error {
	if strings.TrimSpace(request.SourcePath) == "" {
		return fmt.Errorf("storage source path is required")
	}
	_, err := NormalizeObjectPath(request.ObjectPath)
	return err
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
