package resolver

import (
	"context"
	"fmt"
	"path"
	"strings"
)

const (
	// XtreamResolverName is the canonical Xtream resolver name.
	XtreamResolverName = "xtream"
)

// XtreamResolver extracts technical metadata from Xtream-style media URLs.
type XtreamResolver struct{}

// NewXtreamResolver creates the mechanical Xtream resolver.
func NewXtreamResolver() *XtreamResolver { return &XtreamResolver{} }

// Name returns the canonical resolver name.
func (resolver *XtreamResolver) Name() string { return XtreamResolverName }

// CanResolve detects direct Xtream stream paths and player_api.php calls.
func (resolver *XtreamResolver) CanResolve(request Request) bool {
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return false
	}
	_, ok := parseXtreamPath(parsed.Path)
	if ok {
		return true
	}
	base := strings.ToLower(path.Base(parsed.Path))
	query := parsed.Query()
	return base == "player_api.php" && query.Get("username") != "" && query.Get("password") != ""
}

// Resolve parses the Xtream URL without contacting the provider.
func (resolver *XtreamResolver) Resolve(ctx context.Context, request Request) (Result, error) {
	_ = ctx
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return Result{}, err
	}
	metadata := cloneStringMap(request.Metadata)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["engine"] = XtreamResolverName

	if info, ok := parseXtreamPath(parsed.Path); ok {
		metadata["route"] = info.Route
		metadata["media_type"] = info.MediaType
		metadata["username_masked"] = maskSecret(info.Username)
		metadata["password_present"] = "true"
		metadata["credential_mode"] = "path"
		metadata["item_id"] = info.ItemID
		metadata["extension"] = info.Extension
		return Result{Resolver: XtreamResolverName, Type: ResolverTypeXtream, OriginalURL: parsed.String(), ResolvedURL: parsed.String(), Metadata: metadata}, nil
	}

	query := parsed.Query()
	if strings.ToLower(path.Base(parsed.Path)) == "player_api.php" && query.Get("username") != "" && query.Get("password") != "" {
		metadata["route"] = "player_api"
		metadata["media_type"] = strings.TrimSpace(query.Get("action"))
		metadata["username_masked"] = maskSecret(query.Get("username"))
		metadata["password_present"] = "true"
		metadata["credential_mode"] = "query"
		metadata["action"] = strings.TrimSpace(query.Get("action"))
		return Result{Resolver: XtreamResolverName, Type: ResolverTypeXtream, OriginalURL: parsed.String(), ResolvedURL: parsed.String(), Metadata: metadata}, nil
	}
	return Result{}, fmt.Errorf("url is not an xtream endpoint")
}

type xtreamPathInfo struct {
	Route     string
	MediaType string
	Username  string
	Password  string
	ItemID    string
	Extension string
}

func parseXtreamPath(rawPath string) (xtreamPathInfo, bool) {
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	if len(parts) < 4 {
		return xtreamPathInfo{}, false
	}
	route := strings.ToLower(strings.TrimSpace(parts[0]))
	mediaType := ""
	switch route {
	case "live":
		mediaType = "live"
	case "movie", "vod":
		mediaType = "vod"
	case "series":
		mediaType = "series"
	default:
		return xtreamPathInfo{}, false
	}
	item := strings.TrimSpace(parts[len(parts)-1])
	extension := strings.TrimPrefix(path.Ext(item), ".")
	itemID := strings.TrimSuffix(item, path.Ext(item))
	return xtreamPathInfo{Route: route, MediaType: mediaType, Username: parts[1], Password: parts[2], ItemID: itemID, Extension: extension}, true
}
