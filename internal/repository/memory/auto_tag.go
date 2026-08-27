package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// AutoTagSuggestionMemoryRepo is an in-memory implementation of AutoTagSuggestionRepository.
type AutoTagSuggestionMemoryRepo struct {
	mu   sync.RWMutex
	sugs map[string]repository.AutoTagSuggestion
}

// NewAutoTagSuggestionRepo creates a new in-memory AutoTagSuggestionRepository.
func NewAutoTagSuggestionRepo() *AutoTagSuggestionMemoryRepo {
	return &AutoTagSuggestionMemoryRepo{sugs: map[string]repository.AutoTagSuggestion{}}
}

func (r *AutoTagSuggestionMemoryRepo) Seed(s repository.AutoTagSuggestion) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sugs[s.ID] = s
}

func (r *AutoTagSuggestionMemoryRepo) List(
	_ context.Context,
	workspaceID, assetID string,
) ([]repository.AutoTagSuggestion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.AutoTagSuggestion{}
	for _, s := range r.sugs {
		if s.WorkspaceID == workspaceID && s.AssetID == assetID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r *AutoTagSuggestionMemoryRepo) Get(
	_ context.Context,
	workspaceID, id string,
) (repository.AutoTagSuggestion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sugs[id]
	if !ok || s.WorkspaceID != workspaceID {
		return repository.AutoTagSuggestion{}, apperr.ErrNotFound
	}
	return s, nil
}

func (r *AutoTagSuggestionMemoryRepo) Create(
	_ context.Context,
	params repository.CreateAutoTagSuggestionParams,
) (repository.AutoTagSuggestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := repository.AutoTagSuggestion{
		ID:             params.ID,
		WorkspaceID:    params.WorkspaceID,
		AssetID:        params.AssetID,
		AssetVersionID: params.AssetVersionID,
		TagName:        params.TagName,
		CreatedAt:      time.Now().UTC(),
	}
	r.sugs[s.ID] = s
	return s, nil
}

func (r *AutoTagSuggestionMemoryRepo) Delete(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sugs[id]
	if !ok || s.WorkspaceID != workspaceID {
		return apperr.ErrNotFound
	}
	delete(r.sugs, id)
	return nil
}

func (r *AutoTagSuggestionMemoryRepo) DeleteByAsset(_ context.Context, workspaceID, assetID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.sugs {
		if s.WorkspaceID == workspaceID && s.AssetID == assetID {
			delete(r.sugs, id)
		}
	}
	return nil
}
