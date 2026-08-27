//go:build demo

package demo

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

// Wipe deletes all demo workspace content but preserves the workspace row,
// the demo user row, and the workspace_members row. This keeps the workspace ID
// and user ID stable across resets so that existing JWT tokens remain valid.
func (s *Seeder) Wipe(ctx context.Context) error {
	// Find the demo workspace
	var workspaceID string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE is_demo = 1 LIMIT 1`).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		return nil // nothing to wipe
	}
	if err != nil {
		return fmt.Errorf("demo: wipe find workspace: %w", err)
	}

	// Collect all storage keys before deleting DB rows
	storageKeys, err := s.collectStorageKeys(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("demo: collect storage keys: %w", err)
	}

	var assetsDeleted, versionsDeleted int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM assets WHERE workspace_id = ?`, workspaceID).Scan(&assetsDeleted)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_versions WHERE workspace_id = ?`, workspaceID).Scan(&versionsDeleted)

	// Delete all child data in a transaction.
	// Order matters: delete leaves first to avoid FK violations on tables
	// that don't have ON DELETE CASCADE.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("demo: wipe begin tx: %w", err)
	}
	defer tx.Rollback()

	tables := []string{
		"asset_field_values",
		"asset_tags",
		"asset_events",
		"project_events",
		"variants",
		"jobs",
		"share_comments",
		"shares",
		"ingress_log",
		"ingress_rules",
		"ingress_sources",
		"field_definitions",
		"assets",
		"asset_versions",
		"folders",
		"projects",
	}

	for _, table := range tables {
		if err := wipeDemoRows(ctx, tx, table, workspaceID); err != nil {
			return fmt.Errorf("demo: wipe %s: %w", table, err)
		}
	}

	// Delete ghost users (alice and marc) — they will be recreated on next seed
	base := strings.TrimPrefix(s.cfg.UserEmail, "demo") // e.g. "@example.com"
	_, err = tx.ExecContext(
		ctx,
		`DELETE FROM users WHERE email IN (?, ?)`,
		"alice"+base,
		"marc"+base,
	)
	if err != nil {
		return fmt.Errorf("demo: wipe ghost users: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("demo: wipe commit: %w", err)
	}

	// Delete storage files after the transaction succeeds
	if err := s.storage.Delete(ctx, fmt.Sprintf("demo/%s", workspaceID)); err != nil {
		// Non-fatal: log and continue
		slog.WarnContext(ctx, "demo: wipe storage delete", "error", err)
	}

	slog.InfoContext(ctx, "demo: wipe complete", "assets_deleted", assetsDeleted, "versions_deleted", versionsDeleted, "storage_files_deleted", len(storageKeys))
	return nil
}

func wipeDemoRows(ctx context.Context, tx *sql.Tx, table, workspaceID string) error {
	var q string
	switch table {
	case "asset_field_values":
		q = `DELETE FROM asset_field_values WHERE asset_id IN (SELECT id FROM assets WHERE workspace_id = ?)`
	case "asset_tags":
		q = `DELETE FROM asset_tags WHERE asset_id IN (SELECT id FROM assets WHERE workspace_id = ?)`
	case "asset_events":
		q = `DELETE FROM asset_events WHERE workspace_id = ?`
	case "project_events":
		q = `DELETE FROM project_events WHERE workspace_id = ?`
	case "share_comments":
		q = `DELETE FROM share_comments WHERE share_id IN (SELECT id FROM shares WHERE workspace_id = ?)`
	case "ingress_log":
		q = `DELETE FROM ingress_log WHERE source_id IN (SELECT id FROM ingress_sources WHERE workspace_id = ?)`
	case "ingress_rules":
		q = `DELETE FROM ingress_rules WHERE source_id IN (SELECT id FROM ingress_sources WHERE workspace_id = ?)`
	default:
		q = `DELETE FROM ` + table + ` WHERE workspace_id = ?`
	}
	_, err := tx.ExecContext(ctx, q, workspaceID)
	return err
}

func (s *Seeder) collectStorageKeys(ctx context.Context, workspaceID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT storage_key FROM asset_versions WHERE workspace_id = ?
		UNION
		SELECT DISTINCT storage_key FROM assets WHERE workspace_id = ?
		UNION
		SELECT DISTINCT thumbnail_key FROM assets WHERE workspace_id = ? AND thumbnail_key IS NOT NULL
		UNION
		SELECT DISTINCT thumbnail_key FROM asset_versions WHERE workspace_id = ? AND thumbnail_key IS NOT NULL
	`, workspaceID, workspaceID, workspaceID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
