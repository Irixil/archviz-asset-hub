-- name: CreateCollection :one
INSERT INTO collections (id, workspace_id, name, description, created_by)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetCollection :one
SELECT * FROM collections WHERE id = ? AND workspace_id = ?;

-- name: ListCollections :many
SELECT c.*, COUNT(ca.asset_id) AS asset_count
FROM collections c
LEFT JOIN collection_assets ca ON ca.collection_id = c.id
WHERE c.workspace_id = ?
GROUP BY c.id
ORDER BY c.created_at DESC;

-- name: UpdateCollection :one
UPDATE collections
SET name = ?, description = ?, updated_at = datetime('now')
WHERE id = ? AND workspace_id = ?
RETURNING *;

-- name: DeleteCollection :exec
DELETE FROM collections WHERE id = ? AND workspace_id = ?;

-- name: AddCollectionAsset :exec
INSERT OR IGNORE INTO collection_assets (collection_id, asset_id, position)
VALUES (?, ?, (SELECT COALESCE(MAX(ca2.position), -1) + 1 FROM collection_assets ca2 WHERE ca2.collection_id = ?));

-- name: RemoveCollectionAsset :exec
DELETE FROM collection_assets WHERE collection_id = ? AND asset_id = ?;

-- name: ListCollectionAssets :many
SELECT a.*
FROM assets a
JOIN collection_assets ca ON ca.asset_id = a.id
WHERE ca.collection_id = ?
ORDER BY ca.position ASC, ca.added_at ASC;

-- name: CollectionBelongsToWorkspace :one
SELECT COUNT(*) FROM collections WHERE id = ? AND workspace_id = ?;

-- name: ListCollectionsForAsset :many
SELECT c.*, COUNT(ca2.asset_id) AS asset_count
FROM collections c
JOIN collection_assets ca ON ca.collection_id = c.id AND ca.asset_id = ?
LEFT JOIN collection_assets ca2 ON ca2.collection_id = c.id
WHERE c.workspace_id = ?
GROUP BY c.id
ORDER BY c.created_at DESC;
