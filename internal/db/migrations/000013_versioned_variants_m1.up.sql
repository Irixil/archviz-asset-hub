-- VV-M1: Add asset_version_id column to variants and index.
-- Keep asset_id for now — back-filled in VV-M2, dropped in VV-M3.

ALTER TABLE variants ADD COLUMN asset_version_id TEXT
  REFERENCES asset_versions(id) ON DELETE CASCADE;

CREATE INDEX idx_variants_version ON variants(asset_version_id);
