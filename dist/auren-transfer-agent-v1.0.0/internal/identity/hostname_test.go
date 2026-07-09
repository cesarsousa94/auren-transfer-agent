package identity

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeHostnameCanonicalizesValue(t *testing.T) {
	got, err := NormalizeHostname("  MEDIA-NODE-01.AUREN.LOCAL.  ")
	if err != nil {
		t.Fatalf("NormalizeHostname returned error: %v", err)
	}
	if got != "media-node-01.auren.local" {
		t.Fatalf("unexpected normalized hostname %q", got)
	}
}

func TestValidateHostnameRejectsInvalidValues(t *testing.T) {
	longLabel := strings.Repeat("a", HostnameLabelMaxLength+1)
	longHost := strings.Repeat("a", HostnameMaxLength+1)
	cases := []string{
		"",
		" ",
		"-node",
		"node-",
		"node..local",
		"node_local",
		"node local",
		longLabel + ".local",
		longHost,
	}

	for _, value := range cases {
		if err := ValidateHostname(value); err == nil {
			t.Fatalf("expected invalid hostname %q to fail", value)
		}
	}
}

func TestResolveHostnameWithProviderUsesOSSource(t *testing.T) {
	info := ResolveHostnameWith(func() (string, error) {
		return "AUREN-AGENT-01", nil
	})

	if info.Raw != "AUREN-AGENT-01" {
		t.Fatalf("expected raw hostname to be preserved, got %q", info.Raw)
	}
	if info.Normalized != "auren-agent-01" {
		t.Fatalf("expected normalized hostname, got %q", info.Normalized)
	}
	if info.Source != HostnameSourceOS {
		t.Fatalf("expected source %q, got %q", HostnameSourceOS, info.Source)
	}
}

func TestResolveHostnameWithProviderFallsBackOnErrorOrInvalidValue(t *testing.T) {
	cases := []HostnameProvider{
		func() (string, error) { return "", errors.New("hostname unavailable") },
		func() (string, error) { return "bad host", nil },
		nil,
	}

	for _, provider := range cases {
		info := ResolveHostnameWith(provider)
		if info.Normalized != UnknownHostname {
			t.Fatalf("expected fallback hostname %q, got %q", UnknownHostname, info.Normalized)
		}
		if info.Source != HostnameSourceFallback {
			t.Fatalf("expected fallback source, got %q", info.Source)
		}
	}
}

func TestIsHostname(t *testing.T) {
	if !IsHostname("node-01.local") {
		t.Fatal("expected valid hostname")
	}
	if IsHostname("node_01") {
		t.Fatal("expected invalid hostname")
	}
}
