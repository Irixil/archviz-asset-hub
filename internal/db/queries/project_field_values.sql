-- name: UpsertProjectFieldValue :one
INSERT INTO project_field_values (id, project_id, field_id, value_text, value_number, value_date, value_boolean, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(project_id, field_id) DO UPDATE SET
  value_text    = excluded.value_text,
  value_number  = excluded.value_number,
  value_date    = excluded.value_date,
  value_boolean = excluded.value_boolean,
  updated_at    = datetime('now')
RETURNING *;

-- name: GetProjectFieldValues :many
SELECT
  v.id,
  v.project_id,
  v.field_id,
  v.value_text,
  v.value_number,
  v.value_date,
  v.value_boolean,
  v.created_by,
  v.created_at,
  v.updated_at,
  f.key        AS field_key,
  f.name       AS field_name,
  f.field_type AS field_type,
  f.options    AS field_options,
  CASE WHEN f.deleted_at IS NOT NULL THEN 1 ELSE 0 END AS definition_deleted
FROM project_field_values v
JOIN field_definitions f ON f.id = v.field_id
WHERE v.project_id = ?
ORDER BY f.position ASC, f.created_at ASC;

-- name: GetProjectFieldValue :one
SELECT v.* FROM project_field_values v
WHERE v.project_id = ? AND v.field_id = ?;

-- name: DeleteProjectFieldValue :exec
DELETE FROM project_field_values WHERE project_id = ? AND field_id = ?;

-- name: DeleteProjectFieldValuesByField :exec
DELETE FROM project_field_values WHERE field_id = ?;
