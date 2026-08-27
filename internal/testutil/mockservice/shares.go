package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockShareService is a no-op implementation of service.ShareService.
type MockShareService struct {
	ListFn   func(ctx context.Context, workspaceID string) ([]*service.ShareDTO, error)
	GetFn    func(ctx context.Context, workspaceID, id string) (*service.ShareDTO, error)
	CreateFn func(ctx context.Context, workspaceID string, p service.CreateShareParams) (*service.ShareDTO, error)
	UpdateFn func(ctx context.Context, workspaceID, id string, p service.UpdateShareParams) (*service.ShareDTO, error)
	RevokeFn func(ctx context.Context, workspaceID, id string) error
}

func NewShareService() *MockShareService { return &MockShareService{} }

func (m *MockShareService) List(ctx context.Context, workspaceID string) ([]*service.ShareDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockShareService) Get(ctx context.Context, workspaceID, id string) (*service.ShareDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, id)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockShareService) Create(
	ctx context.Context,
	workspaceID string,
	p service.CreateShareParams,
) (*service.ShareDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, workspaceID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockShareService) Update(
	ctx context.Context,
	workspaceID, id string,
	p service.UpdateShareParams,
) (*service.ShareDTO, error) {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, workspaceID, id, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockShareService) Revoke(ctx context.Context, workspaceID, id string) error {
	if m.RevokeFn != nil {
		return m.RevokeFn(ctx, workspaceID, id)
	}
	return nil
}

// MockSharePublicService is a no-op implementation of service.SharePublicService.
type MockSharePublicService struct {
	GetActiveFn                   func(ctx context.Context, shareID string) (*service.ShareDTO, error)
	IncrementViewCountFn          func(ctx context.Context, shareID string) error
	ListAssetsFn                  func(ctx context.Context, targetType, targetID string) ([]*service.PublicAssetDTO, error)
	GetAssetFn                    func(ctx context.Context, assetID string) (*service.PublicAssetDTO, error)
	GetAssetFileFn                func(ctx context.Context, assetID string) (*service.PublicAssetFileDTO, error)
	GetAssetThumbFn               func(ctx context.Context, assetID string) (*service.PublicAssetThumbDTO, error)
	IsAssetInTargetFn             func(ctx context.Context, targetType, targetID, assetID string) (bool, error)
	CreateCommentFn               func(ctx context.Context, p service.CreateShareCommentParams) (*service.ShareCommentDTO, error)
	ListCommentsByShareFn         func(ctx context.Context, shareID string) ([]*service.ShareCommentDTO, error)
	ListCommentsByShareAndAssetFn func(ctx context.Context, shareID, assetID string) ([]*service.ShareCommentDTO, error)
	DeleteCommentFn               func(ctx context.Context, shareID, commentID string) error
	GetOwnerShareFn               func(ctx context.Context, workspaceID, shareID string) (*service.ShareDTO, error)
}

func NewSharePublicService() *MockSharePublicService { return &MockSharePublicService{} }

func (m *MockSharePublicService) GetActive(ctx context.Context, shareID string) (*service.ShareDTO, error) {
	if m.GetActiveFn != nil {
		return m.GetActiveFn(ctx, shareID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockSharePublicService) IncrementViewCount(ctx context.Context, shareID string) error {
	if m.IncrementViewCountFn != nil {
		return m.IncrementViewCountFn(ctx, shareID)
	}
	return nil
}

func (m *MockSharePublicService) ListAssets(
	ctx context.Context,
	targetType, targetID string,
) ([]*service.PublicAssetDTO, error) {
	if m.ListAssetsFn != nil {
		return m.ListAssetsFn(ctx, targetType, targetID)
	}
	return nil, nil
}

func (m *MockSharePublicService) GetAsset(ctx context.Context, assetID string) (*service.PublicAssetDTO, error) {
	if m.GetAssetFn != nil {
		return m.GetAssetFn(ctx, assetID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockSharePublicService) GetAssetFile(
	ctx context.Context,
	assetID string,
) (*service.PublicAssetFileDTO, error) {
	if m.GetAssetFileFn != nil {
		return m.GetAssetFileFn(ctx, assetID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockSharePublicService) GetAssetThumb(
	ctx context.Context,
	assetID string,
) (*service.PublicAssetThumbDTO, error) {
	if m.GetAssetThumbFn != nil {
		return m.GetAssetThumbFn(ctx, assetID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockSharePublicService) IsAssetInTarget(
	ctx context.Context,
	targetType, targetID, assetID string,
) (bool, error) {
	if m.IsAssetInTargetFn != nil {
		return m.IsAssetInTargetFn(ctx, targetType, targetID, assetID)
	}
	return false, nil
}

func (m *MockSharePublicService) CreateComment(
	ctx context.Context,
	p service.CreateShareCommentParams,
) (*service.ShareCommentDTO, error) {
	if m.CreateCommentFn != nil {
		return m.CreateCommentFn(ctx, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockSharePublicService) ListCommentsByShare(
	ctx context.Context,
	shareID string,
) ([]*service.ShareCommentDTO, error) {
	if m.ListCommentsByShareFn != nil {
		return m.ListCommentsByShareFn(ctx, shareID)
	}
	return nil, nil
}

func (m *MockSharePublicService) ListCommentsByShareAndAsset(
	ctx context.Context,
	shareID, assetID string,
) ([]*service.ShareCommentDTO, error) {
	if m.ListCommentsByShareAndAssetFn != nil {
		return m.ListCommentsByShareAndAssetFn(ctx, shareID, assetID)
	}
	return nil, nil
}

func (m *MockSharePublicService) DeleteComment(ctx context.Context, shareID, commentID string) error {
	if m.DeleteCommentFn != nil {
		return m.DeleteCommentFn(ctx, shareID, commentID)
	}
	return nil
}

func (m *MockSharePublicService) GetOwnerShare(
	ctx context.Context,
	workspaceID, shareID string,
) (*service.ShareDTO, error) {
	if m.GetOwnerShareFn != nil {
		return m.GetOwnerShareFn(ctx, workspaceID, shareID)
	}
	return nil, nil //nolint:nilnil // mock
}
