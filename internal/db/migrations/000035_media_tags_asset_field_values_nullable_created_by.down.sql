CREATE TABLE asset_field_values_old (
  id            TEXT PRIMARY KEY,
  asset_id      TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  field_id      TEXT NOT NULL REFERENCES field_definitions(id),
  value_text    TEXT,
  value_number  REAL,
  value_date    TEXT,
  value_boolean INTEGER,
  created_by    TEXT NOT NULL REFERENCES users(id),
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(asset_id, field_id)
);

INSERT INTO asset_field_values_old
  SELECT * FROM asset_field_values
  WHERE created_by IS NOT NULL;

DROP TABLE asset_field_values;
ALTER TABLE asset_field_values_old RENAME TO asset_field_values;

CREATE INDEX idx_afv_asset    ON asset_field_values(asset_id);
CREATE INDEX idx_afv_field    ON asset_field_values(field_id);
CREATE INDEX idx_afv_text     ON asset_field_values(field_id, value_text);
CREATE INDEX idx_afv_number   ON asset_field_values(field_id, value_number);
CREATE INDEX idx_afv_date     ON asset_field_values(field_id, value_date);
CREATE INDEX idx_afv_boolean  ON asset_field_values(field_id, value_boolean);
