-- name: CreateAutoTagSuggestion :one
INSERT INTO auto_tag_suggestions (
    id, workspace_id, asset_id, asset_version_id, tag_name
) VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListAutoTagSuggestions :many
SELECT * FROM auto_tag_suggestions
WHERE asset_id = ? AND workspace_id = ?
ORDER BY created_at ASC;

-- name: GetAutoTagSuggestion :one
SELECT * FROM auto_tag_suggestions
WHERE id = ? AND workspace_id = ?;

-- name: DeleteAutoTagSuggestion :exec
DELETE FROM auto_tag_suggestions
WHERE id = ? AND workspace_id = ?;

-- name: DeleteAutoTagSuggestionsByAsset :exec
DELETE FROM auto_tag_suggestions
WHERE asset_id = ? AND workspace_id = ?;

-- name: UpdateWorkspaceAutoTagSettings :exec
UPDATE workspaces
SET auto_tag_enabled = ?,
    auto_tag_mode    = ?,
    updated_at       = datetime('now')
WHERE id = ?;
