-- Migration 011: event log

-- 001: asset_events
CREATE TABLE asset_events (
  id           TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id),
  asset_id     TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  user_id      TEXT REFERENCES users(id),
  actor_type   TEXT NOT NULL DEFAULT 'user' CHECK(actor_type IN ('user','system')),
  event_type   TEXT NOT NULL,
  payload      TEXT NOT NULL DEFAULT '{}',
  created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_aevents_asset     ON asset_events(asset_id, created_at DESC);
CREATE INDEX idx_aevents_workspace ON asset_events(workspace_id, created_at DESC);
CREATE INDEX idx_aevents_user      ON asset_events(user_id, created_at DESC);
CREATE INDEX idx_aevents_type      ON asset_events(workspace_id, event_type);

-- 002: project_events
CREATE TABLE project_events (
  id           TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id),
  project_id   TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id      TEXT REFERENCES users(id),
  actor_type   TEXT NOT NULL DEFAULT 'user' CHECK(actor_type IN ('user','system')),
  event_type   TEXT NOT NULL,
  payload      TEXT NOT NULL DEFAULT '{}',
  created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_pevents_project   ON project_events(project_id, created_at DESC);
CREATE INDEX idx_pevents_workspace ON project_events(workspace_id, created_at DESC);
CREATE INDEX idx_pevents_user      ON project_events(user_id, created_at DESC);

-- 003: workspace event log retention settings
ALTER TABLE workspaces ADD COLUMN event_log_retention_days INTEGER NOT NULL DEFAULT 365;
ALTER TABLE workspaces ADD COLUMN download_log_retention_days INTEGER NOT NULL DEFAULT 30;
