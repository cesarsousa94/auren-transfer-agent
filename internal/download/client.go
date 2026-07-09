package download

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
)

const (
	// HTTPClientName is the canonical foundation HTTP client name.
	HTTPClientName = "http"
)

// HTTPClientOptions configures the foundation HTTP client.
type HTTPClientOptions struct {
	UserAgent             string
	ConnectTimeout        string
	ResponseHeaderTimeout string
	IdleTimeout           string
	FollowRedirects       bool
	MaxRedirects          int
	CookieEngine          *CookieEngine
}

// HTTPClient wraps net/http with Agent foundation contracts for redirects, cookies and user agent.
type HTTPClient struct {
	client    *http.Client
	userAgent string
	redirects *RedirectEngine
	cookies   *CookieEngine
	transport *http.Transport
	connect   time.Duration
	headers   time.Duration
	idle      time.Duration
}

// OptionsFromConfig derives HTTP client options from validated Agent configuration.
func OptionsFromConfig(cfg config.Config) HTTPClientOptions {
	return HTTPClientOptions{
		UserAgent:             cfg.Resolver.DefaultUserAgent,
		ConnectTimeout:        cfg.Download.ConnectTimeout,
		ResponseHeaderTimeout: cfg.Download.ResponseHeaderTimeout,
		IdleTimeout:           cfg.Download.IdleTimeout,
		FollowRedirects:       cfg.Resolver.FollowRedirects,
		MaxRedirects:          cfg.Resolver.MaxRedirects,
	}
}

// NewHTTPClientFromConfig creates the foundation HTTP client from Agent configuration.
func NewHTTPClientFromConfig(cfg config.Config) (*HTTPClient, error) {
	return NewHTTPClient(OptionsFromConfig(cfg))
}

// NewHTTPClient creates a foundation HTTP client without starting any download.
func NewHTTPClient(options HTTPClientOptions) (*HTTPClient, error) {
	userAgent := strings.TrimSpace(options.UserAgent)
	if userAgent == "" {
		return nil, fmt.Errorf("user agent is required")
	}
	connectTimeout, err := parsePositiveDuration("connect timeout", options.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	responseHeaderTimeout, err := parsePositiveDuration("response header timeout", options.ResponseHeaderTimeout)
	if err != nil {
		return nil, err
	}
	idleTimeout, err := parsePositiveDuration("idle timeout", options.IdleTimeout)
	if err != nil {
		return nil, err
	}
	redirects, err := NewRedirectEngine(RedirectOptions{Follow: options.FollowRedirects, MaxRedirects: options.MaxRedirects})
	if err != nil {
		return nil, err
	}
	cookies := options.CookieEngine
	if cookies == nil {
		cookies, err = NewCookieEngine()
		if err != nil {
			return nil, err
		}
	}

	dialer := &net.Dialer{Timeout: connectTimeout, KeepAlive: idleTimeout}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ResponseHeaderTimeout: responseHeaderTimeout,
		IdleConnTimeout:       idleTimeout,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   16,
		ForceAttemptHTTP2:     true,
	}

	client := &http.Client{Transport: transport, Jar: cookies.Jar(), CheckRedirect: redirects.CheckRedirect}
	return &HTTPClient{client: client, userAgent: userAgent, redirects: redirects, cookies: cookies, transport: transport, connect: connectTimeout, headers: responseHeaderTimeout, idle: idleTimeout}, nil
}

// Do applies the canonical user agent and delegates to net/http.
func (client *HTTPClient) Do(request *http.Request) (*http.Response, error) {
	if client == nil || client.client == nil {
		return nil, fmt.Errorf("http client cannot be nil")
	}
	if request == nil {
		return nil, fmt.Errorf("http request cannot be nil")
	}
	if strings.TrimSpace(request.Header.Get("User-Agent")) == "" {
		request.Header.Set("User-Agent", client.userAgent)
	}
	return client.client.Do(request)
}

// StandardClient returns the configured Go HTTP client.
func (client *HTTPClient) StandardClient() *http.Client {
	if client == nil {
		return nil
	}
	return client.client
}

// Redirects returns the redirect engine used by this client.
func (client *HTTPClient) Redirects() *RedirectEngine {
	if client == nil {
		return nil
	}
	return client.redirects
}

// Cookies returns the cookie engine used by this client.
func (client *HTTPClient) Cookies() *CookieEngine {
	if client == nil {
		return nil
	}
	return client.cookies
}

// UserAgent returns the canonical user agent applied to requests.
func (client *HTTPClient) UserAgent() string {
	if client == nil {
		return ""
	}
	return client.userAgent
}

// ConnectTimeout returns the configured TCP connect timeout.
func (client *HTTPClient) ConnectTimeout() time.Duration {
	if client == nil {
		return 0
	}
	return client.connect
}

// ResponseHeaderTimeout returns the configured response header timeout.
func (client *HTTPClient) ResponseHeaderTimeout() time.Duration {
	if client == nil {
		return 0
	}
	return client.headers
}

// IdleTimeout returns the configured idle connection timeout.
func (client *HTTPClient) IdleTimeout() time.Duration {
	if client == nil {
		return 0
	}
	return client.idle
}

func parsePositiveDuration(field string, value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("%s is required", field)
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration", field)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", field)
	}
	return duration, nil
}
