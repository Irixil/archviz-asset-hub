DROP INDEX IF EXISTS idx_workflows_trigger_config;

ALTER TABLE workflows DROP COLUMN trigger_config;
