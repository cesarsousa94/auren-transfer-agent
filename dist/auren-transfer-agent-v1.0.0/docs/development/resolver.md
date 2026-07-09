# Resolver Development Notes

The Resolver Engine is a mechanical URL-resolution layer. It must never decide plan policy, customer access, billing, entitlement, storage routing or provider selection. Those decisions remain in Auren Media Hub.

## v0.1.31 scope

This delivery continues EPIC 7 with three subphases:

- 7.9 — Google Drive Resolver;
- 7.10 — MEGA Resolver;
- 7.11 — OneDrive Resolver.

## Contracts

Current resolver contracts:

```text
resolver.Resolver
resolver.Registry
resolver.Request
resolver.Result
resolver.NewRegistry
resolver.Registry.Resolve
resolver.NewHTTPResolver
resolver.NewXtreamResolver
resolver.NewShuiResolver
resolver.NewRedirectResolver
resolver.NewCloudflareResolver
resolver.NewM3U8Resolver
resolver.NewHLSResolver
resolver.NewGoogleDriveResolver
resolver.NewMEGAResolver
resolver.NewOneDriveResolver
resolver.ParseM3U8Manifest
```

## Resolver order

Bootstrap registers semantic resolvers before the generic HTTP resolver:

```text
xtream -> shui -> cloudflare -> hls -> m3u8 -> google_drive -> mega -> onedrive -> redirect -> http
```

This allows provider-specific, manifest-specific and cloud-sharing URLs to be classified before generic HTTP fallback. Xtream, Shui/XUI, Google Drive, MEGA and OneDrive URLs are classified without network calls. HLS/M3U8 resolvers fetch only bounded manifests. Redirect and HTTP resolvers remain generic HTTP fallbacks.

## HTTP resolver

The HTTP resolver uses the existing Download HTTP Client. It performs metadata resolution with `HEAD` by default and follows the configured redirect policy. If a server rejects `HEAD` with `405 Method Not Allowed`, it retries with `GET` and closes the body without buffering media content.

## Xtream resolver

The Xtream resolver recognizes direct stream paths such as:

```text
/live/{username}/{password}/{item}.{extension}
/movie/{username}/{password}/{item}.{extension}
/series/{username}/{password}/{item}.{extension}
```

It also recognizes `player_api.php` URLs that include `username` and `password`. Credentials are not exposed in resolver metadata; secret values are masked or represented by presence flags.

## Shui resolver

The Shui resolver recognizes Shui/XUI Admin API-style URLs that include an access-code path and/or query parameters such as `api_key` and `action`.

API keys are not exposed in clear text in resolver metadata.


## Redirect resolver

The redirect resolver performs metadata-only HTTP resolution and reports the final URL, redirect count and first/last redirect targets. It uses the same configured HTTP client and redirect policy as the Download Engine.

## Cloudflare resolver

The Cloudflare resolver is an opt-in classifier selected through request metadata such as `resolver=cloudflare` or `cloudflare=true`. It detects Cloudflare response headers and likely challenge statuses, but it never solves, bypasses or automates challenge circumvention.

## M3U8 resolver

The M3U8 resolver fetches a bounded manifest sample and extracts technical metadata such as segment count, variant count, keys, target duration, media sequence and first URI. It does not download media segments.

## HLS resolver

The HLS resolver uses the M3U8 parser to classify master and media playlists. It reports playlist kind, variants, segments and encryption presence. It is still a mechanical resolver and does not decide playback policy or provider access.

## Google Drive resolver

The Google Drive resolver recognizes `drive.google.com` and related Google Drive sharing forms such as `/file/d/{id}/view`, `/drive/folders/{id}` and `uc?id={id}`. For file IDs, it can derive the mechanical `uc?export=download&id=...` URL. Folder links are classified as folders and are not converted into a file download URL.

## MEGA resolver

The MEGA resolver recognizes modern `/file/{handle}#{key}` and `/folder/{handle}#{key}` links as well as legacy `#!handle!key` and `#F!handle!key` fragments. It reports node type, handle and whether a key is present. Keys are masked in metadata. It does not call MEGA APIs or decrypt metadata.

## OneDrive resolver

The OneDrive resolver recognizes `onedrive.live.com`, `1drv.ms` and SharePoint sharing URLs. It extracts `resid`, `id`, `cid`, `authkey`, share path and provider classification. When a `resid` is present, it can derive the mechanical OneDrive download endpoint. Auth keys are masked in metadata.
