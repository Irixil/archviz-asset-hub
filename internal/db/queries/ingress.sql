-- ============================================================
-- ingress_sources
-- ============================================================

-- name: CreateIngressSource :one
INSERT INTO ingress_sources (
    id, workspace_id, created_by, type, label, config, public_token,
    dest_folder_id, dest_project_id, enabled, poll_interval_min
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetIngressSource :one
SELECT * FROM ingress_sources
WHERE id = ? AND workspace_id = ?;

-- name: ListIngressSources :many
SELECT * FROM ingress_sources
WHERE workspace_id = ?
ORDER BY created_at DESC;

-- name: UpdateIngressSource :one
UPDATE ingress_sources SET
    label             = ?,
    config            = ?,
    dest_folder_id    = ?,
    dest_project_id   = ?,
    enabled           = ?,
    poll_interval_min = ?,
    error_count       = 0,
    updated_at        = datetime('now')
WHERE id = ? AND workspace_id = ?
RETURNING *;

-- name: DeleteIngressSource :exec
DELETE FROM ingress_sources WHERE id = ? AND workspace_id = ?;

-- name: MarkIngressSourceScheduled :exec
UPDATE ingress_sources
SET last_polled_at = datetime('now'),
    updated_at     = datetime('now')
WHERE id = ?;

-- name: MarkIngressSourceSuccess :exec
UPDATE ingress_sources
SET last_polled_at = datetime('now'),
    last_error     = NULL,
    error_count    = 0,
    updated_at     = datetime('now')
WHERE id = ?;

-- name: MarkIngressSourceError :exec
UPDATE ingress_sources
SET last_polled_at = datetime('now'),
    last_error     = ?,
    error_count    = error_count + 1,
    updated_at     = datetime('now')
WHERE id = ?;

-- name: ListDueIngressSources :many
SELECT * FROM ingress_sources
WHERE enabled = 1
  AND error_count <= 5
  AND (
      last_polled_at IS NULL
      OR datetime(last_polled_at, '+' || poll_interval_min || ' minutes') <= datetime('now')
  )
ORDER BY last_polled_at ASC
LIMIT 20;

-- name: GetIngressSourceByPublicToken :one
SELECT * FROM ingress_sources WHERE public_token = ?;

-- name: SetWorkspaceIngestToken :exec
UPDATE workspaces SET ingest_token = ? WHERE id = ?;

-- name: GetWorkspaceByIngestToken :one
SELECT * FROM workspaces WHERE ingest_token = ?;

-- ============================================================
-- ingress_log
-- ============================================================

-- name: InsertIngressLogEntry :one
INSERT OR IGNORE INTO ingress_log (id, source_id, remote_id, filename, status)
VALUES (?, ?, ?, ?, 'pending')
RETURNING *;

-- name: GetIngressLogEntry :one
SELECT * FROM ingress_log WHERE id = ?;

-- name: UpdateIngressLogEntry :exec
UPDATE ingress_log
SET status = sqlc.arg('status'),
    asset_id = sqlc.arg('asset_id'),
    error = sqlc.arg('error'),
    duplicate_of_asset_id = sqlc.arg('duplicate_of_asset_id'),
    imported_at = datetime('now')
WHERE id = sqlc.arg('id');

-- name: ListIngressSourceLog :many
SELECT * FROM ingress_log
WHERE source_id = ?
ORDER BY imported_at DESC
LIMIT ? OFFSET ?;

-- name: ListWorkspaceIngressLog :many
SELECT l.* FROM ingress_log l
JOIN ingress_sources s ON s.id = l.source_id
WHERE s.workspace_id = sqlc.arg('workspace_id')
  AND (sqlc.narg('status') IS NULL OR l.status = sqlc.narg('status'))
ORDER BY l.imported_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: DeleteIngressLogEntry :exec
DELETE FROM ingress_log WHERE id = ?;

-- Workspace-scoped variants for repository.IngressRepository (internal/repository/sqlc/ingress.go).
-- ListWorkspaceIngressLog above is already workspace-scoped and reused as-is.

-- name: ListIngressSourceLogForWorkspace :many
SELECT l.* FROM ingress_log l
JOIN ingress_sources s ON s.id = l.source_id
WHERE l.source_id = sqlc.arg('source_id') AND s.workspace_id = sqlc.arg('workspace_id')
ORDER BY l.imported_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetIngressLogEntryForWorkspace :one
SELECT l.* FROM ingress_log l
JOIN ingress_sources s ON s.id = l.source_id
WHERE l.id = sqlc.arg('id') AND s.workspace_id = sqlc.arg('workspace_id');

-- name: UpdateIngressLogEntryForWorkspace :execrows
UPDATE ingress_log
SET status = sqlc.arg('status'), asset_id = sqlc.arg('asset_id'), error = sqlc.arg('error'),
    imported_at = datetime('now')
WHERE ingress_log.id = sqlc.arg('id')
  AND source_id IN (SELECT ingress_sources.id FROM ingress_sources WHERE workspace_id = sqlc.arg('workspace_id'));

-- name: DeleteIngressLogEntryForWorkspace :execrows
DELETE FROM ingress_log
WHERE ingress_log.id = sqlc.arg('id')
  AND source_id IN (SELECT ingress_sources.id FROM ingress_sources WHERE workspace_id = sqlc.arg('workspace_id'));

-- ============================================================
-- ingress_rules
-- ============================================================

-- name: CreateIngressRule :one
INSERT INTO ingress_rules (id, source_id, position, field, operator, value, action)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListIngressRules :many
SELECT * FROM ingress_rules
WHERE source_id = ?
ORDER BY position ASC;

-- name: UpdateIngressRule :one
UPDATE ingress_rules
SET position = ?, field = ?, operator = ?, value = ?, action = ?
WHERE id = ?
RETURNING *;

-- name: GetIngressRule :one
SELECT * FROM ingress_rules WHERE id = ?;

-- name: DeleteIngressRule :exec
DELETE FROM ingress_rules WHERE id = ?;

-- Workspace-scoped variants for repository.IngressRepository (internal/repository/sqlc/ingress.go).
-- These enforce workspace_id filtering at the repository boundary; the queries above remain in
-- use by internal/ingress/worker.go which resolves its source (and workspace) first.

-- name: CreateIngressRuleForWorkspace :one
INSERT INTO ingress_rules (id, source_id, position, field, operator, value, action)
SELECT sqlc.arg('id'), sqlc.arg('source_id'), sqlc.arg('position'), sqlc.arg('field'),
       sqlc.arg('operator'), sqlc.arg('value'), sqlc.arg('action')
WHERE EXISTS (SELECT 1 FROM ingress_sources WHERE id = sqlc.arg('source_id') AND workspace_id = sqlc.arg('workspace_id'))
RETURNING *;

-- name: ListIngressRulesForWorkspace :many
SELECT r.* FROM ingress_rules r
JOIN ingress_sources s ON s.id = r.source_id
WHERE r.source_id = sqlc.arg('source_id') AND s.workspace_id = sqlc.arg('workspace_id')
ORDER BY r.position ASC;

-- name: GetIngressRuleForWorkspace :one
SELECT r.* FROM ingress_rules r
JOIN ingress_sources s ON s.id = r.source_id
WHERE r.id = sqlc.arg('id') AND s.workspace_id = sqlc.arg('workspace_id');

-- name: UpdateIngressRuleForWorkspace :one
UPDATE ingress_rules
SET position = sqlc.arg('position'), field = sqlc.arg('field'), operator = sqlc.arg('operator'),
    value = sqlc.arg('value'), action = sqlc.arg('action')
WHERE ingress_rules.id = sqlc.arg('id')
  AND source_id IN (SELECT ingress_sources.id FROM ingress_sources WHERE workspace_id = sqlc.arg('workspace_id'))
RETURNING *;

-- name: DeleteIngressRuleForWorkspace :execrows
DELETE FROM ingress_rules
WHERE ingress_rules.id = sqlc.arg('id')
  AND source_id IN (SELECT ingress_sources.id FROM ingress_sources WHERE workspace_id = sqlc.arg('workspace_id'));
