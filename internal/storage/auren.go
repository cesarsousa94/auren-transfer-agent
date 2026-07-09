package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// AurenStorageAdapterName is the canonical Auren Storage adapter name.
	AurenStorageAdapterName = "auren_storage"

	defaultAurenPartSize int64 = 16 * 1024 * 1024
)

// AurenOptions configures the Auren Storage HTTP adapter.
type AurenOptions struct {
	Endpoint         string
	Bucket           string
	BucketUUID       string
	Region           string
	TokenHeader      string
	APIKey           string
	HTTPClient       *http.Client
	MultipartEnabled bool
	PartSize         int64
}

// AurenAdapter uploads objects to the Auren Storage HTTP API.
type AurenAdapter struct {
	endpoint         string
	bucket           string
	bucketUUID       string
	region           string
	tokenHeader      string
	apiKey           string
	client           *http.Client
	multipartEnabled bool
	partSize         int64
}

// AurenConfigured reports whether enough endpoint settings are present to build the adapter.
func AurenConfigured(endpoint string, bucket string) bool {
	return strings.TrimSpace(endpoint) != "" && strings.TrimSpace(bucket) != ""
}

// NewAurenStorageAdapter validates and creates the HTTP adapter.
func NewAurenStorageAdapter(options AurenOptions) (*AurenAdapter, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(options.Endpoint), "/")
	bucket := firstNonEmpty(options.BucketUUID, options.Bucket)
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
		return nil, fmt.Errorf("auren storage bucket_uuid or bucket is required")
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	tokenHeader := strings.TrimSpace(options.TokenHeader)
	if tokenHeader == "" {
		tokenHeader = "Authorization"
	}
	partSize := options.PartSize
	if partSize <= 0 {
		partSize = defaultAurenPartSize
	}
	return &AurenAdapter{endpoint: endpoint, bucket: strings.TrimSpace(options.Bucket), bucketUUID: strings.TrimSpace(options.BucketUUID), region: strings.TrimSpace(options.Region), tokenHeader: tokenHeader, apiKey: strings.TrimSpace(options.APIKey), client: client, multipartEnabled: options.MultipartEnabled, partSize: partSize}, nil
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

// Bucket returns the configured bucket or bucket UUID.
func (adapter *AurenAdapter) Bucket() string {
	if adapter == nil {
		return ""
	}
	return firstNonEmpty(adapter.bucketUUID, adapter.bucket)
}

// Upload streams a source file to the Auren Storage v1 object endpoint.
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
	checksum := strings.TrimSpace(request.ChecksumSHA256)
	if checksum == "" || strings.EqualFold(request.ChecksumAlgorithm, "sha256") {
		computed, err := sha256File(ctx, request.SourcePath)
		if err != nil {
			return UploadResult{}, err
		}
		checksum = computed
	}
	request.ObjectPath = objectPath
	request.ChecksumSHA256 = checksum
	request.ChecksumAlgorithm = firstNonEmpty(request.ChecksumAlgorithm, "sha256")
	request.BucketUUID = firstNonEmpty(request.BucketUUID, adapter.bucketUUID)
	request.Bucket = firstNonEmpty(request.Bucket, adapter.bucket)
	if request.MimeType == "" {
		request.MimeType = "application/octet-stream"
	}
	if request.Visibility == "" {
		request.Visibility = "private"
	}

	if adapter.multipartEnabled && info.Size() > adapter.partSize {
		return adapter.multipartUpload(ctx, request, info.Size())
	}
	return adapter.directUpload(ctx, request, info.Size())
}

func (adapter *AurenAdapter) directUpload(ctx context.Context, request UploadRequest, size int64) (UploadResult, error) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		defer writer.Close()
		if err := writeAurenFormFields(multipartWriter, request, size); err != nil {
			errCh <- err
			_ = writer.CloseWithError(err)
			return
		}
		file, err := os.Open(request.SourcePath)
		if err != nil {
			errCh <- err
			_ = writer.CloseWithError(err)
			return
		}
		defer file.Close()
		partHeader := make(textproto.MIMEHeader)
		partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeQuotes(filepath.Base(request.ObjectPath))))
		partHeader.Set("Content-Type", request.MimeType)
		part, err := multipartWriter.CreatePart(partHeader)
		if err != nil {
			errCh <- err
			_ = writer.CloseWithError(err)
			return
		}
		progress := &progressReader{reader: file, total: size, started: time.Now(), interval: 2 * time.Second, callback: func(current, total, speed int64, percent float64) {
			if request.Progress != nil {
				_ = request.Progress(ctx, UploadProgress{Stage: "upload", CurrentBytes: current, TotalBytes: total, SpeedBps: speed, Percent: percent, Message: "Upload para Auren Storage em andamento.", Metrics: map[string]any{"driver": DriverAurenStorage, "mode": "direct"}})
			}
		}}
		if _, err := io.Copy(part, progress); err != nil {
			errCh <- err
			_ = writer.CloseWithError(err)
			return
		}
		if err := multipartWriter.Close(); err != nil {
			errCh <- err
			_ = writer.CloseWithError(err)
			return
		}
	}()

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, adapter.objectURL(adapter.bucketID(request)), reader)
	if err != nil {
		return UploadResult{}, err
	}
	httpRequest.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	httpRequest.Header.Set("Accept", "application/json")
	adapter.applyAuth(httpRequest)
	response, err := adapter.client.Do(httpRequest)
	if err != nil {
		return UploadResult{}, err
	}
	defer response.Body.Close()
	if writeErr := <-errCh; writeErr != nil {
		return UploadResult{}, writeErr
	}
	result, err := adapter.parseObjectResponse(response, request, size, false, nil)
	if err != nil {
		return UploadResult{}, err
	}
	if request.Progress != nil {
		_ = request.Progress(ctx, UploadProgress{Stage: "upload", CurrentBytes: size, TotalBytes: size, Percent: 100, Message: "Upload para Auren Storage concluído.", Metrics: map[string]any{"driver": DriverAurenStorage, "mode": "direct", "object_uuid": result.ObjectUUID}})
	}
	return result, nil
}

func (adapter *AurenAdapter) multipartUpload(ctx context.Context, request UploadRequest, size int64) (UploadResult, error) {
	initiate, err := adapter.initiateMultipart(ctx, request, size)
	if err != nil {
		return UploadResult{}, err
	}
	if initiate.UploadID == "" {
		return UploadResult{}, fmt.Errorf("auren storage multipart initiate response missing upload_id")
	}
	parts := make([]UploadPartResult, 0, (size+adapter.partSize-1)/adapter.partSize)
	file, err := os.Open(request.SourcePath)
	if err != nil {
		_ = adapter.abortMultipart(context.Background(), request, initiate.UploadID, "open_source_failed")
		return UploadResult{}, err
	}
	defer file.Close()

	started := time.Now()
	var uploaded int64
	for number, offset := 1, int64(0); offset < size || (size == 0 && number == 1); number++ {
		partSize := adapter.partSize
		if offset+partSize > size {
			partSize = size - offset
		}
		if size == 0 {
			partSize = 0
		}
		part, err := adapter.uploadPart(ctx, request, initiate.UploadID, number, file, offset, partSize, size)
		if err != nil {
			_ = adapter.abortMultipart(context.Background(), request, initiate.UploadID, err.Error())
			return UploadResult{}, err
		}
		parts = append(parts, part)
		uploaded += part.Size
		if request.Progress != nil {
			elapsed := time.Since(started).Seconds()
			speed := int64(0)
			if elapsed > 0 {
				speed = int64(float64(uploaded) / elapsed)
			}
			percent := float64(0)
			if size > 0 {
				percent = (float64(uploaded) / float64(size)) * 100
			}
			_ = request.Progress(ctx, UploadProgress{Stage: "upload", CurrentBytes: uploaded, TotalBytes: size, SpeedBps: speed, Percent: percent, Message: "Upload multipart para Auren Storage em andamento.", Metrics: map[string]any{"driver": DriverAurenStorage, "mode": "multipart", "upload_id": initiate.UploadID, "part_number": number}})
		}
		if size == 0 {
			break
		}
		offset += partSize
	}
	result, err := adapter.completeMultipart(ctx, request, initiate.UploadID, size, parts)
	if err != nil {
		_ = adapter.abortMultipart(context.Background(), request, initiate.UploadID, err.Error())
		return UploadResult{}, err
	}
	if request.Progress != nil {
		_ = request.Progress(ctx, UploadProgress{Stage: "upload", CurrentBytes: size, TotalBytes: size, Percent: 100, Message: "Upload multipart para Auren Storage concluído.", Metrics: map[string]any{"driver": DriverAurenStorage, "mode": "multipart", "upload_id": initiate.UploadID, "object_uuid": result.ObjectUUID}})
	}
	return result, nil
}

type multipartInitiateResult struct {
	UploadID string
}

func (adapter *AurenAdapter) initiateMultipart(ctx context.Context, request UploadRequest, size int64) (multipartInitiateResult, error) {
	payload := map[string]any{
		"path":               request.ObjectPath,
		"object_path":        request.ObjectPath,
		"directory_path":     request.DirectoryPath,
		"relative_path":      request.RelativePath,
		"visibility":         request.Visibility,
		"mime_type":          request.MimeType,
		"checksum_algorithm": request.ChecksumAlgorithm,
		"checksum_sha256":    request.ChecksumSHA256,
		"size":               size,
		"metadata":           request.Metadata,
	}
	response, err := adapter.doJSON(ctx, http.MethodPost, adapter.multipartURL(adapter.bucketID(request)), payload)
	if err != nil {
		return multipartInitiateResult{}, err
	}
	return multipartInitiateResult{UploadID: firstStringRecursive(response, "upload_id", "multipart_upload_id", "id", "uuid")}, nil
}

func (adapter *AurenAdapter) uploadPart(ctx context.Context, request UploadRequest, uploadID string, number int, file *os.File, offset int64, size int64, totalSize int64) (UploadPartResult, error) {
	section := io.NewSectionReader(file, offset, size)
	checksum, err := sha256Reader(ctx, io.NewSectionReader(file, offset, size))
	if err != nil {
		return UploadPartResult{}, err
	}
	url := adapter.multipartPartURL(adapter.bucketID(request), uploadID, number)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPut, url, section)
	if err != nil {
		return UploadPartResult{}, err
	}
	httpRequest.ContentLength = size
	httpRequest.Header.Set("Content-Type", "application/octet-stream")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("X-Auren-Part-Number", strconv.Itoa(number))
	httpRequest.Header.Set("X-Auren-Part-SHA256", checksum)
	if size > 0 {
		httpRequest.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+size-1, totalSize))
	}
	adapter.applyAuth(httpRequest)
	response, err := adapter.client.Do(httpRequest)
	if err != nil {
		return UploadPartResult{}, err
	}
	defer response.Body.Close()
	body, err := readJSONResponse(response)
	if err != nil {
		return UploadPartResult{}, err
	}
	etag := response.Header.Get("ETag")
	if etag == "" {
		etag = firstStringRecursive(body, "etag", "e_tag")
	}
	return UploadPartResult{Number: number, Size: size, ChecksumSHA256: checksum, ETag: strings.Trim(etag, `"`)}, nil
}

func (adapter *AurenAdapter) completeMultipart(ctx context.Context, request UploadRequest, uploadID string, size int64, parts []UploadPartResult) (UploadResult, error) {
	payload := map[string]any{
		"upload_id":       uploadID,
		"path":            request.ObjectPath,
		"object_path":     request.ObjectPath,
		"size":            size,
		"checksum_sha256": request.ChecksumSHA256,
		"parts":           parts,
		"metadata":        request.Metadata,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return UploadResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, adapter.multipartCompleteURL(adapter.bucketID(request), uploadID), bytes.NewReader(encoded))
	if err != nil {
		return UploadResult{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	adapter.applyAuth(httpRequest)
	response, err := adapter.client.Do(httpRequest)
	if err != nil {
		return UploadResult{}, err
	}
	defer response.Body.Close()
	return adapter.parseObjectResponse(response, request, size, true, parts)
}

func (adapter *AurenAdapter) abortMultipart(ctx context.Context, request UploadRequest, uploadID string, reason string) error {
	payload := map[string]any{"upload_id": uploadID, "reason": reason}
	_, err := adapter.doJSON(ctx, http.MethodPost, adapter.multipartAbortURL(adapter.bucketID(request), uploadID), payload)
	return err
}

func (adapter *AurenAdapter) doJSON(ctx context.Context, method string, endpoint string, payload any) (map[string]any, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	adapter.applyAuth(req)
	resp, err := adapter.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return readJSONResponse(resp)
}

func (adapter *AurenAdapter) parseObjectResponse(response *http.Response, request UploadRequest, size int64, multipart bool, parts []UploadPartResult) (UploadResult, error) {
	body, err := readJSONResponse(response)
	if err != nil {
		return UploadResult{}, err
	}
	location := response.Header.Get("Location")
	if strings.TrimSpace(location) == "" {
		location = firstStringRecursive(body, "url", "signed_url", "public_url", "temporary_url", "location")
	}
	return UploadResult{
		Adapter:        adapter.Name(),
		Driver:         adapter.Driver(),
		BucketUUID:     firstNonEmpty(firstStringRecursive(body, "bucket_uuid"), request.BucketUUID),
		Bucket:         firstNonEmpty(firstStringRecursive(body, "bucket", "bucket_name"), request.Bucket),
		ObjectUUID:     firstStringRecursive(body, "object_uuid", "uuid", "id"),
		ObjectPath:     firstNonEmpty(firstStringRecursive(body, "path", "object_path", "relative_path"), request.ObjectPath),
		Location:       location,
		BytesWritten:   firstInt64Recursive(body, size, "size", "bytes", "bytes_written"),
		ChecksumSHA256: firstNonEmpty(firstStringRecursive(body, "checksum_sha256", "sha256", "checksum"), request.ChecksumSHA256),
		Visibility:     firstNonEmpty(firstStringRecursive(body, "visibility"), request.Visibility),
		MimeType:       firstNonEmpty(firstStringRecursive(body, "mime_type", "content_type"), request.MimeType),
		Multipart:      multipart,
		Parts:          parts,
		Metadata:       cloneStringMap(request.Metadata),
	}, nil
}

func (adapter *AurenAdapter) objectURL(bucket string) string {
	return adapter.endpoint + "/api/v1/buckets/" + url.PathEscape(bucket) + "/objects"
}

func (adapter *AurenAdapter) multipartURL(bucket string) string {
	return adapter.endpoint + "/api/v1/buckets/" + url.PathEscape(bucket) + "/multipart-uploads"
}

func (adapter *AurenAdapter) multipartPartURL(bucket string, uploadID string, number int) string {
	return adapter.endpoint + "/api/v1/buckets/" + url.PathEscape(bucket) + "/multipart-uploads/" + url.PathEscape(uploadID) + "/parts/" + strconv.Itoa(number)
}

func (adapter *AurenAdapter) multipartCompleteURL(bucket string, uploadID string) string {
	return adapter.endpoint + "/api/v1/buckets/" + url.PathEscape(bucket) + "/multipart-uploads/" + url.PathEscape(uploadID) + "/complete"
}

func (adapter *AurenAdapter) multipartAbortURL(bucket string, uploadID string) string {
	return adapter.endpoint + "/api/v1/buckets/" + url.PathEscape(bucket) + "/multipart-uploads/" + url.PathEscape(uploadID) + "/abort"
}

func (adapter *AurenAdapter) bucketID(request UploadRequest) string {
	return firstNonEmpty(request.BucketUUID, request.Bucket, adapter.bucketUUID, adapter.bucket)
}

func (adapter *AurenAdapter) applyAuth(request *http.Request) {
	if adapter == nil || request == nil || adapter.apiKey == "" {
		return
	}
	value := adapter.apiKey
	if strings.EqualFold(adapter.tokenHeader, "Authorization") && !strings.HasPrefix(strings.ToLower(value), "bearer ") {
		value = "Bearer " + value
	}
	request.Header.Set(adapter.tokenHeader, value)
}

func writeAurenFormFields(writer *multipart.Writer, request UploadRequest, size int64) error {
	fields := map[string]string{
		"path":               request.ObjectPath,
		"object_path":        request.ObjectPath,
		"directory_path":     request.DirectoryPath,
		"relative_path":      request.RelativePath,
		"visibility":         request.Visibility,
		"mime_type":          request.MimeType,
		"checksum_algorithm": request.ChecksumAlgorithm,
		"checksum_sha256":    request.ChecksumSHA256,
		"size":               strconv.FormatInt(size, 10),
	}
	for key, value := range fields {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}
	if len(request.Metadata) > 0 {
		encoded, err := json.Marshal(request.Metadata)
		if err != nil {
			return err
		}
		if err := writer.WriteField("metadata", string(encoded)); err != nil {
			return err
		}
		for key, value := range request.Metadata {
			if strings.ContainsAny(key, "\r\n\x00") {
				continue
			}
			if err := writer.WriteField("metadata["+key+"]", value); err != nil {
				return err
			}
		}
	}
	return nil
}

func readJSONResponse(response *http.Response) (map[string]any, error) {
	if response == nil {
		return nil, fmt.Errorf("auren storage response cannot be nil")
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("auren storage returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(payload)))
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return map[string]any{}, nil
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	return body, nil
}

func firstStringRecursive(input map[string]any, keys ...string) string {
	if len(input) == 0 {
		return ""
	}
	wanted := map[string]struct{}{}
	for _, key := range keys {
		wanted[strings.ToLower(key)] = struct{}{}
	}
	var walk func(any) string
	walk = func(value any) string {
		switch typed := value.(type) {
		case map[string]any:
			for key, nested := range typed {
				if _, ok := wanted[strings.ToLower(key)]; ok {
					if text := anyToString(nested); text != "" {
						return text
					}
				}
			}
			for _, nested := range typed {
				if text := walk(nested); text != "" {
					return text
				}
			}
		case []any:
			for _, nested := range typed {
				if text := walk(nested); text != "" {
					return text
				}
			}
		}
		return ""
	}
	return walk(input)
}

func firstInt64Recursive(input map[string]any, fallback int64, keys ...string) int64 {
	text := firstStringRecursive(input, keys...)
	if text == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func anyToString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int:
		return strconv.Itoa(typed)
	}
	return ""
}

func sha256File(ctx context.Context, sourcePath string) (string, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return sha256Reader(ctx, file)
}

func sha256Reader(ctx context.Context, reader io.Reader) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	hasher := sha256.New()
	if err := copyHash(ctx, hasher, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func copyHash(ctx context.Context, hasher hash.Hash, reader io.Reader) error {
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		read, err := reader.Read(buffer)
		if read > 0 {
			if _, hashErr := hasher.Write(buffer[:read]); hashErr != nil {
				return hashErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

type progressReader struct {
	reader   io.Reader
	current  int64
	total    int64
	started  time.Time
	interval time.Duration
	last     time.Time
	callback func(current int64, total int64, speed int64, percent float64)
}

func (reader *progressReader) Read(p []byte) (int, error) {
	n, err := reader.reader.Read(p)
	if n > 0 {
		reader.current += int64(n)
		now := time.Now()
		if reader.last.IsZero() || now.Sub(reader.last) >= reader.interval || reader.current == reader.total {
			reader.last = now
			elapsed := time.Since(reader.started).Seconds()
			speed := int64(0)
			if elapsed > 0 {
				speed = int64(float64(reader.current) / elapsed)
			}
			percent := float64(0)
			if reader.total > 0 {
				percent = (float64(reader.current) / float64(reader.total)) * 100
			}
			if reader.callback != nil {
				reader.callback(reader.current, reader.total, speed, percent)
			}
		}
	}
	return n, err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func escapeQuotes(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}
