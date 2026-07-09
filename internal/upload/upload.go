// Package upload contains business-rule-free upload primitives.
package upload

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// InterfaceName is the canonical name for the upload interface foundation.
	InterfaceName = "uploader"

	// DriverLocal is the foundation local upload driver.
	DriverLocal = "local"
)

// Request describes one mechanical upload operation.
type Request struct {
	SourcePath      string            `json:"source_path"`
	DestinationPath string            `json:"destination_path"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// Result describes one completed mechanical upload operation.
type Result struct {
	Uploader        string            `json:"uploader"`
	Driver          string            `json:"driver"`
	SourcePath      string            `json:"source_path"`
	DestinationPath string            `json:"destination_path"`
	BytesWritten    int64             `json:"bytes_written"`
	Duration        time.Duration     `json:"duration"`
	Multipart       bool              `json:"multipart"`
	Resumed         bool              `json:"resumed"`
	AlreadyComplete bool              `json:"already_complete"`
	Parts           []Part            `json:"parts,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// Uploader is the foundation contract implemented by upload targets.
type Uploader interface {
	Name() string
	Driver() string
	Upload(context.Context, Request) (Result, error)
}

// ValidateRequest checks upload primitives only.
func ValidateRequest(request Request) error {
	if strings.TrimSpace(request.SourcePath) == "" {
		return fmt.Errorf("upload source path is required")
	}
	if strings.TrimSpace(request.DestinationPath) == "" {
		return fmt.Errorf("upload destination path is required")
	}
	return nil
}

// Clone returns a defensive copy of the request.
func (request Request) Clone() Request {
	return Request{SourcePath: request.SourcePath, DestinationPath: request.DestinationPath, Metadata: cloneStringMap(request.Metadata)}
}

// Clone returns a defensive copy of the result.
func (result Result) Clone() Result {
	parts := make([]Part, len(result.Parts))
	copy(parts, result.Parts)
	return Result{Uploader: result.Uploader, Driver: result.Driver, SourcePath: result.SourcePath, DestinationPath: result.DestinationPath, BytesWritten: result.BytesWritten, Duration: result.Duration, Multipart: result.Multipart, Resumed: result.Resumed, AlreadyComplete: result.AlreadyComplete, Parts: parts, Metadata: cloneStringMap(result.Metadata)}
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
