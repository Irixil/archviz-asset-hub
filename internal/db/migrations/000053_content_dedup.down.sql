PRAGMA foreign_keys = OFF;

CREATE TABLE ingress_log_old (
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

INSERT INTO ingress_log_old
SELECT id, source_id, remote_id, filename, asset_id,
       CASE WHEN status = 'skipped_duplicate' THEN 'skipped' ELSE status END,
       error, imported_at
FROM ingress_log;

DROP TABLE ingress_log;
ALTER TABLE ingress_log_old RENAME TO ingress_log;

CREATE INDEX idx_ingress_log_source ON ingress_log(source_id, imported_at);
CREATE INDEX idx_ingress_log_status ON ingress_log(status);

PRAGMA foreign_keys = ON;

DROP INDEX idx_versions_workspace_hash;
ALTER TABLE workspaces DROP COLUMN duplicate_detection_mode;
