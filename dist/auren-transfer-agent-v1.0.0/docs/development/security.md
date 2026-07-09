# Security Foundation

Version `v0.1.38` completes EPIC 12 security contracts for the foundation line.

The Agent remains business-rule free. These primitives are mechanical building blocks that later communication, cluster and production phases can reuse.

## Included contracts

- JWT: HS256 signing and validation with issuer, audience, TTL and role claims.
- API Keys: raw-key and SHA-256 hash verification with constant-time comparison.
- mTLS: TLS peer-certificate validation contract using verified chains and optional common name matching.
- RBAC: deterministic role/permission policy for `admin`, `worker` and `observer` roles.
- Rate Limit: fixed-window per-key limiter with disabled mode when the configured limit is zero.
- Secrets: local secret lookup, JSON-file loading and deterministic redaction.

## Configuration

Security keys live under the `security` section:

```yaml
security:
  api_key_required: false
  api_key: ""
  api_key_hash: ""
  token_header: Authorization
  allow_insecure_http: true
  jwt_enabled: false
  jwt_secret: ""
  jwt_ttl: 15m
  mtls_enabled: false
  mtls_required_cn: ""
  rbac_enabled: false
  rate_limit_enabled: false
  rate_limit_per_minute: 60
  secrets_provider: env
  secrets_file: ""
```

Environment overrides use the same dotted path converted to `AUREN_*`, for example `AUREN_SECURITY_JWT_ENABLED=true` or `AUREN_SECURITY_RATE_LIMIT_PER_MINUTE=120`.

## Notes

This version does not start a public HTTP listener, issue production JWTs to remote callers, load certificate authorities, or manage a centralized secrets backend. Those integrations remain reserved for later production hardening.
