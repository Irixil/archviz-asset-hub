package memory

import (
	"context"
	"fmt"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// ShareRepo is a map-backed ShareRepository for unit tests.
type ShareRepo struct {
	mapStore[repository.Share]
}

func NewRealShareRepo() *ShareRepo {
	return &ShareRepo{mapStore: newMapStore[repository.Share]()}
}

func (r *ShareRepo) Seed(shares ...repository.Share) {
	r.mapStore.seed(shares, func(s repository.Share) string { return s.ID })
}

func (r *ShareRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Share, error) {
	return r.mapStore.get("share", id, workspaceID, func(s repository.Share) string { return s.WorkspaceID })
}

func (r *ShareRepo) List(_ context.Context, workspaceID string) ([]repository.Share, error) {
	var out []repository.Share
	for _, sh := range r.mapStore.all() {
		if sh.WorkspaceID == workspaceID {
			out = append(out, sh)
		}
	}
	return out, nil
}

func (r *ShareRepo) Create(_ context.Context, sh repository.Share) (repository.Share, error) {
	r.mapStore.put(sh.ID, sh)
	return sh, nil
}

func (r *ShareRepo) Update(_ context.Context, sh repository.Share) (repository.Share, error) {
	err := r.mapStore.putChecked("share", sh.ID, sh.WorkspaceID,
		func(x repository.Share) string { return x.WorkspaceID }, sh)
	return sh, err
}

func (r *ShareRepo) Revoke(_ context.Context, workspaceID, id string) error {
	return r.mapStore.mutate("share", id, workspaceID,
		func(s repository.Share) string { return s.WorkspaceID },
		func(s repository.Share) (repository.Share, error) {
			now := time.Now().UTC().Format("2006-01-02 15:04:05")
			s.RevokedAt = &now
			return s, nil
		},
	)
}

func (r *ShareRepo) GetPublic(_ context.Context, id string) (repository.Share, error) {
	r.mapStore.mu.RLock()
	defer r.mapStore.mu.RUnlock()
	sh, ok := r.mapStore.items[id]
	if !ok {
		return repository.Share{}, fmt.Errorf("share %q: %w", id, apperr.ErrNotFound)
	}
	return sh, nil
}

func (r *ShareRepo) GetByIDAndWorkspace(_ context.Context, workspaceID, id string) (repository.Share, error) {
	return r.mapStore.get("share", id, workspaceID, func(s repository.Share) string { return s.WorkspaceID })
}

func (r *ShareRepo) IncrementViewCount(_ context.Context, _ string) error { return nil }

func (r *ShareRepo) ListAssetsByTarget(_ context.Context, _, _ string) ([]repository.PublicAsset, error) {
	return nil, nil
}

func (r *ShareRepo) GetPublicAsset(_ context.Context, _ string) (repository.PublicAsset, error) {
	return repository.PublicAsset{}, fmt.Errorf("%w", apperr.ErrNotFound)
}

func (r *ShareRepo) GetPublicAssetFile(_ context.Context, _ string) (repository.PublicAssetFile, error) {
	return repository.PublicAssetFile{}, fmt.Errorf("%w", apperr.ErrNotFound)
}

func (r *ShareRepo) GetPublicAssetThumb(_ context.Context, _ string) (*string, time.Time, error) {
	return nil, time.Time{}, fmt.Errorf("%w", apperr.ErrNotFound)
}

func (r *ShareRepo) IsAssetInTarget(_ context.Context, _, _, _ string) (bool, error) {
	return false, nil
}

func (r *ShareRepo) CreateComment(_ context.Context, c repository.ShareComment) (repository.ShareComment, error) {
	return c, nil
}

func (r *ShareRepo) ListCommentsByShare(_ context.Context, _ string) ([]repository.ShareComment, error) {
	return nil, nil
}

func (r *ShareRepo) ListCommentsByShareAndAsset(_ context.Context, _, _ string) ([]repository.ShareComment, error) {
	return nil, nil
}

func (r *ShareRepo) DeleteComment(_ context.Context, _, _ string) error { return nil }
