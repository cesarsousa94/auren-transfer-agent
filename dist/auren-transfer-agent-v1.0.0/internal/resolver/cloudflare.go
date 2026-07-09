package resolver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/auren/auren-transfer-agent/internal/download"
)

const (
	// CloudflareResolverName is the canonical Cloudflare resolver name.
	CloudflareResolverName = "cloudflare"
)

// CloudflareResolver classifies Cloudflare-protected responses without bypassing them.
type CloudflareResolver struct {
	client *download.HTTPClient
}

// NewCloudflareResolver creates a Cloudflare classification resolver.
func NewCloudflareResolver(client *download.HTTPClient) (*CloudflareResolver, error) {
	if client == nil {
		return nil, fmt.Errorf("download http client is required")
	}
	return &CloudflareResolver{client: client}, nil
}

// Name returns the canonical resolver name.
func (resolver *CloudflareResolver) Name() string { return CloudflareResolverName }

// CanResolve is opt-in through request metadata because Cloudflare can only be confirmed after a response.
func (resolver *CloudflareResolver) CanResolve(request Request) bool {
	if _, err := parseHTTPURL(request.URL); err != nil {
		return false
	}
	return resolverRequested(request, CloudflareResolverName) || truthyMetadata(request, "cloudflare")
}

// Resolve performs metadata-only Cloudflare classification. It does not solve or bypass challenges.
func (resolver *CloudflareResolver) Resolve(ctx context.Context, request Request) (Result, error) {
	if resolver == nil || resolver.client == nil {
		return Result{}, fmt.Errorf("cloudflare resolver cannot be nil")
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
	metadata := metadataWith(request.Metadata, "method", method)
	metadata["engine"] = CloudflareResolverName
	metadata["cf_detected"] = fmt.Sprintf("%t", isCloudflareResponse(response.Header))
	metadata["cf_challenge_detected"] = fmt.Sprintf("%t", isCloudflareChallenge(response.StatusCode, response.Header))
	metadata["cf_bypass_attempted"] = "false"
	metadata["cf_ray_present"] = fmt.Sprintf("%t", response.Header.Get("CF-Ray") != "")
	metadata["cf_cache_status"] = response.Header.Get("CF-Cache-Status")
	return Result{Resolver: CloudflareResolverName, Type: ResolverTypeCloudflare, OriginalURL: parsed.String(), ResolvedURL: finalURL, StatusCode: response.StatusCode, ContentLength: response.ContentLength, ContentType: response.Header.Get("Content-Type"), Headers: map[string]string{"server": response.Header.Get("Server"), "cf_cache_status": response.Header.Get("CF-Cache-Status")}, Metadata: metadata}, nil
}

func (resolver *CloudflareResolver) do(ctx context.Context, method string, rawURL string, headers download.HeaderSet) (*http.Response, error) {
	httpRequest, err := download.NewRequest(ctx, download.RequestOptions{Method: method, URL: rawURL, Headers: headers})
	if err != nil {
		return nil, err
	}
	return resolver.client.Do(httpRequest)
}

func isCloudflareResponse(headers http.Header) bool {
	server := strings.ToLower(headers.Get("Server"))
	return strings.Contains(server, "cloudflare") || headers.Get("CF-Ray") != "" || headers.Get("CF-Cache-Status") != ""
}

func isCloudflareChallenge(status int, headers http.Header) bool {
	if !isCloudflareResponse(headers) {
		return false
	}
	return status == http.StatusForbidden || status == http.StatusServiceUnavailable || status == http.StatusTooManyRequests
}
