DROP INDEX IF EXISTS idx_variants_version;
-- SQLite <3.35 cannot DROP COLUMN; recreate the table without asset_version_id.
CREATE TABLE variants_bak AS SELECT id, asset_id, workspace_id, type, storage_key, transform_params, size, created_at FROM variants;
DROP TABLE variants;
ALTER TABLE variants_bak RENAME TO variants;
CREATE INDEX idx_variants_asset ON variants(asset_id);
CREATE INDEX idx_variants_workspace ON variants(workspace_id);
