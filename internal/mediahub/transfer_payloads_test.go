package mediahub

import "testing"

func TestParseClaimResponseWithJob(t *testing.T) {
	result, err := ParseClaimResponse(map[string]any{
		"success": true,
		"job": map[string]any{
			"uuid":        "job-1",
			"operation":   "remote_download",
			"source":      map[string]any{"url": "https://example.test/movie.mp4", "resume_enabled": true},
			"destination": map[string]any{"driver": "local", "object_path": "movie.mp4"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Job.UUID != "job-1" || result.Job.Source.URL == "" {
		t.Fatalf("unexpected claim response: %+v", result)
	}
}

func TestValidateTransferJobRequiresSourceAndDestination(t *testing.T) {
	if err := ValidateTransferJob(TransferJob{UUID: "job-1", Operation: "remote_download"}); err == nil {
		t.Fatal("expected invalid job")
	}
}
