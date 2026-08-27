package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockUserService is a no-op implementation of service.UserService.
type MockUserService struct {
	RegisterFn           func(ctx context.Context, p service.RegisterUserParams) (*service.RegisterUserResult, error)
	LoginFn              func(ctx context.Context, p service.LoginUserParams) (*service.LoginUserResult, error)
	GetByIDFn            func(ctx context.Context, userID string) (*service.UserDTO, error)
	CreateWorkspaceFn    func(ctx context.Context, userID, name string) (*service.WorkspaceDTO, error)
	GetProfileFn         func(ctx context.Context, userID string) (*service.OIDCUserDTO, error)
	GetProfileByEmailFn  func(ctx context.Context, email string) (*service.OIDCUserDTO, error)
	UpdateProfileFn      func(ctx context.Context, userID, displayName string) (*service.OIDCUserDTO, error)
	UploadAvatarFn       func(ctx context.Context, userID string, data []byte) (*service.OIDCUserDTO, error)
	DeleteAvatarFn       func(ctx context.Context, userID string) error
	UpdateAvatarKeyFn    func(ctx context.Context, userID, storageKey string) (*service.OIDCUserDTO, error)
	ClearAvatarFn        func(ctx context.Context, userID string) (*service.OIDCUserDTO, error)
	ResetPasswordFn      func(ctx context.Context, userID, passwordHash string) error
	ChangePasswordFn     func(ctx context.Context, userID, currentPassword, newPasswordHash string) error
	RequestEmailChangeFn func(ctx context.Context, userID, email string) error
	CancelEmailChangeFn  func(ctx context.Context, userID string) error
	ConfirmEmailChangeFn func(ctx context.Context, userID, email string) (*service.OIDCUserDTO, error)
	DeleteAccountFn      func(ctx context.Context, userID, password string, hardDelete bool) error
	UpsertOIDCUserFn     func(ctx context.Context, p service.UpsertOIDCUserParams) (*service.OIDCUserDTO, error)
	UpsertCanvaUserFn    func(ctx context.Context, p service.UpsertCanvaUserParams) (*service.OIDCUserDTO, error)
	UnlinkProviderFn     func(ctx context.Context, userID, provider string) (*service.OIDCUserDTO, error)
}

func NewUserService() *MockUserService { return &MockUserService{} }

func (m *MockUserService) Register(
	ctx context.Context,
	p service.RegisterUserParams,
) (*service.RegisterUserResult, error) {
	if m.RegisterFn != nil {
		return m.RegisterFn(ctx, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) Login(ctx context.Context, p service.LoginUserParams) (*service.LoginUserResult, error) {
	if m.LoginFn != nil {
		return m.LoginFn(ctx, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) GetByID(ctx context.Context, userID string) (*service.UserDTO, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, userID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) CreateWorkspace(ctx context.Context, userID, name string) (*service.WorkspaceDTO, error) {
	if m.CreateWorkspaceFn != nil {
		return m.CreateWorkspaceFn(ctx, userID, name)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) GetProfile(ctx context.Context, userID string) (*service.OIDCUserDTO, error) {
	if m.GetProfileFn != nil {
		return m.GetProfileFn(ctx, userID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) GetProfileByEmail(ctx context.Context, email string) (*service.OIDCUserDTO, error) {
	if m.GetProfileByEmailFn != nil {
		return m.GetProfileByEmailFn(ctx, email)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) UpdateProfile(ctx context.Context, userID, displayName string) (*service.OIDCUserDTO, error) {
	if m.UpdateProfileFn != nil {
		return m.UpdateProfileFn(ctx, userID, displayName)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) UploadAvatar(ctx context.Context, userID string, data []byte) (*service.OIDCUserDTO, error) {
	if m.UploadAvatarFn != nil {
		return m.UploadAvatarFn(ctx, userID, data)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) DeleteAvatar(ctx context.Context, userID string) error {
	if m.DeleteAvatarFn != nil {
		return m.DeleteAvatarFn(ctx, userID)
	}
	return nil
}

func (m *MockUserService) UpdateAvatarKey(
	ctx context.Context,
	userID, storageKey string,
) (*service.OIDCUserDTO, error) {
	if m.UpdateAvatarKeyFn != nil {
		return m.UpdateAvatarKeyFn(ctx, userID, storageKey)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) ClearAvatar(ctx context.Context, userID string) (*service.OIDCUserDTO, error) {
	if m.ClearAvatarFn != nil {
		return m.ClearAvatarFn(ctx, userID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) ResetPassword(ctx context.Context, userID, passwordHash string) error {
	if m.ResetPasswordFn != nil {
		return m.ResetPasswordFn(ctx, userID, passwordHash)
	}
	return nil
}

func (m *MockUserService) ChangePassword(ctx context.Context, userID, currentPassword, newPasswordHash string) error {
	if m.ChangePasswordFn != nil {
		return m.ChangePasswordFn(ctx, userID, currentPassword, newPasswordHash)
	}
	return nil
}

func (m *MockUserService) RequestEmailChange(ctx context.Context, userID, email string) error {
	if m.RequestEmailChangeFn != nil {
		return m.RequestEmailChangeFn(ctx, userID, email)
	}
	return nil
}

func (m *MockUserService) CancelEmailChange(ctx context.Context, userID string) error {
	if m.CancelEmailChangeFn != nil {
		return m.CancelEmailChangeFn(ctx, userID)
	}
	return nil
}

func (m *MockUserService) ConfirmEmailChange(ctx context.Context, userID, email string) (*service.OIDCUserDTO, error) {
	if m.ConfirmEmailChangeFn != nil {
		return m.ConfirmEmailChangeFn(ctx, userID, email)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) DeleteAccount(ctx context.Context, userID, password string, hardDelete bool) error {
	if m.DeleteAccountFn != nil {
		return m.DeleteAccountFn(ctx, userID, password, hardDelete)
	}
	return nil
}

func (m *MockUserService) UpsertOIDCUser(
	ctx context.Context,
	p service.UpsertOIDCUserParams,
) (*service.OIDCUserDTO, error) {
	if m.UpsertOIDCUserFn != nil {
		return m.UpsertOIDCUserFn(ctx, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) UpsertCanvaUser(
	ctx context.Context,
	p service.UpsertCanvaUserParams,
) (*service.OIDCUserDTO, error) {
	if m.UpsertCanvaUserFn != nil {
		return m.UpsertCanvaUserFn(ctx, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockUserService) UnlinkProvider(ctx context.Context, userID, provider string) (*service.OIDCUserDTO, error) {
	if m.UnlinkProviderFn != nil {
		return m.UnlinkProviderFn(ctx, userID, provider)
	}
	return nil, nil //nolint:nilnil // mock
}
