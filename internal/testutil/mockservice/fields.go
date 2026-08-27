package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockFieldService is a no-op implementation of service.FieldService.
type MockFieldService struct {
	ListFn                  func(ctx context.Context, workspaceID, scope string) ([]*service.FieldDefinitionDTO, error)
	GetFn                   func(ctx context.Context, workspaceID, id string) (*service.FieldDefinitionDTO, error)
	CreateFn                func(ctx context.Context, workspaceID string, p service.CreateFieldDefinitionParams) (*service.FieldDefinitionDTO, error)
	UpdateFn                func(ctx context.Context, workspaceID, id string, p service.UpdateFieldDefinitionParams) (*service.FieldDefinitionDTO, error)
	DeleteFn                func(ctx context.Context, workspaceID, id string) error
	GetStatsFn              func(ctx context.Context, workspaceID, id string) (service.FieldStatsDTO, error)
	ReorderFn               func(ctx context.Context, workspaceID string, items []service.ReorderFieldItem) error
	InheritProjectFieldsFn  func(ctx context.Context, workspaceID, assetID, projectID, userID string) error
	ListAssetsMissingExifFn func(ctx context.Context, workspaceID string) ([]string, error)
}

func NewFieldService() *MockFieldService { return &MockFieldService{} }

func (m *MockFieldService) List(ctx context.Context, workspaceID, scope string) ([]*service.FieldDefinitionDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID, scope)
	}
	return nil, nil
}

func (m *MockFieldService) Get(ctx context.Context, workspaceID, id string) (*service.FieldDefinitionDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, id)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockFieldService) Create(
	ctx context.Context,
	workspaceID string,
	p service.CreateFieldDefinitionParams,
) (*service.FieldDefinitionDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, workspaceID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockFieldService) Update(
	ctx context.Context,
	workspaceID, id string,
	p service.UpdateFieldDefinitionParams,
) (*service.FieldDefinitionDTO, error) {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, workspaceID, id, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockFieldService) Delete(ctx context.Context, workspaceID, id string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, id)
	}
	return nil
}

func (m *MockFieldService) GetStats(ctx context.Context, workspaceID, id string) (service.FieldStatsDTO, error) {
	if m.GetStatsFn != nil {
		return m.GetStatsFn(ctx, workspaceID, id)
	}
	return service.FieldStatsDTO{}, nil
}

func (m *MockFieldService) Reorder(ctx context.Context, workspaceID string, items []service.ReorderFieldItem) error {
	if m.ReorderFn != nil {
		return m.ReorderFn(ctx, workspaceID, items)
	}
	return nil
}

func (m *MockFieldService) InheritProjectFields(
	ctx context.Context,
	workspaceID, assetID, projectID, userID string,
) error {
	if m.InheritProjectFieldsFn != nil {
		return m.InheritProjectFieldsFn(ctx, workspaceID, assetID, projectID, userID)
	}
	return nil
}

func (m *MockFieldService) ListAssetsMissingExif(ctx context.Context, workspaceID string) ([]string, error) {
	if m.ListAssetsMissingExifFn != nil {
		return m.ListAssetsMissingExifFn(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockFieldService) PurgeExpiredFields(_ context.Context) (int, error) { return 0, nil }

// MockAssetFieldService is a no-op implementation of service.AssetFieldService.
type MockAssetFieldService struct {
	GetValuesFn     func(ctx context.Context, workspaceID, assetID string) ([]*service.FieldValueDTO, error)
	SetValuesFn     func(ctx context.Context, workspaceID, assetID, userID string, inputs []service.SetFieldValueInput) ([]*service.FieldValueDTO, error)
	BulkSetValuesFn func(ctx context.Context, workspaceID, userID string, assetIDs []string, inputs []service.SetFieldValueInput) (service.BulkSetValuesResult, error)
	BulkPreviewFn   func(ctx context.Context, workspaceID string, assetIDs, fieldIDs []string) ([]service.BulkPreviewEntry, error)
}

func NewAssetFieldService() *MockAssetFieldService { return &MockAssetFieldService{} }

func (m *MockAssetFieldService) GetValues(
	ctx context.Context,
	workspaceID, assetID string,
) ([]*service.FieldValueDTO, error) {
	if m.GetValuesFn != nil {
		return m.GetValuesFn(ctx, workspaceID, assetID)
	}
	return nil, nil
}

func (m *MockAssetFieldService) SetValues(
	ctx context.Context,
	workspaceID, assetID, userID string,
	inputs []service.SetFieldValueInput,
) ([]*service.FieldValueDTO, error) {
	if m.SetValuesFn != nil {
		return m.SetValuesFn(ctx, workspaceID, assetID, userID, inputs)
	}
	return nil, nil
}

func (m *MockAssetFieldService) BulkSetValues(
	ctx context.Context,
	workspaceID, userID string,
	assetIDs []string,
	inputs []service.SetFieldValueInput,
) (service.BulkSetValuesResult, error) {
	if m.BulkSetValuesFn != nil {
		return m.BulkSetValuesFn(ctx, workspaceID, userID, assetIDs, inputs)
	}
	return service.BulkSetValuesResult{}, nil
}

func (m *MockAssetFieldService) BulkPreview(
	ctx context.Context,
	workspaceID string,
	assetIDs, fieldIDs []string,
) ([]service.BulkPreviewEntry, error) {
	if m.BulkPreviewFn != nil {
		return m.BulkPreviewFn(ctx, workspaceID, assetIDs, fieldIDs)
	}
	return nil, nil
}

// MockProjectFieldService is a no-op implementation of service.ProjectFieldService.
type MockProjectFieldService struct {
	GetValuesFn func(ctx context.Context, workspaceID, projectID string) ([]*service.FieldValueDTO, error)
	SetValuesFn func(ctx context.Context, workspaceID, projectID, userID string, inputs []service.SetFieldValueInput) ([]*service.FieldValueDTO, error)
}

func NewProjectFieldService() *MockProjectFieldService { return &MockProjectFieldService{} }

func (m *MockProjectFieldService) GetValues(
	ctx context.Context,
	workspaceID, projectID string,
) ([]*service.FieldValueDTO, error) {
	if m.GetValuesFn != nil {
		return m.GetValuesFn(ctx, workspaceID, projectID)
	}
	return nil, nil
}

func (m *MockProjectFieldService) SetValues(
	ctx context.Context,
	workspaceID, projectID, userID string,
	inputs []service.SetFieldValueInput,
) ([]*service.FieldValueDTO, error) {
	if m.SetValuesFn != nil {
		return m.SetValuesFn(ctx, workspaceID, projectID, userID, inputs)
	}
	return nil, nil
}
