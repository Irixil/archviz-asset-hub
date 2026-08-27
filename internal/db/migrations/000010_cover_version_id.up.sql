-- Migration 010: version-pinned cover/icon references
-- Adds cover_version_id to projects and icon_version_id to workspaces.
-- When set, the cover/icon shows that specific version's thumbnail.
-- Falls back to the asset's current version thumbnail if the pinned
-- version has been deleted (handled in application code).

ALTER TABLE projects   ADD COLUMN cover_version_id TEXT REFERENCES asset_versions(id);
ALTER TABLE workspaces ADD COLUMN icon_asset_id    TEXT REFERENCES assets(id);
ALTER TABLE workspaces ADD COLUMN icon_version_id  TEXT REFERENCES asset_versions(id);
