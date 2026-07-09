package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// AurenStorageAdapterName is the canonical Auren Storage adapter name.
	AurenStorageAdapterName = "auren_storage"
)

// AurenOptions configures the Auren Storage HTTP adapter.
type AurenOptions struct {
	Endpoint    string
	Bucket      string
	Region      string
	TokenHeader string
	APIKey      string
	HTTPClient  *http.Client
}

// AurenAdapter uploads objects to the Auren Storage HTTP API.
type AurenAdapter struct {
	endpoint    string
	bucket      string
	region      string
	tokenHeader string
	apiKey      string
	client      *http.Client
}

// AurenConfigured reports whether enough endpoint settings are present to build the adapter.
func AurenConfigured(endpoint string, bucket string) bool {
	return strings.TrimSpace(endpoint) != "" && strings.TrimSpace(bucket) != ""
}

// NewAurenStorageAdapter validates and creates the HTTP adapter.
func NewAurenStorageAdapter(options AurenOptions) (*AurenAdapter, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(options.Endpoint), "/")
	bucket := strings.TrimSpace(options.Bucket)
	if endpoint == "" {
		return nil, fmt.Errorf("auren storage endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("auren storage endpoint must be an absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("auren storage endpoint scheme must be http or https")
	}
	if bucket == "" {
		return nil, fmt.Errorf("auren storage bucket is required")
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	tokenHeader := strings.TrimSpace(options.TokenHeader)
	if tokenHeader == "" {
		tokenHeader = "Authorization"
	}
	return &AurenAdapter{endpoint: endpoint, bucket: bucket, region: strings.TrimSpace(options.Region), tokenHeader: tokenHeader, apiKey: strings.TrimSpace(options.APIKey), client: client}, nil
}

// NewAurenAdapter is kept as a compatibility alias for NewAurenStorageAdapter.
func NewAurenAdapter(options AurenOptions) (*AurenAdapter, error) {
	return NewAurenStorageAdapter(options)
}

// Name returns the adapter implementation name.
func (adapter *AurenAdapter) Name() string { return AurenStorageAdapterName }

// Driver returns the adapter driver name.
func (adapter *AurenAdapter) Driver() string { return DriverAurenStorage }

// Endpoint returns the configured endpoint without trailing slash.
func (adapter *AurenAdapter) Endpoint() string {
	if adapter == nil {
		return ""
	}
	return adapter.endpoint
}

// Bucket returns the configured bucket.
func (adapter *AurenAdapter) Bucket() string {
	if adapter == nil {
		return ""
	}
	return adapter.bucket
}

// Upload streams a source file to the Auren Storage object endpoint.
func (adapter *AurenAdapter) Upload(ctx context.Context, request UploadRequest) (UploadResult, error) {
	if adapter == nil {
		return UploadResult{}, fmt.Errorf("auren storage adapter cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ValidateUploadRequest(request); err != nil {
		return UploadResult{}, err
	}
	objectPath, err := NormalizeObjectPath(request.ObjectPath)
	if err != nil {
		return UploadResult{}, err
	}
	file, err := os.Open(request.SourcePath)
	if err != nil {
		return UploadResult{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return UploadResult{}, err
	}
	if info.IsDir() {
		return UploadResult{}, fmt.Errorf("storage source path must be a file")
	}
	objectURL, err := adapter.objectURL(objectPath)
	if err != nil {
		return UploadResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, file)
	if err != nil {
		return UploadResult{}, err
	}
	httpRequest.ContentLength = info.Size()
	httpRequest.Header.Set("Content-Type", "application/octet-stream")
	httpRequest.Header.Set("X-Auren-Bucket", adapter.bucket)
	httpRequest.Header.Set("X-Auren-Object-Path", objectPath)
	if adapter.region != "" {
		httpRequest.Header.Set("X-Auren-Region", adapter.region)
	}
	if adapter.apiKey != "" {
		value := adapter.apiKey
		if strings.EqualFold(adapter.tokenHeader, "Authorization") && !strings.HasPrefix(strings.ToLower(value), "bearer ") {
			value = "Bearer " + value
		}
		httpRequest.Header.Set(adapter.tokenHeader, value)
	}
	response, err := adapter.client.Do(httpRequest)
	if err != nil {
		return UploadResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return UploadResult{}, fmt.Errorf("auren storage upload failed with status %d", response.StatusCode)
	}
	location := response.Header.Get("Location")
	if strings.TrimSpace(location) == "" {
		location = objectURL
	}
	return UploadResult{Adapter: adapter.Name(), Driver: adapter.Driver(), ObjectPath: objectPath, Location: location, BytesWritten: info.Size(), Metadata: cloneStringMap(request.Metadata)}, nil
}

func (adapter *AurenAdapter) objectURL(objectPath string) (string, error) {
	parsed, err := url.Parse(adapter.endpoint + "/api/v1/buckets/" + url.PathEscape(adapter.bucket) + "/objects")
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("path", objectPath)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
