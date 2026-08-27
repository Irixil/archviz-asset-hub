package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockAuditLogService is a no-op implementation of service.AuditLogService.
type MockAuditLogService struct {
	ListAssetEventsFn       func(ctx context.Context, p service.ListAssetEventsParams) (*service.AuditEventListDTO, error)
	ListProjectEventsFn     func(ctx context.Context, p service.ListProjectEventsParams) (*service.AuditEventListDTO, error)
	ListWorkspaceActivityFn func(ctx context.Context, p service.ListWorkspaceActivityParams) (*service.ActivityListDTO, error)
	ExportActivityFn        func(ctx context.Context, p service.ExportActivityParams) (string, error)
}

func NewAuditLogService() *MockAuditLogService { return &MockAuditLogService{} }

func (m *MockAuditLogService) ListAssetEvents(
	ctx context.Context,
	p service.ListAssetEventsParams,
) (*service.AuditEventListDTO, error) {
	if m.ListAssetEventsFn != nil {
		return m.ListAssetEventsFn(ctx, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockAuditLogService) ListProjectEvents(
	ctx context.Context,
	p service.ListProjectEventsParams,
) (*service.AuditEventListDTO, error) {
	if m.ListProjectEventsFn != nil {
		return m.ListProjectEventsFn(ctx, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockAuditLogService) ListWorkspaceActivity(
	ctx context.Context,
	p service.ListWorkspaceActivityParams,
) (*service.ActivityListDTO, error) {
	if m.ListWorkspaceActivityFn != nil {
		return m.ListWorkspaceActivityFn(ctx, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockAuditLogService) ExportActivity(ctx context.Context, p service.ExportActivityParams) (string, error) {
	if m.ExportActivityFn != nil {
		return m.ExportActivityFn(ctx, p)
	}
	return "", nil
}
