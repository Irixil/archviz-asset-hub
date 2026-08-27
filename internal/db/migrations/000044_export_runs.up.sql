CREATE TABLE export_runs (
    id               TEXT PRIMARY KEY,
    export_config_id TEXT NOT NULL REFERENCES export_configs(id) ON DELETE CASCADE,
    workspace_id     TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    triggered_by     TEXT REFERENCES users(id),  -- NULL = scheduler
    status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK(status IN ('pending','running','done','failed')),
    assets_total     INTEGER NOT NULL DEFAULT 0,
    assets_exported  INTEGER NOT NULL DEFAULT 0,
    assets_skipped   INTEGER NOT NULL DEFAULT 0,
    bytes_written    INTEGER NOT NULL DEFAULT 0,
    error            TEXT,
    started_at       DATETIME,
    completed_at     DATETIME,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_export_runs_config    ON export_runs(export_config_id, created_at DESC);
CREATE INDEX idx_export_runs_workspace ON export_runs(workspace_id, created_at DESC);
CREATE INDEX idx_export_runs_status    ON export_runs(status);
