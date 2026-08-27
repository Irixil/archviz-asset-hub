package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockEmbedTokenService is a no-op implementation of service.EmbedTokenService.
type MockEmbedTokenService struct {
	GetOrCreateFn func(ctx context.Context, workspaceID, assetID, userID string) (*service.EmbedTokenDTO, error)
	GetActiveFn   func(ctx context.Context, workspaceID, assetID string) (*service.EmbedTokenDTO, error)
	RevokeFn      func(ctx context.Context, workspaceID, assetID string) error
	ResolveFn     func(ctx context.Context, tokenID string) (*service.ResolvedEmbed, error)
}

func NewEmbedTokenService() *MockEmbedTokenService { return &MockEmbedTokenService{} }

func (m *MockEmbedTokenService) GetOrCreate(
	ctx context.Context,
	workspaceID, assetID, userID string,
) (*service.EmbedTokenDTO, error) {
	if m.GetOrCreateFn != nil {
		return m.GetOrCreateFn(ctx, workspaceID, assetID, userID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockEmbedTokenService) GetActive(
	ctx context.Context,
	workspaceID, assetID string,
) (*service.EmbedTokenDTO, error) {
	if m.GetActiveFn != nil {
		return m.GetActiveFn(ctx, workspaceID, assetID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockEmbedTokenService) Revoke(ctx context.Context, workspaceID, assetID string) error {
	if m.RevokeFn != nil {
		return m.RevokeFn(ctx, workspaceID, assetID)
	}
	return nil
}

func (m *MockEmbedTokenService) ResolveCurrentFile(
	ctx context.Context,
	tokenID string,
) (*service.ResolvedEmbed, error) {
	if m.ResolveFn != nil {
		return m.ResolveFn(ctx, tokenID)
	}
	return nil, nil //nolint:nilnil // mock
}
