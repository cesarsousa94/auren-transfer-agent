package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// MultipartUploaderName is the canonical multipart upload capability name.
	MultipartUploaderName = "multipart"
)

// MultipartOptions configures a mechanical multipart local upload.
type MultipartOptions struct {
	Request  Request
	PartSize int64
}

// Part describes one local upload part.
type Part struct {
	Index        int   `json:"index"`
	Start        int64 `json:"start"`
	End          int64 `json:"end"`
	BytesWritten int64 `json:"bytes_written"`
}

// Plan describes a deterministic multipart upload plan.
type Plan struct {
	Size     int64  `json:"size"`
	PartSize int64  `json:"part_size"`
	Parts    []Part `json:"parts"`
}

// NewPlan splits size into deterministic upload parts.
func NewPlan(size int64, partSize int64) (Plan, error) {
	if size < 0 {
		return Plan{}, fmt.Errorf("upload size must be zero or greater")
	}
	if partSize <= 0 {
		return Plan{}, fmt.Errorf("upload part size must be greater than zero")
	}
	if size == 0 {
		return Plan{Size: size, PartSize: partSize, Parts: []Part{{Index: 0, Start: 0, End: -1}}}, nil
	}
	parts := make([]Part, 0, (size+partSize-1)/partSize)
	for start, index := int64(0), 0; start < size; start, index = start+partSize, index+1 {
		end := start + partSize - 1
		if end >= size {
			end = size - 1
		}
		parts = append(parts, Part{Index: index, Start: start, End: end})
	}
	return Plan{Size: size, PartSize: partSize, Parts: parts}, nil
}

// PartsSnapshot returns a defensive copy of plan parts.
func (plan Plan) PartsSnapshot() []Part {
	parts := make([]Part, len(plan.Parts))
	copy(parts, plan.Parts)
	return parts
}

// MultipartUpload copies SourcePath to the local destination in deterministic parts.
func (uploader *LocalUploader) MultipartUpload(ctx context.Context, options MultipartOptions) (Result, error) {
	if uploader == nil {
		return Result{}, fmt.Errorf("local uploader cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	request := options.Request.Clone()
	if err := ValidateRequest(request); err != nil {
		return Result{}, err
	}
	if options.PartSize <= 0 {
		return Result{}, fmt.Errorf("upload part size must be greater than zero")
	}
	destination, err := uploader.ResolveDestination(request.DestinationPath)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(request.SourcePath)
	if err != nil {
		return Result{}, err
	}
	if info.IsDir() {
		return Result{}, fmt.Errorf("upload source must be a file")
	}
	plan, err := NewPlan(info.Size(), options.PartSize)
	if err != nil {
		return Result{}, err
	}
	return uploader.multipartCopy(ctx, request, destination, plan)
}

func (uploader *LocalUploader) multipartCopy(ctx context.Context, request Request, destination string, plan Plan) (Result, error) {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return Result{}, err
	}
	started := time.Now()
	source, err := os.Open(request.SourcePath)
	if err != nil {
		return Result{}, err
	}
	defer source.Close()
	target, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return Result{}, err
	}
	defer target.Close()

	parts := plan.PartsSnapshot()
	buffer := make([]byte, minInt64(plan.PartSize, 1024*1024))
	var total int64
	for index, part := range parts {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		remaining := int64(0)
		if part.End >= part.Start {
			remaining = part.End - part.Start + 1
		}
		written, err := copyLimited(target, source, buffer, remaining)
		part.BytesWritten = written
		parts[index] = part
		total += written
		if err != nil {
			return Result{}, err
		}
		if written != remaining {
			return Result{}, fmt.Errorf("upload part %d wrote %d bytes, expected %d", part.Index, written, remaining)
		}
	}
	return Result{Uploader: uploader.Name(), Driver: uploader.Driver(), SourcePath: request.SourcePath, DestinationPath: destination, BytesWritten: total, Duration: time.Since(started), Multipart: true, Parts: parts, Metadata: cloneStringMap(request.Metadata)}, nil
}

// ParsePartSize parses a positive byte size string used by upload configuration.
func ParsePartSize(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("upload part size is required")
	}
	lower := strings.ToLower(trimmed)
	units := []struct {
		suffix     string
		multiplier int64
	}{
		{suffix: "kib", multiplier: 1024},
		{suffix: "mib", multiplier: 1024 * 1024},
		{suffix: "gib", multiplier: 1024 * 1024 * 1024},
		{suffix: "kb", multiplier: 1000},
		{suffix: "mb", multiplier: 1000 * 1000},
		{suffix: "gb", multiplier: 1000 * 1000 * 1000},
		{suffix: "b", multiplier: 1},
	}
	multiplier := int64(1)
	number := lower
	for _, unit := range units {
		if strings.HasSuffix(lower, unit.suffix) {
			multiplier = unit.multiplier
			number = strings.TrimSpace(strings.TrimSuffix(lower, unit.suffix))
			break
		}
	}
	parsed, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("upload part size must be positive")
	}
	return parsed * multiplier, nil
}

func copyLimited(writer io.Writer, reader io.Reader, buffer []byte, remaining int64) (int64, error) {
	if remaining <= 0 {
		return 0, nil
	}
	var written int64
	for remaining > 0 {
		chunk := int64(len(buffer))
		if chunk > remaining {
			chunk = remaining
		}
		read, readErr := reader.Read(buffer[:chunk])
		if read > 0 {
			out, writeErr := writer.Write(buffer[:read])
			written += int64(out)
			remaining -= int64(out)
			if writeErr != nil {
				return written, writeErr
			}
			if out != read {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF && remaining == 0 {
				return written, nil
			}
			return written, readErr
		}
	}
	return written, nil
}

func minInt64(left int64, right int64) int {
	if left < right {
		return int(left)
	}
	return int(right)
}
