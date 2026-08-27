package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// RealVersionRepo is a map-backed VersionRepository for unit tests.
type RealVersionRepo struct {
	mu            sync.RWMutex
	versions      map[string]repository.AssetVersion
	coverVersions map[string]bool // versionID -> is referenced as cover
}

func NewRealVersionRepo() *RealVersionRepo {
	return &RealVersionRepo{
		versions:      make(map[string]repository.AssetVersion),
		coverVersions: make(map[string]bool),
	}
}

func (r *RealVersionRepo) Seed(versions ...repository.AssetVersion) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range versions {
		r.versions[v.ID] = v
	}
}

// MarkAsCover marks a version as referenced by a project cover (for testing conflict checks).
func (r *RealVersionRepo) MarkAsCover(versionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.coverVersions[versionID] = true
}

func (r *RealVersionRepo) GetByID(_ context.Context, id string) (repository.AssetVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.versions[id]
	if !ok {
		return repository.AssetVersion{}, fmt.Errorf("version %q: %w", id, apperr.ErrNotFound)
	}
	return v, nil
}

func (r *RealVersionRepo) GetCurrentByAsset(_ context.Context, assetID string) (repository.AssetVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.versions {
		if v.AssetID == assetID && v.IsCurrent && v.DeletedAt == nil {
			return v, nil
		}
	}
	return repository.AssetVersion{}, fmt.Errorf("no current version for asset %q: %w", assetID, apperr.ErrNotFound)
}

func (r *RealVersionRepo) GetFirstByAsset(_ context.Context, assetID string) (repository.AssetVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var first *repository.AssetVersion
	for i := range r.versions {
		v := r.versions[i]
		if v.AssetID != assetID || v.DeletedAt != nil {
			continue
		}
		if first == nil || v.VersionNum < first.VersionNum {
			first = &v
		}
	}
	if first == nil {
		return repository.AssetVersion{}, fmt.Errorf("no version for asset %q: %w", assetID, apperr.ErrNotFound)
	}
	return *first, nil
}

func (r *RealVersionRepo) GetByIDForWorkspace(
	_ context.Context,
	workspaceID, id string,
) (repository.AssetVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.versions[id]
	if !ok || v.WorkspaceID != workspaceID {
		return repository.AssetVersion{}, fmt.Errorf("version %q: %w", id, apperr.ErrNotFound)
	}
	return v, nil
}

func (r *RealVersionRepo) ListByAsset(_ context.Context, assetID string) ([]repository.AssetVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.AssetVersion
	for _, v := range r.versions {
		if v.AssetID == assetID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *RealVersionRepo) Create(_ context.Context, v repository.AssetVersion) (repository.AssetVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.versions[v.ID] = v
	return v, nil
}

func (r *RealVersionRepo) SoftDelete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.versions[id]
	if !ok {
		return fmt.Errorf("version %q: %w", id, apperr.ErrNotFound)
	}
	now := "deleted"
	v.DeletedAt = &now
	r.versions[id] = v
	return nil
}

func (r *RealVersionRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.versions[id]; !ok {
		return fmt.Errorf("version %q: %w", id, apperr.ErrNotFound)
	}
	delete(r.versions, id)
	return nil
}

func (r *RealVersionRepo) CountByAsset(_ context.Context, assetID string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count int64
	for _, v := range r.versions {
		if v.AssetID == assetID && v.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (r *RealVersionRepo) IsReferencedAsCover(_ context.Context, versionID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.coverVersions[versionID], nil
}

func (r *RealVersionRepo) GetByHash(_ context.Context, assetID, contentHash string) (repository.AssetVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.versions {
		if v.AssetID == assetID && v.ContentHash == contentHash {
			return v, nil
		}
	}
	return repository.AssetVersion{}, fmt.Errorf("version not found: %w", apperr.ErrNotFound)
}

func (r *RealVersionRepo) NextVersionNum(_ context.Context, assetID string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var maxVersion int64
	for _, v := range r.versions {
		if v.AssetID == assetID && v.VersionNum > maxVersion {
			maxVersion = v.VersionNum
		}
	}
	return maxVersion + 1, nil
}

func (r *RealVersionRepo) SetCurrent(_ context.Context, assetID, versionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, v := range r.versions {
		if v.AssetID == assetID {
			v.IsCurrent = id == versionID
			r.versions[id] = v
		}
	}
	return nil
}

func (r *RealVersionRepo) SetAssetThumbnail(_ context.Context, _ string, _ *string) error { return nil }

func (r *RealVersionRepo) RunInTx(_ context.Context, fn func(tx repository.VersionRepository) error) error {
	return fn(r)
}

func (r *RealVersionRepo) FindDuplicateVersions(
	_ context.Context,
	workspaceID, contentHash, excludeAssetID string,
) ([]repository.DuplicateVersionMatch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.DuplicateVersionMatch
	for _, v := range r.versions {
		if v.WorkspaceID != workspaceID || v.ContentHash != contentHash || v.AssetID == excludeAssetID {
			continue
		}
		out = append(out, repository.DuplicateVersionMatch{
			VersionID:  v.ID,
			AssetID:    v.AssetID,
			VersionNum: v.VersionNum,
			StorageKey: v.StorageKey,
			IsCurrent:  v.IsCurrent,
			DeletedAt:  v.DeletedAt,
			CreatedAt:  v.CreatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsCurrent != out[j].IsCurrent {
			return out[i].IsCurrent
		}
		iDeleted, jDeleted := out[i].DeletedAt != nil, out[j].DeletedAt != nil
		if iDeleted != jDeleted {
			return !iDeleted
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (r *RealVersionRepo) ListWithVariantCount(
	_ context.Context,
	assetID string,
) ([]repository.AssetVersionWithCount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.AssetVersionWithCount
	for _, v := range r.versions {
		if v.AssetID == assetID && v.DeletedAt == nil {
			out = append(out, repository.AssetVersionWithCount{AssetVersion: v})
		}
	}
	return out, nil
}
