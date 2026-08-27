-- VV-M2: Back-fill asset_version_id for all existing variant rows.
-- Idempotent: only touches rows where asset_version_id IS NULL.

UPDATE variants
SET asset_version_id = (
  SELECT id FROM asset_versions
  WHERE asset_id = variants.asset_id
    AND is_current = 1
  LIMIT 1
)
WHERE asset_version_id IS NULL;
