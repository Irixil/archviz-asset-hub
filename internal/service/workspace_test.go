package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"damask/server/internal/ai"
	"damask/server/internal/apperr"
	"damask/server/internal/auth"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

var stubResolver ai.KeyResolver = func(_ context.Context, _, _ string) (string, ai.KeySource, error) {
	return "", "", nil
}

func newWorkspaceSvc(t *testing.T) (service.WorkspaceService, *memory.RealWorkspaceRepo) {
	t.Helper()
	repo := memory.NewRealWorkspaceRepo()
	return service.NewWorkspaceService(repo, memory.NewUserRepo(), "test-app-secret", stubResolver), repo
}

// --- Get ---

func TestWorkspaceService_Get_NotFound(t *testing.T) {
	svc, _ := newWorkspaceSvc(t)
	_, err := svc.Get(context.Background(), "ws_nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkspaceService_Get_OK(t *testing.T) {
	svc, repo := newWorkspaceSvc(t)
	repo.Seed(repository.Workspace{ID: "ws_1", Name: "Acme", VersionRetentionCount: 5})
	dto, err := svc.Get(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Acme" {
		t.Errorf("Name: got %q, want %q", dto.Name, "Acme")
	}
	if dto.VersionRetentionCount != 5 {
		t.Errorf("VersionRetentionCount: got %d, want 5", dto.VersionRetentionCount)
	}
}

// --- Update ---

func TestWorkspaceService_Update_VersionRetention(t *testing.T) {
	svc, repo := newWorkspaceSvc(t)
	repo.Seed(repository.Workspace{ID: "ws_1", Name: "Test", VersionRetentionCount: 3})
	newCount := int64(10)
	dto, err := svc.Update(
		context.Background(),
		"ws_1",
		service.UpdateWorkspaceParams{VersionRetentionCount: &newCount},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.VersionRetentionCount != 10 {
		t.Errorf("VersionRetentionCount: got %d, want 10", dto.VersionRetentionCount)
	}
}

func TestWorkspaceService_Update_ExifSettings(t *testing.T) {
	svc, repo := newWorkspaceSvc(t)
	repo.Seed(repository.Workspace{ID: "ws_1", Name: "Test", ExifKeep: false, ExifKeepGps: false})
	keepTrue := true
	dto, err := svc.Update(context.Background(), "ws_1", service.UpdateWorkspaceParams{ExifKeep: &keepTrue})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dto.ExifKeep {
		t.Error("ExifKeep should be true after update")
	}
}

func TestWorkspaceService_Update_NotFound(t *testing.T) {
	svc, _ := newWorkspaceSvc(t)
	count := int64(5)
	_, err := svc.Update(context.Background(), "ws_nope", service.UpdateWorkspaceParams{VersionRetentionCount: &count})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Me ---

func newWorkspaceSvcWithUsers(
	t *testing.T,
) (service.WorkspaceService, *memory.RealWorkspaceRepo, *memory.RealUserRepo) {
	t.Helper()
	wsRepo := memory.NewRealWorkspaceRepo()
	userRepo := memory.NewRealUserRepo()
	wsRepo.SetUserRepo(userRepo)
	return service.NewWorkspaceService(wsRepo, userRepo, "test-app-secret", stubResolver), wsRepo, userRepo
}

func TestWorkspaceService_Me_OK(t *testing.T) {
	t.Parallel()
	svc, wsRepo, userRepo := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Acme"})
	userRepo.Seed(repository.User{ID: "u_1", Email: "alice@example.com", Name: "Alice"})
	_ = wsRepo.CreateMember(context.Background(), repository.Member{
		WorkspaceID: "ws_1", UserID: "u_1", Role: string(auth.Owner),
	})

	dto, err := svc.Me(context.Background(), "ws_1", "u_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.UserEmail != "alice@example.com" {
		t.Errorf("UserEmail: got %q, want alice@example.com", dto.UserEmail)
	}
	if dto.Role != string(auth.Owner) {
		t.Errorf("Role: got %q, want owner", dto.Role)
	}
	if dto.Workspace.Name != "Acme" {
		t.Errorf("Workspace.Name: got %q, want Acme", dto.Workspace.Name)
	}
}

func TestWorkspaceService_Me_NotMember(t *testing.T) {
	t.Parallel()
	svc, wsRepo, userRepo := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Acme"})
	userRepo.Seed(repository.User{ID: "u_1", Email: "alice@example.com", Name: "Alice"})

	_, err := svc.Me(context.Background(), "ws_1", "u_1")
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for non-member, got %v", err)
	}
}

// --- ListForUser ---

func TestWorkspaceService_ListForUser_OK(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(
		repository.Workspace{ID: "ws_1", Name: "Alpha"},
		repository.Workspace{ID: "ws_2", Name: "Beta"},
	)
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_1", Role: string(auth.Owner)},
	)
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_2", UserID: "u_1", Role: string(auth.Editor)},
	)

	list, err := svc.ListForUser(context.Background(), "u_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(list))
	}
}

func TestWorkspaceService_ListForUser_Empty(t *testing.T) {
	t.Parallel()
	svc, _, _ := newWorkspaceSvcWithUsers(t)
	list, err := svc.ListForUser(context.Background(), "u_nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

// --- CountAssets ---

func TestWorkspaceService_CountAssets_OK(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	n, err := svc.CountAssets(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n < 0 {
		t.Errorf("expected non-negative count, got %d", n)
	}
}

// --- GetMember ---

func TestWorkspaceService_GetMember_OK(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	_ = wsRepo.CreateMember(context.Background(), repository.Member{
		WorkspaceID: "ws_1", UserID: "u_1", Email: "bob@example.com", Name: "Bob", Role: string(auth.Editor),
	})

	m, err := svc.GetMember(context.Background(), "ws_1", "u_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Email != "bob@example.com" {
		t.Errorf("Email: got %q, want bob@example.com", m.Email)
	}
	if m.Role != string(auth.Editor) {
		t.Errorf("Role: got %q, want editor", m.Role)
	}
}

func TestWorkspaceService_GetMember_NotFound(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	_, err := svc.GetMember(context.Background(), "ws_1", "u_nobody")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- ListMembers ---

func TestWorkspaceService_ListMembers_OK(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_1", Role: string(auth.Owner)},
	)
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_2", Role: string(auth.Editor)},
	)

	members, err := svc.ListMembers(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

// --- RemoveMember ---

func TestWorkspaceService_RemoveMember_OK(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_owner", Role: string(auth.Owner)},
	)
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_editor", Role: string(auth.Editor)},
	)

	err := svc.RemoveMember(context.Background(), "ws_1", "u_owner", "u_editor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	members, _ := svc.ListMembers(context.Background(), "ws_1")
	if len(members) != 1 {
		t.Errorf("expected 1 member remaining, got %d", len(members))
	}
}

func TestWorkspaceService_RemoveMember_CannotRemoveSelf(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_1", Role: string(auth.Owner)},
	)

	err := svc.RemoveMember(context.Background(), "ws_1", "u_1", "u_1")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput removing self, got %v", err)
	}
}

func TestWorkspaceService_RemoveMember_CannotRemoveLastOwner(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_owner", Role: string(auth.Owner)},
	)
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_caller", Role: string(auth.Editor)},
	)

	err := svc.RemoveMember(context.Background(), "ws_1", "u_caller", "u_owner")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput removing last owner, got %v", err)
	}
}

// --- UpdateMemberRole ---

func TestWorkspaceService_UpdateMemberRole_OK(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_owner", Role: string(auth.Owner)},
	)
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_editor", Role: string(auth.Editor)},
	)

	err := svc.UpdateMemberRole(context.Background(), "ws_1", "u_owner", "u_editor", string(auth.Owner))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, _ := svc.GetMember(context.Background(), "ws_1", "u_editor")
	if m.Role != string(auth.Owner) {
		t.Errorf("Role: got %q, want owner", m.Role)
	}
}

func TestWorkspaceService_UpdateMemberRole_CannotDemoteLastOwner(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	_ = wsRepo.CreateMember(
		context.Background(),
		repository.Member{WorkspaceID: "ws_1", UserID: "u_owner", Role: string(auth.Owner)},
	)

	err := svc.UpdateMemberRole(context.Background(), "ws_1", "u_owner", "u_owner", string(auth.Editor))
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput demoting last owner, got %v", err)
	}
}

// --- CreateInvite / ListInvites / DeleteInvite ---

func TestWorkspaceService_CreateInvite_OK(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})

	inv, err := svc.CreateInvite(context.Background(), "ws_1", "u_owner", service.CreateInviteParams{
		Email: "newbie@example.com",
		Role:  auth.Editor,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Email != "newbie@example.com" {
		t.Errorf("Email: got %q, want newbie@example.com", inv.Email)
	}
	if inv.InviteToken == "" {
		t.Error("expected non-empty InviteToken")
	}
}

func TestWorkspaceService_ListInvites_TokenOmitted(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	_, _ = svc.CreateInvite(context.Background(), "ws_1", "u_owner", service.CreateInviteParams{
		Email: "x@example.com",
		Role:  auth.Editor,
	})

	list, err := svc.ListInvites(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 invite, got %d", len(list))
	}
	if list[0].InviteToken != "" {
		t.Error("InviteToken should be empty in list response")
	}
}

func TestWorkspaceService_DeleteInvite_OK(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})
	inv, _ := svc.CreateInvite(context.Background(), "ws_1", "u_owner", service.CreateInviteParams{
		Email: "x@example.com",
		Role:  auth.Editor,
	})

	err := svc.DeleteInvite(context.Background(), "ws_1", inv.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list, _ := svc.ListInvites(context.Background(), "ws_1")
	if len(list) != 0 {
		t.Errorf("expected 0 invites after delete, got %d", len(list))
	}
}

// --- AcceptInvite ---

func TestWorkspaceService_AcceptInvite_OK(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})

	// create invite directly in the repo so we have the token
	rawInv, _ := wsRepo.CreateInvite(context.Background(), repository.Invite{
		WorkspaceID: "ws_1",
		Email:       "newbie@example.com",
		Role:        string(auth.Editor),
		InvitedBy:   "u_owner",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	})

	result, err := svc.AcceptInvite(context.Background(), service.AcceptInviteParams{
		Token:        rawInv.Token,
		Name:         "Newbie",
		PasswordHash: "hashed",
		UserID:       "u_new",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UserEmail != "newbie@example.com" {
		t.Errorf("UserEmail: got %q, want newbie@example.com", result.UserEmail)
	}
	if result.InviterID != "u_owner" {
		t.Errorf("InviterID: got %q, want u_owner", result.InviterID)
	}
	if result.InviteRole != string(auth.Editor) {
		t.Errorf("InviteRole: got %q, want editor", result.InviteRole)
	}
	// invite should now be marked accepted (no longer in pending list)
	list, _ := svc.ListInvites(context.Background(), "ws_1")
	if len(list) != 0 {
		t.Errorf("expected invite to be consumed, got %d pending", len(list))
	}
}

func TestWorkspaceService_AcceptInvite_InvalidToken(t *testing.T) {
	t.Parallel()
	svc, wsRepo, _ := newWorkspaceSvcWithUsers(t)
	wsRepo.Seed(repository.Workspace{ID: "ws_1", Name: "Test"})

	_, err := svc.AcceptInvite(context.Background(), service.AcceptInviteParams{
		Token:        "bad-token",
		Name:         "Newbie",
		PasswordHash: "hashed",
		UserID:       "u_new",
	})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for invalid token, got %v", err)
	}
}
