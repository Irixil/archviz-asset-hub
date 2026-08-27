-- Migration 053: Cross-asset content-hash duplicate detection
-- Adds a per-workspace enforcement mode (off/warn/block), a workspace-scoped
-- index for fast content_hash lookups, and an ingress_log column + status
-- value so ingress can surface/skip duplicates the same way interactive
-- upload does.

ALTER TABLE workspaces
  ADD COLUMN duplicate_detection_mode TEXT NOT NULL DEFAULT 'warn'
  CHECK (duplicate_detection_mode IN ('off', 'warn', 'block'));

-- Workspace-wide lookup needs to be fast without relying on the existing
-- single-column content_hash index (idx_versions_hash), which doesn't prune
-- by workspace.
CREATE INDEX idx_versions_workspace_hash ON asset_versions(workspace_id, content_hash);

-- SQLite doesn't support ALTER TABLE ... DROP/ADD CONSTRAINT; recreate
-- ingress_log to widen its status CHECK and add the duplicate reference column.
PRAGMA foreign_keys = OFF;

CREATE TABLE ingress_log_new (
    id                    TEXT PRIMARY KEY,
    source_id             TEXT NOT NULL REFERENCES ingress_sources(id) ON DELETE CASCADE,
    remote_id             TEXT NOT NULL,
    filename              TEXT NOT NULL,
    asset_id              TEXT REFERENCES assets(id) ON DELETE SET NULL,
    status                TEXT NOT NULL DEFAULT 'pending'
                              CHECK(status IN ('pending', 'imported', 'skipped', 'error', 'skipped_duplicate')),
    error                 TEXT,
    duplicate_of_asset_id TEXT REFERENCES assets(id) ON DELETE SET NULL,
    imported_at           DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(source_id, remote_id)
);

INSERT INTO ingress_log_new
SELECT id, source_id, remote_id, filename, asset_id, status, error, NULL, imported_at
FROM ingress_log;

DROP TABLE ingress_log;
ALTER TABLE ingress_log_new RENAME TO ingress_log;

CREATE INDEX idx_ingress_log_source ON ingress_log(source_id, imported_at);
CREATE INDEX idx_ingress_log_status ON ingress_log(status);

PRAGMA foreign_keys = ON;
