package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/storage"
)

const dupTestWorkspaceID = "ws_dup"

func newDuplicateEnv(t *testing.T) (
	service.DuplicateService,
	*memory.AssetRepo,
	*memory.RealVersionRepo,
	*memory.RealWorkspaceRepo,
	storage.Storage,
) {
	t.Helper()
	assets := memory.NewAssetRepo()
	versions := memory.NewRealVersionRepo()
	workspaces := memory.NewRealWorkspaceRepo()
	workspaces.Seed(repository.Workspace{ID: dupTestWorkspaceID, Name: "ws", DuplicateDetectionMode: "warn"})
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	svc := service.NewDuplicateService(versions, assets, workspaces, stor)
	return svc, assets, versions, workspaces, stor
}

func seedDupAsset(assets *memory.AssetRepo, assetID, filename string) {
	assets.Seed(repository.Asset{
		ID: assetID, WorkspaceID: dupTestWorkspaceID,
		OriginalFilename: filename, StorageKey: "k/" + assetID, MimeType: "image/jpeg",
	})
}

func TestDuplicateService_FindDuplicate_NoMatch(t *testing.T) {
	svc, assets, versions, _, _ := newDuplicateEnv(t)
	seedDupAsset(assets, "ast_1", "a.jpg")
	versions.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: dupTestWorkspaceID, ContentHash: "hash-a", StorageKey: "k/ast_1",
	})

	dup, err := svc.FindDuplicate(context.Background(), dupTestWorkspaceID, "hash-b", "ast_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup != nil {
		t.Fatalf("expected no match, got %+v", dup)
	}
}

func TestDuplicateService_FindDuplicate_MatchesOtherAsset(t *testing.T) {
	svc, assets, versions, _, stor := newDuplicateEnv(t)
	seedDupAsset(assets, "ast_1", "existing.jpg")
	versions.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: dupTestWorkspaceID,
		ContentHash: "hash-a", StorageKey: "k/ast_1", IsCurrent: true, CreatedAt: time.Now(),
	})
	if err := stor.Put(context.Background(), "k/ast_1", strings.NewReader("bytes")); err != nil {
		t.Fatalf("seed storage: %v", err)
	}

	dup, err := svc.FindDuplicate(context.Background(), dupTestWorkspaceID, "hash-a", "ast_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup == nil {
		t.Fatal("expected a match")
	}
	if dup.AssetID != "ast_1" || dup.OriginalFilename != "existing.jpg" {
		t.Errorf("got %+v", dup)
	}
	if !dup.StorageAvailable {
		t.Errorf("expected StorageAvailable=true when the storage key exists")
	}
}

func TestDuplicateService_FindDuplicate_ExcludesSelf(t *testing.T) {
	svc, assets, versions, _, _ := newDuplicateEnv(t)
	seedDupAsset(assets, "ast_1", "a.jpg")
	versions.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: dupTestWorkspaceID, ContentHash: "hash-a", StorageKey: "k/ast_1",
	})

	dup, err := svc.FindDuplicate(context.Background(), dupTestWorkspaceID, "hash-a", "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup != nil {
		t.Fatalf("expected self-match to be excluded, got %+v", dup)
	}
}

func TestDuplicateService_FindDuplicate_PrefersLiveOverDeleted(t *testing.T) {
	svc, assets, versions, _, _ := newDuplicateEnv(t)
	seedDupAsset(assets, "ast_deleted", "deleted.jpg")
	seedDupAsset(assets, "ast_live", "live.jpg")
	deletedAt := "deleted"
	versions.Seed(
		repository.AssetVersion{
			ID: "v_deleted", AssetID: "ast_deleted", WorkspaceID: dupTestWorkspaceID,
			ContentHash: "hash-a", StorageKey: "k/ast_deleted", DeletedAt: &deletedAt,
			CreatedAt: time.Now().Add(-time.Hour),
		},
		repository.AssetVersion{
			ID: "v_live", AssetID: "ast_live", WorkspaceID: dupTestWorkspaceID,
			ContentHash: "hash-a", StorageKey: "k/ast_live", IsCurrent: true,
			CreatedAt: time.Now(),
		},
	)

	dup, err := svc.FindDuplicate(context.Background(), dupTestWorkspaceID, "hash-a", "ast_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup == nil || dup.AssetID != "ast_live" {
		t.Fatalf("expected live version to win, got %+v", dup)
	}
	if dup.IsDeletedVersion {
		t.Errorf("expected IsDeletedVersion=false for the live match")
	}
}

func TestDuplicateService_FindDuplicate_MatchesSoftDeletedOnly(t *testing.T) {
	svc, assets, versions, _, _ := newDuplicateEnv(t)
	seedDupAsset(assets, "ast_deleted", "deleted.jpg")
	deletedAt := "deleted"
	versions.Seed(repository.AssetVersion{
		ID: "v_deleted", AssetID: "ast_deleted", WorkspaceID: dupTestWorkspaceID,
		ContentHash: "hash-a", StorageKey: "k/ast_deleted", DeletedAt: &deletedAt,
	})

	dup, err := svc.FindDuplicate(context.Background(), dupTestWorkspaceID, "hash-a", "ast_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup == nil {
		t.Fatal("expected a match against the soft-deleted version")
	}
	if !dup.IsDeletedVersion {
		t.Errorf("expected IsDeletedVersion=true")
	}
}

func TestDuplicateService_FindDuplicate_StorageUnavailable(t *testing.T) {
	svc, assets, versions, _, _ := newDuplicateEnv(t)
	seedDupAsset(assets, "ast_1", "a.jpg")
	versions.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: dupTestWorkspaceID,
		ContentHash: "hash-a", StorageKey: "k/does-not-exist-in-storage",
	})

	dup, err := svc.FindDuplicate(context.Background(), dupTestWorkspaceID, "hash-a", "ast_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup == nil {
		t.Fatal("expected a match")
	}
	if dup.StorageAvailable {
		t.Errorf("expected StorageAvailable=false for a storage key never written")
	}
}

func TestDuplicateService_FindDuplicate_ScopedToWorkspace(t *testing.T) {
	svc, assets, versions, _, _ := newDuplicateEnv(t)
	seedDupAsset(assets, "ast_other_ws", "a.jpg")
	versions.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_other_ws", WorkspaceID: "ws_other", ContentHash: "hash-a", StorageKey: "k/ast_other_ws",
	})

	dup, err := svc.FindDuplicate(context.Background(), dupTestWorkspaceID, "hash-a", "ast_new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup != nil {
		t.Fatalf("expected no cross-workspace match, got %+v", dup)
	}
}

func TestDuplicateService_Mode_DefaultsToWarn(t *testing.T) {
	svc, _, _, workspaces, _ := newDuplicateEnv(t)
	workspaces.Seed(repository.Workspace{ID: "ws_empty_mode", Name: "ws"})

	mode, err := svc.Mode(context.Background(), "ws_empty_mode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != service.DuplicateModeWarn {
		t.Errorf("got %q, want warn", mode)
	}
}

func TestDuplicateService_Mode_Off(t *testing.T) {
	svc, _, _, workspaces, _ := newDuplicateEnv(t)
	workspaces.Seed(repository.Workspace{ID: "ws_off", Name: "ws", DuplicateDetectionMode: "off"})

	mode, err := svc.Mode(context.Background(), "ws_off")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != service.DuplicateModeOff {
		t.Errorf("got %q, want off", mode)
	}
}

func TestDuplicateService_Mode_Block(t *testing.T) {
	svc, _, _, workspaces, _ := newDuplicateEnv(t)
	workspaces.Seed(repository.Workspace{ID: "ws_block", Name: "ws", DuplicateDetectionMode: "block"})

	mode, err := svc.Mode(context.Background(), "ws_block")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != service.DuplicateModeBlock {
		t.Errorf("got %q, want block", mode)
	}
}
