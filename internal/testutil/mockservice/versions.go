package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockVersionService is a no-op implementation of service.VersionService.
type MockVersionService struct {
	ListFn                 func(ctx context.Context, assetID string) ([]*service.VersionDTO, error)
	ListWithVariantCountFn func(ctx context.Context, assetID string) ([]*service.VersionWithCountDTO, error)
	GetFn                  func(ctx context.Context, workspaceID, id string) (*service.VersionDTO, error)
	GetCurrentByAssetFn    func(ctx context.Context, assetID string) (*service.VersionDTO, error)
	GetFirstByAssetFn      func(ctx context.Context, assetID string) (*service.VersionDTO, error)
	GetByHashFn            func(ctx context.Context, assetID, contentHash string) (*service.VersionDTO, error)
	NextVersionNumFn       func(ctx context.Context, assetID string) (int64, error)
	CreateFn               func(ctx context.Context, v *service.VersionDTO) (*service.VersionDTO, error)
	UploadNewVersionFn     func(ctx context.Context, p service.UploadAssetVersionParams) (*service.UploadAssetVersionResult, error)
	SetCurrentFn           func(ctx context.Context, assetID, versionID string) error
	SetAssetThumbnailFn    func(ctx context.Context, assetID string, key *string) error
	DeleteFn               func(ctx context.Context, workspaceID, assetID, versionID string) error
}

func NewVersionService() *MockVersionService { return &MockVersionService{} }

func (m *MockVersionService) List(ctx context.Context, assetID string) ([]*service.VersionDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, assetID)
	}
	return nil, nil
}

func (m *MockVersionService) ListWithVariantCount(
	ctx context.Context,
	assetID string,
) ([]*service.VersionWithCountDTO, error) {
	if m.ListWithVariantCountFn != nil {
		return m.ListWithVariantCountFn(ctx, assetID)
	}
	return nil, nil
}

func (m *MockVersionService) Get(ctx context.Context, workspaceID, id string) (*service.VersionDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, id)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockVersionService) GetCurrentByAsset(ctx context.Context, assetID string) (*service.VersionDTO, error) {
	if m.GetCurrentByAssetFn != nil {
		return m.GetCurrentByAssetFn(ctx, assetID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockVersionService) GetFirstByAsset(ctx context.Context, assetID string) (*service.VersionDTO, error) {
	if m.GetFirstByAssetFn != nil {
		return m.GetFirstByAssetFn(ctx, assetID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockVersionService) GetByHash(ctx context.Context, assetID, contentHash string) (*service.VersionDTO, error) {
	if m.GetByHashFn != nil {
		return m.GetByHashFn(ctx, assetID, contentHash)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockVersionService) NextVersionNum(ctx context.Context, assetID string) (int64, error) {
	if m.NextVersionNumFn != nil {
		return m.NextVersionNumFn(ctx, assetID)
	}
	return 0, nil
}

func (m *MockVersionService) Create(ctx context.Context, v *service.VersionDTO) (*service.VersionDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, v)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockVersionService) UploadNewVersion(
	ctx context.Context,
	p service.UploadAssetVersionParams,
) (*service.UploadAssetVersionResult, error) {
	if m.UploadNewVersionFn != nil {
		return m.UploadNewVersionFn(ctx, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockVersionService) SetCurrent(ctx context.Context, assetID, versionID string) error {
	if m.SetCurrentFn != nil {
		return m.SetCurrentFn(ctx, assetID, versionID)
	}
	return nil
}

func (m *MockVersionService) SetAssetThumbnail(ctx context.Context, assetID string, key *string) error {
	if m.SetAssetThumbnailFn != nil {
		return m.SetAssetThumbnailFn(ctx, assetID, key)
	}
	return nil
}

func (m *MockVersionService) Delete(ctx context.Context, workspaceID, assetID, versionID string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, assetID, versionID)
	}
	return nil
}

func (m *MockVersionService) WriteVersionUploaded(_ context.Context, _, _ string, _ *service.VersionDTO, _ string) {
}

func (m *MockVersionService) WriteVersionRestored(_ context.Context, _, _ string, _, _ int64) {
}

// MockVariantService is a no-op implementation of service.VariantService.
type MockVariantService struct {
	ListFn           func(ctx context.Context, p service.ListVariantsParams) (*service.ListVariantsResult, error)
	GetFn            func(ctx context.Context, workspaceID, id string) (*service.VariantDTO, error)
	PrepareCreateFn  func(ctx context.Context, p service.PrepareCreateVariantParams) (service.PreparedCreateVariant, error)
	CreateFn         func(ctx context.Context, p service.CreateVariantParams) (*service.VariantDTO, error)
	CommitDraftFn    func(ctx context.Context, p service.CommitDraftParams) (*service.VariantDTO, error)
	UpdateTitleFn    func(ctx context.Context, workspaceID, variantID, title string) error
	UpdateSharingFn  func(ctx context.Context, p service.UpdateVariantsSharingParams) error
	ListSharedFn     func(ctx context.Context, assetIDs []string) ([]service.SharedVariantDTO, error)
	GetSharedForFn   func(ctx context.Context, variantID, assetID string) (*service.VariantDTO, error)
	DeleteFn         func(ctx context.Context, workspaceID, assetID, variantID string) error
	PromoteFn        func(ctx context.Context, p service.PromoteVariantParams) (service.PromoteVariantResult, error)
	SetAsThumbnailFn func(ctx context.Context, workspaceID, assetID, variantID string) error
	RerunFn          func(ctx context.Context, p service.RerunVariantParams) error
	GetParamHistFn   func(ctx context.Context, workspaceID, variantType string) ([]service.ParamHistoryEntry, error)
}

func NewVariantService() *MockVariantService { return &MockVariantService{} }

func (m *MockVariantService) List(
	ctx context.Context,
	p service.ListVariantsParams,
) (*service.ListVariantsResult, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, p)
	}
	return &service.ListVariantsResult{}, nil
}

func (m *MockVariantService) Get(ctx context.Context, workspaceID, id string) (*service.VariantDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, id)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockVariantService) PrepareCreate(
	ctx context.Context,
	p service.PrepareCreateVariantParams,
) (service.PreparedCreateVariant, error) {
	if m.PrepareCreateFn != nil {
		return m.PrepareCreateFn(ctx, p)
	}
	return service.PreparedCreateVariant{Type: p.Type, Params: p.Params}, nil
}

func (m *MockVariantService) Create(ctx context.Context, p service.CreateVariantParams) (*service.VariantDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockVariantService) CommitDraft(
	ctx context.Context,
	p service.CommitDraftParams,
) (*service.VariantDTO, error) {
	if m.CommitDraftFn != nil {
		return m.CommitDraftFn(ctx, p)
	}
	return &service.VariantDTO{ID: "draft-committed", Type: p.VariantType, Status: "ready"}, nil
}

func (m *MockVariantService) UpdateTitle(ctx context.Context, workspaceID, variantID, title string) error {
	if m.UpdateTitleFn != nil {
		return m.UpdateTitleFn(ctx, workspaceID, variantID, title)
	}
	return nil
}

func (m *MockVariantService) UpdateSharing(ctx context.Context, p service.UpdateVariantsSharingParams) error {
	if m.UpdateSharingFn != nil {
		return m.UpdateSharingFn(ctx, p)
	}
	return nil
}

func (m *MockVariantService) ListSharedByAssets(
	ctx context.Context,
	assetIDs []string,
) ([]service.SharedVariantDTO, error) {
	if m.ListSharedFn != nil {
		return m.ListSharedFn(ctx, assetIDs)
	}
	return nil, nil
}

func (m *MockVariantService) GetSharedForShare(
	ctx context.Context,
	variantID, assetID string,
) (*service.VariantDTO, error) {
	if m.GetSharedForFn != nil {
		return m.GetSharedForFn(ctx, variantID, assetID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockVariantService) Delete(ctx context.Context, workspaceID, assetID, variantID string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, assetID, variantID)
	}
	return nil
}

func (m *MockVariantService) Promote(
	ctx context.Context,
	p service.PromoteVariantParams,
) (service.PromoteVariantResult, error) {
	if m.PromoteFn != nil {
		return m.PromoteFn(ctx, p)
	}
	return service.PromoteVariantResult{}, nil
}

func (m *MockVariantService) SetAsThumbnail(ctx context.Context, workspaceID, assetID, variantID string) error {
	if m.SetAsThumbnailFn != nil {
		return m.SetAsThumbnailFn(ctx, workspaceID, assetID, variantID)
	}
	return nil
}

func (m *MockVariantService) Rerun(ctx context.Context, p service.RerunVariantParams) error {
	if m.RerunFn != nil {
		return m.RerunFn(ctx, p)
	}
	return nil
}

func (m *MockVariantService) GetParamHistory(
	ctx context.Context,
	workspaceID, variantType string,
) ([]service.ParamHistoryEntry, error) {
	if m.GetParamHistFn != nil {
		return m.GetParamHistFn(ctx, workspaceID, variantType)
	}
	return nil, nil
}

func (m *MockVariantService) WriteVariantQueued(_ context.Context, _, _, _ string) {}

func (m *MockVariantService) WriteVariantDownloadedAsync(_, _, _, _, _, _ string) {}
