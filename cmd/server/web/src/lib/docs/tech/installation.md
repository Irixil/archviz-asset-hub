# Installation

Damask ships as a single static binary with few runtime dependencies.

> **Single-node only.** Damask uses SQLite and an in-process job queue. Running multiple instances behind a load balancer is not supported and will cause data corruption. For high availability, use a single instance with a reliable host and regular backups.

## Docker (recommended)

```yaml
# docker-compose.yml
services:
  damask:
    image: ghcr.io/your-org/damask:latest
    restart: unless-stopped
    ports:
      - '8080:8080'
      - '2525:2525' # optional: expose for email ingress
    volumes:
      - ./data:/data
    environment:
      BASE_URL: https://dam.example.com
      JWT_SECRET: change-me-to-a-long-random-secret
      APP_SECRET: change-me-to-a-long-random-secret
      DB_PATH: /data/damask.db
      STORAGE_LOCAL_PATH: /data/storage
      ENABLE_SIGNUP: true
```

```bash
docker compose up -d
```

Both `JWT_SECRET` and `APP_SECRET` are required, the server will refuse to start without them. Generate secure values with:

```bash
openssl rand -hex 32
```

## Binary

Download the latest release from the [releases page](https://github.com/your-org/damask/releases).

```bash
chmod +x damask-linux-amd64
JWT_SECRET=... APP_SECRET=... BASE_URL=https://dam.example.com ./damask-linux-amd64
```

### Running as a systemd service

```ini
[Unit]
Description=Damask DAM
After=network.target

[Service]
User=damask
WorkingDirectory=/opt/damask
EnvironmentFile=/opt/damask/.env
ExecStart=/opt/damask/damask-linux-amd64
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now damask
journalctl -u damask -f
```

## Reverse proxy (Caddy)

> Set the upload limit generously : assets can be large video/audio files.

```caddyfile
dam.example.com {
    request_body {
        max_size 300MB
    }
    reverse_proxy localhost:8080
}
```

Caddy handles TLS automatically via Let's Encrypt.

## Reverse proxy (nginx)

```nginx
server {
    listen 443 ssl;
    server_name dam.example.com;
    client_max_body_size 300m;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        client_max_body_size 500M;
    }
}
```

## First run

On first start Damask runs all migrations and creates the database. Visit `BASE_URL` and register the first account, this account automatically becomes the owner of your first workspace.

## Email ingress DNS setup

To receive assets via email (see [Ingress sources](ingress)), you need to point a mail subdomain at your server. If Damask runs at `dam.example.com` with IP `1.2.3.4` and you want attachments sent to `anything@mail.dam.example.com`:

| Type | Name   | Value                  |
| ---- | ------ | ---------------------- |
| A    | `mail` | `1.2.3.4`              |
| MX   | `@`    | `mail.dam.example.com` |
| TXT  | `mail` | `v=spf1 mx ~all`       |

Then set `MAIL_HOST=mail.dam.example.com` and expose port `25` (or `2525` with port forwarding) on your host.

## Next steps

- [Review your Damask configuration](configuration)
