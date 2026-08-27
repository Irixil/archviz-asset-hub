// Package mockservice provides no-op implementations of service interfaces for use in handler tests.
// Each mock has optional override funcs (e.g. GetFn) that tests can set to inject specific behaviour.
package mockservice

import (
	"context"
	"io"

	"damask/server/internal/service"
)

// MockAssetService is a no-op implementation of service.AssetService.
type MockAssetService struct {
	GetFn                           func(ctx context.Context, workspaceID, assetID string) (*service.AssetDTO, error)
	ListFn                          func(ctx context.Context, params service.ListAssetsParams) ([]*service.AssetDTO, error)
	MoveFn                          func(ctx context.Context, workspaceID, assetID string, p service.MoveAssetParams) (*service.AssetDTO, error)
	RenameFn                        func(ctx context.Context, workspaceID, assetID, newStem string) (*service.AssetDTO, error)
	DeleteFn                        func(ctx context.Context, workspaceID, assetID string) error
	HardDeleteFn                    func(ctx context.Context, workspaceID, assetID string) error
	BulkHardDeleteFn                func(ctx context.Context, workspaceID string, assetIDs []string) error
	BulkTagFn                       func(ctx context.Context, workspaceID, tagName string, assetIDs []string) error
	BulkRemoveTagFn                 func(ctx context.Context, workspaceID, tagName string, assetIDs []string) error
	BulkMoveProjectFn               func(ctx context.Context, workspaceID string, assetIDs []string, projectID *string) error
	GetCommentsFn                   func(ctx context.Context, workspaceID, assetID string) ([]service.AssetCommentDTO, error)
	CountVersionsByAssetFn          func(ctx context.Context, assetID string) (int64, error)
	CountVariantsByCurrentVersionFn func(ctx context.Context, assetID string) (int64, error)
	IsRebuildingVariantsFn          func(ctx context.Context, versionID string) (bool, error)
	CountByIDsFn                    func(ctx context.Context, workspaceID string, ids []string) (int64, error)
	RefreshFTSFn                    func(ctx context.Context, assetID string) error
	ListByFieldsFn                  func(ctx context.Context, params service.ListAssetsByFieldsParams) ([]*service.AssetDTO, error)
	BatchVersionCountsFn            func(ctx context.Context, assetIDs []string) (map[string]int64, error)
	BatchVariantCountsFn            func(ctx context.Context, assetIDs []string) (map[string]int64, error)
	RegenerateThumbnailFn           func(ctx context.Context, workspaceID string, assetIDs []string) ([]string, error)
}

func NewAssetService() *MockAssetService { return &MockAssetService{} }

func (m *MockAssetService) Get(ctx context.Context, workspaceID, assetID string) (*service.AssetDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, assetID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockAssetService) List(ctx context.Context, params service.ListAssetsParams) ([]*service.AssetDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, params)
	}
	return nil, nil
}

func (m *MockAssetService) Move(
	ctx context.Context,
	workspaceID, assetID string,
	p service.MoveAssetParams,
) (*service.AssetDTO, error) {
	if m.MoveFn != nil {
		return m.MoveFn(ctx, workspaceID, assetID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockAssetService) Rename(
	ctx context.Context,
	workspaceID, assetID, newStem string,
) (*service.AssetDTO, error) {
	if m.RenameFn != nil {
		return m.RenameFn(ctx, workspaceID, assetID, newStem)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockAssetService) Delete(ctx context.Context, workspaceID, assetID string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, assetID)
	}
	return nil
}

func (m *MockAssetService) HardDelete(ctx context.Context, workspaceID, assetID string) error {
	if m.HardDeleteFn != nil {
		return m.HardDeleteFn(ctx, workspaceID, assetID)
	}
	return nil
}

func (m *MockAssetService) BulkHardDelete(ctx context.Context, workspaceID string, assetIDs []string) error {
	if m.BulkHardDeleteFn != nil {
		return m.BulkHardDeleteFn(ctx, workspaceID, assetIDs)
	}
	return nil
}

func (m *MockAssetService) BulkSetTag(ctx context.Context, workspaceID, tagName string, assetIDs []string) error {
	if m.BulkTagFn != nil {
		return m.BulkTagFn(ctx, workspaceID, tagName, assetIDs)
	}
	return nil
}

func (m *MockAssetService) BulkRemoveTag(ctx context.Context, workspaceID, tagName string, assetIDs []string) error {
	if m.BulkRemoveTagFn != nil {
		return m.BulkRemoveTagFn(ctx, workspaceID, tagName, assetIDs)
	}
	return nil
}

func (m *MockAssetService) BulkMoveProject(
	ctx context.Context,
	workspaceID string,
	assetIDs []string,
	projectID *string,
) error {
	if m.BulkMoveProjectFn != nil {
		return m.BulkMoveProjectFn(ctx, workspaceID, assetIDs, projectID)
	}
	return nil
}

func (m *MockAssetService) GetComments(
	ctx context.Context,
	workspaceID, assetID string,
) ([]service.AssetCommentDTO, error) {
	if m.GetCommentsFn != nil {
		return m.GetCommentsFn(ctx, workspaceID, assetID)
	}
	return nil, nil
}

func (m *MockAssetService) CountVersionsByAsset(ctx context.Context, assetID string) (int64, error) {
	if m.CountVersionsByAssetFn != nil {
		return m.CountVersionsByAssetFn(ctx, assetID)
	}
	return 0, nil
}

func (m *MockAssetService) CountVariantsByCurrentVersion(ctx context.Context, assetID string) (int64, error) {
	if m.CountVariantsByCurrentVersionFn != nil {
		return m.CountVariantsByCurrentVersionFn(ctx, assetID)
	}
	return 0, nil
}

func (m *MockAssetService) IsRebuildingVariants(ctx context.Context, versionID string) (bool, error) {
	if m.IsRebuildingVariantsFn != nil {
		return m.IsRebuildingVariantsFn(ctx, versionID)
	}
	return false, nil
}

func (m *MockAssetService) CountByIDs(ctx context.Context, workspaceID string, ids []string) (int64, error) {
	if m.CountByIDsFn != nil {
		return m.CountByIDsFn(ctx, workspaceID, ids)
	}
	return 0, nil
}

func (m *MockAssetService) RefreshFTS(ctx context.Context, assetID string) error {
	if m.RefreshFTSFn != nil {
		return m.RefreshFTSFn(ctx, assetID)
	}
	return nil
}

func (m *MockAssetService) ListByFields(
	ctx context.Context,
	params service.ListAssetsByFieldsParams,
) ([]*service.AssetDTO, error) {
	if m.ListByFieldsFn != nil {
		return m.ListByFieldsFn(ctx, params)
	}
	return nil, nil
}

func (m *MockAssetService) BatchVersionCounts(ctx context.Context, assetIDs []string) (map[string]int64, error) {
	if m.BatchVersionCountsFn != nil {
		return m.BatchVersionCountsFn(ctx, assetIDs)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockAssetService) BatchVariantCounts(ctx context.Context, assetIDs []string) (map[string]int64, error) {
	if m.BatchVariantCountsFn != nil {
		return m.BatchVariantCountsFn(ctx, assetIDs)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockAssetService) RegenerateThumbnail(
	ctx context.Context,
	workspaceID string,
	assetIDs []string,
) ([]string, error) {
	if m.RegenerateThumbnailFn != nil {
		return m.RegenerateThumbnailFn(ctx, workspaceID, assetIDs)
	}
	return nil, nil
}

func (m *MockAssetService) WriteAssetDownloadedAsync(_, _, _ string) {}

// MockUploadService is a no-op implementation of service.UploadService.
type MockUploadService struct {
	IngestFn func(ctx context.Context, workspaceID string, r io.Reader, meta service.UploadMeta) (*service.AssetDTO, error)
}

func NewUploadService() *MockUploadService { return &MockUploadService{} }

func (m *MockUploadService) Ingest(
	ctx context.Context,
	workspaceID string,
	r io.Reader,
	meta service.UploadMeta,
) (*service.AssetDTO, error) {
	if m.IngestFn != nil {
		return m.IngestFn(ctx, workspaceID, r, meta)
	}
	return nil, nil //nolint:nilnil // mock
}
