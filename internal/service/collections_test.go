package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

func newCollectionSvc(t *testing.T) service.CollectionService {
	t.Helper()
	repo := memory.NewRealCollectionRepo()
	return service.NewCollectionService(repo, memory.NewAssetRepo())
}

// --- List ---

func TestCollectionService_List_WorkspaceIsolation(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	svc.Create(context.Background(), "ws_A", service.CreateCollectionParams{Name: "Alpha", CreatedBy: "u1"})
	svc.Create(context.Background(), "ws_B", service.CreateCollectionParams{Name: "Beta", CreatedBy: "u2"})

	cols, err := svc.List(context.Background(), "ws_A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cols) != 1 {
		t.Errorf("expected 1 collection for ws_A, got %d", len(cols))
	}
	if cols[0].WorkspaceID != "ws_A" {
		t.Errorf("WorkspaceID: got %q, want ws_A", cols[0].WorkspaceID)
	}
}

// --- Create ---

func TestCollectionService_Create_WithForeignAsset(t *testing.T) {
	t.Parallel()
	collRepo := memory.NewRealCollectionRepo()
	assetRepo := memory.NewAssetRepo()
	svc := service.NewCollectionService(collRepo, assetRepo)

	// ast_foreign belongs to ws_B, not ws_A
	assetRepo.Seed(repository.Asset{ID: "ast_foreign", WorkspaceID: "ws_B", OriginalFilename: "x.jpg"})

	_, err := svc.Create(context.Background(), "ws_A", service.CreateCollectionParams{
		Name:      "Bad",
		CreatedBy: "u1",
		AssetIDs:  []string{"ast_foreign"},
	})
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for foreign asset, got %v", err)
	}
}

func TestCollectionService_Create_OK(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", service.CreateCollectionParams{
		Name:      "Favorites",
		CreatedBy: "user_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Favorites" || dto.WorkspaceID != "ws_1" {
		t.Errorf("unexpected dto: %+v", dto)
	}
	if dto.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCollectionService_Create_EmptyName(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	_, err := svc.Create(context.Background(), "ws_1", service.CreateCollectionParams{Name: "  "})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// --- Get ---

func TestCollectionService_Get_NotFound(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Update ---

func TestCollectionService_Update_OK(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", service.CreateCollectionParams{Name: "Old"})
	newName := "New"
	updated, err := svc.Update(context.Background(), "ws_1", dto.ID, service.UpdateCollectionParams{Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "New" {
		t.Errorf("Name: got %q, want %q", updated.Name, "New")
	}
}

func TestCollectionService_Update_EmptyName(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", service.CreateCollectionParams{Name: "X"})
	empty := ""
	_, err := svc.Update(context.Background(), "ws_1", dto.ID, service.UpdateCollectionParams{Name: &empty})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCollectionService_Update_NotFound(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	name := "x"
	_, err := svc.Update(context.Background(), "ws_1", "nope", service.UpdateCollectionParams{Name: &name})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Delete ---

func TestCollectionService_Delete_OK(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", service.CreateCollectionParams{Name: "ToDelete"})
	if err := svc.Delete(context.Background(), "ws_1", dto.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "ws_1", dto.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Error("expected ErrNotFound after delete")
	}
}

func TestCollectionService_Delete_NotFound(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	if err := svc.Delete(context.Background(), "ws_1", "nope"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- AddAsset / RemoveAsset ---

func TestCollectionService_AddAsset_Idempotent(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", service.CreateCollectionParams{Name: "C"})
	if err := svc.AddAsset(context.Background(), "ws_1", dto.ID, "ast_1"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if err := svc.AddAsset(context.Background(), "ws_1", dto.ID, "ast_1"); err != nil {
		t.Fatalf("second add (should be idempotent): %v", err)
	}
}

func TestCollectionService_RemoveAsset_OK(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", service.CreateCollectionParams{Name: "C"})
	svc.AddAsset(context.Background(), "ws_1", dto.ID, "ast_1")
	if err := svc.RemoveAsset(context.Background(), "ws_1", dto.ID, "ast_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCollectionService_AddAsset_CollectionNotFound(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	err := svc.AddAsset(context.Background(), "ws_1", "nope", "ast_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- ListForAsset ---

func TestCollectionService_ListForAsset_OK(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	col, _ := svc.Create(context.Background(), "ws_1", service.CreateCollectionParams{Name: "Picks", CreatedBy: "u1"})
	svc.AddAsset(context.Background(), "ws_1", col.ID, "ast_1")

	cols, err := svc.ListForAsset(context.Background(), "ws_1", "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cols) != 1 || cols[0].ID != col.ID {
		t.Errorf("expected [%s], got %v", col.ID, cols)
	}
}

func TestCollectionService_ListForAsset_Empty(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	cols, err := svc.ListForAsset(context.Background(), "ws_1", "ast_none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cols) != 0 {
		t.Errorf("expected empty, got %d", len(cols))
	}
}

// --- ListAssets ---

func TestCollectionService_ListAssets_OK(t *testing.T) {
	t.Parallel()
	repo := memory.NewRealCollectionRepo()
	assetRepo := memory.NewAssetRepo()
	assetRepo.Seed(
		repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", OriginalFilename: "a.jpg", MimeType: "image/jpeg"},
		repository.Asset{ID: "ast_2", WorkspaceID: "ws_1", OriginalFilename: "b.jpg", MimeType: "image/jpeg"},
	)
	svc := service.NewCollectionService(repo, assetRepo)

	col, _ := svc.Create(context.Background(), "ws_1", service.CreateCollectionParams{Name: "All", CreatedBy: "u1"})
	svc.AddAsset(context.Background(), "ws_1", col.ID, "ast_1")
	svc.AddAsset(context.Background(), "ws_1", col.ID, "ast_2")

	assets, err := svc.ListAssets(context.Background(), "ws_1", col.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 2 {
		t.Errorf("expected 2 assets, got %d", len(assets))
	}
}

func TestCollectionService_ListAssets_CollectionNotFound(t *testing.T) {
	t.Parallel()
	svc := newCollectionSvc(t)
	_, err := svc.ListAssets(context.Background(), "ws_1", "col_nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
