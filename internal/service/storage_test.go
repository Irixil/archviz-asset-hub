package service

import (
	"context"
	"database/sql"
	"testing"

	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
	reposqlc "damask/server/internal/repository/sqlc"
)

// newStorageSvcDB opens an in-memory SQLite DB and returns a StorageService, Queries, and *[sql.DB].
func newStorageSvcDB(t *testing.T) (StorageService, *dbgen.Queries, *sql.DB) {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return NewStorageService(reposqlc.NewStorageStatsRepo(database)), database.WQ, database.Writer
}

// seedWorkspace inserts a minimal workspace row.
func seedWorkspace(ctx context.Context, t *testing.T, queries *dbgen.Queries, id string) {
	t.Helper()
	if _, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: id, Name: "test"}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

// setLimit sets storage_limit_bytes on a workspace via raw SQL.
func setLimit(ctx context.Context, t *testing.T, db *sql.DB, wsID string, limit int64) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`UPDATE workspaces SET storage_limit_bytes = ? WHERE id = ?`, limit, wsID); err != nil {
		t.Fatalf("set limit: %v", err)
	}
}

// ── interface constraints ─────────────────────────────────────────────────────

func TestStorageService_ImplementsInvalidator(_ *testing.T) {
	var _ StorageInvalidator = (*storageService)(nil)
}

// ── buildUsage unit tests (pure function, no DB) ─────────────────────────────

func TestBuildUsage_TypeBucketing(t *testing.T) {
	rows := []repository.StorageProjectTypeRow{
		{AssetType: "image", VersionsBytes: 100, VariantsBytes: 20},
		{AssetType: "video", VersionsBytes: 200, VariantsBytes: 0},
		{AssetType: "audio", VersionsBytes: 50, VariantsBytes: 0},
		{AssetType: "document", VersionsBytes: 30, VariantsBytes: 0},
		{AssetType: "other", VersionsBytes: 10, VariantsBytes: 5},
	}
	u := buildUsage(rows, nil, nil)

	if u.ByType.Image != 120 {
		t.Errorf("image: want 120 got %d", u.ByType.Image)
	}
	if u.ByType.Video != 200 {
		t.Errorf("video: want 200 got %d", u.ByType.Video)
	}
	if u.ByType.Audio != 50 {
		t.Errorf("audio: want 50 got %d", u.ByType.Audio)
	}
	if u.ByType.Document != 30 {
		t.Errorf("doc: want 30 got %d", u.ByType.Document)
	}
	if u.ByType.Other != 15 {
		t.Errorf("other: want 15 got %d", u.ByType.Other)
	}
	if u.LimitBytes != nil {
		t.Error("limit should be nil")
	}
}

func TestBuildUsage_NullProject(t *testing.T) {
	rows := []repository.StorageProjectTypeRow{
		{ProjectID: nil, AssetType: "image", VersionsBytes: 100},
	}
	u := buildUsage(rows, nil, nil)
	if len(u.ByProject) != 1 {
		t.Fatalf("want 1 project row, got %d", len(u.ByProject))
	}
	if u.ByProject[0].ProjectID != nil {
		t.Error("project_id should be nil")
	}
}

// ── CheckLimit unit tests ─────────────────────────────────────────────────────

func TestCheckLimit_Unlimited(t *testing.T) {
	ctx := context.Background()
	svc, queries, _ := newStorageSvcDB(t)
	seedWorkspace(ctx, t, queries, "ws1")

	if err := svc.CheckLimit(ctx, "ws1", 999_000_000); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestCheckLimit_UnderLimit(t *testing.T) {
	ctx := context.Background()
	svc, queries, db := newStorageSvcDB(t)
	seedWorkspace(ctx, t, queries, "ws2")
	setLimit(ctx, t, db, "ws2", 1_000_000_000)

	if err := svc.CheckLimit(ctx, "ws2", 100); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestCheckLimit_AtLimit(t *testing.T) {
	ctx := context.Background()
	svc, queries, db := newStorageSvcDB(t)
	seedWorkspace(ctx, t, queries, "ws3")
	setLimit(ctx, t, db, "ws3", 0)

	if err := svc.CheckLimit(ctx, "ws3", 1); err == nil {
		t.Error("want ErrStorageLimitReached, got nil")
	}
}

func TestCheckLimit_OverLimit(t *testing.T) {
	ctx := context.Background()
	svc, queries, db := newStorageSvcDB(t)
	seedWorkspace(ctx, t, queries, "ws4")
	setLimit(ctx, t, db, "ws4", 0)

	// total=0 + incoming=0 is NOT > 0, so should return nil
	if err := svc.CheckLimit(ctx, "ws4", 0); err != nil {
		t.Errorf("at exact zero limit with zero incoming, want nil, got %v", err)
	}
}

// ── GetUsage / Invalidate ─────────────────────────────────────────────────────

func TestGetUsage_NoLimit(t *testing.T) {
	ctx := context.Background()
	svc, queries, _ := newStorageSvcDB(t)
	seedWorkspace(ctx, t, queries, "ws5")

	u, err := svc.GetUsage(ctx, "ws5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.LimitBytes != nil {
		t.Errorf("want nil limit, got %v", u.LimitBytes)
	}
}

func TestGetUsage_WithLimit(t *testing.T) {
	ctx := context.Background()
	svc, queries, db := newStorageSvcDB(t)
	seedWorkspace(ctx, t, queries, "ws6")
	limit := int64(5_000_000_000)
	setLimit(ctx, t, db, "ws6", limit)

	u, err := svc.GetUsage(ctx, "ws6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.LimitBytes == nil || *u.LimitBytes != limit {
		t.Errorf("want limit=%d, got %v", limit, u.LimitBytes)
	}
}

func TestGetUsage_InvalidateClears(t *testing.T) {
	ctx := context.Background()
	svc, queries, db := newStorageSvcDB(t)
	seedWorkspace(ctx, t, queries, "ws7")

	// Prime cache with no limit
	u1, _ := svc.GetUsage(ctx, "ws7")
	if u1.LimitBytes != nil {
		t.Fatal("precondition: expect no limit")
	}

	// Set limit in DB while cache still holds stale value
	limit := int64(1_000_000)
	setLimit(ctx, t, db, "ws7", limit)

	u2, _ := svc.GetUsage(ctx, "ws7")
	if u2.LimitBytes != nil {
		t.Error("expected cached (no limit) result before invalidation")
	}

	// After invalidate, DB is re-queried
	svc.Invalidate("ws7")
	u3, _ := svc.GetUsage(ctx, "ws7")
	if u3.LimitBytes == nil || *u3.LimitBytes != limit {
		t.Errorf("after invalidate want limit=%d, got %v", limit, u3.LimitBytes)
	}
}

// -- Cross-workspace negative test --
// StorageStatsRepository's aggregate queries must not leak another workspace's
// limit into a workspace that never set one.

func TestStorageStatsRepository_CrossWorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	repo := reposqlc.NewStorageStatsRepo(database)

	seedWorkspace(ctx, t, database.WQ, "ws-a")
	seedWorkspace(ctx, t, database.WQ, "ws-b")
	setLimit(ctx, t, database.Writer, "ws-a", 42)

	limitB, err := repo.GetStorageLimitBytes(ctx, "ws-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limitB != nil {
		t.Errorf("GetStorageLimitBytes cross-workspace: expected nil, got %v", *limitB)
	}

	limitA, err := repo.GetStorageLimitBytes(ctx, "ws-a")
	if err != nil || limitA == nil || *limitA != 42 {
		t.Errorf("GetStorageLimitBytes same-workspace: expected 42, got %v, err=%v", limitA, err)
	}
}
