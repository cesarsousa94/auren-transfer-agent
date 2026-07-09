package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// StreamingEngineName is the canonical name for streaming download support.
	StreamingEngineName = "streaming"
)

// StreamOptions configures a streaming HTTP download attempt.
type StreamOptions struct {
	URL       string
	Method    string
	Headers   HeaderSet
	Resume    ResumeState
	Writer    io.Writer
	Bandwidth *BandwidthController
	Metrics   MetricsRecorder
}

// StreamToFileOptions configures streaming to a local file.
type StreamToFileOptions struct {
	URL        string
	Path       string
	Headers    HeaderSet
	Resume     ResumeState
	Append     bool
	CreateDirs bool
	Bandwidth  *BandwidthController
	Metrics    MetricsRecorder
}

// StreamResult describes one completed HTTP streaming transfer.
type StreamResult struct {
	URL           string        `json:"url"`
	StatusCode    int           `json:"status_code"`
	BytesWritten  int64         `json:"bytes_written"`
	ContentLength int64         `json:"content_length"`
	Duration      time.Duration `json:"duration"`
	Resumed       bool          `json:"resumed"`
	RangeHeader   string        `json:"range_header,omitempty"`
}

// Stream downloads the response body to options.Writer without buffering the full payload.
func (client *HTTPClient) Stream(ctx context.Context, options StreamOptions) (StreamResult, error) {
	if client == nil {
		return StreamResult{}, fmt.Errorf("http client cannot be nil")
	}
	if options.Writer == nil {
		return StreamResult{}, fmt.Errorf("stream writer is required")
	}
	request, err := NewRequest(ctx, RequestOptions{Method: options.Method, URL: options.URL, Headers: options.Headers})
	if err != nil {
		return StreamResult{}, err
	}
	if err := ApplyResume(request, options.Resume); err != nil {
		return StreamResult{}, err
	}

	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		result := StreamResult{URL: request.URL.String(), Duration: time.Since(started), Resumed: options.Resume.RangeHeader != "", RangeHeader: options.Resume.RangeHeader}
		recordStreamMetric(ctx, options, result, err)
		return result, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		result := StreamResult{URL: request.URL.String(), StatusCode: response.StatusCode, ContentLength: response.ContentLength, Duration: time.Since(started), Resumed: options.Resume.RangeHeader != "", RangeHeader: options.Resume.RangeHeader}
		err := fmt.Errorf("unexpected download status: %d", response.StatusCode)
		recordStreamMetric(ctx, options, result, err)
		return result, err
	}
	if options.Resume.RangeHeader != "" && response.StatusCode != http.StatusPartialContent {
		result := StreamResult{URL: request.URL.String(), StatusCode: response.StatusCode, ContentLength: response.ContentLength, Duration: time.Since(started), Resumed: true, RangeHeader: options.Resume.RangeHeader}
		err := fmt.Errorf("resume requested but server returned status %d", response.StatusCode)
		recordStreamMetric(ctx, options, result, err)
		return result, err
	}

	writer := options.Writer
	if options.Bandwidth != nil {
		writer = options.Bandwidth.WrapWriter(ctx, writer)
	}
	written, err := io.Copy(writer, response.Body)
	result := StreamResult{URL: request.URL.String(), StatusCode: response.StatusCode, BytesWritten: written, ContentLength: response.ContentLength, Duration: time.Since(started), Resumed: options.Resume.RangeHeader != "", RangeHeader: options.Resume.RangeHeader}
	recordStreamMetric(ctx, options, result, err)
	if err != nil {
		return result, err
	}
	return result, nil
}

// StreamToFile downloads to a local file, optionally appending for resume flows.
func (client *HTTPClient) StreamToFile(ctx context.Context, options StreamToFileOptions) (StreamResult, error) {
	path := strings.TrimSpace(options.Path)
	if path == "" {
		return StreamResult{}, fmt.Errorf("stream file path is required")
	}
	if options.CreateDirs {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return StreamResult{}, err
		}
	}
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if options.Append {
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	file, err := os.OpenFile(path, flag, 0o644)
	if err != nil {
		return StreamResult{}, err
	}
	defer file.Close()
	return client.Stream(ctx, StreamOptions{URL: options.URL, Method: http.MethodGet, Headers: options.Headers, Resume: options.Resume, Writer: file, Bandwidth: options.Bandwidth, Metrics: options.Metrics})
}

func recordStreamMetric(ctx context.Context, options StreamOptions, result StreamResult, streamErr error) {
	metric := DownloadMetric{
		Engine:              StreamingEngineName,
		URL:                 result.URL,
		StatusCode:          result.StatusCode,
		BytesWritten:        result.BytesWritten,
		ContentLength:       result.ContentLength,
		Duration:            result.Duration,
		Resumed:             result.Resumed,
		RangeHeader:         result.RangeHeader,
		BandwidthLimited:    options.Bandwidth != nil && options.Bandwidth.Enabled(),
		BandwidthBytesLimit: 0,
	}
	if options.Bandwidth != nil {
		metric.BandwidthBytesLimit = options.Bandwidth.LimitBytesPerSecond()
	}
	if streamErr != nil {
		metric.Error = streamErr.Error()
	}
	recordDownloadMetric(ctx, options.Metrics, metric)
}
