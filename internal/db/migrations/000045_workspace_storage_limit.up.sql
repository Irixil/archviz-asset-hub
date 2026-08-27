-- 000045_workspace_storage_limit.up.sql
-- Adds an optional storage cap per workspace.
-- NULL means unlimited. Set via direct DB write or internal admin tooling —
-- not user-modifiable through the API.
ALTER TABLE workspaces ADD COLUMN storage_limit_bytes INTEGER;
