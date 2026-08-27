package mockservice

import (
	"context"

	"damask/server/internal/service"
	"damask/server/internal/workflow"
)

// MockAutoTagService is a no-op implementation of service.AutoTagService.
type MockAutoTagService struct {
	EnqueueFn            func(ctx context.Context, workspaceID, assetID string, manual bool) error
	EnqueueForWorkflowFn func(
		ctx context.Context,
		workspaceID, assetID, mode string,
		continuation *workflow.NodeContinuation,
	) error
	IsProviderAvailableFn func(ctx context.Context, workspaceID, mimeType string) bool
	ListSuggestionsFn     func(ctx context.Context, workspaceID, assetID string) ([]service.AutoTagSuggestionDTO, error)
	AcceptSuggestionFn    func(ctx context.Context, workspaceID, assetID, suggestionID string) (*service.TagDTO, error)
	AcceptAllFn           func(ctx context.Context, workspaceID, assetID string) (int, error)
	DismissSuggestionFn   func(ctx context.Context, workspaceID, assetID, suggestionID string) error
	DismissAllFn          func(ctx context.Context, workspaceID, assetID string) error
}

func NewAutoTagService() *MockAutoTagService { return &MockAutoTagService{} }

func (m *MockAutoTagService) Enqueue(ctx context.Context, workspaceID, assetID string, manual bool) error {
	if m.EnqueueFn != nil {
		return m.EnqueueFn(ctx, workspaceID, assetID, manual)
	}
	return nil
}

func (m *MockAutoTagService) EnqueueForWorkflow(
	ctx context.Context,
	workspaceID, assetID, mode string,
	continuation *workflow.NodeContinuation,
) error {
	if m.EnqueueForWorkflowFn != nil {
		return m.EnqueueForWorkflowFn(ctx, workspaceID, assetID, mode, continuation)
	}
	return nil
}

func (m *MockAutoTagService) IsProviderAvailable(ctx context.Context, workspaceID, mimeType string) bool {
	if m.IsProviderAvailableFn != nil {
		return m.IsProviderAvailableFn(ctx, workspaceID, mimeType)
	}
	return false
}

func (m *MockAutoTagService) ListSuggestions(
	ctx context.Context,
	workspaceID, assetID string,
) ([]service.AutoTagSuggestionDTO, error) {
	if m.ListSuggestionsFn != nil {
		return m.ListSuggestionsFn(ctx, workspaceID, assetID)
	}
	return nil, nil
}

func (m *MockAutoTagService) AcceptSuggestion(
	ctx context.Context,
	workspaceID, assetID, suggestionID string,
) (*service.TagDTO, error) {
	if m.AcceptSuggestionFn != nil {
		return m.AcceptSuggestionFn(ctx, workspaceID, assetID, suggestionID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockAutoTagService) AcceptAll(ctx context.Context, workspaceID, assetID string) (int, error) {
	if m.AcceptAllFn != nil {
		return m.AcceptAllFn(ctx, workspaceID, assetID)
	}
	return 0, nil
}

func (m *MockAutoTagService) DismissSuggestion(ctx context.Context, workspaceID, assetID, suggestionID string) error {
	if m.DismissSuggestionFn != nil {
		return m.DismissSuggestionFn(ctx, workspaceID, assetID, suggestionID)
	}
	return nil
}

func (m *MockAutoTagService) DismissAll(ctx context.Context, workspaceID, assetID string) error {
	if m.DismissAllFn != nil {
		return m.DismissAllFn(ctx, workspaceID, assetID)
	}
	return nil
}
