# Exports

Damask supports three export mechanisms. Two are ephemeral (stack ZIP and activity CSV, triggered on demand with no stored state). The third is **automated export configs**: persistent records that push a project's assets to an external destination, either manually or on a schedule.

## Ephemeral exports

**Stack export** (`POST /api/v1/stack/export`) streams a ZIP of the requested asset files directly in the HTTP response. Nothing is stored server-side.

**Activity export** (`GET /api/v1/activity/export`) generates a CSV of up to 10 000 workspace audit events filtered by optional `since`/`until` date parameters. Again, nothing is stored.

## Automated export configs

Export configs live in Settings → Exports (owner role required to create or modify them). Each config targets a single project and a single destination. When a run is triggered, Damask:

1. Loads all assets in the project (filtered by the `versions` setting)
2. Streams each file to the destination, deduplicating against the previous run via a sidecar manifest
3. Records a run entry with counters (assets exported, skipped, bytes written) and final status

Run statuses: `pending` → `running` → `done` | `failed` | `partial`. A `partial` result means at least one asset was exported and at least one was skipped.

### Scheduling

Two schedule modes are available:

| Mode          | Behaviour                                                                                                                             |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `manual`      | Only runs when a user clicks **Trigger** in the UI or via the API                                                                     |
| `after_quiet` | Automatically runs after the project has had no asset changes for `quiet_minutes` minutes. Valid range: 1–10 080 (1 minute to 1 week) |

The quiet-period check runs on a background scheduler. If multiple asset saves land in quick succession only one export run is enqueued once the quiet window expires.

### Credential security

All passwords and private keys in destination configs are encrypted at rest with AES-256-GCM using the server's `APP_SECRET`. They are never returned in API responses after the initial write.

## SFTP destination

Exports are written as a single ZIP file plus a JSON sidecar manifest to the configured `remote_path` on the SFTP server.

| Field               | Notes                                                                                                                           |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `host`              | Hostname or IP of the SFTP server                                                                                               |
| `port`              | Defaults to `22`                                                                                                                |
| `username`          | SSH username                                                                                                                    |
| `password`          | Used when no private key is provided                                                                                            |
| `private_key`       | PEM-encoded key (recommended over password)                                                                                     |
| `remote_path`       | Remote directory; must be writable by the SSH user                                                                              |
| `insecure_host_key` | Skips SSH host-key verification. Only acceptable on isolated private networks; never use in production over the public internet |

Private key auth is preferred. If both `password` and `private_key` are set, the private key takes precedence.

## Google Drive destination

Requires an active Google OAuth connection. Connect an account at **Settings → Integrations → Google Drive** before creating an export config that targets Drive.

| Field           | Notes                                                                 |
| --------------- | --------------------------------------------------------------------- |
| `connection_id` | ID of the Google OAuth connection (selected from the integrations UI) |
| `folder_id`     | ID of the target Drive folder (visible in the folder URL)             |
| `folder_name`   | Display name, stored for reference only; not used during export       |

The export writes a ZIP file and a sidecar manifest (`<project-slug>__manifest.json`) into the target folder. On subsequent runs only changed or new files are re-uploaded; unchanged files (same content hash) are skipped.

> The Google OAuth token is refreshed automatically. If the connection is revoked in Google's security settings, the next run will fail with an auth error, reconnect the integration and re-enable the config.
