-- Migration 016 down: Restore created_by to NOT NULL
-- For any existing NULL rows, use a sentinel value.

PRAGMA foreign_keys = OFF;

CREATE TABLE asset_versions_new (
    id            TEXT PRIMARY KEY,
    asset_id      TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    workspace_id  TEXT NOT NULL REFERENCES workspaces(id),
    version_num   INTEGER NOT NULL,
    storage_key   TEXT NOT NULL,
    content_hash  TEXT NOT NULL,
    mime_type     TEXT NOT NULL,
    size          INTEGER NOT NULL,
    width         INTEGER,
    height        INTEGER,
    duration_sec  REAL,
    thumbnail_key TEXT,
    comment       TEXT,
    created_by    TEXT NOT NULL REFERENCES users(id),
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    is_current    INTEGER NOT NULL DEFAULT 0,
    deleted_at    TEXT,
    UNIQUE(asset_id, version_num)
);

INSERT INTO asset_versions_new
SELECT
    id, asset_id, workspace_id, version_num, storage_key, content_hash,
    mime_type, size, width, height, duration_sec, thumbnail_key, comment,
    COALESCE(created_by, workspace_id),
    created_at, is_current, deleted_at
FROM asset_versions;

DROP TABLE asset_versions;
ALTER TABLE asset_versions_new RENAME TO asset_versions;

PRAGMA foreign_keys = ON;

-- Recreate indices
CREATE INDEX idx_versions_asset     ON asset_versions(asset_id, is_current);
CREATE INDEX idx_versions_workspace ON asset_versions(workspace_id);
CREATE INDEX idx_versions_hash      ON asset_versions(content_hash);
CREATE INDEX idx_versions_created   ON asset_versions(asset_id, created_at DESC);
