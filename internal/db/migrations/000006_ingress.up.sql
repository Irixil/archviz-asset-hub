-- Ingress sources: each record represents one configured ingest connection
CREATE TABLE ingress_sources (
    id                TEXT PRIMARY KEY,
    workspace_id      TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_by        TEXT NOT NULL REFERENCES users(id),
    type              TEXT NOT NULL,           -- 'imap' | 'sftp' | 'dav' | 's3' | 'email_api'
    label             TEXT NOT NULL DEFAULT '',
    config            TEXT NOT NULL DEFAULT '', -- AES-256-GCM encrypted JSON blob (base64url)
    dest_folder_id    TEXT REFERENCES folders(id),
    dest_project_id   TEXT REFERENCES projects(id),
    enabled           INTEGER NOT NULL DEFAULT 1,
    poll_interval_min INTEGER NOT NULL DEFAULT 15,
    last_polled_at    DATETIME,
    last_error        TEXT,
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_ingress_sources_workspace ON ingress_sources(workspace_id);
CREATE INDEX idx_ingress_sources_due       ON ingress_sources(enabled, last_polled_at);

-- Ingress log: one row per remote item attempted; UNIQUE guard is the dedup key
CREATE TABLE ingress_log (
    id          TEXT PRIMARY KEY,
    source_id   TEXT NOT NULL REFERENCES ingress_sources(id) ON DELETE CASCADE,
    remote_id   TEXT NOT NULL,
    filename    TEXT NOT NULL,
    asset_id    TEXT REFERENCES assets(id) ON DELETE SET NULL,
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK(status IN ('pending', 'imported', 'skipped', 'error')),
    error       TEXT,
    imported_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(source_id, remote_id)
);

CREATE INDEX idx_ingress_log_source ON ingress_log(source_id, imported_at);
CREATE INDEX idx_ingress_log_status ON ingress_log(status);

-- Ingress rules: ordered filter / routing per source
CREATE TABLE ingress_rules (
    id        TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES ingress_sources(id) ON DELETE CASCADE,
    position  INTEGER NOT NULL DEFAULT 0,
    field     TEXT NOT NULL,    -- 'filename' | 'mime_type' | 'size'
    operator  TEXT NOT NULL,    -- 'contains' | 'equals' | 'starts_with' | 'ends_with' | 'gt' | 'lt'
    value     TEXT NOT NULL,
    action    TEXT NOT NULL     -- 'allow' | 'deny' | 'set_project' | 'set_folder'
);

CREATE INDEX idx_ingress_rules_source ON ingress_rules(source_id, position);

-- Workspace ingest token for email_api source
-- Note: SQLite does not allow ADD COLUMN with UNIQUE constraint;
-- uniqueness is enforced via a separate index.
ALTER TABLE workspaces ADD COLUMN ingest_token TEXT;
CREATE UNIQUE INDEX idx_workspaces_ingest_token ON workspaces(ingest_token) WHERE ingest_token IS NOT NULL;
