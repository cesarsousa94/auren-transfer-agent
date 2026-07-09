package resolver

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/cesarsousa94/auren-transfer-agent/internal/download"
)

const (
	// HLSResolverName is the canonical HLS resolver name.
	HLSResolverName = "hls"
)

// HLSResolver classifies HLS master and media playlists.
type HLSResolver struct {
	client    *download.HTTPClient
	readLimit int64
}

// NewHLSResolver creates an HLS-specific resolver.
func NewHLSResolver(client *download.HTTPClient) (*HLSResolver, error) {
	if client == nil {
		return nil, fmt.Errorf("download http client is required")
	}
	return &HLSResolver{client: client, readLimit: DefaultManifestReadLimit}, nil
}

// Name returns the canonical resolver name.
func (resolver *HLSResolver) Name() string { return HLSResolverName }

// CanResolve detects explicit HLS requests and common HLS URL shapes.
func (resolver *HLSResolver) CanResolve(request Request) bool {
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return false
	}
	if resolverRequested(request, HLSResolverName) || truthyMetadata(request, "hls") {
		return true
	}
	lowerPath := strings.ToLower(parsed.Path)
	return strings.EqualFold(path.Ext(lowerPath), ".m3u8") && (strings.Contains(lowerPath, "/hls/") || strings.Contains(path.Base(lowerPath), "master"))
}

// Resolve fetches and classifies an HLS manifest without downloading media segments.
func (resolver *HLSResolver) Resolve(ctx context.Context, request Request) (Result, error) {
	manifest, response, finalURL, err := fetchManifest(ctx, resolver.client, resolver.readLimit, request)
	if err != nil {
		return Result{}, err
	}
	if !manifest.IsHLS {
		return Result{}, fmt.Errorf("manifest is not an HLS playlist")
	}
	metadata := m3u8Metadata(request.Metadata, manifest)
	metadata["engine"] = HLSResolverName
	metadata["playlist_kind"] = hlsPlaylistKind(manifest)
	metadata["encrypted"] = strconv.FormatBool(manifest.KeyCount > 0)
	metadata["media_only"] = strconv.FormatBool(manifest.SegmentCount > 0 && manifest.VariantCount == 0)
	metadata["master"] = strconv.FormatBool(manifest.VariantCount > 0)
	return Result{Resolver: HLSResolverName, Type: ResolverTypeHLS, OriginalURL: request.URL, ResolvedURL: finalURL, StatusCode: response.StatusCode, ContentLength: response.ContentLength, ContentType: response.Header.Get("Content-Type"), Headers: map[string]string{"content_type": response.Header.Get("Content-Type"), "content_length": response.Header.Get("Content-Length")}, Metadata: metadata}, nil
}

func hlsPlaylistKind(manifest M3U8Manifest) string {
	if manifest.VariantCount > 0 {
		return "master"
	}
	if manifest.SegmentCount > 0 {
		return "media"
	}
	return "unknown"
}
