-- name: CreateShare :one
INSERT INTO shares (id, workspace_id, created_by, label, target_type, target_id, password_hash, expires_at, allow_comments, allow_download)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetShareByID :one
SELECT * FROM shares WHERE id = ?;

-- name: GetShareByIDAndWorkspace :one
SELECT * FROM shares WHERE id = ? AND workspace_id = ?;

-- name: ListSharesByWorkspace :many
SELECT * FROM shares WHERE workspace_id = ? ORDER BY created_at DESC;

-- name: UpdateShare :one
UPDATE shares
SET
  label          = sqlc.arg('label'),
  password_hash  = sqlc.narg('password_hash'),
  expires_at     = sqlc.narg('expires_at'),
  allow_comments = sqlc.arg('allow_comments'),
  allow_download = sqlc.arg('allow_download')
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: RevokeShare :exec
UPDATE shares SET revoked_at = datetime('now') WHERE id = ? AND workspace_id = ?;

-- name: IncrementShareViewCount :exec
UPDATE shares SET view_count = view_count + 1 WHERE id = ?;

-- name: CreateComment :one
INSERT INTO share_comments (id, share_id, asset_id, author_name, author_email, body)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListCommentsByShare :many
SELECT * FROM share_comments WHERE share_id = ? ORDER BY created_at ASC;

-- name: ListCommentsByShareAndAsset :many
SELECT * FROM share_comments WHERE share_id = ? AND asset_id = ? ORDER BY created_at ASC;

-- name: DeleteComment :exec
DELETE FROM share_comments WHERE id = ? AND share_id = ?;
