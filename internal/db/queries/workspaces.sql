-- name: CreateWorkspace :one
INSERT INTO workspaces
  (id, name, created_at, updated_at)
VALUES
  (?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: GetWorkspaceByID :one
SELECT *
FROM workspaces
WHERE id = ? LIMIT 1;

-- name: GetWorkspaceAIProviderKey :one
SELECT CASE
    WHEN  @provider_name = 'imagerouter'  THEN COALESCE(imagerouter_api_key_enc, '')
    WHEN  @provider_name = 'openrouter'   THEN COALESCE(openrouter_api_key_enc, '')
    ELSE ''
END AS api_key_enc
FROM workspaces
WHERE id = ?1
LIMIT 1;

-- name: ListWorkspacesWithRetention :many
SELECT *
FROM workspaces
WHERE version_retention_count > 0;

-- name: GetWorkspaceByIconAsset :one
SELECT *
FROM workspaces
WHERE icon_asset_id = ? AND id = ? LIMIT 1;

-- name: UpdateWorkspaceVersionRetention :exec
UPDATE workspaces SET version_retention_count = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateWorkspaceExifSettings :exec
UPDATE workspaces SET exif_keep = ?, exif_keep_gps = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateWorkspaceLockedTaxonomy :exec
UPDATE workspaces SET locked_taxonomy = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateWorkspaceDuplicateDetectionMode :exec
UPDATE workspaces SET duplicate_detection_mode = ?, updated_at = datetime('now') WHERE id = ?;

-- name: SetWorkspaceOpenRouterKey :exec
UPDATE workspaces SET openrouter_api_key_enc = ?, updated_at = datetime('now') WHERE id = ?;

-- name: SetWorkspaceImageRouterKey :exec
UPDATE workspaces SET imagerouter_api_key_enc = ?, updated_at = datetime('now') WHERE id = ?;


-- name: CountWorkspaceAssets :one
SELECT COUNT(*)
FROM assets
WHERE workspace_id = ?;

-- name: GetWorkspaceStorageLimitBytes :one
SELECT storage_limit_bytes
FROM workspaces
WHERE id = ?;

-- name: GetWorkspaceStorageVersionsBytes :one
SELECT COALESCE(SUM(av.size), 0) AS total
FROM asset_versions av
WHERE av.workspace_id = ? AND av.deleted_at IS NULL;

-- name: GetWorkspaceStorageVariantsBytes :one
SELECT COALESCE(SUM(size), 0) AS total
FROM variants
WHERE workspace_id = ? AND size IS NOT NULL;

-- name: GetStorageByProjectAndType :many
SELECT
  a.project_id,
  COALESCE(p.name, '') AS project_name,
  CASE
    WHEN a.mime_type LIKE 'image/%'                                   THEN 'image'
    WHEN a.mime_type LIKE 'video/%'                                   THEN 'video'
    WHEN a.mime_type LIKE 'audio/%'                                   THEN 'audio'
    WHEN a.mime_type = 'application/pdf' OR a.mime_type LIKE 'text/%' THEN 'document'
    ELSE                                                                   'other'
  END AS asset_type,
  COALESCE(SUM(av.size), 0)                              AS versions_bytes,
  COALESCE(SUM(COALESCE(vs.variant_bytes, 0)), 0)        AS variants_bytes
FROM asset_versions av
  JOIN assets a ON a.id = av.asset_id
  LEFT JOIN projects p ON p.id = a.project_id
  LEFT JOIN (
  SELECT vv.asset_version_id, SUM(vv.size) AS variant_bytes
  FROM variants vv
  WHERE vv.workspace_id = ? AND vv.size IS NOT NULL
  GROUP BY vv.asset_version_id
) vs ON vs.asset_version_id = av.id
WHERE av.workspace_id = ? AND av.deleted_at IS NULL
GROUP BY a.project_id, asset_type;

-- name: GetFolderCountsByProject :many
SELECT a.project_id, COUNT(DISTINCT a.folder_id) AS folder_count
FROM assets a
  JOIN folders f ON f.id = a.folder_id AND f.project_id = a.project_id
WHERE a.workspace_id = ?
GROUP BY a.project_id;

-- name: GetStorageByFolder :many
SELECT
  CASE WHEN f.id IS NOT NULL THEN a.folder_id ELSE NULL END AS folder_id,
  COALESCE(f.name, '')                                      AS folder_name,
  COALESCE(SUM(av.size), 0)                                 AS versions_bytes,
  COALESCE(SUM(COALESCE(vs.variant_bytes, 0)), 0)           AS variants_bytes
FROM asset_versions av
  JOIN assets a ON a.id = av.asset_id
  LEFT JOIN folders f ON f.id = a.folder_id AND f.project_id = a.project_id
  LEFT JOIN (
  SELECT vv.asset_version_id, SUM(vv.size) AS variant_bytes
  FROM variants vv
  WHERE vv.workspace_id = ? AND vv.size IS NOT NULL
  GROUP BY vv.asset_version_id
) vs ON vs.asset_version_id = av.id
WHERE av.workspace_id = ? AND a.project_id = ? AND av.deleted_at IS NULL
GROUP BY CASE WHEN f.id IS NOT NULL THEN a.folder_id ELSE NULL END;
