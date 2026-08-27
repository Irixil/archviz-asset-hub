-- name: CreateJob :one
INSERT INTO jobs (id, workspace_id, type, payload, status)
VALUES (?, ?, ?, ?, 'pending')
RETURNING *;

-- name: ClaimNextJob :one
UPDATE jobs
SET status = 'processing', attempts = attempts + 1, updated_at = datetime('now')
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'pending'
      AND (run_after IS NULL OR datetime(run_after) <= datetime('now'))
      -- excluded_types is a comma-delimited list wrapped in commas, e.g.
      -- ",video_transcode,video_watermark," (empty string = no exclusions)
      AND instr(CAST(sqlc.arg(excluded_types) AS TEXT), ',' || type || ',') = 0
    ORDER BY created_at ASC LIMIT 1
)
RETURNING *;

-- Settle queries guard on status+attempts: a worker that outlived its timeout
-- (sweep requeued the job, another worker re-claimed it, bumping attempts)
-- must not clobber the new claimant's state. Zero rows affected = stale writer.

-- name: CompleteJob :execrows
UPDATE jobs SET status = 'done', updated_at = datetime('now')
WHERE id = ? AND status = 'processing' AND attempts = ?;

-- name: FailJob :execrows
UPDATE jobs SET status = 'failed', error = ?, updated_at = datetime('now')
WHERE id = ? AND status = 'processing' AND attempts = ?;

-- name: FailJobRetry :execrows
UPDATE jobs
SET status = 'pending', error = ?, run_after = ?, updated_at = datetime('now')
WHERE id = ? AND status = 'processing' AND attempts = ?;

-- name: RequeueStalledJobs :exec
UPDATE jobs SET status = 'pending', updated_at = datetime('now')
WHERE status = 'processing';

-- name: RequeueStalledJobsOfTypes :exec
UPDATE jobs SET status = 'pending', updated_at = datetime('now')
WHERE status = 'processing'
  AND datetime(updated_at) < datetime(CAST(sqlc.arg(cutoff) AS TEXT))
  AND attempts < CAST(sqlc.arg(max_attempts) AS INTEGER)
  AND instr(CAST(sqlc.arg(types) AS TEXT), ',' || type || ',') > 0;

-- name: RequeueStalledJobsNotOfTypes :exec
UPDATE jobs SET status = 'pending', updated_at = datetime('now')
WHERE status = 'processing'
  AND datetime(updated_at) < datetime(CAST(sqlc.arg(cutoff) AS TEXT))
  AND attempts < CAST(sqlc.arg(max_attempts) AS INTEGER)
  AND instr(CAST(sqlc.arg(types) AS TEXT), ',' || type || ',') = 0;

-- name: FailStalledJobsOfTypes :exec
UPDATE jobs
SET status = 'failed', error = 'stalled: attempt budget exhausted', updated_at = datetime('now')
WHERE status = 'processing'
  AND datetime(updated_at) < datetime(CAST(sqlc.arg(cutoff) AS TEXT))
  AND attempts >= CAST(sqlc.arg(max_attempts) AS INTEGER)
  AND instr(CAST(sqlc.arg(types) AS TEXT), ',' || type || ',') > 0;

-- name: FailStalledJobsNotOfTypes :exec
UPDATE jobs
SET status = 'failed', error = 'stalled: attempt budget exhausted', updated_at = datetime('now')
WHERE status = 'processing'
  AND datetime(updated_at) < datetime(CAST(sqlc.arg(cutoff) AS TEXT))
  AND attempts >= CAST(sqlc.arg(max_attempts) AS INTEGER)
  AND instr(CAST(sqlc.arg(types) AS TEXT), ',' || type || ',') = 0;

-- name: ClaimNextJobIgnoreRunAfter :one
-- Test-only: like ClaimNextJob but ignores run_after so test drains also run
-- jobs parked for a future retry.
UPDATE jobs
SET status = 'processing', attempts = attempts + 1, updated_at = datetime('now')
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'pending'
    ORDER BY created_at ASC LIMIT 1
)
RETURNING *;

-- name: CountPendingJobs :one
SELECT COUNT(*) FROM jobs WHERE status = 'pending';

-- name: GetJobByID :one
SELECT * FROM jobs WHERE id = ?;

-- name: CompleteJobWithResult :exec
UPDATE jobs SET status = 'done', result = ?, updated_at = datetime('now') WHERE id = ?;

-- name: CreateJobForWorkspace :one
INSERT INTO jobs (id, workspace_id, type, payload, status)
VALUES (?, ?, ?, ?, 'pending')
RETURNING *;
