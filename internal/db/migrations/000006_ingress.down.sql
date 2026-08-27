DROP TABLE IF EXISTS ingress_rules;
DROP TABLE IF EXISTS ingress_log;
DROP TABLE IF EXISTS ingress_sources;
DROP INDEX IF EXISTS idx_workspaces_ingest_token;
ALTER TABLE workspaces DROP COLUMN ingest_token;
