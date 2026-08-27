-- name: ListAssetEvents :many
SELECT ae.id, ae.workspace_id, ae.asset_id, ae.user_id, ae.actor_type, ae.event_type, ae.payload, ae.created_at,
       u.name AS user_name
FROM asset_events ae
LEFT JOIN users u ON u.id = ae.user_id
WHERE ae.asset_id = sqlc.arg('asset_id')
  AND ae.workspace_id = sqlc.arg('workspace_id')
  AND (sqlc.narg('cursor') IS NULL OR ae.created_at < sqlc.narg('cursor'))
  AND (sqlc.narg('event_type') IS NULL OR ae.event_type = sqlc.narg('event_type'))
ORDER BY ae.created_at DESC
LIMIT sqlc.arg('limit');

-- name: ListProjectEvents :many
SELECT pe.id, pe.workspace_id, pe.project_id, pe.user_id, pe.actor_type, pe.event_type, pe.payload, pe.created_at,
       u.name AS user_name
FROM project_events pe
LEFT JOIN users u ON u.id = pe.user_id
WHERE pe.project_id = sqlc.arg('project_id')
  AND pe.workspace_id = sqlc.arg('workspace_id')
  AND (sqlc.narg('cursor') IS NULL OR pe.created_at < sqlc.narg('cursor'))
  AND (sqlc.narg('event_type') IS NULL OR pe.event_type = sqlc.narg('event_type'))
ORDER BY pe.created_at DESC
LIMIT sqlc.arg('limit');

-- name: ListWorkspaceAssetEvents :many
SELECT ae.id, ae.workspace_id, ae.asset_id, ae.user_id, ae.actor_type, ae.event_type, ae.payload, ae.created_at,
       u.name AS user_name
FROM asset_events ae
LEFT JOIN users u ON u.id = ae.user_id
WHERE ae.workspace_id = sqlc.arg('workspace_id')
  AND (sqlc.narg('cursor') IS NULL OR ae.created_at < sqlc.narg('cursor'))
  AND (sqlc.narg('user_id') IS NULL OR ae.user_id = sqlc.narg('user_id'))
  AND (sqlc.narg('event_type') IS NULL OR ae.event_type = sqlc.narg('event_type'))
ORDER BY ae.created_at DESC
LIMIT sqlc.arg('limit');

-- name: ListWorkspaceProjectEvents :many
SELECT pe.id, pe.workspace_id, pe.project_id, pe.user_id, pe.actor_type, pe.event_type, pe.payload, pe.created_at,
       u.name AS user_name
FROM project_events pe
LEFT JOIN users u ON u.id = pe.user_id
WHERE pe.workspace_id = sqlc.arg('workspace_id')
  AND (sqlc.narg('cursor') IS NULL OR pe.created_at < sqlc.narg('cursor'))
  AND (sqlc.narg('user_id') IS NULL OR pe.user_id = sqlc.narg('user_id'))
  AND (sqlc.narg('event_type') IS NULL OR pe.event_type = sqlc.narg('event_type'))
ORDER BY pe.created_at DESC
LIMIT sqlc.arg('limit');

-- name: DeleteAssetEventsOlderThan :exec
DELETE FROM asset_events
WHERE workspace_id = sqlc.arg('workspace_id')
  AND created_at < sqlc.arg('cutoff');

-- name: DeleteDownloadEventsOlderThan :exec
DELETE FROM asset_events
WHERE workspace_id = sqlc.arg('workspace_id')
  AND event_type = 'asset_downloaded'
  AND created_at < sqlc.arg('cutoff');

-- name: DeleteProjectEventsOlderThan :exec
DELETE FROM project_events
WHERE workspace_id = sqlc.arg('workspace_id')
  AND created_at < sqlc.arg('cutoff');

-- name: ListWorkspacesForEventRetention :many
SELECT id, name, ingest_token, version_retention_count, event_log_retention_days, download_log_retention_days, icon_asset_id, icon_version_id, created_at, updated_at
FROM workspaces
WHERE event_log_retention_days > 0 OR download_log_retention_days > 0;
