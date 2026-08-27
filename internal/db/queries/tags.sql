-- name: GetOrCreateTag :one
INSERT INTO tags
  (id, workspace_id, name)
VALUES
  (?, ?, ?)
ON CONFLICT
(workspace_id, name) DO
UPDATE SET name = name
RETURNING *;

-- name: GetTagByWorkspaceAndName :one
SELECT *
FROM tags
WHERE workspace_id = ? AND name = ?;

-- name: ListTagsWithCount :many
SELECT t.id, t.workspace_id, t.name, t.color, t.group_name, t.created_at, t.last_used_at,
  COUNT(at.asset_id) AS asset_count
FROM tags t
  LEFT JOIN asset_tags at ON at.tag_id = t.id
WHERE t.workspace_id = ?
  AND CASE WHEN sqlc.arg(include_system) THEN 1=1
ELSE
(t.group_name != 'system' OR t.group_name IS NULL)
END
GROUP BY t.id
ORDER BY t.name ASC;

-- name: GetTagsForAsset :many
SELECT t.id, t.workspace_id, t.name
FROM tags t
  JOIN asset_tags at ON at.tag_id = t.id
WHERE at.asset_id = ?;

-- name: AddTagToAsset :exec
INSERT OR
IGNORE INTO asset_tags (asset_id, tag_id)
VALUES
  (?, ?);

-- name: CopyAssetTags :exec
INSERT OR
IGNORE INTO asset_tags (asset_id, tag_id)
SELECT ?, tag_id
FROM asset_tags
WHERE asset_tags.asset_id = ?;

-- name: RemoveTagFromAsset :exec
DELETE FROM asset_tags
WHERE asset_id = ?
  AND tag_id = (SELECT id
  FROM tags
  WHERE workspace_id = ? AND name = ?);

-- name: TouchTagLastUsed :exec
UPDATE tags SET last_used_at = datetime('now') WHERE workspace_id = ? AND name = ?;

-- name: EnsureSystemTag :exec
INSERT INTO tags
  (id, workspace_id, name, group_name)
VALUES
  (?, ?, ?, 'system')
ON CONFLICT
(workspace_id, name) DO NOTHING;

-- name: CreateTag :one
INSERT INTO tags
  (id, workspace_id, name, color, group_name)
VALUES
  (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTagName :exec
UPDATE tags SET name = ? WHERE workspace_id = ? AND name = ?;

-- name: UpdateTagMetadata :exec
UPDATE tags SET color = ?, group_name = ? WHERE workspace_id = ? AND name = ?;

-- name: DeleteTag :exec
DELETE FROM tags WHERE workspace_id = ? AND name = ?;

-- name: ListTagsInWorkspace :many
SELECT *
FROM tags
WHERE workspace_id = ?
ORDER BY name ASC;

-- name: CountTagAssets :one
SELECT COUNT(*)
FROM asset_tags
WHERE tag_id = ?;

-- name: ReassignTagAssets :exec
INSERT OR
IGNORE INTO asset_tags (asset_id, tag_id)
SELECT src.asset_id, ?
FROM asset_tags src
WHERE src.tag_id = ?;

-- name: FindAssetBySystemTagInFolder :one
SELECT a.*
FROM assets a
  JOIN asset_tags at ON at.asset_id = a.id
  JOIN tags t ON t.id = at.tag_id
WHERE a.workspace_id = ?
  AND t.name = ?
  AND t.group_name = 'system'
  AND a.folder_id = ?
ORDER BY a.created_at ASC
LIMIT 1;

-- name: FindAssetBySystemTagInProject :one
SELECT a.*
FROM assets a
JOIN asset_tags at ON at.asset_id = a.id
JOIN tags t ON t.id
= at.tag_id
WHERE a.workspace_id = ?
  AND t.name = ?
  AND t.group_name = 'system'
  AND a.project_id = ?
ORDER BY a.created_at ASC
LIMIT 1;

-- name: FindAssetBySystemTagInWorkspace :one
SELECT a.*
FROM assets a
  JOIN asset_tags at ON at.asset_id = a.id
  JOIN tags t ON t.id = at.tag_id
WHERE a.workspace_id = ?
  AND t.name = ?
  AND t.group_name = 'system'
ORDER BY a.created_at ASC
LIMIT 1;
