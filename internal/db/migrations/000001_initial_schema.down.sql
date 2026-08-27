DROP INDEX IF EXISTS idx_jobs_status;
DROP INDEX IF EXISTS idx_variants_asset;
DROP INDEX IF EXISTS idx_tags_workspace;
DROP INDEX IF EXISTS idx_assets_project;
DROP INDEX IF EXISTS idx_assets_workspace;
DROP INDEX IF EXISTS idx_users_workspace;

DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS variants;
DROP TABLE IF EXISTS asset_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS workspace_invites;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS workspaces;
