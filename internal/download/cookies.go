package download

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

const (
	// CookieEngineName is the canonical name of the foundation cookie engine.
	CookieEngineName = "cookie_jar"
)

// CookieInfo is a serializable view of cookies currently known for a URL.
type CookieInfo struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
	Path  string `json:"path,omitempty"`
	Raw   string `json:"raw,omitempty"`
}

// CookieEngine wraps Go's RFC6265 cookie jar with Agent-specific validation helpers.
type CookieEngine struct {
	jar http.CookieJar
}

// NewCookieEngine creates an isolated in-memory cookie jar.
func NewCookieEngine() (*CookieEngine, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &CookieEngine{jar: jar}, nil
}

// Jar returns the underlying cookie jar for use by http.Client.
func (engine *CookieEngine) Jar() http.CookieJar {
	if engine == nil {
		return nil
	}
	return engine.jar
}

// SetCookies stores cookies for the provided HTTP or HTTPS URL.
func (engine *CookieEngine) SetCookies(rawURL string, cookies []*http.Cookie) error {
	if engine == nil || engine.jar == nil {
		return fmt.Errorf("cookie engine cannot be nil")
	}
	parsed, err := parseCookieURL(rawURL)
	if err != nil {
		return err
	}
	engine.jar.SetCookies(parsed, cloneHTTPCookies(cookies))
	return nil
}

// Cookies returns defensive copies of cookies for the provided HTTP or HTTPS URL.
func (engine *CookieEngine) Cookies(rawURL string) ([]*http.Cookie, error) {
	if engine == nil || engine.jar == nil {
		return nil, fmt.Errorf("cookie engine cannot be nil")
	}
	parsed, err := parseCookieURL(rawURL)
	if err != nil {
		return nil, err
	}
	return cloneHTTPCookies(engine.jar.Cookies(parsed)), nil
}

// Snapshot returns a stable serializable cookie view for diagnostics and tests.
func (engine *CookieEngine) Snapshot(rawURL string) ([]CookieInfo, error) {
	cookies, err := engine.Cookies(rawURL)
	if err != nil {
		return nil, err
	}
	infos := make([]CookieInfo, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		infos = append(infos, CookieInfo{Name: cookie.Name, Value: cookie.Value, Path: cookie.Path, Raw: cookie.String()})
	}
	return infos, nil
}

func parseCookieURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("cookie url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed == nil || parsed.Host == "" {
		return nil, fmt.Errorf("cookie url must be absolute")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("cookie url must use http or https")
	}
	return parsed, nil
}

func cloneHTTPCookies(input []*http.Cookie) []*http.Cookie {
	if input == nil {
		return nil
	}
	output := make([]*http.Cookie, 0, len(input))
	for _, cookie := range input {
		if cookie == nil {
			continue
		}
		copy := *cookie
		output = append(output, &copy)
	}
	return output
}
