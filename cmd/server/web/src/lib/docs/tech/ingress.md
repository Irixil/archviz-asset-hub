# Ingress sources

Ingress sources let Damask automatically pull assets from external systems on a schedule.

## Overview

Configure ingress sources at **Settings → Ingress**. Each source has:

- A **type** (built-in email address, IMAP, SFTP, S3, WebDAV, Google Drive, or Canva)
- A **poll interval** (5 min, 15 min, 30 min, 1 h, or 6 h, default 15 min)
- **Rules**, filter and routing logic controlling which project/folder files land in
- A **log**, every ingest attempt is recorded with status and error details

![Choosing an ingress source type](/docs/screenshot_ingress_choose_source.png)

**Deduplication:** before importing, Damask checks a content hash of each file. If the same file has been seen before, it is silently skipped, you will never get duplicates from a source that delivers the same file twice.

**Credentials security:** all passwords, API keys, and private keys are encrypted at rest with AES-256-GCM. They are never returned in API responses after the initial save.

## Built-in email address

The easiest way to get files into Damask. Every workspace has a unique ingest email address, find yours at **Settings → Ingress → Email address**.

![Built-in ingest email address](/docs/screenshot_ingress_email_address.png)

Email any file as an attachment to that address. Attachments appear in your target project within seconds. No extra configuration needed beyond choosing the default destination folder.

Share this address with clients or collaborators. "Send to Damask" becomes a one-tap action from any email app.

> This requires the built-in SMTP server to be reachable. See the [DNS setup in the Installation guide](installation#email-ingress-dns-setup).

## IMAP mailbox

Polls any IMAP mailbox for new messages and imports their attachments. Useful for a dedicated client delivery inbox or a shared team address.

| Field        | Description                                                                 |
| ------------ | --------------------------------------------------------------------------- |
| Host         | IMAP server hostname (e.g. `imap.gmail.com`)                                |
| Port         | Usually `993` (TLS)                                                         |
| Username     | Your email address                                                          |
| Password     | Your password or app-specific password                                      |
| Mailbox      | Folder to watch (default: `INBOX`)                                          |
| After import | What to do with processed messages: mark as read, move to folder, or delete |

**Gmail / Google Workspace:** use an App Password, not your main account password. Generate one in Google Account → Security → App passwords.

## SFTP

Polls a remote SFTP directory for new files.

| Field        | Description                                                   |
| ------------ | ------------------------------------------------------------- |
| Host         | Server hostname or IP                                         |
| Port         | Usually `22`                                                  |
| Username     | SSH username                                                  |
| Auth method  | Password or SSH private key                                   |
| Private key  | Paste the PEM-encoded private key directly (stored encrypted) |
| Remote path  | Directory to watch (e.g. `/uploads/incoming`)                 |
| After import | Leave in place, move to a done folder, or delete              |

Private key authentication is recommended over password auth.

## S3

Polls an S3 bucket prefix for new objects.

**Supported providers:** AWS S3, Cloudflare R2, Backblaze B2, MinIO, Wasabi, and any S3-compatible API.

| Field             | Description                                                       |
| ----------------- | ----------------------------------------------------------------- |
| Endpoint          | Leave blank for AWS S3; enter the custom endpoint for R2/MinIO/B2 |
| Region            | AWS region (e.g. `us-east-1`)                                     |
| Bucket            | The bucket name                                                   |
| Prefix            | Only watch files under this key prefix (e.g. `incoming/`)         |
| Access Key ID     | Your access key                                                   |
| Secret Access Key | Your secret key                                                   |
| After import      | Leave, move to another prefix, or delete                          |

## WebDAV / Nextcloud

Connects to any WebDAV collection, including Nextcloud, ownCloud, or any WebDAV-compatible server.

| Field        | Description                                                                                      |
| ------------ | ------------------------------------------------------------------------------------------------ |
| URL          | Full URL of the collection (e.g. `https://cloud.example.com/remote.php/dav/files/user/Uploads/`) |
| Username     | Your account username                                                                            |
| Password     | Your password or app password                                                                    |
| After import | Leave, move to another path, or delete                                                           |

**Nextcloud:** generate an App Password at Nextcloud → Settings → Security → Devices & sessions. The DAV base URL format is `{nextcloud_url}/remote.php/dav/files/{username}/`.

## Google Drive

Connect your Google account at **Settings → Integrations → Google Drive**, then create an ingress source pointing at a specific folder ID.

## Canva

Connect your Canva account at **Settings → Integrations → Canva**. Once linked, create an ingress source to ingest exports from your Canva designs.

## Ingress rules

![Ingress rules configuration](/docs/screenshot_ingress_email_rules.png)

Rules run in order on each incoming file. The first matching rule wins. Files that match no rule land in the source's default destination.

### Rule conditions

| Field        | Operators                                        |
| ------------ | ------------------------------------------------ |
| `mime_type`  | `equals`, `contains`, `starts_with`, `ends_with` |
| `filename`   | `equals`, `contains`, `starts_with`, `ends_with` |
| `sender`     | `equals`, `contains`, `starts_with`, `ends_with` |
| `subject`    | `equals`, `contains`, `starts_with`, `ends_with` |
| `size_bytes` | `gt`, `lt`                                       |

### Rule actions

| Action            | Effect                                |
| ----------------- | ------------------------------------- |
| `allow`           | Import the file (default)             |
| `deny`            | Skip this file                        |
| `route to folder` | Import and place in a specific folder |

### Example rules

- Route all PDFs to the `Briefs` folder: `mime_type starts_with application/pdf → route to folder: Briefs`
- Skip files over 500 MB: `size_bytes gt 524288000 → deny`
- Only import from one client: add `sender equals client@brand.com → allow`, then a catch-all `deny` rule below it

Drag rules to reorder them. Add a catch-all `deny` at the bottom to reject everything that doesn't match an earlier `allow`.

## Ingestion log

Every import attempt is recorded. Viewable from each source's detail page.

| Status     | Meaning                                          |
| ---------- | ------------------------------------------------ |
| `imported` | File downloaded and asset created successfully   |
| `skipped`  | Matched a `deny` rule, or was a duplicate        |
| `failed`   | Download or processing error (see error message) |
| `pending`  | Queued, not yet processed                        |

Failed items can be retried individually from the log.

## Error handling and auto-disable

When a poll fails, Damask records the error and increments a consecutive failure counter on the source.

- After each failure, workspace owners receive an email with the error details.
- After **more than 5 consecutive failures**, the source is automatically excluded from the polling schedule and a "source disabled" email is sent.
- A **successful poll resets the counter to zero** and normal polling resumes.
- **Editing the source** (even without changing credentials) also resets the counter, use this to manually re-enable a disabled source after fixing the underlying issue.

## Manual trigger

Click **Poll now** on any source to trigger an immediate ingest cycle outside the schedule.
