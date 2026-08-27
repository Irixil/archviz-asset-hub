-- VV-M3 down: restore old variants table shape with asset_id.
-- Requires a join to recover asset_id from asset_versions.
CREATE TABLE variants_old (
  id               TEXT PRIMARY KEY,
  asset_id         TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  workspace_id     TEXT NOT NULL REFERENCES workspaces(id),
  type             TEXT NOT NULL,
  storage_key      TEXT NOT NULL,
  transform_params TEXT,
  size             INTEGER,
  created_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO variants_old (id, asset_id, workspace_id, type, storage_key, transform_params, size, created_at)
  SELECT v.id, av.asset_id, v.workspace_id, v.type, v.storage_key, v.transform_params, v.size, v.created_at
  FROM variants v
  JOIN asset_versions av ON av.id = v.asset_version_id;

DROP TABLE variants;
ALTER TABLE variants_old RENAME TO variants;

CREATE INDEX idx_variants_asset     ON variants(asset_id);
CREATE INDEX idx_variants_workspace ON variants(workspace_id);
