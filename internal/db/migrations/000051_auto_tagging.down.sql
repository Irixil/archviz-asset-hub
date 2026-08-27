DROP TABLE IF EXISTS auto_tag_suggestions;
ALTER TABLE workspaces DROP COLUMN auto_tag_enabled;
ALTER TABLE workspaces DROP COLUMN auto_tag_mode;
