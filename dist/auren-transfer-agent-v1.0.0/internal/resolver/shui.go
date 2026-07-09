package resolver

import (
	"context"
	"fmt"
	"strings"
)

const (
	// ShuiResolverName is the canonical Shui/XUI resolver name.
	ShuiResolverName = "shui"
)

// ShuiResolver extracts technical metadata from Shui/XUI Admin API URLs.
type ShuiResolver struct{}

// NewShuiResolver creates the mechanical Shui/XUI resolver.
func NewShuiResolver() *ShuiResolver { return &ShuiResolver{} }

// Name returns the canonical resolver name.
func (resolver *ShuiResolver) Name() string { return ShuiResolverName }

// CanResolve detects URLs that look like Shui/XUI Admin API endpoints.
func (resolver *ShuiResolver) CanResolve(request Request) bool {
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return false
	}
	query := parsed.Query()
	if query.Get("api_key") != "" {
		return true
	}
	return firstPathSegment(parsed.Path) != "" && query.Get("action") != ""
}

// Resolve parses Shui/XUI URL metadata without contacting the panel.
func (resolver *ShuiResolver) Resolve(ctx context.Context, request Request) (Result, error) {
	_ = ctx
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return Result{}, err
	}
	if !resolver.CanResolve(request) {
		return Result{}, fmt.Errorf("url is not a shui endpoint")
	}
	query := parsed.Query()
	metadata := cloneStringMap(request.Metadata)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["engine"] = ShuiResolverName
	metadata["access_code"] = firstPathSegment(parsed.Path)
	metadata["action"] = strings.TrimSpace(query.Get("action"))
	metadata["api_key_present"] = fmt.Sprintf("%t", query.Get("api_key") != "")
	if query.Get("api_key") != "" {
		metadata["api_key_masked"] = maskSecret(query.Get("api_key"))
	}
	metadata["credential_mode"] = "query"
	return Result{Resolver: ShuiResolverName, Type: ResolverTypeShui, OriginalURL: parsed.String(), ResolvedURL: parsed.String(), Metadata: metadata}, nil
}

func firstPathSegment(rawPath string) string {
	for _, part := range strings.Split(strings.Trim(rawPath, "/"), "/") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
