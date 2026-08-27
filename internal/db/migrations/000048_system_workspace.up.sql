-- Insert a sentinel workspace row for system-level jobs (purge, retention, etc.)
-- that have no real workspace context. Foreign-key constraints on the jobs table
-- require a valid workspace_id, so we seed this well-known id at schema init time.
INSERT OR IGNORE INTO workspaces (id, name) VALUES ('system', 'System');
