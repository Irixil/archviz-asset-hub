CREATE TABLE asset_text_tracks (
    id               TEXT PRIMARY KEY,
    workspace_id     TEXT NOT NULL REFERENCES workspaces(id),
    asset_id         TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    asset_version_id TEXT REFERENCES asset_versions(id) ON DELETE SET NULL,
    source           TEXT NOT NULL,
    lang             TEXT,
    content          TEXT NOT NULL DEFAULT '',
    storage_key      TEXT,
    content_type     TEXT,
    meta             TEXT,
    status           TEXT NOT NULL DEFAULT 'ready'
                     CHECK(status IN ('pending', 'processing', 'ready', 'failed')),
    error            TEXT,
    created_by       TEXT REFERENCES users(id),
    created_at       DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_text_tracks_asset     ON asset_text_tracks(asset_id);
CREATE INDEX idx_text_tracks_workspace ON asset_text_tracks(workspace_id);
CREATE INDEX idx_text_tracks_source    ON asset_text_tracks(asset_id, source);

CREATE VIRTUAL TABLE assets_text_fts USING fts5(
    track_id     UNINDEXED,
    asset_id     UNINDEXED,
    workspace_id UNINDEXED,
    source       UNINDEXED,
    lang         UNINDEXED,
    content,
    tokenize = 'porter unicode61'
);
