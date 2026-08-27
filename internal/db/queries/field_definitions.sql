-- name: CreateFieldDefinition :one
INSERT INTO field_definitions (id, workspace_id, created_by, scope, name, key, field_type, options, required, position, inherit_from_project)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetFieldDefinitionByID :one
SELECT * FROM field_definitions WHERE id = ? AND workspace_id = ?;

-- name: ListFieldDefinitions :many
SELECT * FROM field_definitions
WHERE workspace_id = ? AND scope = ? AND source = 'user' AND deleted_at IS NULL
ORDER BY position ASC, created_at ASC;

-- name: ListAllFieldDefinitions :many
SELECT * FROM field_definitions
WHERE workspace_id = ? AND scope = ? AND source = 'user'
ORDER BY position ASC, created_at ASC;

-- name: UpdateFieldDefinition :one
UPDATE field_definitions
SET name                 = COALESCE(sqlc.narg('name'), name),
    options              = COALESCE(sqlc.narg('options'), options),
    required             = COALESCE(sqlc.narg('required'), required),
    position             = COALESCE(sqlc.narg('position'), position),
    inherit_from_project = COALESCE(sqlc.narg('inherit_from_project'), inherit_from_project),
    updated_at           = datetime('now')
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id') AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteFieldDefinition :exec
UPDATE field_definitions
SET deleted_at = datetime('now'), updated_at = datetime('now')
WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL;

-- name: UpdateFieldDefinitionPosition :exec
UPDATE field_definitions SET position = ?, updated_at = datetime('now')
WHERE id = ? AND workspace_id = ?;

-- name: CountFieldDefinitions :one
SELECT COUNT(*) FROM field_definitions
WHERE workspace_id = ? AND scope = ? AND source = 'user' AND deleted_at IS NULL;

-- name: HardDeleteExpiredFieldDefinitions :many
SELECT id FROM field_definitions
WHERE deleted_at IS NOT NULL AND deleted_at < datetime('now', '-30 days');

-- name: HardDeleteFieldDefinition :exec
DELETE FROM field_definitions WHERE id = ?;

-- name: CountFieldDefinitionAssetValues :one
SELECT COUNT(DISTINCT v.asset_id) FROM asset_field_values v WHERE v.field_id = ?;

-- name: CountFieldDefinitionProjectValues :one
SELECT COUNT(DISTINCT v.project_id) FROM project_field_values v WHERE v.field_id = ?;

-- name: ListInheritableAssetFieldDefinitions :many
SELECT * FROM field_definitions
WHERE workspace_id = ? AND scope = 'asset' AND inherit_from_project = 1 AND deleted_at IS NULL;

-- name: GetSystemFieldsBySource :many
SELECT * FROM field_definitions
WHERE workspace_id = ? AND source = ? AND deleted_at IS NULL
ORDER BY position ASC;

-- name: InsertSystemFieldDefinition :exec
INSERT OR IGNORE INTO field_definitions
  (id, workspace_id, created_by, source, scope, name, key, field_type,
   required, position, created_at, updated_at)
VALUES (?, ?, NULL, ?, 'asset', ?, ?, ?, 0, ?, datetime('now'), datetime('now'));

-- name: GetFieldDefinitionByKey :one
SELECT * FROM field_definitions
WHERE workspace_id = ? AND key = ? AND deleted_at IS NULL LIMIT 1;

-- name: ListAssetsMissingExifField :many
SELECT a.id FROM assets a
LEFT JOIN asset_field_values afv ON afv.asset_id = a.id AND afv.field_id = ?
WHERE a.workspace_id = ? AND a.mime_type LIKE 'image/%' AND afv.id IS NULL
LIMIT ?;
