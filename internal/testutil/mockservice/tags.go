package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockTagService is a no-op implementation of service.TagService.
type MockTagService struct {
	ListFn               func(ctx context.Context, workspaceID string, includeSystem bool) ([]*service.TagDTO, error)
	GetByNameFn          func(ctx context.Context, workspaceID, name string) (*service.TagDTO, error)
	CreateFn             func(ctx context.Context, workspaceID string, p service.CreateTagParams) (*service.TagDTO, error)
	PatchFn              func(ctx context.Context, workspaceID, currentName string, p service.PatchTagParams) (*service.TagDTO, error)
	EnsureSystemTagFn    func(ctx context.Context, workspaceID, name string) error
	DeleteFn             func(ctx context.Context, workspaceID string, names []string) error
	BulkDeleteFn         func(ctx context.Context, workspaceID string, names []string) (service.BulkDeleteTagsResult, error)
	MergeFn              func(ctx context.Context, workspaceID string, sources []string, target string) (service.MergeTagsResult, error)
	ResolveSystemTagFn   func(ctx context.Context, workspaceID, tagName string, scope service.SystemTagScope) (*service.AssetDTO, error)
	TouchLastUsedFn      func(ctx context.Context, workspaceID, name string) error
	ListForAssetFn       func(ctx context.Context, assetID string) ([]*service.TagDTO, error)
	AddToAssetFn         func(ctx context.Context, workspaceID, assetID, tagName string) (*service.TagDTO, error)
	RemoveFromAssetFn    func(ctx context.Context, workspaceID, assetID, tagName string) error
	UpsertForAssetFn     func(ctx context.Context, workspaceID, assetID, tagName string) error
	BatchTagsForAssetsFn func(ctx context.Context, assetIDs []string) (map[string][]string, error)
	ApplyTagFn           func(ctx context.Context, workspaceID, assetID, tagName string) error
}

func NewTagService() *MockTagService { return &MockTagService{} }

func (m *MockTagService) List(ctx context.Context, workspaceID string, includeSystem bool) ([]*service.TagDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID, includeSystem)
	}
	return nil, nil
}

func (m *MockTagService) GetByName(ctx context.Context, workspaceID, name string) (*service.TagDTO, error) {
	if m.GetByNameFn != nil {
		return m.GetByNameFn(ctx, workspaceID, name)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockTagService) Create(
	ctx context.Context,
	workspaceID string,
	p service.CreateTagParams,
) (*service.TagDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, workspaceID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockTagService) Patch(
	ctx context.Context,
	workspaceID, currentName string,
	p service.PatchTagParams,
) (*service.TagDTO, error) {
	if m.PatchFn != nil {
		return m.PatchFn(ctx, workspaceID, currentName, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockTagService) EnsureSystemTag(ctx context.Context, workspaceID, name string) error {
	if m.EnsureSystemTagFn != nil {
		return m.EnsureSystemTagFn(ctx, workspaceID, name)
	}
	return nil
}

func (m *MockTagService) Delete(ctx context.Context, workspaceID string, names []string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, names)
	}
	return nil
}

func (m *MockTagService) BulkDelete(
	ctx context.Context,
	workspaceID string,
	names []string,
) (service.BulkDeleteTagsResult, error) {
	if m.BulkDeleteFn != nil {
		return m.BulkDeleteFn(ctx, workspaceID, names)
	}
	return service.BulkDeleteTagsResult{}, nil
}

func (m *MockTagService) Merge(
	ctx context.Context,
	workspaceID string,
	sources []string,
	target string,
) (service.MergeTagsResult, error) {
	if m.MergeFn != nil {
		return m.MergeFn(ctx, workspaceID, sources, target)
	}
	return service.MergeTagsResult{}, nil
}

func (m *MockTagService) ResolveSystemTag(
	ctx context.Context,
	workspaceID, tagName string,
	scope service.SystemTagScope,
) (*service.AssetDTO, error) {
	if m.ResolveSystemTagFn != nil {
		return m.ResolveSystemTagFn(ctx, workspaceID, tagName, scope)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockTagService) TouchLastUsed(ctx context.Context, workspaceID, name string) error {
	if m.TouchLastUsedFn != nil {
		return m.TouchLastUsedFn(ctx, workspaceID, name)
	}
	return nil
}

func (m *MockTagService) ListForAsset(ctx context.Context, assetID string) ([]*service.TagDTO, error) {
	if m.ListForAssetFn != nil {
		return m.ListForAssetFn(ctx, assetID)
	}
	return nil, nil
}

func (m *MockTagService) AddToAsset(
	ctx context.Context,
	workspaceID, assetID, tagName string,
) (*service.TagDTO, error) {
	if m.AddToAssetFn != nil {
		return m.AddToAssetFn(ctx, workspaceID, assetID, tagName)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockTagService) RemoveFromAsset(ctx context.Context, workspaceID, assetID, tagName string) error {
	if m.RemoveFromAssetFn != nil {
		return m.RemoveFromAssetFn(ctx, workspaceID, assetID, tagName)
	}
	return nil
}

func (m *MockTagService) UpsertForAsset(ctx context.Context, workspaceID, assetID, tagName string) error {
	if m.UpsertForAssetFn != nil {
		return m.UpsertForAssetFn(ctx, workspaceID, assetID, tagName)
	}
	return nil
}

func (m *MockTagService) BatchTagsForAssets(ctx context.Context, assetIDs []string) (map[string][]string, error) {
	if m.BatchTagsForAssetsFn != nil {
		return m.BatchTagsForAssetsFn(ctx, assetIDs)
	}
	return map[string][]string{}, nil
}

func (m *MockTagService) ApplyTag(ctx context.Context, workspaceID, assetID, tagName string) error {
	if m.ApplyTagFn != nil {
		return m.ApplyTagFn(ctx, workspaceID, assetID, tagName)
	}
	return nil
}
