-- name: CreateFolder :one
INSERT INTO folders (id, workspace_id, project_id, parent_id, name, slug, position)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetFolderByID :one
SELECT * FROM folders WHERE id = ? AND workspace_id = ?;

-- name: GetFolderChildren :many
SELECT * FROM folders WHERE parent_id = ? AND workspace_id = ? ORDER BY position, name;

-- name: UpdateFolder :one
UPDATE folders
SET
    name     = COALESCE(sqlc.narg('name'), name),
    slug     = COALESCE(sqlc.narg('slug'), slug),
    position = COALESCE(sqlc.narg('position'), position)
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: GetFolderBySlug :one
SELECT * FROM folders
WHERE workspace_id = ? AND slug = ?
ORDER BY created_at ASC
LIMIT 1;

-- name: DeleteFolder :exec
DELETE FROM folders WHERE id = ? AND workspace_id = ?;

-- name: NullifyFolderAssets :exec
UPDATE assets SET folder_id = NULL, updated_at = datetime('now')
WHERE folder_id = ? AND workspace_id = ?;
