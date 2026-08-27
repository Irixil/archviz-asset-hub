package memory

import (
	"context"
	"errors"
	"fmt"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

const deletedAtMarker = "deleted"

// FieldRepo is a map-backed FieldRepository for unit tests.
type FieldRepo struct {
	mapStore[repository.FieldDefinition]
}

func NewRealFieldRepo() *FieldRepo {
	return &FieldRepo{mapStore: newMapStore[repository.FieldDefinition]()}
}

func (r *FieldRepo) Seed(fields ...repository.FieldDefinition) {
	r.mapStore.seed(fields, func(f repository.FieldDefinition) string { return f.ID })
}

func (r *FieldRepo) GetByID(_ context.Context, workspaceID, id string) (repository.FieldDefinition, error) {
	return r.mapStore.get("field", id, workspaceID, func(f repository.FieldDefinition) string { return f.WorkspaceID })
}

func (r *FieldRepo) List(_ context.Context, workspaceID, scope string) ([]repository.FieldDefinition, error) {
	var out []repository.FieldDefinition
	for _, f := range r.mapStore.all() {
		if f.WorkspaceID == workspaceID && f.Scope == scope && f.DeletedAt == nil {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *FieldRepo) Create(_ context.Context, f repository.FieldDefinition) (repository.FieldDefinition, error) {
	r.mapStore.mu.Lock()
	defer r.mapStore.mu.Unlock()
	for _, existing := range r.mapStore.items {
		if existing.WorkspaceID == f.WorkspaceID && existing.Scope == f.Scope && existing.Key == f.Key &&
			existing.DeletedAt == nil {
			return repository.FieldDefinition{}, errors.New("UNIQUE constraint failed: field_definitions.key")
		}
	}
	r.mapStore.items[f.ID] = f
	return f, nil
}

func (r *FieldRepo) Update(_ context.Context, f repository.FieldDefinition) (repository.FieldDefinition, error) {
	err := r.mapStore.putChecked("field", f.ID, f.WorkspaceID,
		func(x repository.FieldDefinition) string { return x.WorkspaceID }, f)
	return f, err
}

func (r *FieldRepo) SoftDelete(_ context.Context, workspaceID, id string) error {
	return r.mapStore.mutate("field", id, workspaceID,
		func(f repository.FieldDefinition) string { return f.WorkspaceID },
		func(f repository.FieldDefinition) (repository.FieldDefinition, error) {
			now := deletedAtMarker
			f.DeletedAt = &now
			return f, nil
		},
	)
}

func (r *FieldRepo) CountByWorkspaceAndScope(_ context.Context, workspaceID, scope string) (int64, error) {
	var count int64
	for _, f := range r.mapStore.all() {
		if f.WorkspaceID == workspaceID && f.Scope == scope && f.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (r *FieldRepo) PurgeExpired(_ context.Context) (int, error) { return 0, nil }

func (r *FieldRepo) CountAssetValues(_ context.Context, _ string) (int64, error)   { return 0, nil }
func (r *FieldRepo) CountProjectValues(_ context.Context, _ string) (int64, error) { return 0, nil }

func (r *FieldRepo) UpdatePosition(_ context.Context, workspaceID, id string, position int64) error {
	return r.mapStore.mutate("field", id, workspaceID,
		func(f repository.FieldDefinition) string { return f.WorkspaceID },
		func(f repository.FieldDefinition) (repository.FieldDefinition, error) {
			f.Position = position
			return f, nil
		},
	)
}

func (r *FieldRepo) InheritProjectFields(_ context.Context, _, _, _, _ string) error { return nil }

func (r *FieldRepo) GetByKey(_ context.Context, workspaceID, key string) (repository.FieldDefinition, error) {
	for _, f := range r.mapStore.all() {
		if f.WorkspaceID == workspaceID && f.Key == key && f.DeletedAt == nil {
			return f, nil
		}
	}
	return repository.FieldDefinition{}, fmt.Errorf("field key %q: %w", key, apperr.ErrNotFound)
}

func (r *FieldRepo) ListImageAssetIDs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (r *FieldRepo) ListMissingExifField(_ context.Context, _, _ string, _ int64) ([]string, error) {
	return nil, nil
}

func (r *FieldRepo) EnsureSystemField(_ context.Context, params repository.EnsureSystemFieldParams) error {
	r.mapStore.mu.Lock()
	defer r.mapStore.mu.Unlock()
	for _, f := range r.mapStore.items {
		if f.WorkspaceID == params.WorkspaceID && f.Key == params.Key && f.DeletedAt == nil {
			return nil
		}
	}
	r.mapStore.items[params.ID] = repository.FieldDefinition{
		ID:          params.ID,
		WorkspaceID: params.WorkspaceID,
		Source:      params.Source,
		Scope:       "asset",
		Name:        params.Name,
		Key:         params.Key,
		FieldType:   params.FieldType,
		Position:    params.Position,
	}
	return nil
}

func (r *FieldRepo) ListBySource(_ context.Context, workspaceID, source string) ([]repository.FieldDefinition, error) {
	out := []repository.FieldDefinition{}
	for _, f := range r.mapStore.all() {
		if f.WorkspaceID == workspaceID && f.Source == source && f.DeletedAt == nil {
			out = append(out, f)
		}
	}
	return out, nil
}
