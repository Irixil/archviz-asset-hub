CREATE TABLE folders (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    project_id   TEXT NOT NULL REFERENCES projects(id),
    parent_id    TEXT REFERENCES folders(id),
    name         TEXT NOT NULL,
    position     INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, parent_id, name)
);

ALTER TABLE assets ADD COLUMN folder_id TEXT REFERENCES folders(id);

CREATE INDEX idx_folders_project ON folders(project_id);
CREATE INDEX idx_assets_folder ON assets(folder_id);
