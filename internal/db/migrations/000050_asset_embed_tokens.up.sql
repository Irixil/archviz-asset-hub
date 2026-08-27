CREATE TABLE asset_embed_tokens (
    id           TEXT PRIMARY KEY,          -- 16-char base62, the public token
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    asset_id     TEXT NOT NULL REFERENCES assets(id)     ON DELETE CASCADE,
    created_by   TEXT NOT NULL REFERENCES users(id),
    label        TEXT NOT NULL DEFAULT '',  -- optional human name, for future mgmt UI
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    revoked_at   DATETIME                   -- NULL = active; soft-delete only
);

CREATE INDEX idx_embed_tokens_workspace ON asset_embed_tokens(workspace_id);
CREATE INDEX idx_embed_tokens_asset     ON asset_embed_tokens(asset_id);

-- Enforces at most one active token per asset at the DB level.
-- Revoke-then-create is the rotation pattern; no UPDATE needed.
CREATE UNIQUE INDEX idx_embed_tokens_one_active
    ON asset_embed_tokens(asset_id)
    WHERE revoked_at IS NULL;
