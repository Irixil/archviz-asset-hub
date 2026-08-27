-- name: CreateEmbedToken :one
INSERT INTO asset_embed_tokens (id, workspace_id, asset_id, created_by, label, created_at)
VALUES (?, ?, ?, ?, ?, datetime('now'))
RETURNING *;

-- name: GetEmbedTokenByID :one
SELECT * FROM asset_embed_tokens WHERE id = ?;

-- name: GetActiveEmbedTokenByAssetID :one
SELECT * FROM asset_embed_tokens
WHERE workspace_id = ? AND asset_id = ? AND revoked_at IS NULL;

-- name: RevokeEmbedToken :execrows
UPDATE asset_embed_tokens
SET revoked_at = datetime('now')
WHERE id = ? AND workspace_id = ? AND revoked_at IS NULL;
