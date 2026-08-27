package service_test

import (
	"context"
	"testing"

	"damask/server/internal/audit"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"
)

// newBulkFieldSvc returns an AssetFieldService backed by a fresh SQLite DB.
func newBulkFieldSvc(t *testing.T) (service.AssetFieldService, *dbgen.Queries) {
	t.Helper()
	database, err := dbpkg.Open(t.TempDir() + "/bulk_fields.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	queries := database.WQ
	svc := service.NewAssetFieldService(
		reposqlc.NewAssetRepo(database),
		reposqlc.NewFieldRepo(database),
		reposqlc.NewAssetFieldRepo(database),
		audit.NopWriter{},
	)
	return svc, queries
}

// --- BulkPreview service tests ---

func TestBulkPreviewService_DistinctValues(t *testing.T) {
	svc, queries := newBulkFieldSvc(t)
	ctx := context.Background()

	const wsID = "ws_bp_1"
	seedWorkspaceAndUser(t, queries, wsID)
	fieldID := seedFieldDef(t, queries, wsID, "asset", "client", "Client")

	for i, v := range []string{"Nike", "Adidas", "Nike"} {
		assetID := "ast_bp_1_" + string(rune('a'+i))
		seedAsset(t, queries, wsID, assetID)
		_, err := svc.SetValues(ctx, wsID, assetID, fieldTestUserID, []service.SetFieldValueInput{
			{FieldID: fieldID, Value: v},
		})
		if err != nil {
			t.Fatalf("set values asset %s: %v", assetID, err)
		}
	}

	entries, err := svc.BulkPreview(ctx, wsID, []string{"ast_bp_1_a", "ast_bp_1_b", "ast_bp_1_c"}, []string{fieldID})
	if err != nil {
		t.Fatalf("BulkPreview: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.AssetsWithValue != 3 {
		t.Errorf("expected AssetsWithValue=3, got %d", e.AssetsWithValue)
	}
	// 2 distinct values: Nike and Adidas.
	if len(e.DistinctValues) != 2 {
		t.Errorf("expected 2 distinct values, got %v", e.DistinctValues)
	}
}

func TestBulkPreviewService_Cap5(t *testing.T) {
	svc, queries := newBulkFieldSvc(t)
	ctx := context.Background()

	const wsID = "ws_bp_2"
	seedWorkspaceAndUser(t, queries, wsID)
	fieldID := seedFieldDef(t, queries, wsID, "asset", "brand", "Brand")

	values := []string{"A", "B", "C", "D", "E", "F", "G"}
	assetIDs := make([]string, len(values))
	for i, v := range values {
		assetID := "ast_bp_2_" + string(rune('a'+i))
		assetIDs[i] = assetID
		seedAsset(t, queries, wsID, assetID)
		_, err := svc.SetValues(ctx, wsID, assetID, fieldTestUserID, []service.SetFieldValueInput{
			{FieldID: fieldID, Value: v},
		})
		if err != nil {
			t.Fatalf("set values: %v", err)
		}
	}

	entries, err := svc.BulkPreview(ctx, wsID, assetIDs, []string{fieldID})
	if err != nil {
		t.Fatalf("BulkPreview: %v", err)
	}
	e := entries[0]
	// Cap is 5: entries 0-4 + "+2 more"
	if len(e.DistinctValues) != 6 {
		t.Fatalf("expected 6 distinct entries (5 + overflow), got %d: %v", len(e.DistinctValues), e.DistinctValues)
	}
	last := e.DistinctValues[5]
	if last != "+2 more" {
		t.Errorf("expected '+2 more', got %q", last)
	}
}

// --- BulkSetValues cleared count test ---

func TestBulkFieldsService_NullClears(t *testing.T) {
	svc, queries := newBulkFieldSvc(t)
	ctx := context.Background()

	const wsID = "ws_bp_3"
	seedWorkspaceAndUser(t, queries, wsID)
	fieldID := seedFieldDef(t, queries, wsID, "asset", "notes", "Notes")
	const assetID = "ast_bp_3_a"
	seedAsset(t, queries, wsID, assetID)

	// Set value first.
	if _, err := svc.SetValues(ctx, wsID, assetID, fieldTestUserID, []service.SetFieldValueInput{
		{FieldID: fieldID, Value: "hello"},
	}); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Bulk-clear with null.
	result, err := svc.BulkSetValues(ctx, wsID, fieldTestUserID, []string{assetID}, []service.SetFieldValueInput{
		{FieldID: fieldID, Value: nil},
	})
	if err != nil {
		t.Fatalf("BulkSetValues: %v", err)
	}
	if result.Cleared != 1 {
		t.Errorf("expected Cleared=1, got %d", result.Cleared)
	}
	if result.Updated != 1 {
		t.Errorf("expected Updated=1, got %d", result.Updated)
	}

	// Confirm the value is gone.
	vals, err := svc.GetValues(ctx, wsID, assetID)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	for _, v := range vals {
		if v.FieldID == fieldID && v.Value != nil {
			t.Errorf("expected field value to be cleared, got %v", v.Value)
		}
	}
}
