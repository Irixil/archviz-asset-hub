-- name: CreateAssetVersion :one
INSERT INTO asset_versions (
  id, asset_id, workspace_id, version_num, storage_key, content_hash,
  mime_type, size, width, height, duration_sec, thumbnail_key, comment, created_by, is_current
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetCurrentVersion :one
SELECT * FROM asset_versions
WHERE asset_id = ? AND is_current = 1 AND deleted_at IS NULL
LIMIT 1;

-- name: GetVersionByID :one
SELECT * FROM asset_versions WHERE id = ? AND workspace_id = ?;

-- name: GetVersionByNum :one
SELECT * FROM asset_versions WHERE asset_id = ? AND version_num = ?;

-- name: ListVersions :many
SELECT * FROM asset_versions
WHERE asset_id = ? AND deleted_at IS NULL
ORDER BY version_num DESC;

-- name: ListAllVersions :many
SELECT * FROM asset_versions
WHERE asset_id = ?
ORDER BY version_num DESC;

-- name: GetVersionByHash :one
SELECT * FROM asset_versions
WHERE asset_id = ? AND content_hash = ? AND deleted_at IS NULL
ORDER BY version_num DESC
LIMIT 1;

-- name: SoftDeleteVersion :exec
UPDATE asset_versions
SET deleted_at = datetime('now')
WHERE id = ?;

-- name: CountActiveVersions :one
SELECT COUNT(*) FROM asset_versions
WHERE asset_id = ? AND deleted_at IS NULL;

-- name: ListVersionsBeyondRetention :many
SELECT * FROM asset_versions
WHERE asset_id = ?
  AND deleted_at IS NULL
  AND is_current = 0
ORDER BY version_num DESC
LIMIT -1 OFFSET ?;

-- name: SetVersionThumbnail :exec
UPDATE asset_versions SET thumbnail_key = ?, thumbnail_content_type = ? WHERE id = ?;

-- name: HardDeleteVersion :exec
DELETE FROM asset_versions WHERE id = ?;

-- name: ListAssetsWithVersions :many
SELECT DISTINCT asset_id FROM asset_versions WHERE workspace_id = ? AND deleted_at IS NULL;

-- name: GetVersionByIDUnchecked :one
SELECT * FROM asset_versions WHERE id = ?;

-- name: IsVersionReferencedAsCover :one
SELECT
  (SELECT COUNT(*) FROM projects  WHERE cover_version_id = ?) +
  (SELECT COUNT(*) FROM workspaces WHERE icon_version_id  = ?)
AS ref_count;

-- name: ClearCurrentVersionFlags :exec
UPDATE asset_versions SET is_current = 0 WHERE asset_id = ?;

-- name: SetCurrentVersionFlag :exec
UPDATE asset_versions SET is_current = 1 WHERE id = ?;

-- name: GetFirstVersion :one
SELECT * FROM asset_versions
WHERE asset_id = ? AND deleted_at IS NULL
ORDER BY version_num ASC
LIMIT 1;

-- name: ListVersionsWithVariantCount :many
-- Versions for an asset with per-version variant count (for History tab VV-4.2).
SELECT av.*, COUNT(v.id) AS variant_count
FROM asset_versions av
LEFT JOIN variants v ON v.asset_version_id = av.id
WHERE av.asset_id = ? AND av.deleted_at IS NULL
GROUP BY av.id
ORDER BY av.version_num DESC;

-- name: FindVersionsByContentHash :many
-- Every version in the workspace matching the given hash, ranked so the most
-- actionable match (live/non-deleted, then most recently created) comes
-- first, INCLUDING soft-deleted versions. Caller excludes the asset currently
-- being created/uploaded (excludeAssetID) since sqlc doesn't make conditional
-- exclusion clean and the exclude list is always exactly one asset in
-- practice. Asset-level details (filename, thumbnail, project) are resolved
-- by the caller via a separate GetAssetByID lookup on the top match.
SELECT
    id AS version_id,
    asset_id,
    version_num,
    storage_key,
    is_current,
    deleted_at,
    created_at
FROM asset_versions
WHERE workspace_id = sqlc.arg('workspace_id')
  AND content_hash = sqlc.arg('content_hash')
  AND asset_id != sqlc.arg('exclude_asset_id')
ORDER BY is_current DESC, deleted_at IS NOT NULL ASC, created_at DESC;

