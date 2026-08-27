CREATE TABLE export_configs (
    id              TEXT PRIMARY KEY,
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_by      TEXT NOT NULL REFERENCES users(id),
    label           TEXT NOT NULL,

    -- destination
    dest_type       TEXT NOT NULL CHECK(dest_type IN ('sftp', 'gdrive')),
    dest_config     TEXT NOT NULL DEFAULT '{}',  -- JSON, encrypted at rest

    -- content options
    versions        TEXT NOT NULL DEFAULT 'current'
                    CHECK(versions IN ('current', 'all')),
    include_variants INTEGER NOT NULL DEFAULT 1,

    -- schedule
    schedule_type   TEXT NOT NULL DEFAULT 'manual'
                    CHECK(schedule_type IN ('manual', 'after_quiet')),
    quiet_minutes   INTEGER,  -- NULL when schedule_type = 'manual'

    enabled         INTEGER NOT NULL DEFAULT 1,
    last_run_at     DATETIME,
    last_run_status TEXT,     -- 'ok' | 'partial' | 'failed' | 'pending' | NULL
    last_error      TEXT,

    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_export_configs_workspace ON export_configs(workspace_id);
CREATE INDEX idx_export_configs_project   ON export_configs(project_id);
CREATE INDEX idx_export_configs_schedule
    ON export_configs(schedule_type, enabled, last_run_at);
