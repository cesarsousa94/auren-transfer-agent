// Package plugins exposes the public, business-rule-free plugin contracts used
// by the Auren Transfer Agent foundation.
package plugins

import (
	"context"
	"fmt"
	"strings"
)

const (
	// SDKVersion is the current plugin SDK contract version.
	SDKVersion = "v0.1"

	// KindResolver identifies URL resolver plugins.
	KindResolver = "resolver"

	// KindUploader identifies upload target plugins.
	KindUploader = "uploader"
)

// Manifest describes a plugin without loading provider-specific behavior.
type Manifest struct {
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ResolveRequest describes a generic resolver plugin request.
type ResolveRequest struct {
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ResolveResult describes a generic resolver plugin result.
type ResolveResult struct {
	URL      string            `json:"url"`
	Type     string            `json:"type,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UploadRequest describes a generic upload plugin request.
type UploadRequest struct {
	SourcePath      string            `json:"source_path"`
	DestinationPath string            `json:"destination_path"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// UploadResult describes a generic upload plugin result.
type UploadResult struct {
	DestinationPath string            `json:"destination_path"`
	BytesWritten    int64             `json:"bytes_written"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// ResolverPlugin is the public contract for resolver extensions.
type ResolverPlugin interface {
	Manifest() Manifest
	CanResolve(ResolveRequest) bool
	Resolve(context.Context, ResolveRequest) (ResolveResult, error)
}

// UploaderPlugin is the public contract for uploader extensions.
type UploaderPlugin interface {
	Manifest() Manifest
	Upload(context.Context, UploadRequest) (UploadResult, error)
}

// ValidateManifest validates public plugin metadata without executing a plugin.
func ValidateManifest(manifest Manifest) error {
	if strings.TrimSpace(manifest.Name) == "" {
		return fmt.Errorf("plugin manifest name is required")
	}
	kind := strings.ToLower(strings.TrimSpace(manifest.Kind))
	if kind != KindResolver && kind != KindUploader {
		return fmt.Errorf("plugin manifest kind must be one of %s, %s", KindResolver, KindUploader)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return fmt.Errorf("plugin manifest version is required")
	}
	return nil
}

// NormalizeManifest returns a canonical manifest copy.
func NormalizeManifest(manifest Manifest) (Manifest, error) {
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Kind = strings.ToLower(strings.TrimSpace(manifest.Kind))
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Description = strings.TrimSpace(manifest.Description)
	manifest.Capabilities = cloneStringSlice(manifest.Capabilities)
	manifest.Metadata = cloneStringMap(manifest.Metadata)
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Clone returns a defensive copy of the manifest.
func (manifest Manifest) Clone() Manifest {
	return Manifest{Name: manifest.Name, Kind: manifest.Kind, Version: manifest.Version, Description: manifest.Description, Capabilities: cloneStringSlice(manifest.Capabilities), Metadata: cloneStringMap(manifest.Metadata)}
}

func cloneStringSlice(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	output := make([]string, len(input))
	copy(output, input)
	return output
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
