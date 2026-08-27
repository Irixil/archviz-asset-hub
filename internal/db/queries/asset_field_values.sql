-- name: UpsertAssetFieldValue :one
INSERT INTO asset_field_values
  (id, asset_id, field_id, value_text, value_number, value_date, value_boolean, created_by)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT
(asset_id, field_id) DO
UPDATE SET
  value_text    = excluded.value_text,
  value_number  = excluded.value_number,
  value_date    = excluded.value_date,
  value_boolean = excluded.value_boolean,
  updated_at    = datetime('now')
RETURNING *;

-- name: GetAssetFieldValues :many
SELECT
  v.id,
  v.asset_id,
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
  f.source     AS field_source,
  f.options    AS field_options,
  CASE WHEN f.deleted_at IS NOT NULL THEN 1 ELSE 0 END AS definition_deleted
FROM asset_field_values v
  JOIN field_definitions f ON f.id = v.field_id
WHERE v.asset_id = ?
ORDER BY f.position ASC, f.created_at ASC;

-- name: CopyAssetFieldValues :exec
INSERT OR
IGNORE INTO asset_field_values (
  id, asset_id, field_id, value_text, value_number, value_date, value_boolean,
  created_by, created_at, updated_at
)
SELECT
  lower(hex(randomblob(16))),
  ?,
  field_id,
  value_text,
  value_number,
  value_date,
  value_boolean,
  created_by,
  datetime('now'),
  datetime('now')
FROM asset_field_values
WHERE asset_field_values.asset_id = ?;

-- name: DeleteAssetFieldValue :exec
DELETE FROM asset_field_values WHERE asset_id = ? AND field_id = ?;

-- name: DeleteAssetFieldValuesByField :exec
DELETE FROM asset_field_values WHERE field_id = ?;

-- name: GetAssetFieldValueByAssetAndField :one
SELECT *
FROM asset_field_values
WHERE asset_id = ? AND field_id = ? LIMIT 1;
