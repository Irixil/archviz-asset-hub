package visualsimilarity

import (
	"context"
	"database/sql"
	"testing"

	dbpkg "damask/server/internal/db"

	"github.com/google/uuid"
)

func newTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	queries, sqlDB := database.WQ, database.Writer
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return NewService(queries, sqlDB), sqlDB
}

func insertWorkspaceAndVersion(t *testing.T, sqlDB *sql.DB) (workspaceID, versionID string) {
	t.Helper()
	workspaceID = uuid.NewString()
	assetID := uuid.NewString()
	versionID = uuid.NewString()

	_, err := sqlDB.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, workspaceID, "test-workspace")
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	_, err = sqlDB.Exec(
		`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size) 
		  VALUES (?, ?, ?, ?, ?, ?)`,
		assetID, workspaceID, "test.jpg", "key/test.jpg", "image/jpeg", 1024,
	)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	_, err = sqlDB.Exec(
		`INSERT INTO asset_versions (id, asset_id, workspace_id, version_num, storage_key, content_hash, mime_type, size, is_current)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		versionID,
		assetID,
		workspaceID,
		1,
		"key/test.jpg",
		"abc123",
		"image/jpeg",
		1024,
		1,
	)
	if err != nil {
		t.Fatalf("insert asset_version: %v", err)
	}
	return workspaceID, versionID
}

func TestStore_IdempotentUpsert(t *testing.T) {
	svc, sqlDB := newTestService(t)
	ctx := context.Background()

	workspaceID, versionID := insertWorkspaceAndVersion(t, sqlDB)

	h := Hashes{CentralHash: 42, HashSet: []uint64{1, 2, 3}}

	if err := svc.Store(ctx, workspaceID, versionID, h); err != nil {
		t.Fatalf("first Store: %v", err)
	}
	if err := svc.Store(ctx, workspaceID, versionID, h); err != nil {
		t.Fatalf("second Store (idempotent): %v", err)
	}

	var count int
	_ = sqlDB.QueryRow(`SELECT COUNT(*) FROM asset_visual_similarity_hashes 
		WHERE asset_version_id = ?`, versionID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

func TestFindSimilar_ReturnsMatchesExcludesSelf(t *testing.T) {
	svc, sqlDB := newTestService(t)
	ctx := context.Background()

	wsID, v1 := insertWorkspaceAndVersion(t, sqlDB)

	// Create a second version in the same workspace that shares a hash bucket.
	assetID2 := uuid.NewString()
	v2 := uuid.NewString()
	_, _ = sqlDB.Exec(
		`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size) VALUES (?, ?, ?, ?, ?, ?)`,
		assetID2,
		wsID,
		"copy.jpg",
		"key/copy.jpg",
		"image/jpeg",
		1024,
	)
	_, _ = sqlDB.Exec(
		`INSERT INTO asset_versions (id, asset_id, workspace_id, version_num, storage_key, content_hash, mime_type, size, is_current) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v2,
		assetID2,
		wsID,
		1,
		"key/copy.jpg",
		"def456",
		"image/jpeg",
		1024,
		1,
	)

	// v1 and v2 share hash bucket 999.
	h1 := Hashes{CentralHash: 999, HashSet: []uint64{999, 777}}
	h2 := Hashes{CentralHash: 999, HashSet: []uint64{999, 888}}

	if err := svc.Store(ctx, wsID, v1, h1); err != nil {
		t.Fatalf("store v1: %v", err)
	}
	if err := svc.Store(ctx, wsID, v2, h2); err != nil {
		t.Fatalf("store v2: %v", err)
	}

	results, err := svc.FindSimilar(ctx, wsID, v1)
	if err != nil {
		t.Fatalf("FindSimilar: %v", err)
	}

	if len(results) != 1 || results[0] != v2 {
		t.Errorf("expected [%s], got %v", v2, results)
	}
}

func TestFindSimilar_EmptyWhenNoSimilar(t *testing.T) {
	svc, sqlDB := newTestService(t)
	ctx := context.Background()

	wsID, v1 := insertWorkspaceAndVersion(t, sqlDB)

	if err := svc.Store(ctx, wsID, v1, Hashes{CentralHash: 111, HashSet: []uint64{111}}); err != nil {
		t.Fatalf("store: %v", err)
	}

	results, err := svc.FindSimilar(ctx, wsID, v1)
	if err != nil {
		t.Fatalf("FindSimilar: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty, got %v", results)
	}
}

func TestFindSimilar_NoHashRow(t *testing.T) {
	svc, sqlDB := newTestService(t)
	ctx := context.Background()

	wsID, v1 := insertWorkspaceAndVersion(t, sqlDB)

	results, err := svc.FindSimilar(ctx, wsID, v1)
	if err != nil {
		t.Fatalf("FindSimilar: %v", err)
	}
	if results == nil || len(results) != 0 {
		t.Errorf("expected empty slice, got %v", results)
	}
}
