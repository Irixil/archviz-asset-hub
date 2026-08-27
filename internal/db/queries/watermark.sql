-- name: FindWatermarkAssetInFolder :one
SELECT * FROM assets
WHERE workspace_id = ?
  AND folder_id = ?
  AND LOWER(original_filename) LIKE '%watermark%'
ORDER BY created_at ASC
LIMIT 1;

-- name: FindWatermarkAssetInProject :one
SELECT a.* FROM assets a
JOIN folders f ON a.folder_id = f.id
WHERE a.workspace_id = ?
  AND f.project_id = ?
  AND LOWER(a.original_filename) LIKE '%watermark%'
ORDER BY a.created_at ASC
LIMIT 1;

-- name: FindWatermarkAssetInWorkspace :one
SELECT * FROM assets
WHERE workspace_id = ?
  AND LOWER(original_filename) LIKE '%watermark%'
ORDER BY created_at ASC
LIMIT 1;
