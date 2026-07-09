package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// CallbackEngineName is the canonical callback capability name.
	CallbackEngineName = "callback_engine"

	// CallbackEventUploadCompleted is emitted after a completed mechanical upload.
	CallbackEventUploadCompleted = "upload.completed"

	// CallbackEventUploadFailed is emitted after a failed mechanical upload.
	CallbackEventUploadFailed = "upload.failed"
)

// CallbackPayload is the JSON body sent to callback endpoints.
type CallbackPayload struct {
	Event     string            `json:"event"`
	Status    string            `json:"status"`
	JobID     string            `json:"job_id,omitempty"`
	Result    *Result           `json:"result,omitempty"`
	Integrity *IntegrityResult  `json:"integrity,omitempty"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// CallbackSender sends callback payloads.
type CallbackSender interface {
	Send(context.Context, CallbackPayload) error
}

// HTTPCallbackSender posts callback payloads as JSON.
type HTTPCallbackSender struct {
	endpoint string
	headers  map[string]string
	client   *http.Client
}

// HTTPCallbackOptions configures HTTP callbacks.
type HTTPCallbackOptions struct {
	Endpoint string
	Headers  map[string]string
	Timeout  time.Duration
	Client   *http.Client
}

// NewHTTPCallbackSender validates and creates an HTTP callback sender.
func NewHTTPCallbackSender(options HTTPCallbackOptions) (*HTTPCallbackSender, error) {
	endpoint := strings.TrimSpace(options.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("callback endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("callback endpoint must be an absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("callback endpoint scheme must be http or https")
	}
	client := options.Client
	if client == nil {
		timeout := options.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &HTTPCallbackSender{endpoint: endpoint, headers: cloneStringMap(options.Headers), client: client}, nil
}

// Endpoint returns the configured callback endpoint.
func (sender *HTTPCallbackSender) Endpoint() string {
	if sender == nil {
		return ""
	}
	return sender.endpoint
}

// Send posts the callback payload and requires a 2xx response.
func (sender *HTTPCallbackSender) Send(ctx context.Context, payload CallbackPayload) error {
	if sender == nil {
		return fmt.Errorf("callback sender cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload.Timestamp = payload.Timestamp.UTC()
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now().UTC()
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, sender.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range sender.headers {
		request.Header.Set(key, value)
	}
	response, err := sender.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("callback failed with status %d", response.StatusCode)
	}
	return nil
}
