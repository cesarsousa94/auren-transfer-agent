package security

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJWTServiceSignsAndValidatesHS256Token(t *testing.T) {
	service, err := NewJWTService(JWTOptions{Enabled: true, Secret: "0123456789abcdef", Issuer: "agent", Audience: "hub", TTL: time.Minute})
	if err != nil {
		t.Fatalf("new jwt service: %v", err)
	}
	now := time.Unix(1000, 0).UTC()
	token, err := service.Sign("agent-1", []string{RoleWorker, RoleWorker}, now)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	claims, err := service.Validate("Bearer "+token, now.Add(30*time.Second))
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if claims.Subject != "agent-1" || claims.Issuer != "agent" || claims.Audience != "hub" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != RoleWorker {
		t.Fatalf("unexpected roles: %#v", claims.Roles)
	}
}

func TestJWTServiceRejectsExpiredAndTamperedTokens(t *testing.T) {
	service, err := NewJWTService(JWTOptions{Enabled: true, Secret: "0123456789abcdef", TTL: time.Second})
	if err != nil {
		t.Fatalf("new jwt service: %v", err)
	}
	now := time.Unix(1000, 0).UTC()
	token, err := service.Sign("agent-1", nil, now)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := service.Validate(token, now.Add(2*time.Second)); err == nil {
		t.Fatalf("expected expired token to fail")
	}
	if _, err := service.Validate(token+"x", now); err == nil {
		t.Fatalf("expected tampered token to fail")
	}
}

func TestAPIKeyVerifierAcceptsRawBearerApiKeyAndHash(t *testing.T) {
	verifier, err := NewAPIKeyVerifier(APIKeyOptions{Required: true, RawKey: "secret-key", Header: "X-Agent-Key"})
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}
	for _, value := range []string{"secret-key", "Bearer secret-key", "ApiKey secret-key"} {
		if !verifier.Verify(value) {
			t.Fatalf("expected %q to verify", value)
		}
	}
	hashed, err := NewAPIKeyVerifier(APIKeyOptions{Required: true, Hash: HashAPIKey("secret-key")})
	if err != nil {
		t.Fatalf("new hashed verifier: %v", err)
	}
	if !hashed.Verify("secret-key") || hashed.Verify("wrong") {
		t.Fatalf("unexpected hash verification result")
	}
}

func TestMTLSPolicyValidatesVerifiedPeerCertificate(t *testing.T) {
	cert := &x509.Certificate{}
	cert.Subject.CommonName = "agent-client"
	policy := NewMTLSPolicy(MTLSOptions{Enabled: true, RequiredCN: "agent-client", MinVersion: tls.VersionTLS12})
	if !policy.ValidateState(VerifiedTLSState(cert, tls.VersionTLS12)) {
		t.Fatalf("expected verified peer certificate to pass")
	}
	if policy.ValidateState(VerifiedTLSState(cert, tls.VersionTLS10)) {
		t.Fatalf("expected old TLS version to fail")
	}
	if policy.ValidateState(nil) {
		t.Fatalf("expected nil state to fail")
	}
}

func TestRBACPolicyAllowsExpectedPermissions(t *testing.T) {
	policy := NewDefaultPolicy()
	if !policy.Allows([]string{RoleWorker}, PermissionJobWrite) {
		t.Fatalf("worker should write jobs")
	}
	if policy.Allows([]string{RoleObserver}, PermissionJobWrite) {
		t.Fatalf("observer should not write jobs")
	}
	if !policy.Allows([]string{RoleAdmin}, "anything:anywhere") {
		t.Fatalf("admin wildcard should allow any permission")
	}
	if roles := policy.Roles(); len(roles) != 3 {
		t.Fatalf("unexpected role count: %#v", roles)
	}
}

func TestRateLimiterUsesFixedWindow(t *testing.T) {
	limiter, err := NewRateLimiter(2, time.Minute)
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	now := time.Unix(1000, 0)
	if !limiter.Allow("agent", now) || !limiter.Allow("agent", now.Add(time.Second)) {
		t.Fatalf("expected first two calls to pass")
	}
	if limiter.Allow("agent", now.Add(2*time.Second)) {
		t.Fatalf("expected third call in same window to fail")
	}
	if !limiter.Allow("agent", now.Add(time.Minute)) {
		t.Fatalf("expected next window to pass")
	}
}

func TestSecretsLoadNamesAndRedact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.json")
	if err := os.WriteFile(path, []byte(`{"token":"abcdef","short":"abc"}`), 0o600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}
	secrets, err := LoadSecretsFile(path)
	if err != nil {
		t.Fatalf("load secrets: %v", err)
	}
	if secrets.Count() != 2 {
		t.Fatalf("unexpected count: %d", secrets.Count())
	}
	if value, ok := secrets.Get("token"); !ok || value != "abcdef" {
		t.Fatalf("unexpected secret lookup")
	}
	if got := secrets.Redact("Bearer abcdef and abc"); got != "Bearer [REDACTED] and abc" {
		t.Fatalf("unexpected redaction: %q", got)
	}
}
