# Identity Development Notes

Auren Transfer Agent v0.1.20 completes EPIC 4 with two subphases delivered together: 4.4 — Fingerprint and 4.5 — Identity API.

The identity foundation now covers UUID generation, durable local Agent ID storage, hostname diagnostics, deterministic fingerprinting and a canonical identity API route contract. The Agent remains stateless for jobs, media, queue state and business decisions; only the local `agent_id` is persisted.

## Package

The identity foundation lives at:

```text
internal/identity
```

Current public surface:

```go
identity.NewUUID() (string, error)
identity.ValidateUUID(value string) error
identity.NormalizeUUID(value string) (string, error)
identity.IsUUID(value string) bool

identity.DefaultStorePath(dataDir string) string
identity.NewFileStore(path string, options ...identity.StoreOption) identity.FileStore
identity.FileStore.Ensure() (identity.StoreResult, error)
identity.FileStore.Load() (identity.Record, error)
identity.FileStore.Save(record identity.Record) error
identity.NewRecord(agentID string, timestamp time.Time) (identity.Record, error)
identity.ValidateRecord(record identity.Record) error

identity.ResolveHostname() identity.HostInfo
identity.ResolveHostnameWith(provider identity.HostnameProvider) identity.HostInfo
identity.NormalizeHostname(value string) (string, error)
identity.ValidateHostname(value string) error
identity.IsHostname(value string) bool

identity.NewFingerprint(agentID string, hostname string) (string, error)
identity.ValidateFingerprint(value string) error
identity.IsFingerprint(value string) bool
identity.NewSnapshot(result identity.StoreResult, host identity.HostInfo) (identity.Snapshot, error)
```

## UUID contract

`identity.NewUUID` generates a cryptographically random RFC 4122 version 4 UUID using Go's `crypto/rand` package.

Canonical UUIDs use lowercase textual form:

```text
xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
```

Where `y` is one of:

```text
8, 9, a, b
```

## Local storage contract

The default store path is derived from `runtime.data_dir`:

```text
<runtime.data_dir>/identity/agent.json
```

With built-in defaults, the path resolves to:

```text
data/identity/agent.json
```

The stored JSON document has this shape:

```json
{
  "schema_version": 1,
  "agent_id": "123e4567-e89b-42d3-a456-426614174000",
  "created_at": "2026-07-09T03:00:00Z",
  "updated_at": "2026-07-09T03:00:00Z"
}
```

Storage rules:

- directory permissions are private by default;
- file permissions are private by default;
- writes are performed through a temporary file followed by rename;
- malformed or invalid existing identity files are rejected instead of silently replaced;
- timestamps use RFC3339/RFC3339Nano-compatible UTC strings;
- `agent_id` is normalized and validated before being saved.

## Hostname contract

`identity.ResolveHostname` reads the local operating-system host name with `os.Hostname`, normalizes it and returns an `identity.HostInfo` payload:

```go
type HostInfo struct {
    Raw        string `json:"raw"`
    Normalized string `json:"normalized"`
    Source     string `json:"source"`
}
```

Normalization rules:

- trim surrounding whitespace;
- lowercase the value;
- remove one trailing dot;
- require DNS-style labels separated by dots;
- allow only `a-z`, `0-9` and `-` inside labels;
- reject empty labels;
- reject labels that start or end with `-`;
- enforce 63-character label and 253-character full hostname limits.

If the operating-system hostname cannot be read or cannot be normalized, the resolver returns:

```text
hostname=unknown-host source=fallback
```

## Fingerprint contract

`identity.NewFingerprint` creates a deterministic SHA-256 fingerprint from canonical local identity material:

```text
auren-transfer-agent:identity:v1:<agent_id>:<hostname>
```

Rules:

- `agent_id` is normalized and validated as UUID v4 before hashing;
- hostname is normalized and validated before hashing;
- the algorithm is `sha256`;
- output is 64 lowercase hexadecimal characters;
- fingerprint changes if the durable Agent ID or normalized hostname changes.

The fingerprint is technical identity metadata. It is not an authorization credential and must not be used as a secret.

## Snapshot contract

`identity.NewSnapshot` combines durable identity storage and host diagnostics into a stable local payload:

```go
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
```

This snapshot is used by bootstrap logs and by the foundation identity API route.

## Identity API contract

EPIC 4.5 adds the canonical route contract:

```text
GET /identity
```

The handler returns HTTP 200 and JSON containing runtime metadata plus identity snapshot fields:

```json
{
  "status": "ok",
  "name": "auren-transfer-agent",
  "version": "v0.1.20",
  "runtime_status": "foundation/identity-api",
  "router": "chi",
  "agent_id": "123e4567-e89b-42d3-a456-426614174000",
  "fingerprint": "...",
  "fingerprint_algorithm": "sha256",
  "hostname": "agent-node-01",
  "hostname_source": "os",
  "persistence": "persistent",
  "store_source": "loaded",
  "store_path": "data/identity/agent.json"
}
```

The route is a contract only in v0.1.20. The bootstrap builds the router with `/identity`, but still does not call `ListenAndServe`.

## Bootstrap behavior

At startup, the bootstrap creates a file store from `runtime.data_dir`, resolves host diagnostics and then builds the snapshot:

```go
identityStore := identity.NewFileStore(identity.DefaultStorePath(cfg.Runtime.DataDir))
identityResult, err := identityStore.Ensure()
hostInfo := identity.ResolveHostname()
identitySnapshot, err := identity.NewSnapshot(identityResult, hostInfo)
```

The startup log includes:

```text
agent_id
fingerprint
hostname
hostname_source
```

The default startup summary prints:

```text
identity: agent_id=... fingerprint=... algorithm=sha256 persistence=persistent source=created path=data/identity/agent.json
host: hostname=... source=os raw="..."
```

## Persistence boundary

v0.1.20 persists only the local technical Agent ID. It does not persist jobs, transfer progress, queue state, media data, business decisions, credentials, hostname, fingerprint or remote registry state.

EPIC 4 — Identity is complete. Worker engine features begin in EPIC 5.
