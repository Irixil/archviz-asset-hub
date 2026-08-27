package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockCollectionService is a no-op implementation of service.CollectionService.
type MockCollectionService struct {
	ListFn         func(ctx context.Context, workspaceID string) ([]*service.CollectionDTO, error)
	GetFn          func(ctx context.Context, workspaceID, id string) (*service.CollectionDTO, error)
	CreateFn       func(ctx context.Context, workspaceID string, p service.CreateCollectionParams) (*service.CollectionDTO, error)
	UpdateFn       func(ctx context.Context, workspaceID, id string, p service.UpdateCollectionParams) (*service.CollectionDTO, error)
	DeleteFn       func(ctx context.Context, workspaceID, id string) error
	AddAssetFn     func(ctx context.Context, workspaceID, collectionID, assetID string) error
	RemoveAssetFn  func(ctx context.Context, workspaceID, collectionID, assetID string) error
	ListForAssetFn func(ctx context.Context, workspaceID, assetID string) ([]*service.CollectionDTO, error)
	ListAssetsFn   func(ctx context.Context, workspaceID, collectionID string) ([]*service.AssetDTO, error)
}

func NewCollectionService() *MockCollectionService { return &MockCollectionService{} }

func (m *MockCollectionService) List(ctx context.Context, workspaceID string) ([]*service.CollectionDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockCollectionService) Get(ctx context.Context, workspaceID, id string) (*service.CollectionDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, id)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockCollectionService) Create(
	ctx context.Context,
	workspaceID string,
	p service.CreateCollectionParams,
) (*service.CollectionDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, workspaceID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockCollectionService) Update(
	ctx context.Context,
	workspaceID, id string,
	p service.UpdateCollectionParams,
) (*service.CollectionDTO, error) {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, workspaceID, id, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockCollectionService) Delete(ctx context.Context, workspaceID, id string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, id)
	}
	return nil
}

func (m *MockCollectionService) AddAsset(ctx context.Context, workspaceID, collectionID, assetID string) error {
	if m.AddAssetFn != nil {
		return m.AddAssetFn(ctx, workspaceID, collectionID, assetID)
	}
	return nil
}

func (m *MockCollectionService) RemoveAsset(ctx context.Context, workspaceID, collectionID, assetID string) error {
	if m.RemoveAssetFn != nil {
		return m.RemoveAssetFn(ctx, workspaceID, collectionID, assetID)
	}
	return nil
}

func (m *MockCollectionService) ListForAsset(
	ctx context.Context,
	workspaceID, assetID string,
) ([]*service.CollectionDTO, error) {
	if m.ListForAssetFn != nil {
		return m.ListForAssetFn(ctx, workspaceID, assetID)
	}
	return nil, nil
}

func (m *MockCollectionService) ListAssets(
	ctx context.Context,
	workspaceID, collectionID string,
) ([]*service.AssetDTO, error) {
	if m.ListAssetsFn != nil {
		return m.ListAssetsFn(ctx, workspaceID, collectionID)
	}
	return nil, nil
}
