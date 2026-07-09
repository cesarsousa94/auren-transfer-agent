// Package resolver contains business-rule-free URL resolution primitives.
package resolver

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

const (
	// InterfaceName is the canonical name for the resolver interface foundation.
	InterfaceName = "resolver"

	// ResolverTypeHTTP is the generic HTTP resolver type.
	ResolverTypeHTTP = "http"

	// ResolverTypeXtream is the Xtream URL resolver type.
	ResolverTypeXtream = "xtream"

	// ResolverTypeShui is the Shui/XUI API resolver type.
	ResolverTypeShui = "shui"

	// ResolverTypeRedirect is the redirect resolver type.
	ResolverTypeRedirect = "redirect"

	// ResolverTypeCloudflare is the Cloudflare classification resolver type.
	ResolverTypeCloudflare = "cloudflare"

	// ResolverTypeM3U8 is the generic M3U8 manifest resolver type.
	ResolverTypeM3U8 = "m3u8"

	// ResolverTypeHLS is the HLS playlist resolver type.
	ResolverTypeHLS = "hls"

	// ResolverTypeGoogleDrive is the Google Drive resolver type.
	ResolverTypeGoogleDrive = "google_drive"

	// ResolverTypeMEGA is the MEGA resolver type.
	ResolverTypeMEGA = "mega"

	// ResolverTypeOneDrive is the OneDrive resolver type.
	ResolverTypeOneDrive = "onedrive"
)

// Request describes a mechanical URL resolution attempt.
type Request struct {
	URL      string            `json:"url"`
	Method   string            `json:"method,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Result describes a resolved URL and technical metadata.
type Result struct {
	Resolver      string            `json:"resolver"`
	Type          string            `json:"type"`
	OriginalURL   string            `json:"original_url"`
	ResolvedURL   string            `json:"resolved_url"`
	StatusCode    int               `json:"status_code,omitempty"`
	ContentLength int64             `json:"content_length,omitempty"`
	ContentType   string            `json:"content_type,omitempty"`
	Filename      string            `json:"filename,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// Resolver is the foundation contract implemented by all URL resolvers.
type Resolver interface {
	Name() string
	CanResolve(Request) bool
	Resolve(context.Context, Request) (Result, error)
}

// Registry resolves requests by trying registered resolvers in deterministic order.
type Registry struct {
	resolvers []Resolver
}

// NewRegistry validates and stores resolver implementations.
func NewRegistry(resolvers ...Resolver) (*Registry, error) {
	registry := &Registry{}
	seen := map[string]struct{}{}
	for _, item := range resolvers {
		if item == nil {
			return nil, fmt.Errorf("resolver cannot be nil")
		}
		name := strings.TrimSpace(item.Name())
		if name == "" {
			return nil, fmt.Errorf("resolver name is required")
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("resolver %s already registered", name)
		}
		seen[name] = struct{}{}
		registry.resolvers = append(registry.resolvers, item)
	}
	return registry, nil
}

// Len returns the number of registered resolvers.
func (registry *Registry) Len() int {
	if registry == nil {
		return 0
	}
	return len(registry.resolvers)
}

// Names returns registered resolver names in execution order.
func (registry *Registry) Names() []string {
	if registry == nil {
		return nil
	}
	names := make([]string, 0, len(registry.resolvers))
	for _, item := range registry.resolvers {
		names = append(names, item.Name())
	}
	return names
}

// Resolve selects the first resolver that can process the request.
func (registry *Registry) Resolve(ctx context.Context, request Request) (Result, error) {
	if registry == nil {
		return Result{}, fmt.Errorf("resolver registry cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(request.URL) == "" {
		return Result{}, fmt.Errorf("resolver request url is required")
	}
	for _, item := range registry.resolvers {
		if item.CanResolve(request) {
			return item.Resolve(ctx, request)
		}
	}
	return Result{}, fmt.Errorf("no resolver available for url")
}

// Clone returns a defensive copy of the request.
func (request Request) Clone() Request {
	return Request{URL: request.URL, Method: request.Method, Headers: cloneStringMap(request.Headers), Metadata: cloneStringMap(request.Metadata)}
}

// Clone returns a defensive copy of the result.
func (result Result) Clone() Result {
	return Result{Resolver: result.Resolver, Type: result.Type, OriginalURL: result.OriginalURL, ResolvedURL: result.ResolvedURL, StatusCode: result.StatusCode, ContentLength: result.ContentLength, ContentType: result.ContentType, Filename: result.Filename, Headers: cloneStringMap(result.Headers), Metadata: cloneStringMap(result.Metadata)}
}

func parseHTTPURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q", parsed.Scheme)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return nil, fmt.Errorf("url host is required")
	}
	return parsed, nil
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	clone := make(map[string]string, len(input))
	for key, value := range input {
		clone[key] = value
	}
	return clone
}

func metadataWith(values map[string]string, key string, value string) map[string]string {
	output := cloneStringMap(values)
	if output == nil {
		output = map[string]string{}
	}
	if strings.TrimSpace(value) != "" {
		output[key] = value
	}
	return output
}

func maskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 4 {
		return "****"
	}
	return trimmed[:2] + "..." + trimmed[len(trimmed)-2:]
}

func resolverRequested(request Request, name string) bool {
	wanted := strings.ToLower(strings.TrimSpace(request.Metadata["resolver"]))
	if wanted == "" {
		wanted = strings.ToLower(strings.TrimSpace(request.Metadata["engine"]))
	}
	return wanted == strings.ToLower(strings.TrimSpace(name))
}

func truthyMetadata(request Request, key string) bool {
	value := strings.ToLower(strings.TrimSpace(request.Metadata[key]))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
