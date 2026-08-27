DROP INDEX IF EXISTS idx_assets_derived_from;
ALTER TABLE assets DROP COLUMN derived_from_asset_id;
