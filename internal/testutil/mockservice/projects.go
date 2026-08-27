package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockProjectService is a no-op implementation of service.ProjectService.
type MockProjectService struct {
	CreateFn func(ctx context.Context, workspaceID string, p service.CreateProjectParams) (*service.ProjectDTO, error)
	GetFn    func(ctx context.Context, workspaceID, id string) (*service.ProjectDTO, error)
	ListFn   func(ctx context.Context, workspaceID string) ([]*service.ProjectDTO, error)
	UpdateFn func(ctx context.Context, workspaceID, id string, p service.UpdateProjectParams) (*service.ProjectDTO, error)
	DeleteFn func(ctx context.Context, workspaceID, id string) error
}

func NewProjectService() *MockProjectService { return &MockProjectService{} }

func (m *MockProjectService) Create(
	ctx context.Context,
	workspaceID string,
	p service.CreateProjectParams,
) (*service.ProjectDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, workspaceID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockProjectService) Get(ctx context.Context, workspaceID, id string) (*service.ProjectDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, id)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockProjectService) List(ctx context.Context, workspaceID string) ([]*service.ProjectDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockProjectService) Update(
	ctx context.Context,
	workspaceID, id string,
	p service.UpdateProjectParams,
) (*service.ProjectDTO, error) {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, workspaceID, id, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockProjectService) Delete(ctx context.Context, workspaceID, id string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, id)
	}
	return nil
}
