package mediahub

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	// HeaderNodeUUID is accepted by the Media Hub NodeAgentContractService.
	HeaderNodeUUID = "X-Auren-Node-UUID"

	// HeaderNodeSecret is the non-HMAC fallback credential header.
	HeaderNodeSecret = "X-Auren-Node-Secret"

	// HeaderAgentTimestamp carries the signature timestamp.
	HeaderAgentTimestamp = "X-Auren-Agent-Timestamp"

	// HeaderAgentNonce carries a per-request nonce.
	HeaderAgentNonce = "X-Auren-Agent-Nonce"

	// HeaderAgentSignature carries the HMAC-SHA256 signature.
	HeaderAgentSignature = "X-Auren-Agent-Signature"

	// HeaderAgentSignatureVersion documents the signature canonicalization version.
	HeaderAgentSignatureVersion = "X-Auren-Agent-Signature-Version"

	SignatureVersion = "v1"
)

// SignatureInput contains all canonical HMAC material.
type SignatureInput struct {
	Method    string
	PathQuery string
	Timestamp string
	Nonce     string
	Body      []byte
}

// BodySHA256 returns the lowercase SHA-256 digest used by the canonical string.
func BodySHA256(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

// CanonicalString renders the stable v1 HMAC string.
func CanonicalString(input SignatureInput) string {
	return strings.ToUpper(strings.TrimSpace(input.Method)) + "\n" +
		strings.TrimSpace(input.PathQuery) + "\n" +
		strings.TrimSpace(input.Timestamp) + "\n" +
		strings.TrimSpace(input.Nonce) + "\n" +
		BodySHA256(input.Body)
}

// Sign returns a lowercase hex HMAC-SHA256 signature.
func Sign(secret string, input SignatureInput) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(CanonicalString(input)))
	return hex.EncodeToString(mac.Sum(nil))
}

// ApplyNodeAuthentication attaches Media Hub node credentials and optional HMAC headers.
func ApplyNodeAuthentication(request *http.Request, body []byte, state NodeState, hmacEnabled bool, now time.Time, nonce string) error {
	if request == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if state.Empty() {
		return fmt.Errorf("media hub node credentials are required")
	}
	request.Header.Set(HeaderNodeUUID, strings.TrimSpace(state.NodeUUID))
	request.Header.Set(HeaderNodeSecret, strings.TrimSpace(state.NodeSecret))
	if !hmacEnabled {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if strings.TrimSpace(nonce) == "" {
		generated, err := NewNonce()
		if err != nil {
			return err
		}
		nonce = generated
	}
	timestamp := now.UTC().Format(time.RFC3339)
	pathQuery := request.URL.EscapedPath()
	if request.URL.RawQuery != "" {
		pathQuery += "?" + request.URL.RawQuery
	}
	request.Header.Set(HeaderAgentTimestamp, timestamp)
	request.Header.Set(HeaderAgentNonce, nonce)
	request.Header.Set(HeaderAgentSignatureVersion, SignatureVersion)
	request.Header.Set(HeaderAgentSignature, Sign(state.NodeSecret, SignatureInput{Method: request.Method, PathQuery: pathQuery, Timestamp: timestamp, Nonce: nonce, Body: body}))
	return nil
}

// NewNonce returns a cryptographically-random lowercase hex nonce.
func NewNonce() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate media hub nonce: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
