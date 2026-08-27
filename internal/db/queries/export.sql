-- name: CreateExportConfig :one
INSERT INTO export_configs (
    id, workspace_id, project_id, created_by, label,
    dest_type, dest_config, versions, include_variants,
    schedule_type, quiet_minutes, enabled
) VALUES (
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?
) RETURNING *;

-- name: GetExportConfig :one
SELECT * FROM export_configs
WHERE id = ? AND workspace_id = ?;

-- name: ListExportConfigs :many
SELECT * FROM export_configs
WHERE workspace_id = ?
ORDER BY created_at DESC;

-- name: ListExportConfigsByProject :many
SELECT * FROM export_configs
WHERE project_id = ? AND workspace_id = ?
ORDER BY created_at DESC;

-- name: UpdateExportConfig :one
UPDATE export_configs SET
    label            = ?,
    dest_type        = ?,
    dest_config      = ?,
    versions         = ?,
    include_variants = ?,
    schedule_type    = ?,
    quiet_minutes    = ?,
    enabled          = ?,
    updated_at       = datetime('now')
WHERE id = ? AND workspace_id = ?
RETURNING *;

-- name: DeleteExportConfig :exec
DELETE FROM export_configs
WHERE id = ? AND workspace_id = ?;

-- name: SetExportConfigLastRun :exec
UPDATE export_configs
SET last_run_at = ?, last_run_status = ?,
    last_error = ?, updated_at = datetime('now')
WHERE id = ?;

-- name: ListDueExportConfigs :many
SELECT ec.* FROM export_configs ec
JOIN (
    SELECT project_id, MAX(touched_at) AS last_touch
    FROM assets
    GROUP BY project_id
) pt ON pt.project_id = ec.project_id
WHERE ec.schedule_type = 'after_quiet'
  AND ec.enabled = 1
  AND datetime(pt.last_touch, '+' || ec.quiet_minutes || ' minutes') <= datetime('now')
  AND (
      ec.last_run_at IS NULL
      OR datetime(ec.last_run_at, '+' || ec.quiet_minutes || ' minutes') <= datetime('now')
  )
  AND NOT EXISTS (
      SELECT 1 FROM export_runs er
      WHERE er.export_config_id = ec.id
        AND er.status IN ('pending', 'running')
  );

-- name: CreateExportRun :one
INSERT INTO export_runs (
    id, export_config_id, workspace_id, triggered_by, status
) VALUES (
    ?, ?, ?, ?, 'pending'
) RETURNING *;

-- name: GetExportRun :one
SELECT * FROM export_runs
WHERE id = ? AND workspace_id = ?;

-- name: ListExportRuns :many
SELECT * FROM export_runs
WHERE export_config_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: StartExportRun :exec
UPDATE export_runs
SET status = 'running', started_at = datetime('now')
WHERE id = ?;

-- name: UpdateExportRunProgress :exec
UPDATE export_runs
SET assets_exported = ?,
    assets_skipped  = ?,
    bytes_written   = ?
WHERE id = ?;

-- name: FinishExportRun :exec
UPDATE export_runs
SET status          = ?,
    assets_total    = ?,
    assets_exported = ?,
    assets_skipped  = ?,
    bytes_written   = ?,
    error           = ?,
    completed_at    = datetime('now')
WHERE id = ?;

-- name: GetProjectAssetsForExport :many
SELECT
    a.id,
    a.original_filename,
    a.folder_id,
    f.name AS folder_name,
    a.touched_at,
    av.id            AS version_id,
    av.version_num,
    av.storage_key,
    av.content_hash,
    av.mime_type,
    av.size,
    av.comment,
    av.created_at    AS version_created_at,
    1                AS is_current
FROM assets a
JOIN asset_versions av ON av.id = a.current_version_id
LEFT JOIN folders f ON f.id = a.folder_id
WHERE a.project_id = ?
  AND a.workspace_id = ?
ORDER BY f.name NULLS FIRST, a.original_filename;

-- name: GetProjectAllVersionsForExport :many
SELECT
    a.id            AS asset_id,
    a.original_filename,
    a.folder_id,
    f.name          AS folder_name,
    av.id           AS version_id,
    av.version_num,
    av.storage_key,
    av.content_hash,
    av.mime_type,
    av.size,
    av.comment,
    av.created_at   AS version_created_at,
    CASE WHEN av.id = a.current_version_id THEN 1 ELSE 0 END AS is_current
FROM assets a
JOIN asset_versions av ON av.asset_id = a.id
LEFT JOIN folders f ON f.id = a.folder_id
WHERE a.project_id  = ?
  AND a.workspace_id = ?
  AND av.deleted_at IS NULL
ORDER BY f.name NULLS FIRST, a.original_filename, av.version_num;

-- name: GetVariantsForVersionIDs :many
SELECT * FROM variants
WHERE asset_version_id IN (/*SLICE:version_ids*/?)
  AND workspace_id = ?
  AND status = 'ready'
ORDER BY asset_version_id, type, title;

-- name: GetAssetTagsForProject :many
SELECT at.asset_id, t.name AS tag_name
FROM asset_tags at
JOIN tags t ON t.id = at.tag_id
JOIN assets a ON a.id = at.asset_id
WHERE a.project_id   = ?
  AND a.workspace_id = ?;

-- name: GetFieldValuesForProject :many
SELECT
    afv.asset_id,
    fd.name       AS field_name,
    fd.field_type,
    afv.value_text,
    afv.value_number,
    afv.value_date,
    afv.value_boolean
FROM asset_field_values afv
JOIN field_definitions fd ON fd.id = afv.field_id
JOIN assets a ON a.id = afv.asset_id
WHERE a.project_id   = ?
  AND a.workspace_id = ?
  AND fd.deleted_at  IS NULL;
