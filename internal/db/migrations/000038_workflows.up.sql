CREATE TABLE workflows (
    id                      TEXT PRIMARY KEY,
    workspace_id            TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name                    TEXT NOT NULL,
    description             TEXT NOT NULL DEFAULT '',
    enabled                 INTEGER NOT NULL DEFAULT 1,
    trigger_type            TEXT NOT NULL,
    graph                   TEXT NOT NULL,
    notify_on_failure_email TEXT NOT NULL DEFAULT '',
    last_run_at             DATETIME,
    created_by              TEXT NOT NULL REFERENCES users(id),
    created_at              DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at              DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_workflows_workspace ON workflows(workspace_id);
CREATE INDEX idx_workflows_trigger ON workflows(trigger_type, enabled);

CREATE TABLE workflow_runs (
    id           TEXT PRIMARY KEY,
    workflow_id  TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'pending',
    trigger_data TEXT NOT NULL DEFAULT '{}',
    context      TEXT NOT NULL DEFAULT '{}',
    error        TEXT,
    started_at   DATETIME,
    completed_at DATETIME,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_workflow_runs_workflow ON workflow_runs(workflow_id, created_at DESC);
CREATE INDEX idx_workflow_runs_workspace ON workflow_runs(workspace_id, created_at DESC);
CREATE INDEX idx_workflow_runs_status ON workflow_runs(status);

CREATE TABLE workflow_run_steps (
    id           TEXT PRIMARY KEY,
    run_id       TEXT NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    node_id      TEXT NOT NULL,
    node_type    TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    attempt      INTEGER NOT NULL DEFAULT 1,
    input_ctx    TEXT NOT NULL DEFAULT '{}',
    output_ctx   TEXT,
    error        TEXT,
    started_at   DATETIME,
    completed_at DATETIME
);

CREATE INDEX idx_workflow_run_steps_run ON workflow_run_steps(run_id);

CREATE TABLE workflow_webhook_tokens (
    workflow_id TEXT PRIMARY KEY REFERENCES workflows(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
