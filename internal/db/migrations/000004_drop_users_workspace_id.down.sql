ALTER TABLE users ADD COLUMN workspace_id TEXT NOT NULL DEFAULT '' REFERENCES workspaces(id);
CREATE INDEX idx_users_workspace ON users(workspace_id);
