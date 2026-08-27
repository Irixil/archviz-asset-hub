-- VV-M3: Recreate variants table with asset_version_id NOT NULL, dropping asset_id.
-- VV-M2 must have run first (back-fills asset_version_id). If any rows still have
-- NULL asset_version_id the INSERT below will fail with a NOT NULL constraint error,
-- which is the correct safety net.

CREATE TABLE variants_new (
  id               TEXT PRIMARY KEY,
  workspace_id     TEXT NOT NULL REFERENCES workspaces(id),
  asset_version_id TEXT NOT NULL REFERENCES asset_versions(id) ON DELETE CASCADE,
  type             TEXT NOT NULL,
  transform_params TEXT,
  storage_key      TEXT NOT NULL,
  size             INTEGER,
  created_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO variants_new (id, workspace_id, asset_version_id, type, transform_params, storage_key, size, created_at)
  SELECT id, workspace_id, asset_version_id, type, transform_params, storage_key, size, created_at
  FROM variants;

DROP TABLE variants;
ALTER TABLE variants_new RENAME TO variants;

CREATE INDEX idx_variants_version   ON variants(asset_version_id);
CREATE INDEX idx_variants_workspace ON variants(workspace_id);
CREATE INDEX idx_variants_type      ON variants(asset_version_id, type);
