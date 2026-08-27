-- Migration 012: demo workspace flag

ALTER TABLE workspaces ADD COLUMN is_demo INTEGER NOT NULL DEFAULT 0;

-- Partial unique index: at most one workspace can be the demo workspace
CREATE UNIQUE INDEX idx_workspaces_demo ON workspaces(is_demo) WHERE is_demo = 1;
