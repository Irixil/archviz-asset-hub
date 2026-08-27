-- name: CreateProject :one
INSERT INTO projects (id, workspace_id, name, description, color, cover_asset_id)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetProjectByID :one
SELECT * FROM projects WHERE id = ? AND workspace_id = ?;

-- name: ListProjectsWithCount :many
SELECT p.*, COUNT(a.id) AS asset_count
FROM projects p
LEFT JOIN assets a ON a.project_id = p.id
WHERE p.workspace_id = ?
GROUP BY p.id
ORDER BY p.name ASC;

-- name: UpdateProject :one
UPDATE projects
SET name           = COALESCE(sqlc.narg('name'), name),
    description    = COALESCE(sqlc.narg('description'), description),
    color          = COALESCE(sqlc.narg('color'), color),
    cover_asset_id = COALESCE(sqlc.narg('cover_asset_id'), cover_asset_id),
    updated_at     = datetime('now')
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: GetProjectByCoverAsset :one
SELECT * FROM projects WHERE cover_asset_id = ? AND workspace_id = ? LIMIT 1;

-- name: NullifyProjectAssets :exec
UPDATE assets SET project_id = NULL, updated_at = datetime('now')
WHERE project_id = ? AND workspace_id = ?;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = ? AND workspace_id = ?;
