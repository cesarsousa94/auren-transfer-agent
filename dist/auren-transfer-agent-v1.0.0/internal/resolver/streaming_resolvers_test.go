package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auren/auren-transfer-agent/internal/download"
)

func testResolverHTTPClient(t *testing.T) *download.HTTPClient {
	t.Helper()
	client, err := download.NewHTTPClient(download.HTTPClientOptions{UserAgent: "AurenResolverTest/1", ConnectTimeout: "1s", ResponseHeaderTimeout: "1s", IdleTimeout: "1s", FollowRedirects: true, MaxRedirects: 4})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return client
}

func TestRedirectResolverReportsFinalURLAndChain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", "12")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resolver, err := NewRedirectResolver(testResolverHTTPClient(t))
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	result, err := resolver.Resolve(context.Background(), Request{URL: server.URL + "/start"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != RedirectResolverName || result.Type != ResolverTypeRedirect {
		t.Fatalf("unexpected resolver: %#v", result)
	}
	if result.ResolvedURL != server.URL+"/final" || result.Metadata["redirect_count"] != "1" || result.Metadata["redirected"] != "true" {
		t.Fatalf("unexpected redirect metadata: %#v", result)
	}
}

func TestCloudflareResolverClassifiesWithoutBypass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("CF-Ray", "abc123-GRU")
		w.Header().Set("CF-Cache-Status", "DYNAMIC")
		w.Header().Set("Server", "cloudflare")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	resolver, err := NewCloudflareResolver(testResolverHTTPClient(t))
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	request := Request{URL: server.URL + "/protected", Metadata: map[string]string{"resolver": CloudflareResolverName}}
	if !resolver.CanResolve(request) {
		t.Fatalf("expected opt-in cloudflare resolver")
	}
	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != CloudflareResolverName || result.Metadata["cf_detected"] != "true" || result.Metadata["cf_challenge_detected"] != "true" {
		t.Fatalf("unexpected cloudflare metadata: %#v", result.Metadata)
	}
	if result.Metadata["cf_bypass_attempted"] != "false" {
		t.Fatalf("cloudflare resolver must not bypass challenges")
	}
}

func TestM3U8ResolverParsesManifestMetadata(t *testing.T) {
	body := "#EXTM3U\n#EXT-X-TARGETDURATION:8\n#EXT-X-MEDIA-SEQUENCE:42\n#EXTINF:8.0,\nsegment-42.ts\n#EXTINF:8.0,\nsegment-43.ts\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	resolver, err := NewM3U8Resolver(testResolverHTTPClient(t))
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	result, err := resolver.Resolve(context.Background(), Request{URL: server.URL + "/playlist.m3u8"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != M3U8ResolverName || result.Metadata["is_m3u8"] != "true" || result.Metadata["segment_count"] != "2" || result.Metadata["first_uri"] != "segment-42.ts" {
		t.Fatalf("unexpected m3u8 metadata: %#v", result.Metadata)
	}
}

func TestHLSResolverClassifiesMasterPlaylist(t *testing.T) {
	body := "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-INDEPENDENT-SEGMENTS\n#EXT-X-STREAM-INF:BANDWIDTH=1280000,RESOLUTION=1280x720\n720p.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=2560000,RESOLUTION=1920x1080\n1080p.m3u8\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-mpegURL")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	resolver, err := NewHLSResolver(testResolverHTTPClient(t))
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	request := Request{URL: server.URL + "/hls/master.m3u8"}
	if !resolver.CanResolve(request) {
		t.Fatalf("expected hls resolver to detect master playlist url")
	}
	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != HLSResolverName || result.Metadata["playlist_kind"] != "master" || result.Metadata["variant_count"] != "2" || result.Metadata["master"] != "true" {
		t.Fatalf("unexpected hls metadata: %#v", result.Metadata)
	}
}
