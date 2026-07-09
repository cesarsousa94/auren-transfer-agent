// Package security contains foundation security primitives for the Agent.
package security

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	JWTName       = "jwt"
	APIKeysName   = "api_keys"
	MTLSName      = "mtls"
	RBACName      = "rbac"
	RateLimitName = "rate_limit"
	SecretsName   = "secrets"

	PermissionAgentRead = "agent:read"
	PermissionJobRead   = "job:read"
	PermissionJobWrite  = "job:write"
	PermissionAdmin     = "admin:*"
	RoleAdmin           = "admin"
	RoleWorker          = "worker"
	RoleObserver        = "observer"
	RedactedPlaceholder = "[REDACTED]"
)

var errInvalidToken = errors.New("invalid jwt token")

type JWTOptions struct {
	Enabled  bool
	Secret   string
	Issuer   string
	Audience string
	TTL      time.Duration
}
type Claims struct {
	Subject   string   `json:"sub"`
	Roles     []string `json:"roles,omitempty"`
	Issuer    string   `json:"iss,omitempty"`
	Audience  string   `json:"aud,omitempty"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
}
type JWTService struct {
	enabled  bool
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
}

func NewJWTService(options JWTOptions) (JWTService, error) {
	ttl := options.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	secret := strings.TrimSpace(options.Secret)
	if options.Enabled && len(secret) < 16 {
		return JWTService{}, fmt.Errorf("jwt secret must contain at least 16 characters when enabled")
	}
	return JWTService{enabled: options.Enabled, secret: []byte(secret), issuer: strings.TrimSpace(options.Issuer), audience: strings.TrimSpace(options.Audience), ttl: ttl}, nil
}
func (service JWTService) Enabled() bool      { return service.enabled }
func (service JWTService) TTL() time.Duration { return service.ttl }
func (service JWTService) Sign(subject string, roles []string, now time.Time) (string, error) {
	if len(service.secret) == 0 {
		return "", fmt.Errorf("jwt secret is not configured")
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", fmt.Errorf("subject is required")
	}
	claims := Claims{Subject: subject, Roles: normalizeList(roles), Issuer: service.issuer, Audience: service.audience, IssuedAt: now.UTC().Unix(), ExpiresAt: now.UTC().Add(service.ttl).Unix()}
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb)
	sig := signHS256([]byte(input), service.secret)
	return input + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
func (service JWTService) Validate(token string, now time.Time) (Claims, error) {
	token = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(token), "Bearer "))
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(service.secret) == 0 {
		return Claims{}, errInvalidToken
	}
	expected := signHS256([]byte(parts[0]+"."+parts[1]), service.secret)
	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(expected, actual) != 1 {
		return Claims{}, errInvalidToken
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, errInvalidToken
	}
	var header map[string]string
	if err := json.Unmarshal(hb, &header); err != nil || header["alg"] != "HS256" {
		return Claims{}, errInvalidToken
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(pb, &claims); err != nil {
		return Claims{}, errInvalidToken
	}
	if strings.TrimSpace(claims.Subject) == "" || claims.ExpiresAt <= now.UTC().Unix() {
		return Claims{}, errInvalidToken
	}
	if service.issuer != "" && claims.Issuer != service.issuer {
		return Claims{}, errInvalidToken
	}
	if service.audience != "" && claims.Audience != service.audience {
		return Claims{}, errInvalidToken
	}
	claims.Roles = normalizeList(claims.Roles)
	return claims, nil
}
func signHS256(payload []byte, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}
func HashAPIKey(value string) string {
	sum := sha256.Sum256([]byte("auren-transfer-agent-api-key:" + strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

type APIKeyVerifier struct {
	required bool
	rawKey   string
	hash     string
	header   string
}
type APIKeyOptions struct {
	Required bool
	RawKey   string
	Hash     string
	Header   string
}

func NewAPIKeyVerifier(options APIKeyOptions) (APIKeyVerifier, error) {
	header := strings.TrimSpace(options.Header)
	if header == "" {
		header = "Authorization"
	}
	if strings.ContainsAny(header, "\r\n") {
		return APIKeyVerifier{}, fmt.Errorf("api key header cannot contain newline characters")
	}
	raw := strings.TrimSpace(options.RawKey)
	hash := strings.TrimSpace(options.Hash)
	if options.Required && raw == "" && hash == "" {
		return APIKeyVerifier{}, fmt.Errorf("api key or api key hash is required")
	}
	return APIKeyVerifier{required: options.Required, rawKey: raw, hash: hash, header: header}, nil
}
func (verifier APIKeyVerifier) Required() bool { return verifier.required }
func (verifier APIKeyVerifier) Header() string { return verifier.header }
func (verifier APIKeyVerifier) Mode() string {
	if verifier.required {
		return APIKeysName
	}
	return "disabled"
}
func (verifier APIKeyVerifier) Verify(value string) bool {
	if !verifier.required {
		return true
	}
	candidate := normalizeAPIKeyValue(value)
	if candidate == "" {
		return false
	}
	if verifier.rawKey != "" && subtle.ConstantTimeCompare([]byte(candidate), []byte(verifier.rawKey)) == 1 {
		return true
	}
	if verifier.hash != "" && subtle.ConstantTimeCompare([]byte(HashAPIKey(candidate)), []byte(verifier.hash)) == 1 {
		return true
	}
	return false
}
func (verifier APIKeyVerifier) VerifyRequest(request *http.Request) bool {
	if request == nil {
		return !verifier.required
	}
	return verifier.Verify(request.Header.Get(verifier.header))
}
func normalizeAPIKeyValue(value string) string {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"Bearer ", "ApiKey "} {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
	}
	return value
}

type MTLSPolicy struct {
	enabled       bool
	requiredCN    string
	minTLSVersion uint16
}
type MTLSOptions struct {
	Enabled    bool
	RequiredCN string
	MinVersion uint16
}

func NewMTLSPolicy(options MTLSOptions) MTLSPolicy {
	min := options.MinVersion
	if min == 0 {
		min = tls.VersionTLS12
	}
	return MTLSPolicy{enabled: options.Enabled, requiredCN: strings.TrimSpace(options.RequiredCN), minTLSVersion: min}
}
func (policy MTLSPolicy) Enabled() bool      { return policy.enabled }
func (policy MTLSPolicy) MinVersion() uint16 { return policy.minTLSVersion }
func (policy MTLSPolicy) ValidateState(state *tls.ConnectionState) bool {
	if !policy.enabled {
		return true
	}
	if state == nil || state.Version < policy.minTLSVersion || len(state.PeerCertificates) == 0 || len(state.VerifiedChains) == 0 {
		return false
	}
	if policy.requiredCN == "" {
		return true
	}
	return state.PeerCertificates[0].Subject.CommonName == policy.requiredCN
}
func VerifiedTLSState(cert *x509.Certificate, version uint16) *tls.ConnectionState {
	if cert == nil {
		return nil
	}
	return &tls.ConnectionState{Version: version, PeerCertificates: []*x509.Certificate{cert}, VerifiedChains: [][]*x509.Certificate{{cert}}}
}

type Policy struct {
	permissions map[string]map[string]struct{}
}

func NewDefaultPolicy() Policy {
	return NewPolicy(map[string][]string{RoleAdmin: {PermissionAdmin, PermissionAgentRead, PermissionJobRead, PermissionJobWrite}, RoleWorker: {PermissionAgentRead, PermissionJobRead, PermissionJobWrite}, RoleObserver: {PermissionAgentRead, PermissionJobRead}})
}
func NewPolicy(input map[string][]string) Policy {
	output := map[string]map[string]struct{}{}
	for role, permissions := range input {
		r := strings.ToLower(strings.TrimSpace(role))
		if r == "" {
			continue
		}
		output[r] = map[string]struct{}{}
		for _, permission := range normalizeList(permissions) {
			output[r][permission] = struct{}{}
		}
	}
	return Policy{permissions: output}
}
func (policy Policy) Allows(roles []string, permission string) bool {
	permission = strings.ToLower(strings.TrimSpace(permission))
	if permission == "" {
		return false
	}
	for _, role := range normalizeList(roles) {
		allowed := policy.permissions[role]
		if _, ok := allowed[PermissionAdmin]; ok {
			return true
		}
		if _, ok := allowed[permission]; ok {
			return true
		}
	}
	return false
}
func (policy Policy) Roles() []string {
	roles := make([]string, 0, len(policy.permissions))
	for role := range policy.permissions {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

type RateLimiter struct {
	limit   int
	window  time.Duration
	mu      sync.Mutex
	entries map[string]rateEntry
}
type rateEntry struct {
	windowStart time.Time
	count       int
}

func NewRateLimiter(limit int, window time.Duration) (*RateLimiter, error) {
	if limit < 0 {
		return nil, fmt.Errorf("rate limit must be zero or greater")
	}
	if window <= 0 {
		return nil, fmt.Errorf("rate limit window must be greater than zero")
	}
	return &RateLimiter{limit: limit, window: window, entries: map[string]rateEntry{}}, nil
}
func (limiter *RateLimiter) Limit() int {
	if limiter == nil {
		return 0
	}
	return limiter.limit
}
func (limiter *RateLimiter) Window() time.Duration {
	if limiter == nil {
		return 0
	}
	return limiter.window
}
func (limiter *RateLimiter) Allow(key string, now time.Time) bool {
	if limiter == nil || limiter.limit == 0 {
		return true
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "anonymous"
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	entry := limiter.entries[key]
	if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= limiter.window {
		limiter.entries[key] = rateEntry{windowStart: now, count: 1}
		return true
	}
	if entry.count >= limiter.limit {
		return false
	}
	entry.count++
	limiter.entries[key] = entry
	return true
}

type Secrets struct{ values map[string]string }

func NewSecrets(values map[string]string) Secrets {
	output := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			output[key] = value
		}
	}
	return Secrets{values: output}
}
func LoadSecretsFile(path string) (Secrets, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return NewSecrets(nil), nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return Secrets{}, err
	}
	data := map[string]string{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return Secrets{}, err
	}
	return NewSecrets(data), nil
}
func (secrets Secrets) Get(key string) (string, bool) {
	value, ok := secrets.values[strings.TrimSpace(key)]
	return value, ok
}
func (secrets Secrets) Count() int { return len(secrets.values) }
func (secrets Secrets) Names() []string {
	names := make([]string, 0, len(secrets.values))
	for name := range secrets.values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func (secrets Secrets) Redact(input string) string {
	output := input
	values := make([]string, 0, len(secrets.values))
	for _, value := range secrets.values {
		if len(value) >= 4 {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	for _, value := range values {
		output = strings.ReplaceAll(output, value, RedactedPlaceholder)
	}
	return output
}

type contextKey string

const rolesContextKey contextKey = "auren.security.roles"

func ContextWithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, rolesContextKey, normalizeList(roles))
}
func RolesFromContext(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	roles, _ := ctx.Value(rolesContextKey).([]string)
	return normalizeList(roles)
}
func normalizeList(values []string) []string {
	seen := map[string]struct{}{}
	output := []string{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		output = append(output, value)
	}
	sort.Strings(output)
	return output
}
