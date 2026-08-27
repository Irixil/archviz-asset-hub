CREATE TABLE field_definitions_old (
  id            TEXT PRIMARY KEY,
  workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  created_by    TEXT NOT NULL REFERENCES users(id),
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

INSERT INTO field_definitions_old
  SELECT id, workspace_id, created_by, scope, name, key,
         field_type, options, required, position, inherit_from_project,
         created_at, updated_at, deleted_at
  FROM field_definitions
  WHERE created_by IS NOT NULL;

DROP TABLE field_definitions;
ALTER TABLE field_definitions_old RENAME TO field_definitions;

CREATE INDEX idx_field_defs_workspace ON field_definitions(workspace_id, scope);
CREATE INDEX idx_field_defs_active    ON field_definitions(workspace_id, deleted_at);
