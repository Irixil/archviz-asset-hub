DROP INDEX IF EXISTS idx_aevents_type;
DROP INDEX IF EXISTS idx_aevents_user;
DROP INDEX IF EXISTS idx_aevents_workspace;
DROP INDEX IF EXISTS idx_aevents_asset;
DROP TABLE IF EXISTS asset_events;

DROP INDEX IF EXISTS idx_pevents_user;
DROP INDEX IF EXISTS idx_pevents_workspace;
DROP INDEX IF EXISTS idx_pevents_project;
DROP TABLE IF EXISTS project_events;
