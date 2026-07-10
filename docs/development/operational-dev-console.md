# Operational Dev Console

The v1.13.1 local console turns the previous request log into a practical operational dashboard for development.

## Pages

- `/_auren/dev/metrics` — operational overview with active jobs, simultaneous downloads/uploads, queue/claim status, storage mode, gateway status and grouped function/endpoint/event tables.
- `/_auren/dev/requests` — detailed request/activity timeline with filters by direction, kind, function and text.
- `/_auren/dev/settings` — browser-side panel preferences such as refresh interval, max rows and noisy-call hiding.

## JSON endpoints

- `/_auren/dev/api/snapshot`
- `/_auren/dev/api/overview`
- `/_auren/dev/api/requests`
- `/_auren/dev/api/config`

## Noise control

Heartbeat, metrics, events, transfer control polling and Dev Console self-requests can be hidden in the UI without disabling capture. Use the checkbox in the toolbar. The raw JSON endpoints still expose the retained records.

## Active transfer visibility

The transfer tracker now records active job details:

- job UUID;
- operation;
- current stage;
- source URL, sanitized;
- destination driver;
- object path;
- current bytes;
- total bytes;
- percent;
- speed.

This makes it easier to confirm whether a Media Hub download request became an actual Agent-side download/upload.

## Security note

This console is intended for development, VPN, SSH tunnel or private AWS network use. Do not expose it publicly without a reverse proxy/authentication layer.
