package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DriverS3          = "s3"
	S3AdapterName     = "s3"
	defaultS3Endpoint = "https://s3.%s.amazonaws.com"
	s3Service         = "s3"
)

type S3Options struct {
	Endpoint        string
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ForcePathStyle  bool
	HTTPClient      *http.Client
}

type S3Adapter struct {
	endpoint        string
	bucket          string
	region          string
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	forcePathStyle  bool
	client          *http.Client
	now             func() time.Time
}

func NewS3Adapter(options S3Options) (*S3Adapter, error) {
	region := strings.TrimSpace(options.Region)
	if region == "" {
		region = "us-east-1"
	}
	endpoint := strings.TrimRight(strings.TrimSpace(options.Endpoint), "/")
	if endpoint == "" {
		endpoint = fmt.Sprintf(defaultS3Endpoint, region)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("s3 endpoint must be an absolute URL")
	}
	if strings.TrimSpace(options.Bucket) == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	if strings.TrimSpace(options.AccessKeyID) == "" {
		return nil, fmt.Errorf("s3 access_key_id is required")
	}
	if strings.TrimSpace(options.SecretAccessKey) == "" {
		return nil, fmt.Errorf("s3 secret_access_key is required")
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	return &S3Adapter{endpoint: endpoint, bucket: strings.TrimSpace(options.Bucket), region: region, accessKeyID: strings.TrimSpace(options.AccessKeyID), secretAccessKey: strings.TrimSpace(options.SecretAccessKey), sessionToken: strings.TrimSpace(options.SessionToken), forcePathStyle: options.ForcePathStyle, client: client, now: func() time.Time { return time.Now().UTC() }}, nil
}

func S3Configured(bucket, accessKeyID, secretAccessKey string) bool {
	return strings.TrimSpace(bucket) != "" && strings.TrimSpace(accessKeyID) != "" && strings.TrimSpace(secretAccessKey) != ""
}
func (adapter *S3Adapter) Name() string   { return S3AdapterName }
func (adapter *S3Adapter) Driver() string { return DriverS3 }
func (adapter *S3Adapter) Bucket() string {
	if adapter == nil {
		return ""
	}
	return adapter.bucket
}

func (adapter *S3Adapter) Upload(ctx context.Context, request UploadRequest) (UploadResult, error) {
	if adapter == nil {
		return UploadResult{}, fmt.Errorf("s3 adapter cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ValidateUploadRequest(request); err != nil {
		return UploadResult{}, err
	}
	objectPath, err := CanonicalObjectPath(request)
	if err != nil {
		return UploadResult{}, err
	}
	info, err := os.Stat(request.SourcePath)
	if err != nil {
		return UploadResult{}, err
	}
	if info.IsDir() {
		return UploadResult{}, fmt.Errorf("storage source path must be a file")
	}
	file, err := os.Open(request.SourcePath)
	if err != nil {
		return UploadResult{}, err
	}
	defer file.Close()
	checksum := strings.TrimSpace(request.ChecksumSHA256)
	if checksum == "" {
		computed, err := sha256File(ctx, request.SourcePath)
		if err != nil {
			return UploadResult{}, err
		}
		checksum = computed
	}
	mimeType := firstNonEmpty(request.MimeType, "application/octet-stream")
	target := adapter.objectURL(objectPath)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, target, file)
	if err != nil {
		return UploadResult{}, err
	}
	httpReq.ContentLength = info.Size()
	httpReq.Header.Set("Content-Type", mimeType)
	httpReq.Header.Set("X-Amz-Content-Sha256", checksum)
	if request.Visibility != "" && request.Visibility != "private" {
		httpReq.Header.Set("X-Amz-Acl", request.Visibility)
	}
	for key, value := range request.Metadata {
		if strings.TrimSpace(key) != "" {
			httpReq.Header.Set("X-Amz-Meta-"+strings.ToLower(strings.TrimSpace(key)), value)
		}
	}
	adapter.sign(httpReq, checksum)
	started := adapter.now()
	resp, err := adapter.client.Do(httpReq)
	if err != nil {
		return UploadResult{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return UploadResult{}, fmt.Errorf("s3 upload returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	location := target
	etag := strings.Trim(resp.Header.Get("ETag"), `"`)
	if request.Progress != nil {
		_ = request.Progress(ctx, UploadProgress{Stage: "upload", CurrentBytes: info.Size(), TotalBytes: info.Size(), Percent: 100, Message: "Upload direto para S3 concluído.", Metrics: map[string]any{"driver": DriverS3, "bucket": adapter.bucket, "object_path": objectPath, "etag": etag, "duration_seconds": adapter.now().Sub(started).Seconds()}})
	}
	return UploadResult{Adapter: adapter.Name(), Driver: adapter.Driver(), Bucket: adapter.bucket, ObjectUUID: etag, ObjectPath: objectPath, Location: location, BytesWritten: info.Size(), ChecksumSHA256: checksum, Visibility: firstNonEmpty(request.Visibility, "private"), MimeType: mimeType, Multipart: false, Metadata: cloneStringMap(request.Metadata)}, nil
}

func (adapter *S3Adapter) objectURL(objectPath string) string {
	parsed, _ := url.Parse(adapter.endpoint)
	key := path.Clean(strings.TrimLeft(objectPath, "/"))
	if adapter.forcePathStyle || strings.Contains(parsed.Host, ":") || strings.HasPrefix(parsed.Host, "localhost") || strings.HasPrefix(parsed.Host, "127.") {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + url.PathEscape(adapter.bucket) + "/" + escapePath(key)
		return parsed.String()
	}
	parsed.Host = adapter.bucket + "." + parsed.Host
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + escapePath(key)
	return parsed.String()
}

func (adapter *S3Adapter) sign(req *http.Request, payloadHash string) {
	now := adapter.now()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	req.Header.Set("X-Amz-Date", amzDate)
	if adapter.sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", adapter.sessionToken)
	}
	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := canonicalQuery(req.URL.Query())
	signedHeaders, canonicalHeaders := canonicalHeaders(req.Header, req.Host)
	canonicalRequest := strings.Join([]string{req.Method, canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, payloadHash}, "\n")
	credentialScope := date + "/" + adapter.region + "/" + s3Service + "/aws4_request"
	stringToSign := strings.Join([]string{"AWS4-HMAC-SHA256", amzDate, credentialScope, hexSHA256([]byte(canonicalRequest))}, "\n")
	signingKey := awsSigningKey(adapter.secretAccessKey, date, adapter.region, s3Service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", adapter.accessKeyID, credentialScope, signedHeaders, signature))
}

func canonicalHeaders(headers http.Header, host string) (string, string) {
	values := map[string]string{"host": host}
	for key, vals := range headers {
		lower := strings.ToLower(key)
		values[lower] = strings.Join(vals, ",")
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteByte(':')
		buf.WriteString(strings.TrimSpace(values[k]))
		buf.WriteByte('\n')
	}
	return strings.Join(keys, ";"), buf.String()
}
func canonicalQuery(q url.Values) string {
	if len(q) == 0 {
		return ""
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		vals := q[k]
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(parts, "&")
}
func awsSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}
func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
func hexSHA256(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func escapePath(value string) string {
	parts := strings.Split(value, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// keep strconv used in old static analyzers when compiling subsets
var _ = strconv.IntSize
