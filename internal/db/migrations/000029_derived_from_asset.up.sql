ALTER TABLE assets ADD COLUMN derived_from_asset_id TEXT REFERENCES assets(id) ON DELETE SET NULL;
CREATE INDEX idx_assets_derived_from ON assets(derived_from_asset_id);
