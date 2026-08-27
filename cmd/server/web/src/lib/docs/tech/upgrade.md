# Upgrading

Damask uses automatic schema migrations - upgrading is pull, stop, start.

## General procedure

1. **Back up your database** before any upgrade:
   ```bash
   cp data/damask.db data/damask.db.bak
   ```
2. Pull the new image (or download the new binary)
3. Stop the running instance
4. Start with the new version - migrations run automatically on startup
5. Verify the health endpoint: `curl https://dam.example.com/healthz`

## Docker

```bash
docker compose pull
docker compose up -d
```

## Binary

```bash
systemctl stop damask
cp damask-linux-amd64-NEW /usr/local/bin/damask
systemctl start damask
journalctl -u damask -f
```

## Failed migration recovery

If a migration fails, Damask exits immediately with an error message. The database is not corrupted - migrations run in transactions.

1. Check the logs for the error
2. Fix the underlying issue (usually a constraint violation from unexpected data)
3. Restore from backup if needed
4. Restart - the failed migration re-runs

## Downgrading

Damask does not support automatic downgrade migrations. Restore from a pre-upgrade backup instead.
