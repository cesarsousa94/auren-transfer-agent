# Storage Adapter Development Notes

Version: v0.1.33

EPIC 8 adds foundation storage adapter contracts under:

```text
internal/storage
```

Current contracts:

```text
storage.Adapter
storage.UploadRequest
storage.UploadResult
storage.NewLocalAdapter
storage.NewAurenStorageAdapter
storage.NormalizeObjectPath
storage.ValidateUploadRequest
```

## Local adapter

The local adapter wraps the local uploader and stores objects below `storage.local_path`. Object paths are normalized with slash semantics and must remain relative.

## Auren Storage adapter

The Auren Storage adapter is an HTTP adapter foundation for Auren Storage. It streams a source file with `PUT` to:

```text
/api/v1/buckets/{bucket}/objects?path={object_path}
```

It sets mechanical headers such as bucket, object path and region, and optionally sends an API key through the configured token header. It does not implement billing, tenant policy, Media Hub authorization decisions or remote provisioning.

The adapter is considered configured when both `storage.endpoint` and `storage.bucket` are present.
