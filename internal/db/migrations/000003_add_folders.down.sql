DROP INDEX IF EXISTS idx_assets_folder;
DROP INDEX IF EXISTS idx_folders_project;
CREATE TABLE assets_backup AS SELECT id, workspace_id, project_id, original_filename, storage_key, mime_type, size, width, height, thumbnail_key, metadata, created_at, updated_at FROM assets;
DROP TABLE assets;
ALTER TABLE assets_backup RENAME TO assets;
CREATE INDEX idx_assets_workspace ON assets(workspace_id);
CREATE INDEX idx_assets_project ON assets(project_id);
DROP TABLE folders;
