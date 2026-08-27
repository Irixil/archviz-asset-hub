package db_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	dbpkg "damask/server/internal/db"
)

func openTestDB(t *testing.T) *dbpkg.DB {
	t.Helper()
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// TestPragmas_AppliedOnEveryConnection asserts foreign_keys and busy_timeout
// are set via DSN params, so every pooled connection gets them (not just
// whichever connection happened to run a one-shot Exec at startup).
func TestPragmas_AppliedOnEveryConnection(t *testing.T) {
	database := openTestDB(t)
	database.Reader.SetMaxOpenConns(2)

	ctx := context.Background()
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			var fk int
			if err := database.Reader.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
				t.Errorf("query foreign_keys pragma: %v", err)
				return
			}
			if fk != 1 {
				t.Errorf("expected foreign_keys=1, got %d", fk)
			}
		})
	}
	wg.Wait()

	var busyTimeout int
	if err := database.Writer.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout pragma: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("expected busy_timeout=5000, got %d", busyTimeout)
	}
}

// TestForeignKeys_Enforced proves the pragma isn't just set but actually
// enforced: inserting a workspace_members row referencing a nonexistent
// workspace must fail.
func TestForeignKeys_Enforced(t *testing.T) {
	database := openTestDB(t)
	ctx := context.Background()

	_, err := database.Writer.ExecContext(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES (?, ?, ?)`,
		"nonexistent-workspace", "nonexistent-user", "owner",
	)
	if err == nil {
		t.Fatal("expected FK violation inserting workspace_members with dangling workspace_id, got nil error")
	}
	if !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Errorf("expected a FOREIGN KEY constraint error, got: %v", err)
	}
}

// TestReaderWriterSplit_ConcurrentReadsDontBlockOnWriteTx proves the pool
// split actually decouples reads from writes: with the old single-pool
// SetMaxOpenConns(1) design, holding a write transaction open would stall
// every one of these reads until it committed. On the split, they complete
// immediately.
func TestReaderWriterSplit_ConcurrentReadsDontBlockOnWriteTx(t *testing.T) {
	database := openTestDB(t)
	ctx := context.Background()

	tx, err := database.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin write tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO workspaces (id, name) VALUES (?, ?)`, "ws-held-open", "held open"); err != nil {
		t.Fatalf("insert inside held-open tx: %v", err)
	}
	// Deliberately left uncommitted for the duration of the test.

	const readers = 8
	done := make(chan struct{}, readers)
	for range readers {
		go func() {
			var n int
			// This workspace row is invisible to readers until the writer
			// commits (or rolls back) — the assertion here is only that the
			// read completes promptly, not what it returns.
			_ = database.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM workspaces`).Scan(&n)
			done <- struct{}{}
		}()
	}

	timeout := time.After(2 * time.Second)
	for range readers {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("reads did not complete promptly while a write transaction was held open — " +
				"reader pool appears to be serialized behind the writer")
		}
	}
}

// TestWorkspaceSegregation_ReaderPool is a regression guard for the pool
// split itself: it doesn't add new authorization logic, it proves the split
// didn't introduce cross-workspace bleed by running two workspaces' asset
// lookups concurrently through the reader pool and asserting disjoint results.
func TestWorkspaceSegregation_ReaderPool(t *testing.T) {
	database := openTestDB(t)
	ctx := context.Background()

	seed := func(wsID, assetID string) {
		t.Helper()
		if _, err := database.Writer.ExecContext(ctx,
			`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, wsID); err != nil {
			t.Fatalf("seed workspace %s: %v", wsID, err)
		}
		if _, err := database.Writer.ExecContext(ctx,
			`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			assetID, wsID, "f.txt", "key/"+assetID, "text/plain", 1); err != nil {
			t.Fatalf("seed asset %s: %v", assetID, err)
		}
	}
	seed("ws-a", "asset-a")
	seed("ws-b", "asset-b")

	var wg sync.WaitGroup
	results := make(map[string][]string, 2)
	var mu sync.Mutex
	for _, wsID := range []string{"ws-a", "ws-b"} {
		wg.Add(1)
		go func(wsID string) {
			defer wg.Done()
			rows, err := database.Reader.QueryContext(ctx,
				`SELECT id FROM assets WHERE workspace_id = ?`, wsID)
			if err != nil {
				t.Errorf("query assets for %s: %v", wsID, err)
				return
			}
			defer rows.Close()
			var ids []string
			for rows.Next() {
				var id string
				if scanErr := rows.Scan(&id); scanErr != nil {
					t.Errorf("scan: %v", scanErr)
					return
				}
				ids = append(ids, id)
			}
			mu.Lock()
			results[wsID] = ids
			mu.Unlock()
		}(wsID)
	}
	wg.Wait()

	if len(results["ws-a"]) != 1 || results["ws-a"][0] != "asset-a" {
		t.Errorf("ws-a: expected [asset-a], got %v", results["ws-a"])
	}
	if len(results["ws-b"]) != 1 || results["ws-b"][0] != "asset-b" {
		t.Errorf("ws-b: expected [asset-b], got %v", results["ws-b"])
	}
}
