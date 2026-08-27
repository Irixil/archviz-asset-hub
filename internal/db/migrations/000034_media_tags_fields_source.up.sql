PRAGMA defer_foreign_keys=ON;

ALTER TABLE field_definitions RENAME TO field_definitions_old;

CREATE TABLE field_definitions (
  id            TEXT PRIMARY KEY,
  workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  created_by    TEXT REFERENCES users(id),
  source        TEXT NOT NULL DEFAULT 'user',
  scope         TEXT NOT NULL CHECK(scope IN ('asset', 'project')),
  name          TEXT NOT NULL,
  key           TEXT NOT NULL,
  field_type    TEXT NOT NULL CHECK(field_type IN ('text','number','date','boolean','select','url')),
  options       TEXT,
  required      INTEGER NOT NULL DEFAULT 0,
  position      INTEGER NOT NULL DEFAULT 0,
  inherit_from_project INTEGER NOT NULL DEFAULT 0,
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
  deleted_at    TEXT,
  UNIQUE(workspace_id, scope, key)
);

INSERT INTO field_definitions
  SELECT fd.id,
         fd.workspace_id,
         CASE
           WHEN fd.created_by IS NOT NULL
             AND EXISTS (SELECT 1 FROM users u WHERE u.id = fd.created_by)
           THEN fd.created_by
           ELSE NULL
         END,
         CASE
           WHEN fd.key LIKE '_exif_%' THEN 'exif'
           WHEN fd.key LIKE '_media_%' THEN 'media_tags'
           ELSE 'user'
         END,
         fd.scope,
         fd.name,
         fd.key,
         fd.field_type,
         fd.options,
         fd.required,
         fd.position,
         fd.inherit_from_project,
         fd.created_at,
         fd.updated_at,
         fd.deleted_at
  FROM field_definitions_old fd;

CREATE TABLE asset_field_values_new (
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

INSERT INTO asset_field_values_new
  SELECT * FROM asset_field_values;

DROP TABLE asset_field_values;
ALTER TABLE asset_field_values_new RENAME TO asset_field_values;

CREATE INDEX idx_afv_asset    ON asset_field_values(asset_id);
CREATE INDEX idx_afv_field    ON asset_field_values(field_id);
CREATE INDEX idx_afv_text     ON asset_field_values(field_id, value_text);
CREATE INDEX idx_afv_number   ON asset_field_values(field_id, value_number);
CREATE INDEX idx_afv_date     ON asset_field_values(field_id, value_date);
CREATE INDEX idx_afv_boolean  ON asset_field_values(field_id, value_boolean);

CREATE TABLE project_field_values_new (
  id            TEXT PRIMARY KEY,
  project_id    TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  field_id      TEXT NOT NULL REFERENCES field_definitions(id),
  value_text    TEXT,
  value_number  REAL,
  value_date    TEXT,
  value_boolean INTEGER,
  created_by    TEXT NOT NULL REFERENCES users(id),
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(project_id, field_id)
);

INSERT INTO project_field_values_new
  SELECT * FROM project_field_values;

DROP TABLE project_field_values;
ALTER TABLE project_field_values_new RENAME TO project_field_values;

CREATE INDEX idx_pfv_project  ON project_field_values(project_id);
CREATE INDEX idx_pfv_field    ON project_field_values(field_id);
CREATE INDEX idx_pfv_text     ON project_field_values(field_id, value_text);
CREATE INDEX idx_pfv_number   ON project_field_values(field_id, value_number);
CREATE INDEX idx_pfv_date     ON project_field_values(field_id, value_date);

DROP TABLE field_definitions_old;

CREATE INDEX idx_field_defs_workspace ON field_definitions(workspace_id, scope);
CREATE INDEX idx_field_defs_active    ON field_definitions(workspace_id, deleted_at);
CREATE INDEX idx_field_defs_source    ON field_definitions(workspace_id, source);
