package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// FingerprintAlgorithm is the canonical hashing algorithm used by the foundation identity layer.
	FingerprintAlgorithm = "sha256"

	// FingerprintVersion is the canonical material version used before hashing.
	FingerprintVersion = 1

	// FingerprintLength is the lowercase hex length of a SHA-256 digest.
	FingerprintLength = 64

	// FingerprintMaterialPrefix keeps Agent fingerprints namespaced and deterministic.
	FingerprintMaterialPrefix = "auren-transfer-agent:identity"
)

// Snapshot is the stable local identity payload used by logs, diagnostics and the identity API.
//
// It intentionally contains only technical Agent identity metadata. It never carries Media Hub
// business rules, transfer job state, credentials or storage objects.
type Snapshot struct {
	AgentID              string `json:"agent_id"`
	Fingerprint          string `json:"fingerprint"`
	FingerprintAlgorithm string `json:"fingerprint_algorithm"`
	Hostname             string `json:"hostname"`
	HostnameSource       string `json:"hostname_source"`
	Persistence          string `json:"persistence"`
	StoreSource          string `json:"store_source"`
	StorePath            string `json:"store_path"`
}

// NewFingerprint creates a deterministic fingerprint from the durable Agent ID and normalized hostname.
func NewFingerprint(agentID string, hostname string) (string, error) {
	normalizedAgentID, err := NormalizeUUID(agentID)
	if err != nil {
		return "", fmt.Errorf("fingerprint agent_id: %w", err)
	}

	normalizedHostname, err := NormalizeHostname(hostname)
	if err != nil {
		return "", fmt.Errorf("fingerprint hostname: %w", err)
	}

	material := fmt.Sprintf("%s:v%d:%s:%s", FingerprintMaterialPrefix, FingerprintVersion, normalizedAgentID, normalizedHostname)
	digest := sha256.Sum256([]byte(material))
	return hex.EncodeToString(digest[:]), nil
}

// ValidateFingerprint validates the canonical lowercase SHA-256 fingerprint shape.
func ValidateFingerprint(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("fingerprint cannot be empty")
	}
	if len(trimmed) != FingerprintLength {
		return fmt.Errorf("fingerprint must be %d lowercase hex characters", FingerprintLength)
	}
	for position, char := range trimmed {
		if !isLowerHex(char) {
			return fmt.Errorf("fingerprint contains invalid character at position %d", position)
		}
	}
	return nil
}

// IsFingerprint reports whether value matches the foundation fingerprint contract.
func IsFingerprint(value string) bool {
	return ValidateFingerprint(value) == nil
}

// NewSnapshot builds the canonical local identity snapshot from durable storage and host diagnostics.
func NewSnapshot(result StoreResult, host HostInfo) (Snapshot, error) {
	if err := ValidateRecord(result.Record); err != nil {
		return Snapshot{}, err
	}
	if err := ValidateHostname(host.Normalized); err != nil {
		return Snapshot{}, fmt.Errorf("identity snapshot hostname: %w", err)
	}
	if strings.TrimSpace(host.Source) == "" {
		return Snapshot{}, fmt.Errorf("identity snapshot hostname_source cannot be empty")
	}

	fingerprint, err := NewFingerprint(result.Record.AgentID, host.Normalized)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		AgentID:              result.Record.AgentID,
		Fingerprint:          fingerprint,
		FingerprintAlgorithm: FingerprintAlgorithm,
		Hostname:             host.Normalized,
		HostnameSource:       host.Source,
		Persistence:          result.Persistence(),
		StoreSource:          result.Source(),
		StorePath:            result.Path,
	}, nil
}

func isLowerHex(char rune) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')
}
