-- name: CreateOAuthConnection :one
INSERT INTO oauth_connections (
    id, workspace_id, created_by, provider, provider_user_id, provider_email,
    scopes, access_token, refresh_token, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetOAuthConnectionByID :one
SELECT * FROM oauth_connections
WHERE id = ? AND workspace_id = ?;

-- name: GetOAuthConnectionByProvider :many
SELECT * FROM oauth_connections
WHERE workspace_id = ? AND provider = ?
ORDER BY created_at DESC;

-- name: GetOAuthConnectionByProviderUserID :one
SELECT * FROM oauth_connections
WHERE workspace_id = ? AND provider = ? AND provider_user_id = ?;

-- name: ListOAuthConnectionsByWorkspace :many
SELECT * FROM oauth_connections
WHERE workspace_id = ?
ORDER BY provider, created_at DESC;

-- name: UpdateOAuthConnectionTokens :one
UPDATE oauth_connections SET
    access_token  = ?,
    refresh_token = ?,
    expires_at    = ?,
    updated_at    = datetime('now')
WHERE id = ?
RETURNING *;

-- name: UpdateOAuthConnectionTokensAndScopes :one
UPDATE oauth_connections SET
    access_token  = ?,
    refresh_token = ?,
    expires_at    = ?,
    scopes        = ?,
    updated_at    = datetime('now')
WHERE id = ?
RETURNING *;

-- name: DeleteOAuthConnection :exec
DELETE FROM oauth_connections WHERE id = ? AND workspace_id = ?;
