-- name: CreateUser :one
INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ? AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByOIDC :one
SELECT * FROM users WHERE oidc_issuer = ? AND oidc_sub = ? AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByGoogleID :one
SELECT * FROM users WHERE google_user_id = ? AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByCanvaID :one
SELECT * FROM users WHERE canva_user_id = ? AND deleted_at IS NULL LIMIT 1;

-- name: LinkOIDC :one
UPDATE users SET oidc_issuer = ?, oidc_sub = ?, avatar_url = COALESCE(?, avatar_url),
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: LinkGoogle :one
UPDATE users SET google_user_id = ?, avatar_url = COALESCE(?, avatar_url),
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: LinkCanva :one
UPDATE users SET canva_user_id = ?, avatar_url = COALESCE(?, avatar_url),
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: UnlinkOIDC :one
UPDATE users SET oidc_issuer = NULL, oidc_sub = NULL,
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: UnlinkGoogle :one
UPDATE users SET google_user_id = NULL,
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: UnlinkCanva :one
UPDATE users SET canva_user_id = NULL,
    auth_methods = ?, updated_at = datetime('now')
WHERE id = ? RETURNING *;

-- name: CreateUserWithOIDC :one
INSERT INTO users (id, email, password_hash, name, oidc_issuer, oidc_sub, avatar_url, auth_methods, created_at, updated_at)
VALUES (?, ?, '', ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: CreateUserWithGoogle :one
INSERT INTO users (id, email, password_hash, name, google_user_id, avatar_url, auth_methods, created_at, updated_at)
VALUES (?, ?, '', ?, ?, ?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: CreateUserWithCanva :one
INSERT INTO users (id, email, password_hash, name, canva_user_id, avatar_url, auth_methods, created_at, updated_at)
VALUES (?, ?, '', ?, ?, ?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: UpdateUserProfile :one
UPDATE users
SET display_name = ?, updated_at = datetime('now')
WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserAvatarKey :one
UPDATE users
SET avatar_storage_key = ?, updated_at = datetime('now')
WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: ClearUserAvatarKey :one
UPDATE users
SET avatar_storage_key = NULL, updated_at = datetime('now')
WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: SetUserPassword :exec
UPDATE users
SET password_hash = ?, updated_at = datetime('now')
WHERE id = ? AND deleted_at IS NULL;

-- name: SetUserAuthMethods :one
UPDATE users
SET auth_methods = ?, updated_at = datetime('now')
WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: SetUserPendingEmail :exec
UPDATE users
SET pending_email = ?, updated_at = datetime('now')
WHERE id = ? AND deleted_at IS NULL;

-- name: ClearUserPendingEmail :exec
UPDATE users
SET pending_email = NULL, updated_at = datetime('now')
WHERE id = ?;

-- name: ConfirmUserEmailChange :one
UPDATE users
SET email = pending_email, pending_email = NULL, updated_at = datetime('now')
WHERE id = ? AND pending_email = ? AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteUser :exec
UPDATE users
SET deleted_at = datetime('now'), updated_at = datetime('now')
WHERE id = ? AND deleted_at IS NULL;

-- name: AnonymizeDeletedUser :exec
UPDATE users
SET email = 'deleted_' || id || '@deleted.invalid',
    display_name = 'Deleted user',
    password_hash = '',
    avatar_storage_key = NULL,
    avatar_url = NULL,
    pending_email = NULL,
    auth_methods = '[]',
    updated_at = datetime('now')
WHERE id = ?;

-- name: HardDeleteUser :exec
DELETE FROM users WHERE id = ?;
