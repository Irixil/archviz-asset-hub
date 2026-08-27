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

func newTextTrackSvc(t *testing.T) service.TextTrackService {
	t.Helper()
	return service.NewTextTrackService(memory.NewTextTrackRepo(), nil, nil)
}

func TestTextTrackService_Create_Manual_OK(t *testing.T) {
	svc := newTextTrackSvc(t)
	dto, err := svc.Create(context.Background(), service.CreateTextTrackParams{
		WorkspaceID:    "ws-1",
		AssetID:        "asset-1",
		Source:         "manual",
		InitialContent: "hello world",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Status != "ready" {
		t.Errorf("Status: got %q, want %q", dto.Status, "ready")
	}
	if dto.Content != "hello world" {
		t.Errorf("Content: got %q, want %q", dto.Content, "hello world")
	}
}

func TestTextTrackService_Create_EmptySource(t *testing.T) {
	svc := newTextTrackSvc(t)
	_, err := svc.Create(context.Background(), service.CreateTextTrackParams{
		WorkspaceID: "ws-1",
		AssetID:     "asset-1",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTextTrackService_Get_NotFound(t *testing.T) {
	svc := newTextTrackSvc(t)
	_, err := svc.Get(context.Background(), "ws-1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTextTrackService_List_WorkspaceIsolation(t *testing.T) {
	svc := newTextTrackSvc(t)
	_, err := svc.Create(context.Background(), service.CreateTextTrackParams{
		WorkspaceID:    "ws-1",
		AssetID:        "asset-1",
		Source:         "manual",
		InitialContent: "hi",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	out, err := svc.List(context.Background(), "ws-2", "asset-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected 0 tracks for other workspace, got %d", len(out))
	}
}

func TestTextTrackService_Delete_OK(t *testing.T) {
	svc := newTextTrackSvc(t)
	dto, err := svc.Create(context.Background(), service.CreateTextTrackParams{
		WorkspaceID:    "ws-1",
		AssetID:        "asset-1",
		Source:         "manual",
		InitialContent: "hi",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if delErr := svc.Delete(context.Background(), "ws-1", dto.ID); delErr != nil {
		t.Fatalf("unexpected error: %v", delErr)
	}
	if _, getErr := svc.Get(context.Background(), "ws-1", dto.ID); !errors.Is(getErr, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", getErr)
	}
}

// -- Cross-workspace negative test --
// Every read/update/delete method on TextTrackRepository must return not-found
// when called with a different workspace ID than the one that owns the track.

func TestTextTrackRepository_CrossWorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewTextTrackRepo()

	wsA, wsB := "ws-a", "ws-b"
	track, err := repo.Create(ctx, repository.CreateTextTrackParams{
		ID: "track-1", WorkspaceID: wsA, AssetID: "asset-1", Source: "manual", Status: "ready",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	checks := []struct {
		name string
		run  func() error
	}{
		{"Get", func() error { _, getErr := repo.Get(ctx, wsB, track.ID); return getErr }},
		{"SetReady", func() error {
			return repo.SetReady(ctx, wsB, track.ID, repository.SetTextTrackReadyParams{Content: "x"})
		}},
		{"SetFailed", func() error { return repo.SetFailed(ctx, wsB, track.ID, "boom") }},
		{"Delete", func() error { return repo.Delete(ctx, wsB, track.ID) }},
	}
	for _, c := range checks {
		if checkErr := c.run(); !errors.Is(checkErr, apperr.ErrNotFound) {
			t.Errorf("%s cross-workspace: expected ErrNotFound, got %v", c.name, checkErr)
		}
	}

	if out, listErr := repo.List(ctx, wsB, "asset-1"); listErr != nil || len(out) != 0 {
		t.Errorf("List cross-workspace: expected empty result, got %v, err=%v", out, listErr)
	}

	// Sanity: same-workspace access still works after all the negative checks above.
	if _, getErr := repo.Get(ctx, wsA, track.ID); getErr != nil {
		t.Errorf("Get same-workspace: unexpected error %v", getErr)
	}
}
