package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strings"
)

const (
	// HeaderEngineName is the canonical name for the foundation header engine.
	HeaderEngineName = "headers"

	// HeaderRange is the canonical HTTP range header used by resume downloads.
	HeaderRange = "Range"

	// HeaderUserAgent is the canonical user agent header.
	HeaderUserAgent = "User-Agent"
)

// HeaderSet contains normalized HTTP headers that can be applied to requests.
type HeaderSet struct {
	values http.Header
}

// RequestOptions describes a mechanical HTTP request without business rules.
type RequestOptions struct {
	Method  string
	URL     string
	Headers HeaderSet
	Body    io.Reader
}

// NewHeaderSet validates and canonicalizes headers.
func NewHeaderSet(values map[string]string) (HeaderSet, error) {
	set := HeaderSet{values: make(http.Header)}
	for name, value := range values {
		canonical, err := NormalizeHeaderName(name)
		if err != nil {
			return HeaderSet{}, err
		}
		trimmed := strings.TrimSpace(value)
		if strings.ContainsAny(trimmed, "\r\n") {
			return HeaderSet{}, fmt.Errorf("header %s contains invalid newline", canonical)
		}
		set.values.Set(canonical, trimmed)
	}
	return set, nil
}

// NormalizeHeaderName returns the canonical MIME-style HTTP header name.
func NormalizeHeaderName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("header name is required")
	}
	if strings.ContainsAny(trimmed, " \t\r\n:") {
		return "", fmt.Errorf("header name %q is invalid", name)
	}
	canonical := textproto.CanonicalMIMEHeaderKey(trimmed)
	if canonical == "" {
		return "", fmt.Errorf("header name %q is invalid", name)
	}
	return canonical, nil
}

// Apply copies this header set to the provided request.
func (set HeaderSet) Apply(request *http.Request) error {
	if request == nil {
		return fmt.Errorf("http request cannot be nil")
	}
	for name, values := range set.values {
		request.Header.Del(name)
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	return nil
}

// With returns a copy of this set plus the provided header.
func (set HeaderSet) With(name string, value string) (HeaderSet, error) {
	clone := HeaderSet{values: set.Clone()}
	canonical, err := NormalizeHeaderName(name)
	if err != nil {
		return HeaderSet{}, err
	}
	trimmed := strings.TrimSpace(value)
	if strings.ContainsAny(trimmed, "\r\n") {
		return HeaderSet{}, fmt.Errorf("header %s contains invalid newline", canonical)
	}
	clone.values.Set(canonical, trimmed)
	return clone, nil
}

// Get returns a header value from this set.
func (set HeaderSet) Get(name string) string {
	canonical, err := NormalizeHeaderName(name)
	if err != nil {
		return ""
	}
	return set.values.Get(canonical)
}

// Clone returns a defensive copy as http.Header.
func (set HeaderSet) Clone() http.Header {
	clone := make(http.Header, len(set.values))
	for name, values := range set.values {
		copied := make([]string, len(values))
		copy(copied, values)
		clone[name] = copied
	}
	return clone
}

// Len returns the number of header keys.
func (set HeaderSet) Len() int {
	return len(set.values)
}

// NewRequest builds a validated HTTP request and applies canonical headers.
func NewRequest(ctx context.Context, options RequestOptions) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	method := strings.ToUpper(strings.TrimSpace(options.Method))
	if method == "" {
		method = http.MethodGet
	}
	url := strings.TrimSpace(options.URL)
	if url == "" {
		return nil, fmt.Errorf("request url is required")
	}
	request, err := http.NewRequestWithContext(ctx, method, url, options.Body)
	if err != nil {
		return nil, err
	}
	if err := options.Headers.Apply(request); err != nil {
		return nil, err
	}
	return request, nil
}
