-- Migration 009: asset versioning

-- 001: asset_versions table
CREATE TABLE asset_versions (
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

CREATE INDEX idx_versions_asset     ON asset_versions(asset_id, is_current);
CREATE INDEX idx_versions_workspace ON asset_versions(workspace_id);
CREATE INDEX idx_versions_hash      ON asset_versions(content_hash);
CREATE INDEX idx_versions_created   ON asset_versions(asset_id, created_at DESC);

-- 002: workspace version retention setting
ALTER TABLE workspaces ADD COLUMN version_retention_count INTEGER NOT NULL DEFAULT 0;

-- 003: data migration — create v1 for all existing assets (idempotent)
INSERT INTO asset_versions (
  id, asset_id, workspace_id, version_num, storage_key, content_hash,
  mime_type, size, width, height, thumbnail_key, created_by, created_at, is_current
)
SELECT
  'ver_init_' || a.id,
  a.id,
  a.workspace_id,
  1,
  a.storage_key,
  'legacy-' || a.id,   -- placeholder hash; real hash computed on next upload
  a.mime_type,
  a.size,
  a.width,
  a.height,
  a.thumbnail_key,
  COALESCE(
    (SELECT user_id FROM workspace_members WHERE workspace_id = a.workspace_id ORDER BY created_at LIMIT 1),
    'system'
  ),
  a.created_at,
  1
FROM assets a
WHERE NOT EXISTS (
  SELECT 1 FROM asset_versions av WHERE av.asset_id = a.id
);

-- 004: add current_version_id shortcut column to assets
ALTER TABLE assets ADD COLUMN current_version_id TEXT REFERENCES asset_versions(id);

UPDATE assets
SET current_version_id = (
  SELECT id FROM asset_versions WHERE asset_id = assets.id AND is_current = 1 LIMIT 1
)
WHERE current_version_id IS NULL;
