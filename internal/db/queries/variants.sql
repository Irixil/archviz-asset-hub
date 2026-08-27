-- name: CreateVariant :one
INSERT INTO variants (id, workspace_id, asset_version_id, type, storage_key, transform_params, size, status, content_hash)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: SetVariantThumbnail :exec
UPDATE variants SET thumbnail_key = ?, thumbnail_content_type = ? WHERE id = ?;

-- name: GetVariantByID :one
SELECT * FROM variants WHERE id = ? AND workspace_id = ?;

-- name: ListVariantsByVersion :many
-- All variants for a specific asset_version_id, ordered newest first.
SELECT * FROM variants WHERE asset_version_id = ? ORDER BY created_at DESC;

-- name: ListVariantsByAssetCurrentVersion :many
-- Variants for the current version of an asset (JOIN to asset_versions).
SELECT v.*
FROM variants v
JOIN asset_versions av ON av.id = v.asset_version_id
WHERE av.asset_id = ? AND av.is_current = 1
ORDER BY v.created_at DESC;

-- name: GetVariantByTypeAndParams :one
-- Dedup check: find variant by version + type + transform_params hash.
SELECT * FROM variants
WHERE asset_version_id = ? AND type = ? AND transform_params = ?
LIMIT 1;

-- name: CopyVariantParamsByVersion :many
-- Returns {type, transform_params} for a given version. Used by rebuild job.
-- Excludes manual variants (type = 'manual') since they are version-specific.
SELECT type, transform_params FROM variants
WHERE asset_version_id = ? AND type != 'manual';

-- name: CountVariantsByVersion :one
SELECT COUNT(*) FROM variants WHERE asset_version_id = ?;

-- name: DeleteVariant :exec
DELETE FROM variants WHERE id = ? AND workspace_id = ?;

-- name: DeleteVariantsByVersion :exec
DELETE FROM variants WHERE asset_version_id = ?;

-- name: MarkVariantPending :exec
UPDATE variants
SET storage_key = '',
    transform_params = ?,
    size = NULL,
    status = 'pending',
    thumbnail_key = NULL,
    thumbnail_content_type = 'image/jpeg'
WHERE id = ? AND workspace_id = ?;

-- name: UpdateVariantResult :exec
UPDATE variants
SET storage_key = ?,
    transform_params = ?,
    size = ?,
    status = 'ready'
WHERE id = ? AND workspace_id = ?;

-- name: SetVariantStatus :exec
UPDATE variants SET status = ? WHERE id = ? AND workspace_id = ?;

-- name: CreateVariantFull :one
INSERT INTO variants (id, workspace_id, asset_version_id, type, storage_key, transform_params, size, status, title, is_shared, content_hash)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;
