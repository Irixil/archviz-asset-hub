package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"
	"damask/server/internal/storage"

	"github.com/google/uuid"
)

// newExifEnv opens a fresh in-memory SQLite DB (sqlc-backed repos are needed here:
// AssetFieldRepository has no memory implementation yet) and seeds a workspace.
// stor may be nil for tests that never reach the storage.Get call (e.g. no-op paths).
func newExifEnv(t *testing.T, stor storage.Storage) (*service.ExifService, *dbpkg.DB, string) {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	wsID := uuid.NewString()
	if _, wsErr := database.WQ.CreateWorkspace(context.Background(), dbgen.CreateWorkspaceParams{
		ID: wsID, Name: "test-workspace",
	}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}

	svc := service.NewExifService(
		reposqlc.NewWorkspaceRepo(database),
		reposqlc.NewAssetRepo(database),
		reposqlc.NewFieldRepo(database),
		reposqlc.NewAssetFieldRepo(database),
		stor,
	)
	return svc, database, wsID
}

func TestExifService_ExtractForAsset_ExifKeepDisabled_NoOp(t *testing.T) {
	svc, _, wsID := newExifEnv(t, nil)
	// exif_keep defaults to 0 (disabled); should return immediately without
	// touching storage or looking up the asset at all.
	if err := svc.ExtractForAsset(context.Background(), wsID, "nonexistent-asset", "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExifService_ExtractForAsset_AssetNotFound_NoOp(t *testing.T) {
	svc, database, wsID := newExifEnv(t, nil)
	if _, err := database.Writer.Exec(`UPDATE workspaces SET exif_keep = 1 WHERE id = ?`, wsID); err != nil {
		t.Fatalf("enable exif_keep: %v", err)
	}
	if err := svc.ExtractForAsset(context.Background(), wsID, "nonexistent-asset", "user-1"); err != nil {
		t.Fatalf("expected nil (not-found is a no-op), got: %v", err)
	}
}

func TestExifService_ExtractForAsset_NonImageAsset_NoOp(t *testing.T) {
	svc, database, wsID := newExifEnv(t, nil)
	if _, err := database.Writer.Exec(`UPDATE workspaces SET exif_keep = 1 WHERE id = ?`, wsID); err != nil {
		t.Fatalf("enable exif_keep: %v", err)
	}
	assetID := uuid.NewString()
	if _, err := database.WQ.CreateAsset(context.Background(), dbgen.CreateAssetParams{
		ID:                   assetID,
		WorkspaceID:          wsID,
		OriginalFilename:     "doc.pdf",
		StorageKey:           "key/doc.pdf",
		MimeType:             "application/pdf",
		Size:                 10,
		ThumbnailContentType: "image/webp",
	}); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	// Non-image asset: should return before ever touching storage (nil storage would panic otherwise).
	if err := svc.ExtractForAsset(context.Background(), wsID, assetID, "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestExifService_ExtractForAsset_MissingStorageObject_WrapsErrNotFound proves
// that storage.ErrNotFound survives multiple layers of wrapping: the storage
// backend wraps its native miss with ErrNotFound, and ExifService.ExtractForAsset
// wraps that again ("open asset: %w") — [errors.Is] must still see through both
// layers. See ROADMAP.73 ST-2.3.
func TestExifService_ExtractForAsset_MissingStorageObject_WrapsErrNotFound(t *testing.T) {
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("NewAferoMemoryStorage: %v", err)
	}
	svc, database, wsID := newExifEnv(t, stor)
	if _, err = database.Writer.Exec(`UPDATE workspaces SET exif_keep = 1 WHERE id = ?`, wsID); err != nil {
		t.Fatalf("enable exif_keep: %v", err)
	}

	assetID := uuid.NewString()
	if _, err = database.WQ.CreateAsset(context.Background(), dbgen.CreateAssetParams{
		ID:                   assetID,
		WorkspaceID:          wsID,
		OriginalFilename:     "photo.jpg",
		StorageKey:           "missing/photo.jpg", // never written to stor
		MimeType:             "image/jpeg",
		Size:                 10,
		ThumbnailContentType: "image/webp",
	}); err != nil {
		t.Fatalf("seed asset: %v", err)
	}

	err = svc.ExtractForAsset(context.Background(), wsID, assetID, "user-1")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("ExtractForAsset: expected errors.Is(err, storage.ErrNotFound), got %v", err)
	}
}

// -- Cross-workspace negative test --
// FieldRepository.EnsureSystemField/ListBySource (added for exif extraction) must
// not leak system field definitions across workspaces.

func TestFieldRepository_SystemFieldCrossWorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewRealFieldRepo()

	wsA, wsB := "ws-a", "ws-b"
	if err := repo.EnsureSystemField(ctx, repository.EnsureSystemFieldParams{
		ID: "field-1", WorkspaceID: wsA, Source: "exif", Name: "Camera maker", Key: "_exif_make", FieldType: "text",
	}); err != nil {
		t.Fatalf("ensure system field: %v", err)
	}

	out, err := repo.ListBySource(ctx, wsB, "exif")
	if err != nil || len(out) != 0 {
		t.Errorf("ListBySource cross-workspace: expected empty, got %v, err=%v", out, err)
	}

	if _, keyErr := repo.GetByKey(ctx, wsB, "_exif_make"); !errors.Is(keyErr, apperr.ErrNotFound) {
		t.Errorf("GetByKey cross-workspace: expected ErrNotFound, got %v", keyErr)
	}

	// Sanity: same-workspace access still works after all the negative checks above.
	sameWS, err := repo.ListBySource(ctx, wsA, "exif")
	if err != nil || len(sameWS) != 1 {
		t.Errorf("ListBySource same-workspace: expected 1 field, got %v, err=%v", sameWS, err)
	}
}
