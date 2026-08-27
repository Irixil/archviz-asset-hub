package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/storage"

	"golang.org/x/crypto/bcrypt"
)

func newUserSvc(t *testing.T) (service.UserService, *memory.RealUserRepo, *memory.RealWorkspaceRepo, storage.Storage) {
	t.Helper()
	users := memory.NewRealUserRepo()
	workspaces := memory.NewRealWorkspaceRepo()
	workspaces.SetUserRepo(users)
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewUserService(users, workspaces, stor)
	return svc, users, workspaces, stor
}

func hashPassword(t *testing.T, plain string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

// --- Register ---

func TestUserService_Register_OK(t *testing.T) {
	svc, _, workspaces, _ := newUserSvc(t)

	result, err := svc.Register(context.Background(), service.RegisterUserParams{
		UserID:        "u_1",
		Name:          "Alice",
		Email:         "alice@example.com",
		PasswordHash:  hashPassword(t, "secret"),
		WorkspaceName: "Alice's Workspace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", result.User.Email)
	}
	if result.WorkspaceID == "" {
		t.Error("expected non-empty WorkspaceID")
	}

	// workspace should exist
	ws, err := workspaces.GetByID(context.Background(), result.WorkspaceID)
	if err != nil {
		t.Fatalf("workspace not found after register: %v", err)
	}
	if ws.Name != "Alice's Workspace" {
		t.Errorf("workspace name = %q, want %q", ws.Name, "Alice's Workspace")
	}
}

func TestUserService_Register_DuplicateEmail(t *testing.T) {
	svc, _, _, _ := newUserSvc(t)

	params := service.RegisterUserParams{
		UserID:        "u_1",
		Name:          "Alice",
		Email:         "alice@example.com",
		PasswordHash:  hashPassword(t, "secret"),
		WorkspaceName: "Alice's Workspace",
	}
	if _, err := svc.Register(context.Background(), params); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	params.UserID = "u_2"
	_, err := svc.Register(context.Background(), params)
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate email, got %v", err)
	}
}

// --- Login ---

func TestUserService_Login_OK(t *testing.T) {
	svc, _, _, _ := newUserSvc(t)

	plain := "s3cret!"
	_, err := svc.Register(context.Background(), service.RegisterUserParams{
		UserID:        "u_1",
		Name:          "Bob",
		Email:         "bob@example.com",
		PasswordHash:  hashPassword(t, plain),
		WorkspaceName: "Bob's Workspace",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	result, err := svc.Login(context.Background(), service.LoginUserParams{
		Email:         "bob@example.com",
		PlainPassword: plain,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if result.User.Email != "bob@example.com" {
		t.Errorf("email = %q", result.User.Email)
	}
	if result.WorkspaceID == "" {
		t.Error("expected non-empty WorkspaceID")
	}
}

func TestUserService_Login_WrongPassword(t *testing.T) {
	svc, _, _, _ := newUserSvc(t)

	_, err := svc.Register(context.Background(), service.RegisterUserParams{
		UserID:        "u_1",
		Name:          "Carol",
		Email:         "carol@example.com",
		PasswordHash:  hashPassword(t, "correct"),
		WorkspaceName: "Carol's Workspace",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err = svc.Login(context.Background(), service.LoginUserParams{
		Email:         "carol@example.com",
		PlainPassword: "wrong",
	})
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for wrong password, got %v", err)
	}
}

func TestUserService_Login_UnknownEmail(t *testing.T) {
	svc, _, _, _ := newUserSvc(t)

	_, err := svc.Login(context.Background(), service.LoginUserParams{
		Email:         "nobody@example.com",
		PlainPassword: "x",
	})
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for unknown email, got %v", err)
	}
}

// --- GetByID ---

func TestUserService_GetByID_OK(t *testing.T) {
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "x@x.com", Name: "X"})

	dto, err := svc.GetByID(context.Background(), "u_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Email != "x@x.com" {
		t.Errorf("email = %q", dto.Email)
	}
}

func TestUserService_GetByID_NotFound(t *testing.T) {
	svc, _, _, _ := newUserSvc(t)
	_, err := svc.GetByID(context.Background(), "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- GetProfile ---

func TestUserService_GetProfile_OK(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "alice@example.com", Name: "Alice"})

	dto, err := svc.GetProfile(context.Background(), "u_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Email != "alice@example.com" {
		t.Errorf("Email: got %q, want alice@example.com", dto.Email)
	}
}

func TestUserService_GetProfile_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newUserSvc(t)
	_, err := svc.GetProfile(context.Background(), "u_nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- GetProfileByEmail ---

func TestUserService_GetProfileByEmail_OK(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "bob@example.com", Name: "Bob"})

	dto, err := svc.GetProfileByEmail(context.Background(), "bob@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.ID != "u_1" {
		t.Errorf("ID: got %q, want u_1", dto.ID)
	}
}

func TestUserService_GetProfileByEmail_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newUserSvc(t)
	_, err := svc.GetProfileByEmail(context.Background(), "nobody@example.com")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- UpdateProfile ---

func TestUserService_UpdateProfile_OK(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "carol@example.com", Name: "Carol"})

	dto, err := svc.UpdateProfile(context.Background(), "u_1", "Caz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.DisplayName != "Caz" {
		t.Errorf("DisplayName: got %q, want Caz", dto.DisplayName)
	}
}

// --- ResetPassword ---

func TestUserService_ResetPassword_AddsPasswordMethod(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	// User with no auth methods
	users.Seed(repository.User{ID: "u_1", Email: "x@x.com", Name: "X", AuthMethods: "[]"})

	err := svc.ResetPassword(context.Background(), "u_1", hashPassword(t, "newpass"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, _ := users.GetByID(context.Background(), "u_1")
	if u.AuthMethods != `["password"]` {
		t.Errorf("AuthMethods: got %q, want [\"password\"]", u.AuthMethods)
	}
}

func TestUserService_ResetPassword_AlreadyHasPasswordMethod(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "x@x.com", Name: "X", AuthMethods: `["password"]`})

	err := svc.ResetPassword(context.Background(), "u_1", hashPassword(t, "newpass"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ChangePassword ---

func TestUserService_ChangePassword_OK(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	plain := "oldpass"
	users.Seed(repository.User{
		ID:           "u_1",
		Email:        "x@x.com",
		Name:         "X",
		AuthMethods:  `["password"]`,
		PasswordHash: hashPassword(t, plain),
	})

	err := svc.ChangePassword(context.Background(), "u_1", plain, hashPassword(t, "newpass"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserService_ChangePassword_WrongCurrentPassword(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{
		ID:           "u_1",
		Email:        "x@x.com",
		Name:         "X",
		AuthMethods:  `["password"]`,
		PasswordHash: hashPassword(t, "correct"),
	})

	err := svc.ChangePassword(context.Background(), "u_1", "wrong", hashPassword(t, "new"))
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for wrong password, got %v", err)
	}
}

// --- RequestEmailChange ---

func TestUserService_RequestEmailChange_OK(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "old@example.com", Name: "X"})

	err := svc.RequestEmailChange(context.Background(), "u_1", "new@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, _ := users.GetByID(context.Background(), "u_1")
	if u.PendingEmail == nil || *u.PendingEmail != "new@example.com" {
		t.Error("expected PendingEmail to be set")
	}
}

func TestUserService_RequestEmailChange_SameEmail(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "same@example.com", Name: "X"})

	err := svc.RequestEmailChange(context.Background(), "u_1", "same@example.com")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for same email, got %v", err)
	}
}

func TestUserService_RequestEmailChange_EmailTaken(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	users.Seed(
		repository.User{ID: "u_1", Email: "alice@example.com", Name: "Alice"},
		repository.User{ID: "u_2", Email: "bob@example.com", Name: "Bob"},
	)

	err := svc.RequestEmailChange(context.Background(), "u_1", "bob@example.com")
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict for taken email, got %v", err)
	}
}

// --- CancelEmailChange ---

func TestUserService_CancelEmailChange_OK(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	pending := "new@example.com"
	users.Seed(repository.User{ID: "u_1", Email: "old@example.com", Name: "X", PendingEmail: &pending})

	err := svc.CancelEmailChange(context.Background(), "u_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, _ := users.GetByID(context.Background(), "u_1")
	if u.PendingEmail != nil {
		t.Errorf("expected PendingEmail to be nil after cancel, got %q", *u.PendingEmail)
	}
}

// --- ConfirmEmailChange ---

func TestUserService_ConfirmEmailChange_OK(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	pending := "new@example.com"
	users.Seed(repository.User{ID: "u_1", Email: "old@example.com", Name: "X", PendingEmail: &pending})

	dto, err := svc.ConfirmEmailChange(context.Background(), "u_1", "new@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Email != "new@example.com" {
		t.Errorf("Email: got %q, want new@example.com", dto.Email)
	}
}

func TestUserService_ConfirmEmailChange_StalePendingEmail(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newUserSvc(t)
	pending := "actual@example.com"
	users.Seed(repository.User{ID: "u_1", Email: "old@example.com", Name: "X", PendingEmail: &pending})

	_, err := svc.ConfirmEmailChange(context.Background(), "u_1", "wrong@example.com")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for stale token, got %v", err)
	}
}

// --- CreateWorkspace ---

// --- UpdateAvatarKey / ClearAvatar ---

func seedUserForAvatar(t *testing.T, users *memory.RealUserRepo, wsRepo *memory.RealWorkspaceRepo) string {
	t.Helper()
	uid := "u_avatar_1"
	_, err := users.Create(context.Background(), repository.User{
		ID:           uid,
		Email:        "avatar@example.com",
		PasswordHash: "hash",
		AuthMethods:  `["password"]`,
		Name:         "Avatar User",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	ws, wsErr := wsRepo.Create(context.Background(), repository.Workspace{ID: "ws_avatar_1", Name: "WS"})
	if wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	if err = wsRepo.CreateMember(context.Background(), repository.Member{
		WorkspaceID: ws.ID, UserID: uid, Role: "owner",
	}); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	return uid
}

func TestUserService_UpdateAvatarKey(t *testing.T) {
	t.Parallel()
	svc, users, wsRepo, _ := newUserSvc(t)
	uid := seedUserForAvatar(t, users, wsRepo)

	dto, err := svc.UpdateAvatarKey(context.Background(), uid, "storage/avatar.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.AvatarStorageKey == nil || *dto.AvatarStorageKey != "storage/avatar.png" {
		t.Errorf("AvatarStorageKey: got %v, want %q", dto.AvatarStorageKey, "storage/avatar.png")
	}
}

func TestUserService_ClearAvatar(t *testing.T) {
	t.Parallel()
	svc, users, wsRepo, _ := newUserSvc(t)
	uid := seedUserForAvatar(t, users, wsRepo)

	if _, err := svc.UpdateAvatarKey(context.Background(), uid, "storage/avatar.png"); err != nil {
		t.Fatalf("setup UpdateAvatarKey: %v", err)
	}

	dto, err := svc.ClearAvatar(context.Background(), uid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.AvatarStorageKey != nil {
		t.Errorf("AvatarStorageKey should be nil after ClearAvatar, got %v", dto.AvatarStorageKey)
	}
}

// --- DeleteAccount ---

func seedUserForDelete(
	t *testing.T,
	users *memory.RealUserRepo,
	wsRepo *memory.RealWorkspaceRepo,
	userID, wsID string,
	role string,
) {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	_, err := users.Create(context.Background(), repository.User{
		ID:           userID,
		Email:        userID + "@example.com",
		PasswordHash: string(hash),
		AuthMethods:  `["password"]`,
		Name:         userID,
	})
	if err != nil {
		t.Fatalf("seed user %s: %v", userID, err)
	}
	if _, wsErr := wsRepo.Create(context.Background(), repository.Workspace{ID: wsID, Name: wsID}); wsErr != nil {
		// workspace may already exist — ignore conflict
		_ = wsErr
	}
	if err = wsRepo.CreateMember(context.Background(), repository.Member{
		WorkspaceID: wsID, UserID: userID, Role: role,
	}); err != nil {
		t.Fatalf("seed member %s/%s: %v", wsID, userID, err)
	}
}

func TestUserService_DeleteAccount_SoleOwner(t *testing.T) {
	t.Parallel()
	svc, users, wsRepo, _ := newUserSvc(t)
	seedUserForDelete(t, users, wsRepo, "u_sole", "ws_sole", "owner")

	err := svc.DeleteAccount(context.Background(), "u_sole", "secret", false)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for sole owner, got %v", err)
	}
}

func TestUserService_DeleteAccount_WithSecondOwner(t *testing.T) {
	t.Parallel()
	svc, users, wsRepo, _ := newUserSvc(t)
	seedUserForDelete(t, users, wsRepo, "u_del", "ws_multi", "owner")
	seedUserForDelete(t, users, wsRepo, "u_other", "ws_multi", "owner")

	err := svc.DeleteAccount(context.Background(), "u_del", "secret", false)
	if err != nil {
		t.Errorf("unexpected error when second owner exists: %v", err)
	}
}

func TestUserService_CreateWorkspace_OK(t *testing.T) {
	t.Parallel()
	svc, _, wsRepo, _ := newUserSvc(t)

	dto, err := svc.CreateWorkspace(context.Background(), "u_1", "My New Workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "My New Workspace" {
		t.Errorf("Name: got %q, want My New Workspace", dto.Name)
	}
	// member should exist in the repo
	m, err := wsRepo.GetMember(context.Background(), dto.ID, "u_1")
	if err != nil {
		t.Fatalf("expected member to be created: %v", err)
	}
	if m.Role != "owner" {
		t.Errorf("Role: got %q, want owner", m.Role)
	}
}
