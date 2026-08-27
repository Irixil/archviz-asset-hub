-- name: CreateInvite :one
INSERT INTO workspace_invites (id, workspace_id, email, token, role, invited_by, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
RETURNING *;

-- name: GetInviteByToken :one
SELECT * FROM workspace_invites
WHERE token = ? AND accepted_at IS NULL AND expires_at > datetime('now')
LIMIT 1;

-- name: AcceptInvite :exec
UPDATE workspace_invites
SET accepted_at = datetime('now')
WHERE id = ?;

-- name: ListPendingInvites :many
SELECT * FROM workspace_invites
WHERE workspace_id = ? AND accepted_at IS NULL AND expires_at > datetime('now')
ORDER BY created_at DESC;

-- name: DeleteInvite :exec
DELETE FROM workspace_invites WHERE id = ? AND workspace_id = ?;
