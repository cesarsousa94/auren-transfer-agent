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

// UploadProgressFunc receives adapter-level upload progress updates.
type UploadProgressFunc func(context.Context, UploadProgress) error

// UploadProgress describes upload-stage progress emitted by storage adapters.
type UploadProgress struct {
	Stage        string         `json:"stage"`
	CurrentBytes int64          `json:"current_bytes"`
	TotalBytes   int64          `json:"total_bytes,omitempty"`
	SpeedBps     int64          `json:"speed_bps,omitempty"`
	Percent      float64        `json:"percent,omitempty"`
	Message      string         `json:"message,omitempty"`
	Metrics      map[string]any `json:"metrics,omitempty"`
}

// UploadPartResult identifies one uploaded multipart part.
type UploadPartResult struct {
	Number         int    `json:"number"`
	Size           int64  `json:"size"`
	ChecksumSHA256 string `json:"checksum_sha256,omitempty"`
	ETag           string `json:"etag,omitempty"`
}

// UploadRequest describes one object upload through a storage adapter.
type UploadRequest struct {
	SourcePath        string             `json:"source_path"`
	ObjectPath        string             `json:"object_path"`
	BucketUUID        string             `json:"bucket_uuid,omitempty"`
	Bucket            string             `json:"bucket,omitempty"`
	DirectoryPath     string             `json:"directory_path,omitempty"`
	RelativePath      string             `json:"relative_path,omitempty"`
	Visibility        string             `json:"visibility,omitempty"`
	MimeType          string             `json:"mime_type,omitempty"`
	ChecksumAlgorithm string             `json:"checksum_algorithm,omitempty"`
	ChecksumSHA256    string             `json:"checksum_sha256,omitempty"`
	Metadata          map[string]string  `json:"metadata,omitempty"`
	Progress          UploadProgressFunc `json:"-"`
}

// UploadResult describes one completed storage upload.
type UploadResult struct {
	Adapter        string             `json:"adapter"`
	Driver         string             `json:"driver"`
	BucketUUID     string             `json:"bucket_uuid,omitempty"`
	Bucket         string             `json:"bucket,omitempty"`
	ObjectUUID     string             `json:"object_uuid,omitempty"`
	ObjectPath     string             `json:"object_path"`
	Location       string             `json:"location"`
	BytesWritten   int64              `json:"bytes_written"`
	ChecksumSHA256 string             `json:"checksum_sha256,omitempty"`
	Visibility     string             `json:"visibility,omitempty"`
	MimeType       string             `json:"mime_type,omitempty"`
	Multipart      bool               `json:"multipart"`
	Parts          []UploadPartResult `json:"parts,omitempty"`
	Metadata       map[string]string  `json:"metadata,omitempty"`
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
	objectPath, err := CanonicalObjectPath(request)
	if err != nil {
		return UploadResult{}, err
	}
	result, err := adapter.uploader.Upload(ctx, upload.Request{SourcePath: request.SourcePath, DestinationPath: objectPath, Metadata: cloneStringMap(request.Metadata)})
	if err != nil {
		return UploadResult{}, err
	}
	if request.Progress != nil {
		_ = request.Progress(ctx, UploadProgress{Stage: "upload", CurrentBytes: result.BytesWritten, TotalBytes: result.BytesWritten, Percent: 100, Message: "Upload local concluído.", Metrics: map[string]any{"driver": DriverLocal}})
	}
	return UploadResult{Adapter: adapter.Name(), Driver: adapter.Driver(), BucketUUID: request.BucketUUID, Bucket: request.Bucket, ObjectPath: objectPath, Location: result.DestinationPath, BytesWritten: result.BytesWritten, ChecksumSHA256: request.ChecksumSHA256, Visibility: request.Visibility, MimeType: request.MimeType, Multipart: result.Multipart, Metadata: cloneStringMap(request.Metadata)}, nil
}

// CanonicalObjectPath returns the effective object path from object_path or directory_path + relative_path.
func CanonicalObjectPath(request UploadRequest) (string, error) {
	if strings.TrimSpace(request.ObjectPath) != "" {
		return NormalizeObjectPath(request.ObjectPath)
	}
	dir := strings.Trim(strings.ReplaceAll(request.DirectoryPath, "\\", "/"), "/")
	relative := strings.Trim(strings.ReplaceAll(request.RelativePath, "\\", "/"), "/")
	if relative == "" {
		return "", fmt.Errorf("storage relative path is required when object_path is empty")
	}
	if dir == "" {
		return NormalizeObjectPath(relative)
	}
	return NormalizeObjectPath(path.Join(dir, relative))
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
	_, err := CanonicalObjectPath(request)
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
