package resolver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cesarsousa94/auren-transfer-agent/internal/download"
)

const (
	// HTTPResolverName is the canonical generic HTTP resolver name.
	HTTPResolverName = "http"
)

// HTTPResolver resolves generic HTTP/HTTPS URLs by asking the remote endpoint for headers.
type HTTPResolver struct {
	client *download.HTTPClient
}

// NewHTTPResolver creates a generic HTTP resolver.
func NewHTTPResolver(client *download.HTTPClient) (*HTTPResolver, error) {
	if client == nil {
		return nil, fmt.Errorf("download http client is required")
	}
	return &HTTPResolver{client: client}, nil
}

// Name returns the canonical resolver name.
func (resolver *HTTPResolver) Name() string { return HTTPResolverName }

// CanResolve returns true for valid HTTP and HTTPS URLs.
func (resolver *HTTPResolver) CanResolve(request Request) bool {
	_, err := parseHTTPURL(request.URL)
	return err == nil
}

// Resolve performs a metadata-only HTTP resolution using HEAD by default.
func (resolver *HTTPResolver) Resolve(ctx context.Context, request Request) (Result, error) {
	if resolver == nil || resolver.client == nil {
		return Result{}, fmt.Errorf("http resolver cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return Result{}, err
	}
	headers, err := download.NewHeaderSet(request.Headers)
	if err != nil {
		return Result{}, err
	}
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		method = http.MethodHead
	}
	response, err := resolver.do(ctx, method, parsed.String(), headers)
	if err != nil {
		return Result{}, err
	}
	if response.StatusCode == http.StatusMethodNotAllowed && method == http.MethodHead {
		response.Body.Close()
		response, err = resolver.do(ctx, http.MethodGet, parsed.String(), headers)
		if err != nil {
			return Result{}, err
		}
	}
	defer response.Body.Close()

	finalURL := parsed.String()
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	return Result{
		Resolver:      HTTPResolverName,
		Type:          ResolverTypeHTTP,
		OriginalURL:   parsed.String(),
		ResolvedURL:   finalURL,
		StatusCode:    response.StatusCode,
		ContentLength: response.ContentLength,
		ContentType:   response.Header.Get("Content-Type"),
		Filename:      filenameFromHeaders(response.Header),
		Headers: map[string]string{
			"content_type":   response.Header.Get("Content-Type"),
			"content_length": response.Header.Get("Content-Length"),
		},
		Metadata: metadataWith(request.Metadata, "method", method),
	}, nil
}

func (resolver *HTTPResolver) do(ctx context.Context, method string, rawURL string, headers download.HeaderSet) (*http.Response, error) {
	httpRequest, err := download.NewRequest(ctx, download.RequestOptions{Method: method, URL: rawURL, Headers: headers})
	if err != nil {
		return nil, err
	}
	return resolver.client.Do(httpRequest)
}

func filenameFromHeaders(headers http.Header) string {
	disposition := headers.Get("Content-Disposition")
	lower := strings.ToLower(disposition)
	marker := "filename="
	index := strings.Index(lower, marker)
	if index < 0 {
		return ""
	}
	value := strings.TrimSpace(disposition[index+len(marker):])
	value = strings.Trim(value, `"'`)
	if comma := strings.Index(value, ";"); comma >= 0 {
		value = strings.TrimSpace(value[:comma])
	}
	return value
}
