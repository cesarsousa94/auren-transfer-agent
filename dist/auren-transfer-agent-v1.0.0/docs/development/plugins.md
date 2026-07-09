# Plugin SDK Development Notes

Version: v0.1.32

The Plugin SDK is the public extension contract for future resolver and uploader extensions.

Current package:

```text
pkg/plugins
```

Current contracts:

```text
plugins.Manifest
plugins.ResolverPlugin
plugins.UploaderPlugin
plugins.ResolveRequest
plugins.ResolveResult
plugins.UploadRequest
plugins.UploadResult
plugins.NormalizeManifest
plugins.ValidateManifest
```

The SDK is intentionally small and dependency-light. It only describes plugin shape and payloads.

It does not yet load external binaries, run sandboxed plugins, manage plugin lifecycle, execute provider APIs or make Media Hub policy decisions.

Supported plugin kinds in this foundation delivery:

```text
resolver
uploader
```

All plugin implementations must remain business-rule free. They may transform or classify technical inputs, but Auren Media Hub owns all decisions about what should be transferred, retried, published or billed.
