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

func newFolderSvc(t *testing.T) (service.FolderService, *memory.FolderRepo) {
	t.Helper()
	repo := memory.NewRealFolderRepo()
	return service.NewFolderService(repo), repo
}

// --- Create ---

func TestFolderService_Create_OK(t *testing.T) {
	t.Parallel()
	svc, _ := newFolderSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{Name: "Drafts"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Drafts" {
		t.Errorf("Name: got %q, want %q", dto.Name, "Drafts")
	}
	if dto.ProjectID != "proj_1" {
		t.Errorf("ProjectID: got %q, want %q", dto.ProjectID, "proj_1")
	}
	if dto.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestFolderService_Create_EmptyName(t *testing.T) {
	t.Parallel()
	svc, _ := newFolderSvc(t)
	_, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{Name: "   "})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestFolderService_Create_ParentNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newFolderSvc(t)
	parentID := "nonexistent"
	_, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{
		Name:     "Child",
		ParentID: &parentID,
	})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFolderService_Create_MaxDepth(t *testing.T) {
	t.Parallel()
	svc, repo := newFolderSvc(t)
	parentID := "parent_1"
	grandparentID := "grand_1"
	// parent_1 itself has a parent (grandparent), so creating a child under parent_1 exceeds depth 2
	repo.Seed(repository.Folder{
		ID:          parentID,
		WorkspaceID: "ws_1",
		ProjectID:   "proj_1",
		ParentID:    &grandparentID,
		Name:        "Level2",
	})
	_, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{
		Name:     "TooDeep",
		ParentID: &parentID,
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (max depth), got %v", err)
	}
}

func TestFolderService_Create_ParentDifferentProject(t *testing.T) {
	t.Parallel()
	svc, repo := newFolderSvc(t)
	repo.Seed(repository.Folder{
		ID:          "parent_1",
		WorkspaceID: "ws_1",
		ProjectID:   "proj_OTHER",
		Name:        "OtherProj",
	})
	parentID := "parent_1"
	_, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{
		Name:     "BadChild",
		ParentID: &parentID,
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (wrong project), got %v", err)
	}
}

func TestFolderService_Create_DuplicateName(t *testing.T) {
	t.Parallel()
	svc, _ := newFolderSvc(t)
	if _, err := svc.Create(
		context.Background(),
		"ws_1",
		"proj_1",
		service.CreateFolderParams{Name: "Drafts"},
	); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{Name: "Drafts"})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate name, got %v", err)
	}
}

// --- Get ---

func TestFolderService_Get_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newFolderSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Update ---

func TestFolderService_Update_EmptyName(t *testing.T) {
	t.Parallel()
	svc, repo := newFolderSvc(t)
	repo.Seed(repository.Folder{ID: "f1", WorkspaceID: "ws_1", ProjectID: "p1", Name: "Original"})
	empty := ""
	_, err := svc.Update(context.Background(), "ws_1", "f1", service.UpdateFolderParams{Name: &empty})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestFolderService_Update_OK(t *testing.T) {
	t.Parallel()
	svc, repo := newFolderSvc(t)
	repo.Seed(repository.Folder{ID: "f1", WorkspaceID: "ws_1", ProjectID: "p1", Name: "Old"})
	newName := "New"
	dto, err := svc.Update(context.Background(), "ws_1", "f1", service.UpdateFolderParams{Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "New" {
		t.Errorf("Name: got %q, want %q", dto.Name, "New")
	}
}

// folderNullifySpyRepo wraps RealFolderRepo and records whether NullifyAssets was called.
type folderNullifySpyRepo struct {
	*memory.FolderRepo

	nullifyCalled bool
}

func (r *folderNullifySpyRepo) NullifyAssets(_ context.Context, _, _ string) error {
	r.nullifyCalled = true
	return nil
}

// --- Delete ---

func TestFolderService_Delete_NullifiesAssets(t *testing.T) {
	t.Parallel()
	inner := memory.NewRealFolderRepo()
	spy := &folderNullifySpyRepo{FolderRepo: inner}
	svc := service.NewFolderService(spy)
	inner.Seed(repository.Folder{ID: "f1", WorkspaceID: "ws_1", ProjectID: "p1", Name: "ToDelete"})

	if err := svc.Delete(context.Background(), "ws_1", "f1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spy.nullifyCalled {
		t.Error("expected NullifyAssets to be called before folder deletion")
	}
}

func TestFolderService_Delete_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newFolderSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFolderService_Delete_CascadesChildren(t *testing.T) {
	t.Parallel()
	svc, repo := newFolderSvc(t)
	parentID := "parent_1"
	repo.Seed(
		repository.Folder{ID: parentID, WorkspaceID: "ws_1", ProjectID: "p1", Name: "Parent"},
		repository.Folder{ID: "child_1", WorkspaceID: "ws_1", ProjectID: "p1", ParentID: &parentID, Name: "Child"},
	)
	if err := svc.Delete(context.Background(), "ws_1", parentID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "ws_1", parentID); !errors.Is(err, apperr.ErrNotFound) {
		t.Error("parent should be deleted")
	}
	if _, err := svc.Get(context.Background(), "ws_1", "child_1"); !errors.Is(err, apperr.ErrNotFound) {
		t.Error("child should be deleted")
	}
}

// --- List ---

func TestFolderService_List_OK(t *testing.T) {
	t.Parallel()
	svc, _ := newFolderSvc(t)
	svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{Name: "Alpha"})
	svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{Name: "Beta"})
	svc.Create(context.Background(), "ws_1", "proj_2", service.CreateFolderParams{Name: "Gamma"})

	folders, err := svc.List(context.Background(), "ws_1", "proj_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(folders) != 2 {
		t.Errorf("expected 2 folders for proj_1, got %d", len(folders))
	}
}

func TestFolderService_List_Empty(t *testing.T) {
	t.Parallel()
	svc, _ := newFolderSvc(t)
	folders, err := svc.List(context.Background(), "ws_1", "proj_none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(folders) != 0 {
		t.Errorf("expected empty, got %d", len(folders))
	}
}

// --- ListTree ---

func TestFolderService_ListTree_OK(t *testing.T) {
	t.Parallel()
	svc, _ := newFolderSvc(t)
	parent, _ := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{Name: "Parent"})
	svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{Name: "Child", ParentID: &parent.ID})

	tree, err := svc.ListTree(context.Background(), "ws_1", "proj_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Tree should have at least 1 root node
	if len(tree) == 0 {
		t.Error("expected non-empty tree")
	}
}
