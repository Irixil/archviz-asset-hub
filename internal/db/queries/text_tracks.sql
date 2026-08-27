-- name: CreateTextTrack :one
INSERT INTO asset_text_tracks (
    id, workspace_id, asset_id, asset_version_id,
    source, lang, content, storage_key, content_type,
    meta, status, created_by
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTextTrack :one
SELECT * FROM asset_text_tracks
WHERE id = ? AND workspace_id = ?;

-- name: ListTextTracksByAsset :many
SELECT * FROM asset_text_tracks
WHERE asset_id = ? AND workspace_id = ?
ORDER BY created_at DESC;

-- name: SetTextTrackProcessing :exec
UPDATE asset_text_tracks
SET status = 'processing', updated_at = datetime('now')
WHERE id = ? AND workspace_id = ?;

-- name: SetTextTrackReady :exec
UPDATE asset_text_tracks
SET content = ?, storage_key = ?, content_type = ?,
    meta = ?, status = 'ready', error = NULL,
    updated_at = datetime('now')
WHERE id = ? AND workspace_id = ?;

-- name: SetTextTrackFailed :exec
UPDATE asset_text_tracks
SET status = 'failed', error = ?,
    updated_at = datetime('now')
WHERE id = ? AND workspace_id = ?;

-- name: DeleteTextTrack :exec
DELETE FROM asset_text_tracks
WHERE id = ? AND workspace_id = ?;

-- name: DeleteTextTracksByAsset :exec
DELETE FROM asset_text_tracks
WHERE asset_id = ? AND workspace_id = ?;

-- name: InsertTextFTS :exec
INSERT INTO assets_text_fts(track_id, asset_id, workspace_id, source, lang, content)
VALUES (?, ?, ?, ?, ?, ?);

-- name: DeleteTextFTS :exec
DELETE FROM assets_text_fts WHERE track_id = ?;

-- name: DeleteTextFTSByAsset :exec
DELETE FROM assets_text_fts WHERE asset_id = ?;

-- name: SearchTextTracksByContent :many
SELECT DISTINCT asset_id
FROM assets_text_fts
WHERE workspace_id = ?
  AND content MATCH ?
ORDER BY rank
LIMIT ?;
