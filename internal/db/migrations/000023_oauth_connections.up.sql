CREATE TABLE oauth_connections (
  id               TEXT PRIMARY KEY,
  workspace_id     TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  created_by       TEXT NOT NULL REFERENCES users(id),
  provider         TEXT NOT NULL,
  provider_user_id TEXT,
  provider_email   TEXT,
  scopes           TEXT NOT NULL DEFAULT '[]',
  access_token     TEXT NOT NULL,
  refresh_token    TEXT,
  expires_at       TEXT,
  created_at       TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(workspace_id, provider, provider_user_id)
);

CREATE INDEX idx_oauth_connections_workspace ON oauth_connections(workspace_id);
