-- name: UpsertVisualSimilarityHash :exec
INSERT INTO asset_visual_similarity_hashes (asset_version_id, workspace_id, central_hash, hash_set)
VALUES (?, ?, ?, ?)
ON CONFLICT(asset_version_id) DO UPDATE SET
    central_hash = excluded.central_hash,
    hash_set     = excluded.hash_set;

-- name: GetVisualSimilarityHash :one
SELECT * FROM asset_visual_similarity_hashes WHERE asset_version_id = ?;

-- name: ListVersionsWithoutVisualSimilarityHash :many
SELECT av.id, av.workspace_id, av.storage_key, av.mime_type
FROM asset_versions av
LEFT JOIN asset_visual_similarity_hashes vsh ON vsh.asset_version_id = av.id
WHERE av.workspace_id = ?
  AND av.mime_type LIKE 'image/%'
  AND vsh.asset_version_id IS NULL;
