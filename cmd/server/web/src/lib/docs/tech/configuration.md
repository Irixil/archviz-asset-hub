# Configuration

Damask is configured via environment variables. All variables are optional unless marked **required**.

## Core

| Variable        | Default                 | Description                                                                                   |
| --------------- | ----------------------- | --------------------------------------------------------------------------------------------- |
| `BASE_URL`      | `http://localhost:5173` | Public URL of your instance (e.g. `https://dam.example.com`). Used in share links and emails. |
| `PORT`          | `8080`                  | HTTP listen port                                                                              |
| `APP_ENV`       | `development`           | Set to `production` to enable secure cookies and stricter settings                            |
| `ENABLE_SIGNUP` | `true`                  | Set to `false` to disable registrations (invitations still work)                              |
| `JWT_SECRET`    | **required**            | HMAC secret for JWT signing, minimum 32 characters. Generate with `openssl rand -hex 32`      |
| `APP_SECRET`    | **required**            | AES-256-GCM key for encrypting ingress credentials at rest, minimum 32 characters             |

## Database & storage

| Variable             | Default       | Description                                        |
| -------------------- | ------------- | -------------------------------------------------- |
| `DB_PATH`            | `./damask.db` | Path to the SQLite database file                   |
| `STORAGE_LOCAL_PATH` | `./storage`   | Directory for local file storage (default backend) |

See the [Storage guide](storage) for S3 and SFTP options.

## Outgoing emails (SMTP)

Required to send invite emails, password reset links, and ingress failure notifications.

| Variable      | Default | Description                                                          |
| ------------- | ------- | -------------------------------------------------------------------- |
| `SMTP_HOST`   | -       | SMTP server hostname                                                 |
| `SMTP_PORT`   | `25`    | SMTP port                                                            |
| `SMTP_USER`   | -       | SMTP username                                                        |
| `SMTP_PASS`   | -       | SMTP password                                                        |
| `SMTP_SENDER` | -       | From address for outgoing mail (e.g. `Damask <noreply@example.com>`) |

## Ingress mail server

Damask includes a built-in SMTP server that receives inbound emails for the [email ingress](ingress) feature.

| Variable    | Default                   | Description                                  |
| ----------- | ------------------------- | -------------------------------------------- |
| `MAIL_PORT` | `2525`                    | Port for the built-in inbound SMTP server    |
| `MAIL_HOST` | `ingress.<BASE_URL host>` | Hostname the built-in SMTP server listens on |

## Background jobs

| Variable             | Default | Description                                                                                        |
| -------------------- | ------- | -------------------------------------------------------------------------------------------------- |
| `QUEUE_WORKERS`      | `4`     | Number of background job workers                                                                   |
| `ENABLE_SCHEDULER`   | `true`  | Set to `false` to disable the background scheduler (thumbnail generation, ingress polling, purges) |
| `SCRATCH_PURGE_TIME` | `03:00` | Daily time (HH:MM, 24h UTC) to purge temporary scratch files left by failed or cancelled jobs      |

## Media processing

| Variable          | Default | Description                                                                    |
| ----------------- | ------- | ------------------------------------------------------------------------------ |
| `FFMPEG_PATH`     | -       | Path to the `ffmpeg` binary. If unset, Damask looks for it in `$PATH`          |
| `FFMPEG_HW_ACCEL` | -       | Hardware acceleration backend. Options: `videotoolbox`, `vaapi`, `qsv`, `cuda` |

## AI providers

Configure API keys for AI image editing. See the [AI providers guide](ai-providers) for setup details.

| Variable                              | Default                                    | Description                          |
| ------------------------------------- | ------------------------------------------ | ------------------------------------ |
| `IMAGEROUTER_API_KEY`                 | -                                          | Server-wide ImageRouter API key      |
| `IMAGEROUTER_DEFAULT_MODEL`           | `black-forest-labs/FLUX-2-klein-4b:free`   | Default model for image editing      |
| `IMAGEROUTER_DEFAULT_BG_REMOVE_MODEL` | `bria/remove-background:free`              | Default model for background removal |
| `OPENROUTER_API_KEY`                  | -                                          | Server-wide OpenRouter API key       |
| `OPENROUTER_DEFAULT_MODEL`            | `openai/dall-e-2`                          | Default model for image editing      |
| `OPENROUTER_DEFAULT_BG_REMOVE_MODEL`  | `stability-ai/stable-diffusion-xl-refiner` | Default model for background removal |

Workspace owners can also set per-workspace keys in **Settings → Integrations → AI services**, which take priority over these environment variables.

## Telemetry

Damask supports OpenTelemetry for observability. Disabled by default.

| Variable        | Default                             | Description                                  |
| --------------- | ----------------------------------- | -------------------------------------------- |
| `OTEL_ENABLED`  | `false`                             | Set to `true` to enable OpenTelemetry export |
| `OTEL_ENDPOINT` | `http://localhost:8082/api/otel/v1` | OTLP HTTP endpoint                           |
| `OTEL_TOKEN`    | -                                   | Bearer token for the OTLP endpoint           |

## OIDC / SSO

See the [OIDC guide](oidc) for full setup instructions.

| Variable             | Default            | Description                          |
| -------------------- | ------------------ | ------------------------------------ |
| `OIDC_ISSUER_URL`    | -                  | OIDC provider issuer URL             |
| `OIDC_CLIENT_ID`     | -                  | OIDC client ID                       |
| `OIDC_CLIENT_SECRET` | -                  | OIDC client secret                   |
| `OIDC_LABEL`         | `Sign in with SSO` | Button label shown on the login page |

## Google & Canva OAuth

| Variable               | Default | Description            |
| ---------------------- | ------- | ---------------------- |
| `GOOGLE_CLIENT_ID`     | -       | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | -       | Google OAuth secret    |
| `CANVA_CLIENT_ID`      | -       | Canva OAuth client ID  |
| `CANVA_CLIENT_SECRET`  | -       | Canva OAuth secret     |

## Next steps

- [Review Damask storage & backups](storage)
