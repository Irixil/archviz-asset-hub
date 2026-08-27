package memory

import (
	"context"
	"sync"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// EmbedTokenRepo is a map-backed EmbedTokenRepository for unit tests.
type EmbedTokenRepo struct {
	mu    sync.RWMutex
	items map[string]repository.EmbedToken
}

func NewEmbedTokenRepo() *EmbedTokenRepo {
	return &EmbedTokenRepo{items: make(map[string]repository.EmbedToken)}
}

func (r *EmbedTokenRepo) Create(
	_ context.Context,
	params repository.CreateEmbedTokenParams,
) (repository.EmbedToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, t := range r.items {
		if t.AssetID == params.AssetID && t.RevokedAt == nil {
			return repository.EmbedToken{}, apperr.ErrConflict
		}
	}

	t := repository.EmbedToken{
		ID:          params.ID,
		WorkspaceID: params.WorkspaceID,
		AssetID:     params.AssetID,
		CreatedBy:   params.CreatedBy,
		Label:       params.Label,
		CreatedAt:   time.Now().UTC(),
	}
	r.items[t.ID] = t
	return t, nil
}

func (r *EmbedTokenRepo) GetByID(_ context.Context, id string) (repository.EmbedToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.items[id]
	if !ok {
		return repository.EmbedToken{}, apperr.ErrNotFound
	}
	return t, nil
}

func (r *EmbedTokenRepo) GetActiveByAssetID(
	_ context.Context,
	workspaceID, assetID string,
) (repository.EmbedToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.items {
		if t.WorkspaceID == workspaceID && t.AssetID == assetID && t.RevokedAt == nil {
			return t, nil
		}
	}
	return repository.EmbedToken{}, apperr.ErrNotFound
}

func (r *EmbedTokenRepo) Revoke(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.items[id]
	if !ok || t.WorkspaceID != workspaceID || t.RevokedAt != nil {
		return apperr.ErrNotFound
	}
	now := time.Now().UTC()
	t.RevokedAt = &now
	r.items[id] = t
	return nil
}
