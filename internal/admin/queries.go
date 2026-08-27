package admin

import (
	"context"
	"database/sql"
	"time"
)

// ── Overview ────────────────────────────────────────────────

type OverviewStats struct {
	TotalUsers       int
	NewUsersToday    int
	NewUsersThisWeek int
	TotalWorkspaces  int
	ActiveWorkspaces int
	TotalAssets      int
	TotalStorageMB   float64
	AssetsToday      int
	JobsFailed       int
	JobsProcessing   int
}

type DayCount struct {
	Day   string
	Count int
}

func QueryOverviewStats(ctx context.Context, db *sql.DB) (OverviewStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return OverviewStats{}, err
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after read-only queries or commit.

	var s OverviewStats

	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&s.TotalUsers)
	if err != nil {
		return s, err
	}
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE created_at >= date('now')`).
		Scan(&s.NewUsersToday)
	if err != nil {
		return s, err
	}
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE created_at >= date('now', '-7 days')`).
		Scan(&s.NewUsersThisWeek)
	if err != nil {
		return s, err
	}
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM workspaces`).Scan(&s.TotalWorkspaces)
	if err != nil {
		return s, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT w.id)
		FROM workspaces w
		JOIN assets a ON a.workspace_id = w.id
		WHERE a.created_at >= datetime('now', '-30 days')`).Scan(&s.ActiveWorkspaces)
	if err != nil {
		return s, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(ROUND(SUM(av.size) / 1048576.0, 1), 0)
		FROM assets a
		JOIN asset_versions av ON av.id = a.current_version_id
		`).Scan(&s.TotalAssets, &s.TotalStorageMB)
	if err != nil {
		return s, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM assets
		WHERE created_at >= date('now')`).Scan(&s.AssetsToday)
	if err != nil {
		return s, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT
		  SUM(CASE WHEN status = 'failed'     THEN 1 ELSE 0 END),
		  SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END)
		FROM jobs`).Scan(&s.JobsFailed, &s.JobsProcessing)
	if err != nil {
		return s, err
	}

	return s, nil
}

func QuerySignupsByDay(ctx context.Context, db *sql.DB) ([]DayCount, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		SELECT date(created_at) AS day, COUNT(*) AS count
		FROM users
		WHERE created_at >= date('now', '-7 days')
		GROUP BY day
		ORDER BY day ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DayCount
	for rows.Next() {
		var d DayCount
		err = rows.Scan(&d.Day, &d.Count)
		if err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func QueryUploadsByDay(ctx context.Context, db *sql.DB) ([]DayCount, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		SELECT date(created_at) AS day, COUNT(*) AS count
		FROM assets
		WHERE created_at >= date('now', '-7 days')
		GROUP BY day
		ORDER BY day ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DayCount
	for rows.Next() {
		var d DayCount
		err = rows.Scan(&d.Day, &d.Count)
		if err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// ── Users ───────────────────────────────────────────────────

type UserRow struct {
	ID            string
	Email         string
	WorkspaceName string
	Role          string
	AssetCount    int
	LastUpload    *time.Time
	CreatedAt     time.Time
}

func QueryUsers(
	ctx context.Context,
	db *sql.DB,
	limit, offset int,
	search, orderBy, orderDir string,
) ([]UserRow, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if orderBy == "" {
		orderBy = "u.created_at"
	}
	if orderDir != "ASC" {
		orderDir = "DESC"
	}

	var (
		countQ string
		dataQ  string
		args   []any
	)

	if search != "" {
		like := "%" + search + "%"
		countQ = `
			SELECT COUNT(DISTINCT u.id)
			FROM users u
			JOIN workspace_members wm ON wm.user_id = u.id
			JOIN workspaces w ON w.id = wm.workspace_id
			WHERE u.email LIKE ? OR w.name LIKE ?`
		dataQ = `
			SELECT u.id, u.email,
			  w.name AS workspace_name,
			  wm.role,
			  COUNT(DISTINCT a.id) AS asset_count,
			  MAX(a.created_at) AS last_upload,
			  u.created_at
			FROM users u
			JOIN workspace_members wm ON wm.user_id = u.id
			JOIN workspaces w ON w.id = wm.workspace_id
			LEFT JOIN assets a ON a.workspace_id = w.id
			WHERE u.email LIKE ? OR w.name LIKE ?
			GROUP BY u.id
			ORDER BY ` + orderBy + ` ` + orderDir + `
			LIMIT ? OFFSET ?`
		args = []any{like, like}
	} else {
		countQ = `SELECT COUNT(DISTINCT u.id) FROM users u JOIN workspace_members wm ON wm.user_id = u.id JOIN workspaces w ON w.id = wm.workspace_id`
		dataQ = `
			SELECT u.id, u.email,
			  w.name AS workspace_name,
			  wm.role,
			  COUNT(DISTINCT a.id) AS asset_count,
			  MAX(a.created_at) AS last_upload,
			  u.created_at
			FROM users u
			JOIN workspace_members wm ON wm.user_id = u.id
			JOIN workspaces w ON w.id = wm.workspace_id
			LEFT JOIN assets a ON a.workspace_id = w.id
			GROUP BY u.id
			ORDER BY ` + orderBy + ` ` + orderDir + `
			LIMIT ? OFFSET ?`
	}

	var total int
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataArgs := append(args, limit, offset) //nolint:gocritic // False positive on append of an unknown slice
	rows, err := db.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []UserRow
	for rows.Next() {
		var r UserRow
		var lastUpload *string
		var createdAtStr string
		err = rows.Scan(
			&r.ID,
			&r.Email,
			&r.WorkspaceName,
			&r.Role,
			&r.AssetCount,
			&lastUpload,
			&createdAtStr,
		)
		if err != nil {
			return nil, 0, err
		}
		if lastUpload != nil {
			t, _ := time.Parse("2006-01-02 15:04:05", *lastUpload)
			r.LastUpload = &t
		}
		r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
		result = append(result, r)
	}
	return result, total, rows.Err()
}

// ── Activity ────────────────────────────────────────────────

type ActivityRow struct {
	EventType     string
	AssetName     string
	ActorEmail    string
	ActorType     string
	WorkspaceName string
	Payload       string
	CreatedAt     time.Time
}

func QueryRecentActivity(ctx context.Context, db *sql.DB, limit int, eventFilter string) ([]ActivityRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	q := `
		SELECT
		  ae.event_type,
		  COALESCE(a.original_filename, '[deleted]') AS asset_name,
		  COALESCE(u.email, 'system') AS actor_email,
		  ae.actor_type,
		  w.name AS workspace_name,
		  ae.payload,
		  ae.created_at
		FROM asset_events ae
		LEFT JOIN users u ON u.id = ae.user_id
		LEFT JOIN assets a ON a.id = ae.asset_id
		LEFT JOIN workspaces w ON w.id = ae.workspace_id
		WHERE ae.event_type != 'asset_downloaded'`

	var args []any
	if eventFilter != "" && eventFilter != "all" {
		q += ` AND ae.event_type = ?`
		args = append(args, eventFilter)
	}
	q += ` ORDER BY ae.created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ActivityRow
	for rows.Next() {
		var r ActivityRow
		var createdAtStr string
		err = rows.Scan(
			&r.EventType,
			&r.AssetName,
			&r.ActorEmail,
			&r.ActorType,
			&r.WorkspaceName,
			&r.Payload,
			&createdAtStr,
		)
		if err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAtStr)
		if r.CreatedAt.IsZero() {
			r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ── Storage ─────────────────────────────────────────────────

type StorageRow struct {
	WorkspaceName string
	AssetCount    int
	VersionCount  int
	TotalMB       float64
	OldestAsset   *time.Time
	NewestAsset   *time.Time
}

type LargestAsset struct {
	Filename      string
	SizeMB        float64
	WorkspaceName string
}

func QueryStorageBreakdown(ctx context.Context, db *sql.DB) ([]StorageRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		SELECT
		  w.name,
		  COUNT(DISTINCT a.id) AS asset_count,
		  COUNT(av.id) AS version_count,
		  COALESCE(ROUND(SUM(av.size) / 1048576.0, 1), 0) AS total_mb,
		  MIN(a.created_at) AS oldest_asset,
		  MAX(a.created_at) AS newest_asset
		FROM workspaces w
		LEFT JOIN assets a ON a.workspace_id = w.id
		LEFT JOIN asset_versions av ON av.asset_id = a.id AND av.deleted_at IS NULL
		GROUP BY w.id
		ORDER BY total_mb DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []StorageRow
	for rows.Next() {
		var r StorageRow
		var oldest, newest *string
		err = rows.Scan(
			&r.WorkspaceName,
			&r.AssetCount,
			&r.VersionCount,
			&r.TotalMB,
			&oldest,
			&newest,
		)
		if err != nil {
			return nil, err
		}
		if oldest != nil {
			t, _ := time.Parse("2006-01-02 15:04:05", *oldest)
			r.OldestAsset = &t
		}
		if newest != nil {
			t, _ := time.Parse("2006-01-02 15:04:05", *newest)
			r.NewestAsset = &t
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func QueryLargestAsset(ctx context.Context, db *sql.DB) (LargestAsset, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var la LargestAsset
	var sizeMB float64
	err := db.QueryRowContext(ctx, `
		SELECT a.original_filename, ROUND(av.size / 1048576.0, 1), w.name
		FROM asset_versions av
		JOIN assets a ON a.id = av.asset_id
		JOIN workspaces w ON w.id = a.workspace_id
		WHERE av.deleted_at IS NULL
		ORDER BY av.size DESC
		LIMIT 1`).Scan(&la.Filename, &sizeMB, &la.WorkspaceName)
	if err == sql.ErrNoRows {
		return la, nil
	}
	la.SizeMB = sizeMB
	return la, err
}

// ── Jobs ────────────────────────────────────────────────────

type JobRow struct {
	Type      string
	Status    string
	Count     int
	OldestSec int
	LastError string
}

type JobDetailRow struct {
	ID        string
	Type      string
	Status    string
	Payload   string
	Attempts  int
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func QueryJobHealth(ctx context.Context, db *sql.DB) ([]JobRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		SELECT
		  type,
		  status,
		  COUNT(*) AS count,
		  CAST((julianday('now') - julianday(MIN(created_at))) * 86400 AS INTEGER) AS oldest_sec,
		  COALESCE(MAX(CASE WHEN status = 'failed' THEN error ELSE NULL END), '') AS last_error
		FROM jobs
		GROUP BY type, status
		ORDER BY
		  CASE status
		    WHEN 'failed'     THEN 1
		    WHEN 'processing' THEN 2
		    WHEN 'pending'    THEN 3
		    ELSE 4
		  END,
		  count DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []JobRow
	for rows.Next() {
		var r JobRow
		err = rows.Scan(&r.Type, &r.Status, &r.Count, &r.OldestSec, &r.LastError)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func QueryFailedJobs(ctx context.Context, db *sql.DB, limit int) ([]JobDetailRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		SELECT id, type, status, payload, attempts, COALESCE(error, ''), created_at, updated_at
		FROM jobs
		WHERE status = 'failed'
		ORDER BY updated_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []JobDetailRow
	for rows.Next() {
		var r JobDetailRow
		var createdStr, updatedStr string
		err = rows.Scan(
			&r.ID,
			&r.Type,
			&r.Status,
			&r.Payload,
			&r.Attempts,
			&r.Error,
			&createdStr,
			&updatedStr,
		)
		if err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdStr)
		r.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedStr)
		result = append(result, r)
	}
	return result, rows.Err()
}
