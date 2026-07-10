package mediahub

import (
	"fmt"
	"strings"
	"time"
)

const (
	TransferClaimPath = "/api/internal/transfer-agent/jobs/claim"
)

// ClaimRequest is sent by the Agent when it has capacity to execute a Media Hub transfer job.
type ClaimRequest struct {
	Capabilities       []string       `json:"capabilities"`
	Capacity           Capacity       `json:"capacity"`
	AcceptedOperations []string       `json:"accepted_operations"`
	Region             string         `json:"region"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

// ClaimResponse is a permissive wrapper for Media Hub transfer claim results.
type ClaimResponse struct {
	Success bool           `json:"success"`
	Job     TransferJob    `json:"job,omitempty"`
	Raw     map[string]any `json:"-"`
}

// TransferJob is the rich job payload emitted by the Media Hub transfer-agent API.
type TransferJob struct {
	UUID        string              `json:"uuid"`
	Operation   string              `json:"operation"`
	Source      TransferSource      `json:"source"`
	Destination TransferDestination `json:"destination"`
	Control     TransferControl     `json:"control"`
	Context     map[string]any      `json:"context,omitempty"`
	Metadata    map[string]any      `json:"metadata,omitempty"`
}

// TransferSource describes a remote source to download.
type TransferSource struct {
	URL              string            `json:"url"`
	Headers          map[string]string `json:"headers,omitempty"`
	ResolverStrategy string            `json:"resolver_strategy,omitempty"`
	FollowRedirects  bool              `json:"follow_redirects"`
	MaxRedirects     int               `json:"max_redirects,omitempty"`
	ResumeEnabled    bool              `json:"resume_enabled"`
	MinBytes         int64             `json:"min_bytes,omitempty"`
	BlockHTML        bool              `json:"block_html"`
	ExpectedSHA256   string            `json:"expected_sha256,omitempty"`
}

// TransferDestination describes where the Agent must put the completed object.
type TransferDestination struct {
	Driver            string            `json:"driver"`
	Endpoint          string            `json:"endpoint,omitempty"`
	UploadURL         string            `json:"upload_url,omitempty"`
	Token             string            `json:"token,omitempty"`
	TokenHeader       string            `json:"token_header,omitempty"`
	AccessKeyID       string            `json:"access_key_id,omitempty"`
	SecretAccessKey   string            `json:"secret_access_key,omitempty"`
	SessionToken      string            `json:"session_token,omitempty"`
	ForcePathStyle    bool              `json:"force_path_style,omitempty"`
	BucketUUID        string            `json:"bucket_uuid,omitempty"`
	Bucket            string            `json:"bucket,omitempty"`
	DirectoryPath     string            `json:"directory_path,omitempty"`
	RelativePath      string            `json:"relative_path,omitempty"`
	ObjectPath        string            `json:"object_path,omitempty"`
	Visibility        string            `json:"visibility,omitempty"`
	MimeType          string            `json:"mime_type,omitempty"`
	ChecksumAlgorithm string            `json:"checksum_algorithm,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

// TransferControl contains lease/progress policy hints from Media Hub.
type TransferControl struct {
	HeartbeatSeconds     int `json:"heartbeat_seconds,omitempty"`
	ProgressEventSeconds int `json:"progress_event_seconds,omitempty"`
	LeaseSeconds         int `json:"lease_seconds,omitempty"`
	MaxAttempts          int `json:"max_attempts,omitempty"`
}

// TransferProgressPayload is sent throughout download/upload execution.
type TransferProgressPayload struct {
	Status       string         `json:"status"`
	Stage        string         `json:"stage"`
	CurrentBytes int64          `json:"current_bytes"`
	TotalBytes   int64          `json:"total_bytes,omitempty"`
	SpeedBps     int64          `json:"speed_bps,omitempty"`
	Percent      float64        `json:"percent,omitempty"`
	Message      string         `json:"message"`
	Metrics      map[string]any `json:"metrics,omitempty"`
	GeneratedAt  time.Time      `json:"generated_at"`
}

// TransferCompletedPayload is sent once the Agent has uploaded the final object.
type TransferCompletedPayload struct {
	Status         string               `json:"status"`
	Stage          string               `json:"stage"`
	Bytes          int64                `json:"bytes"`
	ChecksumSHA256 string               `json:"checksum_sha256,omitempty"`
	Destination    TransferObjectResult `json:"destination"`
	Timing         map[string]float64   `json:"timing,omitempty"`
	Metrics        map[string]any       `json:"metrics,omitempty"`
	GeneratedAt    time.Time            `json:"generated_at"`
}

// TransferObjectResult identifies the object produced by the Agent.
type TransferObjectResult struct {
	Driver         string            `json:"driver"`
	BucketUUID     string            `json:"bucket_uuid,omitempty"`
	Bucket         string            `json:"bucket,omitempty"`
	ObjectUUID     string            `json:"object_uuid,omitempty"`
	Path           string            `json:"path"`
	URL            string            `json:"url,omitempty"`
	Size           int64             `json:"size,omitempty"`
	ChecksumSHA256 string            `json:"checksum_sha256,omitempty"`
	Visibility     string            `json:"visibility,omitempty"`
	MimeType       string            `json:"mime_type,omitempty"`
	Multipart      bool              `json:"multipart,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// TransferFailedPayload is sent when a transfer attempt fails terminally in the Agent.
type TransferFailedPayload struct {
	Status      string         `json:"status"`
	Stage       string         `json:"stage"`
	Message     string         `json:"message"`
	Error       string         `json:"error"`
	Retryable   bool           `json:"retryable"`
	Metrics     map[string]any `json:"metrics,omitempty"`
	GeneratedAt time.Time      `json:"generated_at"`
}

// TransferEventsPayload batches job-scoped events.
type TransferEventsPayload struct {
	Events      []EventPayload `json:"events"`
	GeneratedAt time.Time      `json:"generated_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// TransferControlResult is the current operator control state for a claimed job.
type TransferControlResult struct {
	Action string         `json:"action,omitempty"`
	Reason string         `json:"reason,omitempty"`
	Raw    map[string]any `json:"-"`
}

// ValidateTransferJob checks the minimal Agent-side execution contract.
func ValidateTransferJob(job TransferJob) error {
	var issues []string
	if strings.TrimSpace(job.UUID) == "" {
		issues = append(issues, "uuid is required")
	}
	if strings.TrimSpace(job.Operation) == "" {
		issues = append(issues, "operation is required")
	}
	if strings.TrimSpace(job.Source.URL) == "" {
		issues = append(issues, "source.url is required")
	}
	if strings.TrimSpace(job.Destination.Driver) == "" {
		issues = append(issues, "destination.driver is required")
	}
	if len(issues) > 0 {
		return fmt.Errorf("transfer job validation failed: %s", strings.Join(issues, "; "))
	}
	return nil
}

// ParseClaimResponse accepts the two response shapes used by Media Hub controllers.
func ParseClaimResponse(response map[string]any) (ClaimResponse, error) {
	if response == nil {
		return ClaimResponse{}, nil
	}
	encoded, err := marshalMap(response)
	if err != nil {
		return ClaimResponse{}, err
	}
	var result ClaimResponse
	if err := unmarshalBytes(encoded, &result); err != nil {
		return ClaimResponse{}, err
	}
	if !result.Success {
		if nested, ok := response["data"].(map[string]any); ok {
			return ParseClaimResponse(nested)
		}
	}
	result.Raw = response
	if strings.TrimSpace(result.Job.UUID) == "" {
		return result, nil
	}
	return result, ValidateTransferJob(result.Job)
}
