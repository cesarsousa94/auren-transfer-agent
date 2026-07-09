# Upload Engine Development Notes

Version: v0.1.33

EPIC 8 is complete in v0.1.33. The upload engine remains a set of mechanical primitives. It does not decide what should be uploaded, who owns it, whether a customer is allowed to upload it, or how Media Hub should represent it.

Current package:

```text
internal/upload
```

Current contracts:

```text
upload.Uploader
upload.Request
upload.Result
upload.NewLocalUploader
upload.LocalUploader.Upload
upload.LocalUploader.MultipartUpload
upload.LocalUploader.ResumeUpload
upload.LocalUploader.ResumeFromLocalState
upload.NewPlan
upload.ParsePartSize
upload.ValidateIntegrity
upload.ValidateResultIntegrity
upload.NewHTTPCallbackSender
```

## Local upload

The local uploader writes files below `storage.local_path`. Destination paths must be relative. Absolute paths and `..` traversal are rejected before writing.

## Multipart upload

Multipart upload is deterministic and local in the foundation line. It splits the source file into parts using `upload.part_size`, copies each part sequentially to the destination and reports part metadata.

## Resume upload

Resume upload inspects the source file size and the current destination file size. If the destination is smaller than the source, the uploader seeks to the destination size and appends the missing bytes. If the destination is already the same size, the result is marked `already_complete` and no bytes are written. If the destination is larger than the source, the operation fails.

## Integrity validation

Integrity validation compares source and destination file size and SHA-256 checksum. The validator returns a structured `IntegrityResult` with source checksum, destination checksum, size match and checksum match flags.

## Callback engine

The callback engine posts a JSON `CallbackPayload` to an absolute HTTP or HTTPS endpoint and requires a 2xx response. It is a transport primitive only; payload meaning and retry orchestration remain outside this package.
