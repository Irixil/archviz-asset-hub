ALTER TABLE workflows ADD COLUMN trigger_config TEXT NOT NULL DEFAULT '{}';

CREATE INDEX idx_workflows_trigger_config
    ON workflows(workspace_id, trigger_type, enabled);
