package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	// ResumeUploadName is the canonical resume upload capability name.
	ResumeUploadName = "resume_upload"
)

// ResumeState describes the local state used to resume one upload.
type ResumeState struct {
	SourcePath      string `json:"source_path"`
	DestinationPath string `json:"destination_path"`
	SourceSize      int64  `json:"source_size"`
	DestinationSize int64  `json:"destination_size"`
	Offset          int64  `json:"offset"`
	Complete        bool   `json:"complete"`
}

// ResumeFromLocalState inspects source and destination files and computes the
// next offset. It performs only mechanical file checks.
func (uploader *LocalUploader) ResumeFromLocalState(request Request) (ResumeState, error) {
	if uploader == nil {
		return ResumeState{}, fmt.Errorf("local uploader cannot be nil")
	}
	request = request.Clone()
	if err := ValidateRequest(request); err != nil {
		return ResumeState{}, err
	}
	destination, err := uploader.ResolveDestination(request.DestinationPath)
	if err != nil {
		return ResumeState{}, err
	}
	sourceInfo, err := os.Stat(request.SourcePath)
	if err != nil {
		return ResumeState{}, err
	}
	if sourceInfo.IsDir() {
		return ResumeState{}, fmt.Errorf("upload source must be a file")
	}

	var destinationSize int64
	destinationInfo, err := os.Stat(destination)
	if err != nil {
		if !os.IsNotExist(err) {
			return ResumeState{}, err
		}
	} else {
		if destinationInfo.IsDir() {
			return ResumeState{}, fmt.Errorf("upload destination is a directory")
		}
		destinationSize = destinationInfo.Size()
	}
	if destinationSize > sourceInfo.Size() {
		return ResumeState{}, fmt.Errorf("upload destination is larger than source")
	}
	return ResumeState{SourcePath: request.SourcePath, DestinationPath: destination, SourceSize: sourceInfo.Size(), DestinationSize: destinationSize, Offset: destinationSize, Complete: destinationSize == sourceInfo.Size()}, nil
}

// ResumeUpload appends the missing bytes from SourcePath to the local destination.
func (uploader *LocalUploader) ResumeUpload(ctx context.Context, request Request) (Result, error) {
	if uploader == nil {
		return Result{}, fmt.Errorf("local uploader cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	request = request.Clone()
	state, err := uploader.ResumeFromLocalState(request)
	if err != nil {
		return Result{}, err
	}
	metadata := cloneStringMap(request.Metadata)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["resume"] = "true"
	metadata["resume_offset"] = strconv.FormatInt(state.Offset, 10)
	metadata["resume_complete"] = strconv.FormatBool(state.Complete)

	if state.Complete {
		return Result{Uploader: uploader.Name(), Driver: uploader.Driver(), SourcePath: request.SourcePath, DestinationPath: state.DestinationPath, BytesWritten: 0, Resumed: true, AlreadyComplete: true, Metadata: metadata}, nil
	}
	if err := os.MkdirAll(filepath.Dir(state.DestinationPath), 0o755); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	started := time.Now()
	source, err := os.Open(request.SourcePath)
	if err != nil {
		return Result{}, err
	}
	defer source.Close()
	if _, err := source.Seek(state.Offset, io.SeekStart); err != nil {
		return Result{}, err
	}
	flag := os.O_CREATE | os.O_WRONLY
	if state.Offset == 0 {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_APPEND
	}
	target, err := os.OpenFile(state.DestinationPath, flag, 0o644)
	if err != nil {
		return Result{}, err
	}
	written, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return Result{}, copyErr
	}
	if closeErr != nil {
		return Result{}, closeErr
	}
	return Result{Uploader: uploader.Name(), Driver: uploader.Driver(), SourcePath: request.SourcePath, DestinationPath: state.DestinationPath, BytesWritten: written, Duration: time.Since(started), Resumed: true, Metadata: metadata}, nil
}
