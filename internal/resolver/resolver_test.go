package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auren/auren-transfer-agent/internal/download"
)

func TestRegistryUsesFirstMatchingResolver(t *testing.T) {
	registry, err := NewRegistry(NewXtreamResolver(), NewShuiResolver())
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	result, err := registry.Resolve(context.Background(), Request{URL: "http://example.test/movie/alice/secret/123.mp4"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != XtreamResolverName || result.Metadata["media_type"] != "vod" || result.Metadata["item_id"] != "123" {
		t.Fatalf("unexpected result: %#v", result)
	}
	names := registry.Names()
	names[0] = "changed"
	if registry.Names()[0] != XtreamResolverName {
		t.Fatalf("resolver names must be defensive copies")
	}
}

func TestHTTPResolverFollowsRedirectAndReportsHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		if r.Header.Get("User-Agent") == "" {
			t.Fatalf("user agent not applied")
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", "12")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := download.NewHTTPClient(download.HTTPClientOptions{UserAgent: "AurenTest/1", ConnectTimeout: "1s", ResponseHeaderTimeout: "1s", IdleTimeout: "1s", FollowRedirects: true, MaxRedirects: 4})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	httpResolver, err := NewHTTPResolver(client)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	result, err := httpResolver.Resolve(context.Background(), Request{URL: server.URL + "/start"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != HTTPResolverName || result.StatusCode != http.StatusOK || result.ContentType != "video/mp4" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.ResolvedURL != server.URL+"/final" {
		t.Fatalf("expected final URL, got %s", result.ResolvedURL)
	}
}

func TestXtreamResolverParsesDirectAndPlayerAPIEndpoints(t *testing.T) {
	xtream := NewXtreamResolver()
	direct, err := xtream.Resolve(context.Background(), Request{URL: "https://iptv.example/live/user/pass/456.ts"})
	if err != nil {
		t.Fatalf("direct resolve: %v", err)
	}
	if direct.Metadata["route"] != "live" || direct.Metadata["media_type"] != "live" || direct.Metadata["extension"] != "ts" {
		t.Fatalf("unexpected direct metadata: %#v", direct.Metadata)
	}
	api, err := xtream.Resolve(context.Background(), Request{URL: "https://iptv.example/player_api.php?username=user&password=pass&action=get_live_streams"})
	if err != nil {
		t.Fatalf("api resolve: %v", err)
	}
	if api.Metadata["route"] != "player_api" || api.Metadata["action"] != "get_live_streams" || api.Metadata["password_present"] != "true" {
		t.Fatalf("unexpected api metadata: %#v", api.Metadata)
	}
}

func TestShuiResolverParsesAdminAPIEndpointWithoutExposingAPIKey(t *testing.T) {
	shui := NewShuiResolver()
	result, err := shui.Resolve(context.Background(), Request{URL: "https://panel.example:9000/48e15220/?api_key=abcdef123456&action=get_streams"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != ShuiResolverName || result.Metadata["access_code"] != "48e15220" || result.Metadata["action"] != "get_streams" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Metadata["api_key_masked"] == "abcdef123456" {
		t.Fatalf("api key leaked in metadata")
	}
}

func TestRegistryRejectsDuplicateResolversAndUnknownURL(t *testing.T) {
	if _, err := NewRegistry(NewXtreamResolver(), NewXtreamResolver()); err == nil {
		t.Fatalf("expected duplicate resolver error")
	}
	registry, err := NewRegistry(NewXtreamResolver())
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if _, err := registry.Resolve(context.Background(), Request{URL: "ftp://example.test/file"}); err == nil {
		t.Fatalf("expected unknown resolver error")
	}
}
