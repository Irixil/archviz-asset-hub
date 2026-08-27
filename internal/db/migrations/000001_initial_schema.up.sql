CREATE TABLE workspaces (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL REFERENCES workspaces(id),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name          TEXT NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE workspace_members (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    user_id      TEXT NOT NULL REFERENCES users(id),
    role         TEXT NOT NULL CHECK(role IN ('owner', 'editor', 'viewer')),
    invited_by   TEXT REFERENCES users(id),
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE workspace_invites (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    email        TEXT NOT NULL,
    token        TEXT NOT NULL UNIQUE,
    role         TEXT NOT NULL CHECK(role IN ('editor', 'viewer')) DEFAULT 'editor',
    invited_by   TEXT NOT NULL REFERENCES users(id),
    expires_at   DATETIME NOT NULL,
    accepted_at  DATETIME,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE projects (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL REFERENCES workspaces(id),
    name          TEXT NOT NULL,
    description   TEXT,
    color         TEXT,
    cover_asset_id TEXT,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE assets (
    id                TEXT PRIMARY KEY,
    workspace_id      TEXT NOT NULL REFERENCES workspaces(id),
    project_id        TEXT REFERENCES projects(id),
    original_filename TEXT NOT NULL,
    storage_key       TEXT NOT NULL,
    mime_type         TEXT NOT NULL,
    size              INTEGER NOT NULL,
    width             INTEGER,
    height            INTEGER,
    thumbnail_key     TEXT,
    metadata          TEXT, -- JSON
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE tags (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    name         TEXT NOT NULL,
    UNIQUE(workspace_id, name)
);

CREATE TABLE asset_tags (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    tag_id   TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, tag_id)
);

CREATE TABLE variants (
    id               TEXT PRIMARY KEY,
    asset_id         TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    workspace_id     TEXT NOT NULL REFERENCES workspaces(id),
    type             TEXT NOT NULL,
    storage_key      TEXT NOT NULL,
    transform_params TEXT, -- JSON
    size             INTEGER,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE jobs (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    type         TEXT NOT NULL,
    payload      TEXT NOT NULL, -- JSON
    status       TEXT NOT NULL CHECK(status IN ('pending', 'processing', 'done', 'failed')) DEFAULT 'pending',
    attempts     INTEGER NOT NULL DEFAULT 0,
    error        TEXT,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Indexes for common queries
CREATE INDEX idx_users_workspace ON users(workspace_id);
CREATE INDEX idx_assets_workspace ON assets(workspace_id);
CREATE INDEX idx_assets_project ON assets(project_id);
CREATE INDEX idx_tags_workspace ON tags(workspace_id);
CREATE INDEX idx_variants_asset ON variants(asset_id);
CREATE INDEX idx_jobs_status ON jobs(status);
