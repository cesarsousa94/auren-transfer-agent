package download

import (
	"fmt"
	"net/http"
	"os"
)

const (
	// ResumeEngineName is the canonical name for resume download support.
	ResumeEngineName = "resume"
)

// ResumeOptions describes local state used to resume a partial transfer.
type ResumeOptions struct {
	Enabled       bool
	ExistingBytes int64
	TotalBytes    int64
}

// ResumeState is the validated resume decision for a download attempt.
type ResumeState struct {
	Enabled       bool   `json:"enabled"`
	ExistingBytes int64  `json:"existing_bytes"`
	TotalBytes    int64  `json:"total_bytes,omitempty"`
	RangeHeader   string `json:"range_header,omitempty"`
	Complete      bool   `json:"complete"`
}

// NewResumeState validates resume inputs and builds an HTTP Range header when useful.
func NewResumeState(options ResumeOptions) (ResumeState, error) {
	if options.ExistingBytes < 0 {
		return ResumeState{}, fmt.Errorf("existing bytes must be zero or greater")
	}
	if options.TotalBytes < 0 {
		return ResumeState{}, fmt.Errorf("total bytes must be zero or greater")
	}
	state := ResumeState{Enabled: options.Enabled, ExistingBytes: options.ExistingBytes, TotalBytes: options.TotalBytes}
	if !options.Enabled || options.ExistingBytes == 0 {
		return state, nil
	}
	if options.TotalBytes > 0 && options.ExistingBytes >= options.TotalBytes {
		state.Complete = true
		return state, nil
	}
	state.RangeHeader = fmt.Sprintf("bytes=%d-", options.ExistingBytes)
	return state, nil
}

// ResumeFromFile returns resume state by inspecting a local partial file.
func ResumeFromFile(path string, enabled bool, totalBytes int64) (ResumeState, error) {
	if path == "" {
		return NewResumeState(ResumeOptions{Enabled: enabled, TotalBytes: totalBytes})
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewResumeState(ResumeOptions{Enabled: enabled, TotalBytes: totalBytes})
		}
		return ResumeState{}, err
	}
	if info.IsDir() {
		return ResumeState{}, fmt.Errorf("resume path is a directory: %s", path)
	}
	return NewResumeState(ResumeOptions{Enabled: enabled, ExistingBytes: info.Size(), TotalBytes: totalBytes})
}

// ApplyResume applies the Range header represented by state to a request.
func ApplyResume(request *http.Request, state ResumeState) error {
	if request == nil {
		return fmt.Errorf("http request cannot be nil")
	}
	if state.RangeHeader == "" {
		return nil
	}
	request.Header.Set(HeaderRange, state.RangeHeader)
	return nil
}
