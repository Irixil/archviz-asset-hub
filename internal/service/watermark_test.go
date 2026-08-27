package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

func newWatermarkSvc(t *testing.T) (service.WatermarkService, *memory.AssetRepo) {
	t.Helper()
	assets := memory.NewAssetRepo()
	svc := service.NewWatermarkService(assets, memory.NewRealFolderRepo())
	return svc, assets
}

func TestWatermarkService_ResolveWatermarkAsset_FolderScope(t *testing.T) {
	svc, assets := newWatermarkSvc(t)
	folderID := "folder-1"
	assets.Seed(
		repository.Asset{
			ID: "asset-target", WorkspaceID: "ws-1", FolderID: &folderID,
			OriginalFilename: "photo.jpg", CreatedAt: time.Now(),
		},
		repository.Asset{
			ID: "asset-wm", WorkspaceID: "ws-1", FolderID: &folderID,
			OriginalFilename: "my-watermark.png", StorageKey: "key/wm.png", MimeType: "image/png",
			CreatedAt: time.Now(),
		},
	)

	dto, err := svc.ResolveWatermarkAsset(context.Background(), "ws-1", "asset-target")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.ID != "asset-wm" || dto.Scope != "folder" {
		t.Errorf("expected asset-wm/folder, got %+v", dto)
	}
}

func TestWatermarkService_ResolveWatermarkAsset_FallsBackToWorkspace(t *testing.T) {
	svc, assets := newWatermarkSvc(t)
	folderID := "folder-1"
	assets.Seed(
		repository.Asset{
			ID: "asset-target", WorkspaceID: "ws-1", FolderID: &folderID,
			OriginalFilename: "photo.jpg", CreatedAt: time.Now(),
		},
		repository.Asset{
			ID: "asset-wm", WorkspaceID: "ws-1",
			OriginalFilename: "WATERMARK.png", StorageKey: "key/wm.png", MimeType: "image/png",
			CreatedAt: time.Now(),
		},
	)

	dto, err := svc.ResolveWatermarkAsset(context.Background(), "ws-1", "asset-target")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.ID != "asset-wm" || dto.Scope != "workspace" {
		t.Errorf("expected asset-wm/workspace, got %+v", dto)
	}
}

func TestWatermarkService_ResolveWatermarkAsset_NoneFound(t *testing.T) {
	svc, assets := newWatermarkSvc(t)
	assets.Seed(
		repository.Asset{ID: "asset-target", WorkspaceID: "ws-1", OriginalFilename: "photo.jpg", CreatedAt: time.Now()},
	)

	_, err := svc.ResolveWatermarkAsset(context.Background(), "ws-1", "asset-target")
	if !errors.Is(err, service.ErrNoWatermarkAsset) || !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrNoWatermarkAsset+ErrInvalidInput, got %v", err)
	}
}

// -- Cross-workspace negative test --
// AssetRepository's watermark finders must not leak assets across workspaces.

func TestAssetRepository_WatermarkCrossWorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	assets := memory.NewAssetRepo()
	folderID, projectID := "folder-1", "project-1"
	assets.Seed(repository.Asset{
		ID: "wm-1", WorkspaceID: "ws-a", FolderID: &folderID, ProjectID: &projectID,
		OriginalFilename: "watermark.png", CreatedAt: time.Now(),
	})

	if _, err := assets.FindWatermarkInFolder(ctx, "ws-b", folderID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("FindWatermarkInFolder cross-workspace: expected ErrNotFound, got %v", err)
	}
	if _, err := assets.FindWatermarkInProject(ctx, "ws-b", projectID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("FindWatermarkInProject cross-workspace: expected ErrNotFound, got %v", err)
	}
	if _, err := assets.FindWatermarkInWorkspace(ctx, "ws-b"); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("FindWatermarkInWorkspace cross-workspace: expected ErrNotFound, got %v", err)
	}

	// Sanity: same-workspace access still works after all the negative checks above.
	if _, err := assets.FindWatermarkInWorkspace(ctx, "ws-a"); err != nil {
		t.Errorf("FindWatermarkInWorkspace same-workspace: unexpected error %v", err)
	}
}
