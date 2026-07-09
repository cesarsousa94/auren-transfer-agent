package resolver

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
)

const (
	// GoogleDriveResolverName is the canonical Google Drive resolver name.
	GoogleDriveResolverName = "google_drive"

	// MEGAResolverName is the canonical MEGA resolver name.
	MEGAResolverName = "mega"

	// OneDriveResolverName is the canonical OneDrive resolver name.
	OneDriveResolverName = "onedrive"
)

// GoogleDriveResolver extracts mechanical metadata from Google Drive sharing URLs.
type GoogleDriveResolver struct{}

// NewGoogleDriveResolver creates the Google Drive resolver.
func NewGoogleDriveResolver() *GoogleDriveResolver { return &GoogleDriveResolver{} }

// Name returns the canonical resolver name.
func (resolver *GoogleDriveResolver) Name() string { return GoogleDriveResolverName }

// CanResolve detects Google Drive URLs with a recognizable file/folder id.
func (resolver *GoogleDriveResolver) CanResolve(request Request) bool {
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return false
	}
	if !isGoogleDriveHost(parsed.Hostname()) {
		return false
	}
	info := parseGoogleDriveURL(parsed)
	return info.ID != "" || resolverRequested(request, GoogleDriveResolverName)
}

// Resolve parses Google Drive URL metadata without contacting Google.
func (resolver *GoogleDriveResolver) Resolve(ctx context.Context, request Request) (Result, error) {
	_ = ctx
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return Result{}, err
	}
	if !isGoogleDriveHost(parsed.Hostname()) {
		return Result{}, fmt.Errorf("url is not a google drive endpoint")
	}
	info := parseGoogleDriveURL(parsed)
	if info.ID == "" {
		return Result{}, fmt.Errorf("google drive file id is required")
	}
	metadata := cloneStringMap(request.Metadata)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["engine"] = GoogleDriveResolverName
	metadata["provider"] = "google_drive"
	metadata["file_id"] = info.ID
	metadata["resource_kind"] = info.Kind
	metadata["route"] = info.Route
	metadata["download_url_derived"] = fmt.Sprintf("%t", info.DownloadURL != "")
	resolvedURL := parsed.String()
	if info.DownloadURL != "" {
		resolvedURL = info.DownloadURL
	}
	return Result{Resolver: GoogleDriveResolverName, Type: ResolverTypeGoogleDrive, OriginalURL: parsed.String(), ResolvedURL: resolvedURL, Metadata: metadata}, nil
}

// MEGAResolver extracts mechanical metadata from MEGA public links.
type MEGAResolver struct{}

// NewMEGAResolver creates the MEGA resolver.
func NewMEGAResolver() *MEGAResolver { return &MEGAResolver{} }

// Name returns the canonical resolver name.
func (resolver *MEGAResolver) Name() string { return MEGAResolverName }

// CanResolve detects MEGA public file/folder links.
func (resolver *MEGAResolver) CanResolve(request Request) bool {
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return false
	}
	if !isMEGAHost(parsed.Hostname()) {
		return false
	}
	info := parseMEGAURL(parsed)
	return info.Handle != "" || resolverRequested(request, MEGAResolverName)
}

// Resolve parses MEGA public link metadata without contacting MEGA.
func (resolver *MEGAResolver) Resolve(ctx context.Context, request Request) (Result, error) {
	_ = ctx
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return Result{}, err
	}
	if !isMEGAHost(parsed.Hostname()) {
		return Result{}, fmt.Errorf("url is not a mega endpoint")
	}
	info := parseMEGAURL(parsed)
	if info.Handle == "" {
		return Result{}, fmt.Errorf("mega handle is required")
	}
	metadata := cloneStringMap(request.Metadata)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["engine"] = MEGAResolverName
	metadata["provider"] = "mega"
	metadata["node_type"] = info.Kind
	metadata["handle"] = info.Handle
	metadata["public_link"] = "true"
	metadata["key_present"] = fmt.Sprintf("%t", info.Key != "")
	if info.Key != "" {
		metadata["key_masked"] = maskSecret(info.Key)
	}
	return Result{Resolver: MEGAResolverName, Type: ResolverTypeMEGA, OriginalURL: parsed.String(), ResolvedURL: parsed.String(), Metadata: metadata}, nil
}

// OneDriveResolver extracts mechanical metadata from OneDrive and SharePoint sharing URLs.
type OneDriveResolver struct{}

// NewOneDriveResolver creates the OneDrive resolver.
func NewOneDriveResolver() *OneDriveResolver { return &OneDriveResolver{} }

// Name returns the canonical resolver name.
func (resolver *OneDriveResolver) Name() string { return OneDriveResolverName }

// CanResolve detects OneDrive, 1drv.ms and SharePoint sharing URLs.
func (resolver *OneDriveResolver) CanResolve(request Request) bool {
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return false
	}
	return isOneDriveHost(parsed.Hostname())
}

// Resolve parses OneDrive/SharePoint sharing URL metadata without contacting Microsoft.
func (resolver *OneDriveResolver) Resolve(ctx context.Context, request Request) (Result, error) {
	_ = ctx
	parsed, err := parseHTTPURL(request.URL)
	if err != nil {
		return Result{}, err
	}
	if !isOneDriveHost(parsed.Hostname()) {
		return Result{}, fmt.Errorf("url is not a onedrive endpoint")
	}
	info := parseOneDriveURL(parsed)
	metadata := cloneStringMap(request.Metadata)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["engine"] = OneDriveResolverName
	metadata["provider"] = info.Provider
	metadata["route"] = info.Route
	metadata["share_path"] = info.SharePath
	metadata["short_link"] = fmt.Sprintf("%t", info.ShortLink)
	metadata["authkey_present"] = fmt.Sprintf("%t", info.AuthKey != "")
	if info.ResID != "" {
		metadata["resid"] = info.ResID
	}
	if info.ItemID != "" {
		metadata["item_id"] = info.ItemID
	}
	if info.CID != "" {
		metadata["cid"] = info.CID
	}
	if info.AuthKey != "" {
		metadata["authkey_masked"] = maskSecret(info.AuthKey)
	}
	resolvedURL := parsed.String()
	if info.DownloadURL != "" {
		resolvedURL = info.DownloadURL
	}
	return Result{Resolver: OneDriveResolverName, Type: ResolverTypeOneDrive, OriginalURL: parsed.String(), ResolvedURL: resolvedURL, Metadata: metadata}, nil
}

type googleDriveInfo struct {
	ID          string
	Kind        string
	Route       string
	DownloadURL string
}

func parseGoogleDriveURL(parsed *url.URL) googleDriveInfo {
	query := parsed.Query()
	if id := strings.TrimSpace(query.Get("id")); id != "" {
		return googleDriveInfo{ID: id, Kind: googleDriveKind(parsed.Path), Route: strings.Trim(path.Clean(parsed.Path), "/"), DownloadURL: googleDriveDownloadURL(id)}
	}
	parts := splitPath(parsed.Path)
	for index, part := range parts {
		switch strings.ToLower(part) {
		case "d":
			if index > 0 && strings.EqualFold(parts[index-1], "file") && index+1 < len(parts) {
				id := parts[index+1]
				return googleDriveInfo{ID: id, Kind: "file", Route: "file/d", DownloadURL: googleDriveDownloadURL(id)}
			}
		case "folders":
			if index+1 < len(parts) {
				return googleDriveInfo{ID: parts[index+1], Kind: "folder", Route: "folders"}
			}
		}
	}
	return googleDriveInfo{Kind: googleDriveKind(parsed.Path), Route: strings.Trim(path.Clean(parsed.Path), "/")}
}

func googleDriveKind(rawPath string) string {
	lower := strings.ToLower(rawPath)
	if strings.Contains(lower, "/folders/") {
		return "folder"
	}
	return "file"
}

func googleDriveDownloadURL(id string) string {
	values := url.Values{}
	values.Set("export", "download")
	values.Set("id", id)
	return "https://drive.google.com/uc?" + values.Encode()
}

type megaInfo struct {
	Kind   string
	Handle string
	Key    string
}

func parseMEGAURL(parsed *url.URL) megaInfo {
	parts := splitPath(parsed.Path)
	fragment := strings.TrimSpace(parsed.Fragment)
	if len(parts) >= 2 {
		switch strings.ToLower(parts[0]) {
		case "file", "folder", "embed":
			kind := strings.ToLower(parts[0])
			if kind == "embed" {
				kind = "file"
			}
			return megaInfo{Kind: kind, Handle: parts[1], Key: megaFragmentKey(fragment)}
		}
	}
	old := strings.TrimLeft(fragment, "!")
	oldParts := strings.Split(old, "!")
	if len(oldParts) >= 2 && oldParts[0] != "" {
		kind := "file"
		if strings.EqualFold(oldParts[0], "F") && len(oldParts) >= 3 {
			return megaInfo{Kind: "folder", Handle: oldParts[1], Key: oldParts[2]}
		}
		return megaInfo{Kind: kind, Handle: oldParts[0], Key: oldParts[1]}
	}
	return megaInfo{Kind: "unknown"}
}

func megaFragmentKey(fragment string) string {
	trimmed := strings.TrimSpace(fragment)
	trimmed = strings.TrimLeft(trimmed, "!")
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "!") {
		parts := strings.Split(trimmed, "!")
		return parts[len(parts)-1]
	}
	return trimmed
}

type oneDriveInfo struct {
	Provider    string
	Route       string
	SharePath   string
	ResID       string
	ItemID      string
	CID         string
	AuthKey     string
	ShortLink   bool
	DownloadURL string
}

func parseOneDriveURL(parsed *url.URL) oneDriveInfo {
	query := parsed.Query()
	host := strings.ToLower(parsed.Hostname())
	info := oneDriveInfo{Provider: "onedrive", Route: strings.Trim(path.Clean(parsed.Path), "/"), SharePath: parsed.EscapedPath(), ResID: strings.TrimSpace(query.Get("resid")), ItemID: strings.TrimSpace(query.Get("id")), CID: strings.TrimSpace(query.Get("cid")), AuthKey: strings.TrimSpace(query.Get("authkey")), ShortLink: strings.EqualFold(host, "1drv.ms") || strings.HasSuffix(host, ".1drv.ms")}
	if strings.Contains(host, "sharepoint.com") {
		info.Provider = "sharepoint"
	}
	if info.Route == "." {
		info.Route = ""
	}
	if info.ResID != "" {
		values := url.Values{}
		values.Set("resid", info.ResID)
		if info.AuthKey != "" {
			values.Set("authkey", info.AuthKey)
		}
		info.DownloadURL = "https://onedrive.live.com/download?" + values.Encode()
	}
	return info
}

func isGoogleDriveHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "drive.google.com" || host == "docs.google.com" || strings.HasSuffix(host, ".googleusercontent.com")
}

func isMEGAHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "mega.nz" || host == "mega.co.nz" || host == "mega.io" || strings.HasSuffix(host, ".mega.nz") || strings.HasSuffix(host, ".mega.co.nz")
}

func isOneDriveHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "1drv.ms" || strings.HasSuffix(host, ".1drv.ms") || host == "onedrive.live.com" || strings.HasSuffix(host, ".sharepoint.com") || strings.Contains(host, "-my.sharepoint.com")
}

func splitPath(rawPath string) []string {
	pieces := strings.Split(strings.Trim(rawPath, "/"), "/")
	parts := make([]string, 0, len(pieces))
	for _, piece := range pieces {
		trimmed := strings.TrimSpace(piece)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}
