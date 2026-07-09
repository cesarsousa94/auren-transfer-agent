package identity

import (
	"fmt"
	"os"
	"strings"
)

const (
	// UnknownHostname is the stable fallback used when the host name cannot be resolved safely.
	UnknownHostname = "unknown-host"

	// HostnameSourceOS identifies host names resolved from the local operating system.
	HostnameSourceOS = "os"

	// HostnameSourceFallback identifies host names resolved through the controlled fallback.
	HostnameSourceFallback = "fallback"

	// HostnameMaxLength follows the practical DNS host name length boundary.
	HostnameMaxLength = 253

	// HostnameLabelMaxLength follows the DNS label length boundary.
	HostnameLabelMaxLength = 63
)

// HostInfo describes the local host name discovered for the Agent runtime.
type HostInfo struct {
	Raw        string `json:"raw"`
	Normalized string `json:"normalized"`
	Source     string `json:"source"`
}

// HostnameProvider resolves a raw host name.
type HostnameProvider func() (string, error)

// ResolveHostname resolves the local operating-system host name.
func ResolveHostname() HostInfo {
	return ResolveHostnameWith(os.Hostname)
}

// ResolveHostnameWith resolves and normalizes a host name using provider.
//
// The function intentionally returns a controlled fallback instead of failing
// the Agent bootstrap. Hostname is diagnostic identity metadata; it must not
// make the foundation executable unusable on unusual containers or hosts.
func ResolveHostnameWith(provider HostnameProvider) HostInfo {
	if provider == nil {
		return fallbackHostname("")
	}

	raw, err := provider()
	if err != nil {
		return fallbackHostname(raw)
	}

	normalized, err := NormalizeHostname(raw)
	if err != nil {
		return fallbackHostname(raw)
	}

	return HostInfo{Raw: raw, Normalized: normalized, Source: HostnameSourceOS}
}

// NormalizeHostname trims, lowercases and validates a host name.
func NormalizeHostname(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.TrimSuffix(normalized, ".")
	if err := ValidateHostname(normalized); err != nil {
		return "", err
	}
	return normalized, nil
}

// IsHostname reports whether value is accepted by the foundation host name contract.
func IsHostname(value string) bool {
	_, err := NormalizeHostname(value)
	return err == nil
}

// ValidateHostname validates the canonical host name shape used in identity diagnostics.
func ValidateHostname(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("hostname cannot be empty")
	}
	if len(trimmed) > HostnameMaxLength {
		return fmt.Errorf("hostname must be at most %d characters", HostnameMaxLength)
	}
	if strings.Contains(trimmed, "..") {
		return fmt.Errorf("hostname cannot contain empty labels")
	}

	labels := strings.Split(trimmed, ".")
	for index, label := range labels {
		if err := validateHostnameLabel(index, label); err != nil {
			return err
		}
	}
	return nil
}

func validateHostnameLabel(index int, label string) error {
	if label == "" {
		return fmt.Errorf("hostname label %d cannot be empty", index)
	}
	if len(label) > HostnameLabelMaxLength {
		return fmt.Errorf("hostname label %d must be at most %d characters", index, HostnameLabelMaxLength)
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return fmt.Errorf("hostname label %d cannot start or end with hyphen", index)
	}
	for position, char := range label {
		if !isHostnameChar(char) {
			return fmt.Errorf("hostname label %d contains invalid character at position %d", index, position)
		}
	}
	return nil
}

func isHostnameChar(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-'
}

func fallbackHostname(raw string) HostInfo {
	return HostInfo{Raw: raw, Normalized: UnknownHostname, Source: HostnameSourceFallback}
}
