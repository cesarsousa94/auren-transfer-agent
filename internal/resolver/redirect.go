package resolver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cesarsousa94/auren-transfer-agent/internal/download"
)

const (
	// RedirectResolverName is the canonical redirect resolver name.
	RedirectResolverName = "redirect"
)

// RedirectResolver resolves a URL while exposing the mechanical redirect chain.
type RedirectResolver struct {
	client *download.HTTPClient
}

// NewRedirectResolver creates a redirect-aware HTTP resolver.
func NewRedirectResolver(client *download.HTTPClient) (*RedirectResolver, error) {
	if client == nil {
		return nil, fmt.Errorf("download http client is required")
	}
	return &RedirectResolver{client: client}, nil
}

// Name returns the canonical resolver name.
func (resolver *RedirectResolver) Name() string { return RedirectResolverName }

// CanResolve returns true for HTTP URLs. Specific resolvers should be registered before this one.
func (resolver *RedirectResolver) CanResolve(request Request) bool {
	_, err := parseHTTPURL(request.URL)
	return err == nil
}

// Resolve follows the configured redirect policy and reports final URL metadata.
func (resolver *RedirectResolver) Resolve(ctx context.Context, request Request) (Result, error) {
	if resolver == nil || resolver.client == nil {
		return Result{}, fmt.Errorf("redirect resolver cannot be nil")
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
	redirects := resolver.client.Redirects()
	if redirects != nil {
		redirects.Reset()
	}
	response, err := resolver.do(ctx, method, parsed.String(), headers)
	if err != nil {
		return Result{}, err
	}
	if response.StatusCode == http.StatusMethodNotAllowed && method == http.MethodHead {
		response.Body.Close()
		if redirects != nil {
			redirects.Reset()
		}
		method = http.MethodGet
		response, err = resolver.do(ctx, method, parsed.String(), headers)
		if err != nil {
			return Result{}, err
		}
	}
	defer response.Body.Close()

	finalURL := parsed.String()
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	events := []download.RedirectEvent(nil)
	if redirects != nil {
		events = redirects.Snapshot()
	}
	metadata := metadataWith(request.Metadata, "method", method)
	metadata["engine"] = RedirectResolverName
	metadata["redirect_count"] = fmt.Sprintf("%d", len(events))
	metadata["redirected"] = fmt.Sprintf("%t", finalURL != parsed.String())
	metadata["follow_redirects"] = fmt.Sprintf("%t", resolver.client.Redirects() != nil && resolver.client.Redirects().Follow())
	metadata["max_redirects"] = fmt.Sprintf("%d", resolver.client.Redirects().MaxRedirects())
	if len(events) > 0 {
		metadata["first_redirect_to"] = events[0].To
		metadata["last_redirect_to"] = events[len(events)-1].To
	}
	return Result{
		Resolver:      RedirectResolverName,
		Type:          ResolverTypeRedirect,
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
		Metadata: metadata,
	}, nil
}

func (resolver *RedirectResolver) do(ctx context.Context, method string, rawURL string, headers download.HeaderSet) (*http.Response, error) {
	httpRequest, err := download.NewRequest(ctx, download.RequestOptions{Method: method, URL: rawURL, Headers: headers})
	if err != nil {
		return nil, err
	}
	return resolver.client.Do(httpRequest)
}
