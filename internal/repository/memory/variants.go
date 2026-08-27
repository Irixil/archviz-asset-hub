package memory

import (
	"context"
	"fmt"
	"slices"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// VariantRepo is a map-backed VariantRepository for unit tests.
type VariantRepo struct {
	mapStore[repository.Variant]
}

func NewRealVariantRepo() *VariantRepo {
	return &VariantRepo{mapStore: newMapStore[repository.Variant]()}
}

func (r *VariantRepo) Seed(variants ...repository.Variant) {
	r.mapStore.seed(variants, func(v repository.Variant) string { return v.ID })
}

func (r *VariantRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Variant, error) {
	return r.mapStore.get("variant", id, workspaceID, func(v repository.Variant) string { return v.WorkspaceID })
}

func (r *VariantRepo) ListByAsset(_ context.Context, workspaceID, _ string) ([]repository.Variant, error) {
	// In tests we filter by workspaceID only — without a join to the asset table
	// we can't replicate "current version only", so return all for the workspace.
	var out []repository.Variant
	for _, v := range r.mapStore.all() {
		if v.WorkspaceID == workspaceID {
			out = append(out, v)
		}
	}
	slices.SortFunc(out, func(a, b repository.Variant) int {
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return out, nil
}

func (r *VariantRepo) Create(_ context.Context, v repository.Variant) (repository.Variant, error) {
	if v.Status == "" {
		v.Status = "ready"
	}
	r.mapStore.put(v.ID, v)
	return v, nil
}

func (r *VariantRepo) Delete(_ context.Context, workspaceID, id string) error {
	return r.mapStore.del("variant", id, workspaceID, func(v repository.Variant) string { return v.WorkspaceID })
}

func (r *VariantRepo) UpdateTitle(_ context.Context, workspaceID, variantID string, title *string) error {
	return r.mapStore.mutate("variant", variantID, workspaceID,
		func(v repository.Variant) string { return v.WorkspaceID },
		func(v repository.Variant) (repository.Variant, error) {
			v.Title = title
			return v, nil
		},
	)
}

func (r *VariantRepo) UpdateSharedBatch(_ context.Context, workspaceID string, ids []string, isShared bool) error {
	r.mapStore.mu.Lock()
	defer r.mapStore.mu.Unlock()
	for _, id := range ids {
		v, ok := r.mapStore.items[id]
		if !ok || v.WorkspaceID != workspaceID {
			continue
		}
		v.IsShared = isShared
		r.mapStore.items[id] = v
	}
	return nil
}

func (r *VariantRepo) ListSharedByAssetIDs(
	_ context.Context,
	assetIDs []string,
) ([]repository.VariantWithAssetID, error) {
	if len(assetIDs) == 0 {
		return nil, nil
	}
	assetSet := make(map[string]struct{}, len(assetIDs))
	for _, id := range assetIDs {
		assetSet[id] = struct{}{}
	}
	out := make([]repository.VariantWithAssetID, 0)
	for _, v := range r.mapStore.all() {
		if !v.IsShared {
			continue
		}
		if _, ok := assetSet[v.AssetVersionID]; !ok {
			continue
		}
		out = append(out, repository.VariantWithAssetID{Variant: v, AssetID: v.AssetVersionID})
	}
	slices.SortFunc(out, func(a, b repository.VariantWithAssetID) int {
		if a.AssetID < b.AssetID {
			return -1
		}
		if a.AssetID > b.AssetID {
			return 1
		}
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		return 0
	})
	return out, nil
}

// ListVariantParamHistory returns non-empty transform_params for workspaceID+variantType,
// most recently created first, capped at limit. Mirrors the raw (non-deduplicated) contract
// of the sqlc-backed repository.
func (r *VariantRepo) ListVariantParamHistory(
	_ context.Context,
	workspaceID, variantType string,
	limit int,
) ([]string, error) {
	var matches []repository.Variant
	for _, v := range r.mapStore.all() {
		if v.WorkspaceID != workspaceID || v.Type != variantType {
			continue
		}
		if v.TransformParams == nil || *v.TransformParams == "" || *v.TransformParams == "{}" {
			continue
		}
		matches = append(matches, v)
	}
	slices.SortFunc(matches, func(a, b repository.Variant) int {
		if a.CreatedAt.Before(b.CreatedAt) {
			return 1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return -1
		}
		return 0
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	out := make([]string, len(matches))
	for i, v := range matches {
		out[i] = *v.TransformParams
	}
	return out, nil
}

func (r *VariantRepo) GetSharedByVariantAndAsset(
	_ context.Context,
	variantID, assetID string,
) (repository.Variant, error) {
	r.mapStore.mu.RLock()
	defer r.mapStore.mu.RUnlock()
	v, ok := r.mapStore.items[variantID]
	if !ok || !v.IsShared || v.AssetVersionID != assetID {
		return repository.Variant{}, fmt.Errorf("variant %q: %w", variantID, apperr.ErrNotFound)
	}
	return v, nil
}
