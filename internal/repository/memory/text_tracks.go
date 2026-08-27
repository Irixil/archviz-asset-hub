package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// TextTrackMemoryRepo is an in-memory implementation of TextTrackRepository.
type TextTrackMemoryRepo struct {
	mu     sync.RWMutex
	tracks map[string]repository.TextTrack
	fts    map[string]bool // track IDs currently indexed
}

// NewTextTrackRepo creates a new in-memory TextTrackRepository.
func NewTextTrackRepo() *TextTrackMemoryRepo {
	return &TextTrackMemoryRepo{
		tracks: map[string]repository.TextTrack{},
		fts:    map[string]bool{},
	}
}

func (r *TextTrackMemoryRepo) Seed(t repository.TextTrack) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tracks[t.ID] = t
}

func (r *TextTrackMemoryRepo) List(_ context.Context, workspaceID, assetID string) ([]repository.TextTrack, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.TextTrack{}
	for _, t := range r.tracks {
		if t.WorkspaceID == workspaceID && t.AssetID == assetID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *TextTrackMemoryRepo) Get(_ context.Context, workspaceID, id string) (repository.TextTrack, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tracks[id]
	if !ok || t.WorkspaceID != workspaceID {
		return repository.TextTrack{}, apperr.ErrNotFound
	}
	return t, nil
}

func (r *TextTrackMemoryRepo) Create(
	_ context.Context,
	params repository.CreateTextTrackParams,
) (repository.TextTrack, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	t := repository.TextTrack{
		ID:             params.ID,
		WorkspaceID:    params.WorkspaceID,
		AssetID:        params.AssetID,
		AssetVersionID: params.AssetVersionID,
		Source:         params.Source,
		Lang:           params.Lang,
		Content:        params.Content,
		Meta:           params.Meta,
		Status:         params.Status,
		CreatedBy:      params.CreatedBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	r.tracks[t.ID] = t
	return t, nil
}

func (r *TextTrackMemoryRepo) SetReady(
	_ context.Context,
	workspaceID, id string,
	params repository.SetTextTrackReadyParams,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tracks[id]
	if !ok || t.WorkspaceID != workspaceID {
		return apperr.ErrNotFound
	}
	t.Content = params.Content
	t.StorageKey = params.StorageKey
	t.ContentType = params.ContentType
	t.Meta = params.Meta
	t.Status = "ready"
	t.Error = nil
	t.UpdatedAt = time.Now().UTC()
	r.tracks[id] = t
	return nil
}

func (r *TextTrackMemoryRepo) SetFailed(_ context.Context, workspaceID, id, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tracks[id]
	if !ok || t.WorkspaceID != workspaceID {
		return apperr.ErrNotFound
	}
	t.Status = "failed"
	t.Error = &errMsg
	t.UpdatedAt = time.Now().UTC()
	r.tracks[id] = t
	return nil
}

func (r *TextTrackMemoryRepo) Delete(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tracks[id]
	if !ok || t.WorkspaceID != workspaceID {
		return apperr.ErrNotFound
	}
	delete(r.tracks, id)
	delete(r.fts, id)
	return nil
}

func (r *TextTrackMemoryRepo) InsertFTS(_ context.Context, params repository.InsertTextFTSParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fts[params.TrackID] = true
	return nil
}

func (r *TextTrackMemoryRepo) DeleteFTS(_ context.Context, trackID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.fts, trackID)
	return nil
}
