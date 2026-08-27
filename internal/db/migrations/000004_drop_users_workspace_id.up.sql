DROP INDEX IF EXISTS idx_users_workspace;
ALTER TABLE users DROP COLUMN workspace_id;
