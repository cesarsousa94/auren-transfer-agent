package download

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
)

func TestHTTPClientFromConfigAppliesTimeoutsAndUserAgent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Resolver.DefaultUserAgent = "AurenTest/1.0"
	client, err := NewHTTPClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("new http client: %v", err)
	}
	if client.UserAgent() != "AurenTest/1.0" {
		t.Fatalf("user agent = %q", client.UserAgent())
	}
	if client.ConnectTimeout() != 15*time.Second {
		t.Fatalf("connect timeout = %s", client.ConnectTimeout())
	}
	if client.ResponseHeaderTimeout() != 30*time.Second {
		t.Fatalf("header timeout = %s", client.ResponseHeaderTimeout())
	}
	if client.IdleTimeout() != 60*time.Second {
		t.Fatalf("idle timeout = %s", client.IdleTimeout())
	}
	if client.StandardClient() == nil || client.Redirects() == nil || client.Cookies() == nil {
		t.Fatalf("client components must be configured")
	}
}

func TestHTTPClientDoAddsDefaultUserAgentAndUsesCookieJar(t *testing.T) {
	var observedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		observedUserAgent = request.Header.Get("User-Agent")
		http.SetCookie(writer, &http.Cookie{Name: "edge", Value: "token"})
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := NewHTTPClient(HTTPClientOptions{UserAgent: "AurenTransferAgent/Test", ConnectTimeout: "1s", ResponseHeaderTimeout: "1s", IdleTimeout: "1s", FollowRedirects: true, MaxRedirects: 2})
	if err != nil {
		t.Fatalf("new http client: %v", err)
	}
	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_, _ = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if observedUserAgent != "AurenTransferAgent/Test" {
		t.Fatalf("observed user agent = %q", observedUserAgent)
	}
	cookies, err := client.Cookies().Cookies(server.URL)
	if err != nil {
		t.Fatalf("cookies: %v", err)
	}
	if len(cookies) != 1 || cookies[0].Name != "edge" {
		t.Fatalf("unexpected cookies: %+v", cookies)
	}
}

func TestHTTPClientPreservesExplicitUserAgent(t *testing.T) {
	var observedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		observedUserAgent = request.Header.Get("User-Agent")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewHTTPClient(HTTPClientOptions{UserAgent: "default", ConnectTimeout: "1s", ResponseHeaderTimeout: "1s", IdleTimeout: "1s", FollowRedirects: true, MaxRedirects: 1})
	if err != nil {
		t.Fatalf("new http client: %v", err)
	}
	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("User-Agent", "explicit")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = response.Body.Close()
	if observedUserAgent != "explicit" {
		t.Fatalf("observed user agent = %q", observedUserAgent)
	}
}

func TestHTTPClientRejectsInvalidOptions(t *testing.T) {
	if _, err := NewHTTPClient(HTTPClientOptions{UserAgent: "", ConnectTimeout: "1s", ResponseHeaderTimeout: "1s", IdleTimeout: "1s"}); err == nil {
		t.Fatalf("expected user agent error")
	}
	if _, err := NewHTTPClient(HTTPClientOptions{UserAgent: "agent", ConnectTimeout: "0s", ResponseHeaderTimeout: "1s", IdleTimeout: "1s"}); err == nil {
		t.Fatalf("expected duration error")
	}
}
