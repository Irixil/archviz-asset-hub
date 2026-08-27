# Storage

Damask supports three storage backends for uploaded files.

## Local disk (default)

Files are stored under `STORAGE_LOCAL_PATH` (default `./storage`). No additional configuration needed.

Suitable for single-host deployments. For cloud deployments or external access, use S3.

```
STORAGE=local
STORAGE_LOCAL_PATH=./storage
```

## S3-compatible

Works with AWS S3, MinIO, Backblaze B2, Cloudflare R2, and any S3-compatible service.

```
STORAGE=s3
STORAGE_S3_BUCKET=my-damask-assets
STORAGE_S3_REGION=us-east-1
STORAGE_S3_ACCESSKEY=AKIAIOSFODNN7EXAMPLE
STORAGE_S3_SECRETKEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
STORAGE_S3_BASE_PATH=/                   # optional key prefix
```

## SFTP

```
STORAGE=sftp
STORAGE_SFTP_HOST=files.example.com
STORAGE_SFTP_PORT=22
STORAGE_SFTP_USER=damask
STORAGE_SFTP_AUTH_METHOD=key             # "password" or "key"
STORAGE_SFTP_PRIVATE_KEY=<PEM content>   # paste the private key directly
STORAGE_SFTP_BASE_PATH=/uploads
```

Password auth:

```
STORAGE_SFTP_AUTH_METHOD=password
STORAGE_SFTP_PASSWORD=your-password
```

## Switching backends

Damask does not migrate existing files between backends automatically. To switch:

1. Copy all files to the new backend manually
2. Update environment variables
3. Restart, existing links continue to resolve as long as they map to the same path

## Backups

A complete backup consists of two things:

1. **`damask.db`**, all metadata, users, tags, fields, share links, and job state
2. **The storage directory** (or your S3 bucket), the actual asset files

Both are required. The database without the files leaves broken links; the files without the database are an unorganised folder.

### Back up the database

SQLite WAL mode is enabled by default, so the database is safe to copy while the server is running:

```bash
# Simple file copy (safe while running)
cp /data/damask.db /backups/damask-$(date +%Y%m%d).db

# Fully consistent online backup via sqlite3
sqlite3 /data/damask.db ".backup '/backups/damask.db'"
```

### Back up local file storage

```bash
rsync -a /data/storage/ /backups/storage/
```

For S3 storage, use your provider's cross-region replication or sync to a second bucket.
