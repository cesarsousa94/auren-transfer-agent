# Direct S3 Upload Adapter — v1.13.1

Auren Transfer Agent v1.13.1 adds a direct S3 adapter so the Agent can upload completed downloads straight to AWS S3 or an S3-compatible service without requiring Auren Storage in the test path.

## Static config

```yaml
storage:
  driver: s3
  endpoint: https://s3.sa-east-1.amazonaws.com
  bucket: my-transfer-agent-bucket
  region: sa-east-1
  access_key_id: AKIA...
  secret_access_key: ...
  session_token: ""
  s3_force_path_style: false
```

For MinIO or local S3-compatible labs:

```yaml
storage:
  driver: s3
  endpoint: http://127.0.0.1:9000
  bucket: auren-agent-lab
  region: us-east-1
  access_key_id: minioadmin
  secret_access_key: minioadmin
  s3_force_path_style: true
```

## Per-job destination

Media Hub can select S3 per job by sending:

```json
{
  "destination": {
    "driver": "s3",
    "endpoint": "https://s3.sa-east-1.amazonaws.com",
    "bucket": "my-transfer-agent-bucket",
    "region": "sa-east-1",
    "access_key_id": "...",
    "secret_access_key": "...",
    "session_token": "",
    "object_path": "media-hub/org/originals/asset/original.mp4",
    "visibility": "private",
    "mime_type": "video/mp4",
    "metadata": {
      "source": "auren-transfer-agent"
    }
  }
}
```

Prefer scoped/temporary credentials for production. Static keys are acceptable only for controlled development.

## Implementation notes

- Uses AWS Signature Version 4.
- Uses `PUT Object`.
- Propagates SHA-256 through `X-Amz-Content-Sha256`.
- Supports virtual-hosted and path-style URLs.
- Returns bucket, object path, location, ETag-like object UUID, bytes and checksum in the completion callback.

Multipart S3 upload is not included in v1.13.1. For very large S3 uploads, increase infrastructure timeouts or keep using the Auren Storage multipart adapter until the S3 multipart adapter is added.
