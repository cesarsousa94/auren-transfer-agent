package download

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// MultipartEngineName is the canonical name for multipart download support.
	MultipartEngineName = "multipart"
)

// MultipartOptions configures a mechanical range-based multipart download.
type MultipartOptions struct {
	URL        string
	Path       string
	Headers    HeaderSet
	PartSize   int64
	TotalSize  int64
	CreateDirs bool
	Bandwidth  *BandwidthController
	Metrics    MetricsRecorder
}

// MultipartPart describes one byte range to download.
type MultipartPart struct {
	Index        int    `json:"index"`
	Start        int64  `json:"start"`
	End          int64  `json:"end"`
	RangeHeader  string `json:"range_header"`
	Path         string `json:"path,omitempty"`
	BytesWritten int64  `json:"bytes_written"`
}

// MultipartPlan describes the byte ranges for a multipart download.
type MultipartPlan struct {
	TotalSize int64           `json:"total_size"`
	PartSize  int64           `json:"part_size"`
	Parts     []MultipartPart `json:"parts"`
}

// MultipartResult describes a completed multipart download attempt.
type MultipartResult struct {
	URL          string          `json:"url"`
	Path         string          `json:"path"`
	TotalSize    int64           `json:"total_size"`
	PartSize     int64           `json:"part_size"`
	Parts        []MultipartPart `json:"parts"`
	BytesWritten int64           `json:"bytes_written"`
	Duration     time.Duration   `json:"duration"`
}

// NewMultipartPlan splits totalSize into deterministic byte ranges.
func NewMultipartPlan(totalSize int64, partSize int64) (MultipartPlan, error) {
	if totalSize <= 0 {
		return MultipartPlan{}, fmt.Errorf("multipart total size must be greater than zero")
	}
	if partSize <= 0 {
		return MultipartPlan{}, fmt.Errorf("multipart part size must be greater than zero")
	}
	parts := make([]MultipartPart, 0, (totalSize+partSize-1)/partSize)
	for start, index := int64(0), 0; start < totalSize; start, index = start+partSize, index+1 {
		end := start + partSize - 1
		if end >= totalSize {
			end = totalSize - 1
		}
		parts = append(parts, MultipartPart{Index: index, Start: start, End: end, RangeHeader: fmt.Sprintf("bytes=%d-%d", start, end)})
	}
	return MultipartPlan{TotalSize: totalSize, PartSize: partSize, Parts: parts}, nil
}

// PartsSnapshot returns a defensive copy of the multipart plan parts.
func (plan MultipartPlan) PartsSnapshot() []MultipartPart {
	parts := make([]MultipartPart, len(plan.Parts))
	copy(parts, plan.Parts)
	return parts
}

// MultipartToFile downloads all planned byte ranges and merges them into the target path.
func (client *HTTPClient) MultipartToFile(ctx context.Context, options MultipartOptions) (MultipartResult, error) {
	if client == nil {
		return MultipartResult{}, fmt.Errorf("http client cannot be nil")
	}
	path := strings.TrimSpace(options.Path)
	if path == "" {
		return MultipartResult{}, fmt.Errorf("multipart file path is required")
	}
	plan, err := NewMultipartPlan(options.TotalSize, options.PartSize)
	if err != nil {
		return MultipartResult{}, err
	}
	if options.CreateDirs {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return MultipartResult{}, err
		}
	}

	started := time.Now()
	partsDir := path + ".parts"
	if err := os.RemoveAll(partsDir); err != nil {
		return MultipartResult{}, err
	}
	if err := os.MkdirAll(partsDir, 0o755); err != nil {
		return MultipartResult{}, err
	}
	defer os.RemoveAll(partsDir)

	completed := plan.PartsSnapshot()
	var totalWritten int64
	for index, part := range completed {
		partPath := filepath.Join(partsDir, fmt.Sprintf("part-%06d", part.Index))
		headers, err := options.Headers.With(HeaderRange, part.RangeHeader)
		if err != nil {
			return MultipartResult{}, err
		}
		streamResult, err := client.StreamToFile(ctx, StreamToFileOptions{URL: options.URL, Path: partPath, Headers: headers, CreateDirs: true, Resume: ResumeState{RangeHeader: part.RangeHeader}, Bandwidth: options.Bandwidth})
		part.Path = partPath
		part.BytesWritten = streamResult.BytesWritten
		completed[index] = part
		totalWritten += streamResult.BytesWritten
		if err != nil {
			result := MultipartResult{URL: options.URL, Path: path, TotalSize: plan.TotalSize, PartSize: plan.PartSize, Parts: completed, BytesWritten: totalWritten, Duration: time.Since(started)}
			recordMultipartMetric(ctx, options, result, err)
			return result, err
		}
		if expected := part.End - part.Start + 1; streamResult.BytesWritten != expected {
			result := MultipartResult{URL: options.URL, Path: path, TotalSize: plan.TotalSize, PartSize: plan.PartSize, Parts: completed, BytesWritten: totalWritten, Duration: time.Since(started)}
			err := fmt.Errorf("multipart part %d wrote %d bytes, expected %d", part.Index, streamResult.BytesWritten, expected)
			recordMultipartMetric(ctx, options, result, err)
			return result, err
		}
	}

	if err := mergeMultipartParts(path, completed); err != nil {
		result := MultipartResult{URL: options.URL, Path: path, TotalSize: plan.TotalSize, PartSize: plan.PartSize, Parts: completed, BytesWritten: totalWritten, Duration: time.Since(started)}
		recordMultipartMetric(ctx, options, result, err)
		return result, err
	}
	result := MultipartResult{URL: options.URL, Path: path, TotalSize: plan.TotalSize, PartSize: plan.PartSize, Parts: completed, BytesWritten: totalWritten, Duration: time.Since(started)}
	recordMultipartMetric(ctx, options, result, nil)
	return result, nil
}

func recordMultipartMetric(ctx context.Context, options MultipartOptions, result MultipartResult, multipartErr error) {
	metric := DownloadMetric{
		Engine:              MultipartEngineName,
		URL:                 result.URL,
		BytesWritten:        result.BytesWritten,
		ContentLength:       result.TotalSize,
		Duration:            result.Duration,
		MultipartParts:      len(result.Parts),
		BandwidthLimited:    options.Bandwidth != nil && options.Bandwidth.Enabled(),
		BandwidthBytesLimit: 0,
	}
	if options.Bandwidth != nil {
		metric.BandwidthBytesLimit = options.Bandwidth.LimitBytesPerSecond()
	}
	if multipartErr != nil {
		metric.Error = multipartErr.Error()
	}
	recordDownloadMetric(ctx, options.Metrics, metric)
}

func mergeMultipartParts(path string, parts []MultipartPart) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, part := range parts {
		partFile, err := os.Open(part.Path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(file, partFile)
		closeErr := partFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
