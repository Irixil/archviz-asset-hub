CREATE TABLE IF NOT EXISTS variants (
    id                     TEXT PRIMARY KEY,
    workspace_id           TEXT NOT NULL REFERENCES workspaces(id),
    asset_version_id       TEXT NOT NULL REFERENCES asset_versions(id) ON DELETE CASCADE,
    type                   TEXT NOT NULL,
    storage_key            TEXT NOT NULL,
    transform_params       TEXT,
    size                   INTEGER,
    status                 TEXT NOT NULL DEFAULT 'ready',
    thumbnail_key          TEXT,
    thumbnail_content_type TEXT NOT NULL DEFAULT 'image/jpeg',
    created_at             DATETIME NOT NULL DEFAULT (datetime('now'))
);

ALTER TABLE variants ADD COLUMN title TEXT;
ALTER TABLE variants ADD COLUMN is_shared INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_variants_shared ON variants(asset_version_id, is_shared)
    WHERE is_shared = 1;
