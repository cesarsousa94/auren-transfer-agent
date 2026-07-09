package resolver

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/cesarsousa94/auren-transfer-agent/internal/download"
)

const (
	// M3U8ResolverName is the canonical M3U8 resolver name.
	M3U8ResolverName = "m3u8"

	// DefaultManifestReadLimit is the default manifest inspection limit.
	DefaultManifestReadLimit int64 = 2 * 1024 * 1024
)

// M3U8Manifest contains mechanical metadata extracted from a playlist manifest.
type M3U8Manifest struct {
	IsM3U8              bool
	IsHLS               bool
	VariantCount        int
	SegmentCount        int
	KeyCount            int
	TargetDuration      string
	MediaSequence       string
	PlaylistType        string
	FirstURI            string
	IndependentSegments bool
}

// M3U8Resolver fetches and classifies M3U8 playlist manifests.
type M3U8Resolver struct {
	client    *download.HTTPClient
	readLimit int64
}

// NewM3U8Resolver creates a generic M3U8 manifest resolver.
func NewM3U8Resolver(client *download.HTTPClient) (*M3U8Resolver, error) {
	if client == nil {
		return nil, fmt.Errorf("download http client is required")
	}
	return &M3U8Resolver{client: client, readLimit: DefaultManifestReadLimit}, nil
}

// Name returns the canonical resolver name.
func (resolver *M3U8Resolver) Name() string { return M3U8ResolverName }

// CanResolve detects M3U8 URLs or explicit resolver selection.
func (resolver *M3U8Resolver) CanResolve(request Request) bool {
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return false
	}
	return resolverRequested(request, M3U8ResolverName) || strings.EqualFold(path.Ext(parsed.Path), ".m3u8")
}

// Resolve fetches a bounded manifest sample and returns playlist metadata.
func (resolver *M3U8Resolver) Resolve(ctx context.Context, request Request) (Result, error) {
	manifest, response, finalURL, err := fetchManifest(ctx, resolver.client, resolver.readLimit, request)
	if err != nil {
		return Result{}, err
	}
	metadata := m3u8Metadata(request.Metadata, manifest)
	metadata["engine"] = M3U8ResolverName
	return Result{Resolver: M3U8ResolverName, Type: ResolverTypeM3U8, OriginalURL: request.URL, ResolvedURL: finalURL, StatusCode: response.StatusCode, ContentLength: response.ContentLength, ContentType: response.Header.Get("Content-Type"), Headers: map[string]string{"content_type": response.Header.Get("Content-Type"), "content_length": response.Header.Get("Content-Length")}, Metadata: metadata}, nil
}

func fetchManifest(ctx context.Context, client *download.HTTPClient, readLimit int64, request Request) (M3U8Manifest, *http.Response, string, error) {
	if client == nil {
		return M3U8Manifest{}, nil, "", fmt.Errorf("download http client is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return M3U8Manifest{}, nil, "", err
	}
	headers, err := download.NewHeaderSet(request.Headers)
	if err != nil {
		return M3U8Manifest{}, nil, "", err
	}
	response, err := doManifestRequest(ctx, client, parsed.String(), headers)
	if err != nil {
		return M3U8Manifest{}, nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return M3U8Manifest{}, response, parsed.String(), fmt.Errorf("manifest request returned status %d", response.StatusCode)
	}
	if readLimit <= 0 {
		readLimit = DefaultManifestReadLimit
	}
	limited := io.LimitReader(response.Body, readLimit+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return M3U8Manifest{}, response, parsed.String(), err
	}
	if int64(len(payload)) > readLimit {
		return M3U8Manifest{}, response, parsed.String(), fmt.Errorf("manifest exceeds read limit %d", readLimit)
	}
	manifest, err := ParseM3U8Manifest(string(payload))
	if err != nil {
		return M3U8Manifest{}, response, parsed.String(), err
	}
	finalURL := parsed.String()
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	return manifest, response, finalURL, nil
}

func doManifestRequest(ctx context.Context, client *download.HTTPClient, rawURL string, headers download.HeaderSet) (*http.Response, error) {
	request, err := download.NewRequest(ctx, download.RequestOptions{Method: http.MethodGet, URL: rawURL, Headers: headers})
	if err != nil {
		return nil, err
	}
	return client.Do(request)
}

// ParseM3U8Manifest parses a manifest body into mechanical metadata.
func ParseM3U8Manifest(body string) (M3U8Manifest, error) {
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	manifest := M3U8Manifest{}
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if lineNumber == 1 && line != "#EXTM3U" {
			return M3U8Manifest{}, fmt.Errorf("manifest does not start with #EXTM3U")
		}
		if line == "#EXTM3U" {
			manifest.IsM3U8 = true
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-") {
			manifest.IsHLS = true
		}
		switch {
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF"):
			manifest.VariantCount++
		case strings.HasPrefix(line, "#EXTINF"):
			manifest.SegmentCount++
		case strings.HasPrefix(line, "#EXT-X-KEY"):
			manifest.KeyCount++
		case strings.HasPrefix(line, "#EXT-X-TARGETDURATION:"):
			manifest.TargetDuration = strings.TrimSpace(strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:"))
		case strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"):
			manifest.MediaSequence = strings.TrimSpace(strings.TrimPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"))
		case strings.HasPrefix(line, "#EXT-X-PLAYLIST-TYPE:"):
			manifest.PlaylistType = strings.TrimSpace(strings.TrimPrefix(line, "#EXT-X-PLAYLIST-TYPE:"))
		case line == "#EXT-X-INDEPENDENT-SEGMENTS":
			manifest.IndependentSegments = true
		case !strings.HasPrefix(line, "#") && manifest.FirstURI == "":
			manifest.FirstURI = line
		}
	}
	if err := scanner.Err(); err != nil {
		return M3U8Manifest{}, err
	}
	if !manifest.IsM3U8 {
		return M3U8Manifest{}, fmt.Errorf("manifest does not contain #EXTM3U")
	}
	return manifest, nil
}

func m3u8Metadata(base map[string]string, manifest M3U8Manifest) map[string]string {
	metadata := cloneStringMap(base)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["is_m3u8"] = strconv.FormatBool(manifest.IsM3U8)
	metadata["is_hls"] = strconv.FormatBool(manifest.IsHLS)
	metadata["variant_count"] = strconv.Itoa(manifest.VariantCount)
	metadata["segment_count"] = strconv.Itoa(manifest.SegmentCount)
	metadata["key_count"] = strconv.Itoa(manifest.KeyCount)
	metadata["target_duration"] = manifest.TargetDuration
	metadata["media_sequence"] = manifest.MediaSequence
	metadata["playlist_type"] = manifest.PlaylistType
	metadata["first_uri"] = manifest.FirstURI
	metadata["independent_segments"] = strconv.FormatBool(manifest.IndependentSegments)
	return metadata
}
