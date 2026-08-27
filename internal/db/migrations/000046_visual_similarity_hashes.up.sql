CREATE TABLE asset_visual_similarity_hashes (
    asset_version_id  TEXT PRIMARY KEY
                      REFERENCES asset_versions(id) ON DELETE CASCADE,
    workspace_id      TEXT NOT NULL REFERENCES workspaces(id),
    central_hash      INTEGER NOT NULL,
    hash_set          TEXT NOT NULL,  -- JSON []uint64, typically 1-3 elements
    created_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_visual_similarity_hashes_workspace_hash
    ON asset_visual_similarity_hashes(workspace_id, central_hash);
