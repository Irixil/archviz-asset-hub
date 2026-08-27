-- SQLite does not support DROP COLUMN in older versions; recreate table to roll back.
CREATE TABLE tags_backup AS SELECT id, workspace_id, name FROM tags;
DROP TABLE tags;
ALTER TABLE tags_backup RENAME TO tags;
CREATE UNIQUE INDEX tags_workspace_id_name ON tags(workspace_id, name);
