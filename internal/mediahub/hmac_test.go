package mediahub

import (
	"net/http"
	"testing"
	"time"
)

func TestCanonicalStringAndSignatureAreStable(t *testing.T) {
	input := SignatureInput{Method: "post", PathQuery: "/api/internal/nodes/heartbeat?x=1", Timestamp: "2026-07-09T12:00:00Z", Nonce: "nonce-1", Body: []byte(`{"status":"ready"}`)}
	canonical := CanonicalString(input)
	wantCanonical := "POST\n/api/internal/nodes/heartbeat?x=1\n2026-07-09T12:00:00Z\nnonce-1\n" + BodySHA256(input.Body)
	if canonical != wantCanonical {
		t.Fatalf("canonical = %q, want %q", canonical, wantCanonical)
	}
	if got := Sign("secret", input); got != "d9c0317b67887aff99b33222f8674037b9983f6403449a21afbf81d3da88ad68" {
		t.Fatalf("signature = %s", got)
	}
}

func TestApplyNodeAuthenticationAddsFallbackAndHMACHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://hub.test/api/internal/nodes/heartbeat?x=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewNodeState("node-1", "secret", "", time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"status":"ready"}`)
	stamp := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	if err := ApplyNodeAuthentication(req, body, state, true, stamp, "nonce-1"); err != nil {
		t.Fatalf("ApplyNodeAuthentication() error = %v", err)
	}
	if req.Header.Get(HeaderNodeUUID) != "node-1" {
		t.Fatalf("node uuid header = %q", req.Header.Get(HeaderNodeUUID))
	}
	if req.Header.Get(HeaderNodeSecret) != "secret" {
		t.Fatalf("node secret header = %q", req.Header.Get(HeaderNodeSecret))
	}
	if req.Header.Get(HeaderAgentSignatureVersion) != SignatureVersion {
		t.Fatalf("signature version = %q", req.Header.Get(HeaderAgentSignatureVersion))
	}
	if req.Header.Get(HeaderAgentSignature) == "" {
		t.Fatal("signature header is empty")
	}
}
