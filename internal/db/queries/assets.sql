-- name: CreateAsset :one
INSERT INTO assets (
  id, workspace_id, project_id, folder_id, original_filename,
  storage_key, mime_type, size, width, height,
  thumbnail_key, thumbnail_content_type, metadata, derived_from_asset_id
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAssetByID :one
SELECT * FROM assets WHERE id = ? AND workspace_id = ?;

-- name: ListAssets :many
SELECT * FROM assets
WHERE workspace_id = sqlc.arg('workspace_id')
  AND (sqlc.narg('project_id') IS NULL OR project_id = sqlc.narg('project_id'))
  AND (sqlc.narg('mime_prefix') IS NULL OR mime_type LIKE sqlc.narg('mime_prefix'))
  AND (
    sqlc.narg('cursor_at') IS NULL
    OR created_at < sqlc.narg('cursor_at')
    OR (created_at = sqlc.narg('cursor_at') AND id < sqlc.narg('cursor_id'))
  )
ORDER BY
    CASE WHEN @order_by = 'size_asc' THEN size END ASC,
    CASE WHEN @order_by = 'size_desc' THEN size END DESC,
    CASE WHEN @order_by = 'created_at_asc' THEN created_at END ASC,
    CASE WHEN @order_by = 'created_at_desc' THEN created_at END DESC,
    CASE WHEN @order_by = 'id_asc' THEN id END ASC,
    id DESC
LIMIT sqlc.arg('limit');

-- name: UpdateAssetThumbnail :exec
UPDATE assets SET thumbnail_key = ?, thumbnail_content_type = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateAssetDimensions :exec
UPDATE assets SET width = ?, height = ?, metadata = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateAssetProject :exec
UPDATE assets SET project_id = ?, updated_at = datetime('now') WHERE id = ? AND workspace_id = ?;

-- name: UpdateAssetFolder :exec
UPDATE assets SET folder_id = ?, updated_at = datetime('now') WHERE id = ? AND workspace_id = ?;

-- name: UpdateAssetName :exec
UPDATE assets SET original_filename = ?, updated_at = datetime('now') WHERE id = ? AND workspace_id = ?;

-- name: DeleteAsset :exec
DELETE FROM assets WHERE id = ? AND workspace_id = ?;

-- name: UpdateAssetCurrentVersion :exec
UPDATE assets SET current_version_id = ?, updated_at = datetime('now') WHERE id = ?;

-- name: CountVersionsForAsset :one
SELECT COUNT(*) FROM asset_versions WHERE asset_id = ? AND deleted_at IS NULL;

-- name: ListCommentsOnAsset :many
SELECT * FROM share_comments WHERE asset_id = ? ORDER BY created_at ASC;

-- name: ListImageAssetIDs :many
SELECT id FROM assets WHERE workspace_id = ? AND mime_type LIKE 'image/%' ORDER BY created_at DESC;


-- name: TouchAsset :exec
UPDATE assets
SET touched_at = datetime('now'), updated_at = datetime('now')
WHERE id = ? AND workspace_id = ?;
