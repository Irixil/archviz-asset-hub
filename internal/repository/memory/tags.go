package memory

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

// RealTagRepo is a map-backed TagRepository for unit tests.
// The stub TagRepo in stubs.go remains for cases that just need the interface.
type RealTagRepo struct {
	mu        sync.RWMutex
	tags      map[string]repository.Tag // key: id
	assetTags map[string][]string       // key: assetID -> []tagID
}

func NewRealTagRepo() *RealTagRepo {
	return &RealTagRepo{
		tags:      make(map[string]repository.Tag),
		assetTags: make(map[string][]string),
	}
}

func (r *RealTagRepo) GetByName(_ context.Context, workspaceID, name string) (repository.Tag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.tags {
		if t.WorkspaceID == workspaceID && t.Name == name {
			return t, nil
		}
	}
	return repository.Tag{}, fmt.Errorf("tag %q: %w", name, apperr.ErrNotFound)
}

func (r *RealTagRepo) List(_ context.Context, workspaceID string, includeSystem bool) ([]repository.Tag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.Tag
	for _, t := range r.tags {
		if t.WorkspaceID == workspaceID {
			if !includeSystem && t.GroupName != nil && *t.GroupName == "system" {
				continue
			}
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *RealTagRepo) Upsert(_ context.Context, workspaceID, name string) (repository.Tag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tags {
		if t.WorkspaceID == workspaceID && t.Name == name {
			return t, nil
		}
	}
	t := repository.Tag{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        name,
	}
	r.tags[t.ID] = t
	return t, nil
}

func (r *RealTagRepo) EnsureSystemTag(_ context.Context, workspaceID, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	groupName := "system"
	for id, t := range r.tags {
		if t.WorkspaceID == workspaceID && t.Name == name {
			if t.GroupName == nil || *t.GroupName != groupName {
				t.GroupName = &groupName
				r.tags[id] = t
			}
			return nil
		}
	}
	id := uuid.NewString()
	r.tags[id] = repository.Tag{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        name,
		GroupName:   &groupName,
	}
	return nil
}

func (r *RealTagRepo) UpdateMetadata(_ context.Context, workspaceID, name string, color, groupName *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, t := range r.tags {
		if t.WorkspaceID == workspaceID && t.Name == name {
			t.Color = color
			t.GroupName = groupName
			r.tags[id] = t
			return nil
		}
	}
	return apperr.ErrNotFound
}

func (r *RealTagRepo) Rename(_ context.Context, workspaceID, oldName, newName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, t := range r.tags {
		if t.WorkspaceID == workspaceID && t.Name == oldName {
			t.Name = newName
			r.tags[id] = t
			return nil
		}
	}
	return fmt.Errorf("tag %q: %w", oldName, apperr.ErrNotFound)
}

func (r *RealTagRepo) Delete(_ context.Context, workspaceID string, names []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for id, t := range r.tags {
		if t.WorkspaceID == workspaceID && nameSet[t.Name] {
			delete(r.tags, id)
		}
	}
	return nil
}

func (r *RealTagRepo) ListForAsset(_ context.Context, assetID string) ([]repository.Tag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.assetTags[assetID]
	out := make([]repository.Tag, 0, len(ids))
	for _, id := range ids {
		if t, ok := r.tags[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *RealTagRepo) AddToAsset(_ context.Context, assetID, tagID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slices.Contains(r.assetTags[assetID], tagID) {
		return nil // idempotent
	}
	r.assetTags[assetID] = append(r.assetTags[assetID], tagID)
	return nil
}

func (r *RealTagRepo) RemoveFromAsset(_ context.Context, _, assetID, tagName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var tagID string
	for _, t := range r.tags {
		if t.Name == tagName {
			tagID = t.ID
			break
		}
	}
	if tagID == "" {
		return nil
	}
	ids := r.assetTags[assetID]
	filtered := ids[:0]
	for _, id := range ids {
		if id != tagID {
			filtered = append(filtered, id)
		}
	}
	r.assetTags[assetID] = filtered
	return nil
}

func (r *RealTagRepo) BatchTagsForAssets(_ context.Context, assetIDs []string) (map[string][]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	want := make(map[string]struct{}, len(assetIDs))
	for _, id := range assetIDs {
		want[id] = struct{}{}
	}
	out := make(map[string][]string, len(assetIDs))
	for assetID, tagIDs := range r.assetTags {
		if _, ok := want[assetID]; !ok {
			continue
		}
		for _, tagID := range tagIDs {
			for _, t := range r.tags {
				if t.ID == tagID {
					out[assetID] = append(out[assetID], t.Name)
					break
				}
			}
		}
	}
	return out, nil
}

func (r *RealTagRepo) CountAssets(_ context.Context, tagID string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count int64
	for _, ids := range r.assetTags {
		for _, id := range ids {
			if id == tagID {
				count++
			}
		}
	}
	return count, nil
}

func (r *RealTagRepo) ReassignAssets(_ context.Context, fromTagID, toTagID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for assetID, ids := range r.assetTags {
		hasFrom := false
		hasTo := false
		for _, id := range ids {
			if id == fromTagID {
				hasFrom = true
			}
			if id == toTagID {
				hasTo = true
			}
		}
		if hasFrom && !hasTo {
			r.assetTags[assetID] = append(r.assetTags[assetID], toTagID)
		}
	}
	return nil
}

func (r *RealTagRepo) TouchLastUsed(_ context.Context, _, _ string) error { return nil }

func (r *RealTagRepo) FindAssetBySystemTagInFolder(_ context.Context, _, _, _ string) (repository.Asset, error) {
	return repository.Asset{}, apperr.ErrNotFound
}

func (r *RealTagRepo) FindAssetBySystemTagInProject(_ context.Context, _, _, _ string) (repository.Asset, error) {
	return repository.Asset{}, apperr.ErrNotFound
}

func (r *RealTagRepo) FindAssetBySystemTagInWorkspace(_ context.Context, _, _ string) (repository.Asset, error) {
	return repository.Asset{}, apperr.ErrNotFound
}

func (r *RealTagRepo) RunInTx(_ context.Context, fn func(repository.TagRepository) error) error {
	return fn(r)
}
