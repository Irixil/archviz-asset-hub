package memory

import (
	"context"
	"slices"

	"damask/server/internal/repository"
)

// CollectionRepo is a map-backed CollectionRepository for unit tests.
type CollectionRepo struct {
	mapStore[repository.Collection]

	assets map[string][]string // collectionID -> []assetID
}

func NewRealCollectionRepo() *CollectionRepo {
	return &CollectionRepo{
		mapStore: newMapStore[repository.Collection](),
		assets:   make(map[string][]string),
	}
}

func (r *CollectionRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Collection, error) {
	return r.mapStore.get("collection", id, workspaceID, func(c repository.Collection) string { return c.WorkspaceID })
}

func (r *CollectionRepo) List(_ context.Context, workspaceID string) ([]repository.Collection, error) {
	var out []repository.Collection
	for _, c := range r.mapStore.all() {
		if c.WorkspaceID == workspaceID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *CollectionRepo) Create(_ context.Context, c repository.Collection) (repository.Collection, error) {
	r.mapStore.put(c.ID, c)
	return c, nil
}

func (r *CollectionRepo) Update(_ context.Context, c repository.Collection) (repository.Collection, error) {
	err := r.mapStore.putChecked("collection", c.ID, c.WorkspaceID,
		func(x repository.Collection) string { return x.WorkspaceID }, c)
	return c, err
}

func (r *CollectionRepo) Delete(_ context.Context, workspaceID, id string) error {
	if err := r.mapStore.del(
		"collection",
		id,
		workspaceID,
		func(c repository.Collection) string { return c.WorkspaceID },
	); err != nil {
		return err
	}
	r.mapStore.mu.Lock()
	delete(r.assets, id)
	r.mapStore.mu.Unlock()
	return nil
}

func (r *CollectionRepo) AddAsset(_ context.Context, collectionID, assetID string) error {
	r.mapStore.mu.Lock()
	defer r.mapStore.mu.Unlock()
	if slices.Contains(r.assets[collectionID], assetID) {
		return nil
	}
	r.assets[collectionID] = append(r.assets[collectionID], assetID)
	return nil
}

func (r *CollectionRepo) RemoveAsset(_ context.Context, collectionID, assetID string) error {
	r.mapStore.mu.Lock()
	defer r.mapStore.mu.Unlock()
	ids := r.assets[collectionID]
	filtered := ids[:0]
	for _, id := range ids {
		if id != assetID {
			filtered = append(filtered, id)
		}
	}
	r.assets[collectionID] = filtered
	return nil
}

func (r *CollectionRepo) ListForAsset(
	_ context.Context,
	workspaceID, assetID string,
) ([]repository.Collection, error) {
	r.mapStore.mu.RLock()
	defer r.mapStore.mu.RUnlock()
	var out []repository.Collection
	for collID, assetIDs := range r.assets {
		if slices.Contains(assetIDs, assetID) {
			if c, ok := r.mapStore.items[collID]; ok && c.WorkspaceID == workspaceID {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

func (r *CollectionRepo) CountAssets(_ context.Context, collectionID string) (int64, error) {
	r.mapStore.mu.RLock()
	defer r.mapStore.mu.RUnlock()
	return int64(len(r.assets[collectionID])), nil
}

func (r *CollectionRepo) ListAssetIDs(_ context.Context, collectionID string) ([]string, error) {
	r.mapStore.mu.RLock()
	defer r.mapStore.mu.RUnlock()
	ids := r.assets[collectionID]
	out := make([]string, len(ids))
	copy(out, ids)
	return out, nil
}
