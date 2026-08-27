package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockFolderService is a no-op implementation of service.FolderService.
type MockFolderService struct {
	CreateFn   func(ctx context.Context, workspaceID, projectID string, p service.CreateFolderParams) (*service.FolderDTO, error)
	GetFn      func(ctx context.Context, workspaceID, id string) (*service.FolderDTO, error)
	ListFn     func(ctx context.Context, workspaceID, projectID string) ([]*service.FolderDTO, error)
	ListTreeFn func(ctx context.Context, workspaceID, projectID string) ([]*service.FolderTreeDTO, error)
	UpdateFn   func(ctx context.Context, workspaceID, id string, p service.UpdateFolderParams) (*service.FolderDTO, error)
	DeleteFn   func(ctx context.Context, workspaceID, id string) error
}

func NewFolderService() *MockFolderService { return &MockFolderService{} }

func (m *MockFolderService) Create(
	ctx context.Context,
	workspaceID, projectID string,
	p service.CreateFolderParams,
) (*service.FolderDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, workspaceID, projectID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockFolderService) Get(ctx context.Context, workspaceID, id string) (*service.FolderDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, id)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockFolderService) List(ctx context.Context, workspaceID, projectID string) ([]*service.FolderDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID, projectID)
	}
	return nil, nil
}

func (m *MockFolderService) ListTree(
	ctx context.Context,
	workspaceID, projectID string,
) ([]*service.FolderTreeDTO, error) {
	if m.ListTreeFn != nil {
		return m.ListTreeFn(ctx, workspaceID, projectID)
	}
	return nil, nil
}

func (m *MockFolderService) Update(
	ctx context.Context,
	workspaceID, id string,
	p service.UpdateFolderParams,
) (*service.FolderDTO, error) {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, workspaceID, id, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockFolderService) Delete(ctx context.Context, workspaceID, id string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, id)
	}
	return nil
}
