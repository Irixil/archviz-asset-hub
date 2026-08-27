CREATE TABLE collections (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    created_by   TEXT NOT NULL REFERENCES users(id),
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE collection_assets (
    collection_id TEXT    NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    asset_id      TEXT    NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    position      INTEGER NOT NULL DEFAULT 0,
    added_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (collection_id, asset_id)
);

CREATE INDEX idx_collections_workspace ON collections(workspace_id);
CREATE INDEX idx_collection_assets_collection ON collection_assets(collection_id, position);
