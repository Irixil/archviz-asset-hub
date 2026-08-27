package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

func newProjectSvc(t *testing.T) (service.ProjectService, *memory.ProjectRepo) {
	t.Helper()
	repo := memory.NewProjectRepo()
	svc := service.NewProjectService(repo, audit.NopWriter{})
	return svc, repo
}

func newProjectSvcSpy(t *testing.T) (service.ProjectService, *memory.ProjectRepo, *spyWriter) {
	t.Helper()
	spy := newSpy()
	repo := memory.NewProjectRepo()
	return service.NewProjectService(repo, spy), repo, spy
}

// --- Create ---

func TestProjectService_Create_OK(t *testing.T) {
	svc, _ := newProjectSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", service.CreateProjectParams{Name: "Alpha"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Alpha" || dto.WorkspaceID != "ws_1" {
		t.Errorf("unexpected dto: %+v", dto)
	}
	if dto.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestProjectService_Create_EmptyName(t *testing.T) {
	svc, _ := newProjectSvc(t)
	_, err := svc.Create(context.Background(), "ws_1", service.CreateProjectParams{Name: "   "})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// --- Get ---

func TestProjectService_Get_NotFound(t *testing.T) {
	svc, _ := newProjectSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "missing")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_Get_WrongWorkspace(t *testing.T) {
	svc, repo := newProjectSvc(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_A", Name: "X"})
	_, err := svc.Get(context.Background(), "ws_B", "p1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_Get_OK(t *testing.T) {
	svc, repo := newProjectSvc(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "Beta"})
	dto, err := svc.Get(context.Background(), "ws_1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Beta" {
		t.Errorf("Name: got %q, want %q", dto.Name, "Beta")
	}
}

// --- Update ---

func TestProjectService_Update_EmptyName(t *testing.T) {
	svc, repo := newProjectSvc(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "Gamma"})
	empty := ""
	_, err := svc.Update(context.Background(), "ws_1", "p1", service.UpdateProjectParams{Name: &empty})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestProjectService_Update_NotFound(t *testing.T) {
	svc, _ := newProjectSvc(t)
	name := "X"
	_, err := svc.Update(context.Background(), "ws_1", "nope", service.UpdateProjectParams{Name: &name})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_Update_OK(t *testing.T) {
	svc, repo := newProjectSvc(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "Old"})
	newName := "New"
	dto, err := svc.Update(context.Background(), "ws_1", "p1", service.UpdateProjectParams{Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "New" {
		t.Errorf("Name: got %q, want %q", dto.Name, "New")
	}
}

// --- Delete ---

func TestProjectService_Delete_NotFound(t *testing.T) {
	svc, _ := newProjectSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_Delete_OK(t *testing.T) {
	svc, repo := newProjectSvc(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "ToDelete"})
	if err := svc.Delete(context.Background(), "ws_1", "p1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := svc.Get(context.Background(), "ws_1", "p1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

// nullifySpyRepo wraps ProjectRepo and records whether NullifyAssets was called.
type nullifySpyRepo struct {
	*memory.ProjectRepo

	nullifyCalled bool
}

func (r *nullifySpyRepo) NullifyAssets(_ context.Context, _, _ string) error {
	r.nullifyCalled = true
	return nil
}

func TestProjectService_Delete_UnlinksAssets(t *testing.T) {
	inner := memory.NewProjectRepo()
	spy := &nullifySpyRepo{ProjectRepo: inner}
	svc := service.NewProjectService(spy, audit.NopWriter{})
	inner.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "ToDelete"})

	if err := svc.Delete(context.Background(), "ws_1", "p1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spy.nullifyCalled {
		t.Error("expected NullifyAssets to be called before project deletion")
	}
}

func TestProjectService_Update_PreservesCoverAsset(t *testing.T) {
	svc, repo := newProjectSvc(t)
	coverID := "ast_cover_1"
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "Alpha", CoverAssetID: &coverID})

	newDesc := "updated desc"
	dto, err := svc.Update(context.Background(), "ws_1", "p1", service.UpdateProjectParams{Description: &newDesc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.CoverAssetID == nil || *dto.CoverAssetID != coverID {
		t.Errorf("CoverAssetID: got %v, want %q (should be preserved on partial update)", dto.CoverAssetID, coverID)
	}
}

// --- Audit events ---

func TestProjectService_Create_EmitsAuditEvent(t *testing.T) {
	svc, _, spy := newProjectSvcSpy(t)
	if _, err := svc.Create(context.Background(), "ws_1", service.CreateProjectParams{Name: "Alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastProject()
	if e.EventType != audit.EventProjectCreated {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventProjectCreated)
	}
	if e.WorkspaceID != "ws_1" {
		t.Errorf("WorkspaceID: got %q, want %q", e.WorkspaceID, "ws_1")
	}
}

func TestProjectService_Update_Rename_EmitsAuditEvent(t *testing.T) {
	svc, repo, spy := newProjectSvcSpy(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "Old"})
	newName := "New"
	if _, err := svc.Update(
		context.Background(),
		"ws_1",
		"p1",
		service.UpdateProjectParams{Name: &newName},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastProject()
	if e.EventType != audit.EventProjectRenamed {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventProjectRenamed)
	}
	if e.ProjectID != "p1" {
		t.Errorf("ProjectID: got %q, want %q", e.ProjectID, "p1")
	}
}

func TestProjectService_Delete_EmitsAuditEvent(t *testing.T) {
	svc, repo, spy := newProjectSvcSpy(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "ToDelete"})
	if err := svc.Delete(context.Background(), "ws_1", "p1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastProject()
	if e.EventType != audit.EventProjectDeleted {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventProjectDeleted)
	}
	if e.ProjectID != "p1" {
		t.Errorf("ProjectID: got %q, want %q", e.ProjectID, "p1")
	}
}

// --- List ---

func TestProjectService_List_OK(t *testing.T) {
	t.Parallel()
	svc, _ := newProjectSvc(t)
	svc.Create(context.Background(), "ws_1", service.CreateProjectParams{Name: "Alpha"})
	svc.Create(context.Background(), "ws_1", service.CreateProjectParams{Name: "Beta"})
	svc.Create(context.Background(), "ws_2", service.CreateProjectParams{Name: "Gamma"})

	projects, err := svc.List(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects for ws_1, got %d", len(projects))
	}
}

func TestProjectService_List_Empty(t *testing.T) {
	t.Parallel()
	svc, _ := newProjectSvc(t)
	projects, err := svc.List(context.Background(), "ws_none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected empty list, got %d", len(projects))
	}
}
