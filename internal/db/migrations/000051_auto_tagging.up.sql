-- Pending tag suggestions for an asset.
-- Suggestions are created by the auto_tag job.
-- Accepting writes to asset_tags and deletes this row.
-- Dismissing deletes this row with no further action.
CREATE TABLE auto_tag_suggestions (
    id                TEXT PRIMARY KEY,
    workspace_id      TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    asset_id          TEXT NOT NULL REFERENCES assets(id)     ON DELETE CASCADE,
    -- asset_version_id the suggestion was generated from — used to invalidate
    -- stale suggestions when a new version is uploaded.
    asset_version_id  TEXT REFERENCES asset_versions(id)      ON DELETE SET NULL,
    tag_name          TEXT NOT NULL,
    created_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_auto_tag_suggestions_asset
    ON auto_tag_suggestions(asset_id);
CREATE INDEX idx_auto_tag_suggestions_workspace
    ON auto_tag_suggestions(workspace_id);

-- Workspace settings additions.
-- auto_tag_enabled: whether the auto_tag job fires on upload (0 = off, 1 = on).
-- auto_tag_mode:    'pending' = suggestions await review;
--                   'silent'  = applied immediately with no UI step.
ALTER TABLE workspaces
    ADD COLUMN auto_tag_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workspaces
    ADD COLUMN auto_tag_mode TEXT NOT NULL DEFAULT 'pending'
        CHECK (auto_tag_mode IN ('pending', 'silent'));
