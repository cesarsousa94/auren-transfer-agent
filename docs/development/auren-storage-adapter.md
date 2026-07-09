# Auren Storage Production Adapter — v1.3.0

Auren Transfer Agent v1.3.0 implements the production Auren Storage v1 adapter used by the Media Hub transfer-agent payload.

The Media Hub remains the source of truth for tenant, bucket, directory, visibility and metadata decisions. The Agent only executes the upload contract it receives.

## Direct object upload

Small and normal files use:

```text
POST /api/v1/buckets/{bucket_uuid}/objects
Content-Type: multipart/form-data
```

The form includes:

```text
file
path
object_path
directory_path
relative_path
visibility
mime_type
checksum_algorithm
checksum_sha256
size
metadata
metadata[key]
```

The adapter accepts both `bucket_uuid` and `bucket` hints, but Media Hub should prefer `bucket_uuid`.

## Multipart upload

When `upload.multipart_enabled=true` and the source file is larger than `upload.part_size`, the adapter uses:

```text
POST /api/v1/buckets/{bucket_uuid}/multipart-uploads
PUT  /api/v1/buckets/{bucket_uuid}/multipart-uploads/{upload_id}/parts/{part_number}
POST /api/v1/buckets/{bucket_uuid}/multipart-uploads/{upload_id}/complete
POST /api/v1/buckets/{bucket_uuid}/multipart-uploads/{upload_id}/abort
```

Each part includes `X-Auren-Part-Number`, `X-Auren-Part-SHA256` and `Content-Range` headers. Completion sends the ordered part list and the final source SHA-256.

## Destination payload

The Media Hub job destination can provide:

```json
{
  "driver": "auren_storage",
  "endpoint": "https://storage.example.com",
  "bucket_uuid": "...",
  "directory_path": "media-hub/org/originals/asset-uuid",
  "relative_path": "original.mp4",
  "visibility": "private",
  "mime_type": "video/mp4",
  "checksum_algorithm": "sha256",
  "token": "scoped-or-signed-token",
  "token_header": "Authorization",
  "metadata": {
    "organization_id": "1",
    "media_asset_id": "123"
  }
}
```

Job-scoped tokens are preferred. `storage.api_key` is only a fallback for controlled environments.

## Completion result

The Agent returns the richest object result it can parse from Auren Storage:

```json
{
  "driver": "auren_storage",
  "bucket_uuid": "...",
  "object_uuid": "...",
  "path": "media-hub/org/originals/asset-uuid/original.mp4",
  "url": "https://...",
  "size": 1073741824,
  "checksum_sha256": "...",
  "visibility": "private",
  "mime_type": "video/mp4",
  "multipart": true
}
```

## Validation

The adapter tests cover direct multipart/form-data upload, production response parsing and multipart initiate/parts/complete lifecycle.
