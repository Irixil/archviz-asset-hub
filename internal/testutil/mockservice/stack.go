package mockservice

import (
	"context"
	"io"

	"damask/server/internal/service"
)

// MockStackService is a no-op implementation of service.StackService.
type MockStackService struct {
	ExportZipFn    func(ctx context.Context, workspaceID string, p service.ExportZipParams, w io.Writer) error
	EnqueueMergeFn func(ctx context.Context, workspaceID, userID string, p service.MergeParams) (string, error)
}

func NewStackService() *MockStackService { return &MockStackService{} }

func (m *MockStackService) ExportZip(
	ctx context.Context,
	workspaceID string,
	p service.ExportZipParams,
	w io.Writer,
) error {
	if m.ExportZipFn != nil {
		return m.ExportZipFn(ctx, workspaceID, p, w)
	}
	return nil
}

func (m *MockStackService) EnqueueMerge(
	ctx context.Context,
	workspaceID, userID string,
	p service.MergeParams,
) (string, error) {
	if m.EnqueueMergeFn != nil {
		return m.EnqueueMergeFn(ctx, workspaceID, userID, p)
	}
	return "", nil
}

// MockIntegrationService is a no-op implementation of service.IntegrationService.
type MockIntegrationService struct {
	ListConnectionsFn  func(ctx context.Context, workspaceID string) ([]*service.ConnectionDTO, error)
	DeleteConnectionFn func(ctx context.Context, workspaceID, id string) error
	UpsertConnectionFn func(ctx context.Context, p service.UpsertConnectionParams) error
}

func NewIntegrationService() *MockIntegrationService { return &MockIntegrationService{} }

func (m *MockIntegrationService) ListConnections(
	ctx context.Context,
	workspaceID string,
) ([]*service.ConnectionDTO, error) {
	if m.ListConnectionsFn != nil {
		return m.ListConnectionsFn(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockIntegrationService) DeleteConnection(ctx context.Context, workspaceID, id string) error {
	if m.DeleteConnectionFn != nil {
		return m.DeleteConnectionFn(ctx, workspaceID, id)
	}
	return nil
}

func (m *MockIntegrationService) UpsertConnection(ctx context.Context, p service.UpsertConnectionParams) error {
	if m.UpsertConnectionFn != nil {
		return m.UpsertConnectionFn(ctx, p)
	}
	return nil
}
