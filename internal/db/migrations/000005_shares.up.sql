CREATE TABLE shares (
  id              TEXT PRIMARY KEY,
  workspace_id    TEXT NOT NULL REFERENCES workspaces(id),
  created_by      TEXT NOT NULL REFERENCES users(id),
  label           TEXT NOT NULL DEFAULT '',
  target_type     TEXT NOT NULL CHECK(target_type IN ('collection', 'asset', 'project')),
  target_id       TEXT NOT NULL,
  password_hash   TEXT,
  expires_at      TEXT,
  allow_comments  INTEGER NOT NULL DEFAULT 0,
  allow_download  INTEGER NOT NULL DEFAULT 1,
  view_count      INTEGER NOT NULL DEFAULT 0,
  created_at      TEXT NOT NULL DEFAULT (datetime('now')),
  revoked_at      TEXT
);

CREATE INDEX idx_shares_workspace ON shares(workspace_id);
CREATE INDEX idx_shares_target ON shares(target_type, target_id);

CREATE TABLE share_comments (
  id           TEXT PRIMARY KEY,
  share_id     TEXT NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
  asset_id     TEXT NOT NULL,
  author_name  TEXT NOT NULL,
  author_email TEXT,
  body         TEXT NOT NULL,
  created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_comments_share ON share_comments(share_id);
CREATE INDEX idx_comments_asset ON share_comments(asset_id);
