package memory

import (
	"context"

	"damask/server/internal/repository"

	"github.com/google/uuid"
)

// FolderRepo is a map-backed FolderRepository for unit tests.
type FolderRepo struct {
	mapStore[repository.Folder]
}

func NewRealFolderRepo() *FolderRepo {
	return &FolderRepo{mapStore: newMapStore[repository.Folder]()}
}

func (r *FolderRepo) Seed(folders ...repository.Folder) {
	r.mapStore.seed(folders, func(f repository.Folder) string { return f.ID })
}

func (r *FolderRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Folder, error) {
	return r.mapStore.get("folder", id, workspaceID, func(f repository.Folder) string { return f.WorkspaceID })
}

func (r *FolderRepo) ListByProject(_ context.Context, workspaceID, projectID string) ([]repository.Folder, error) {
	var out []repository.Folder
	for _, f := range r.mapStore.all() {
		if f.WorkspaceID == workspaceID && f.ProjectID == projectID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *FolderRepo) Create(_ context.Context, f repository.Folder) (repository.Folder, error) {
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	r.mapStore.put(f.ID, f)
	return f, nil
}

func (r *FolderRepo) Update(_ context.Context, f repository.Folder) (repository.Folder, error) {
	err := r.mapStore.putChecked("folder", f.ID, f.WorkspaceID,
		func(x repository.Folder) string { return x.WorkspaceID }, f)
	return f, err
}

func (r *FolderRepo) Delete(_ context.Context, workspaceID, id string) error {
	return r.mapStore.del("folder", id, workspaceID, func(f repository.Folder) string { return f.WorkspaceID })
}

func (r *FolderRepo) GetChildren(_ context.Context, workspaceID, parentID string) ([]repository.Folder, error) {
	var out []repository.Folder
	for _, f := range r.mapStore.all() {
		if f.WorkspaceID == workspaceID && f.ParentID != nil && *f.ParentID == parentID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *FolderRepo) NullifyAssets(_ context.Context, _, _ string) error { return nil }

func (r *FolderRepo) ListTree(_ context.Context, workspaceID, projectID string) ([]repository.FolderTree, error) {
	var roots []repository.FolderTree
	for _, f := range r.mapStore.all() {
		if f.WorkspaceID == workspaceID && f.ProjectID == projectID && f.ParentID == nil {
			roots = append(roots, repository.FolderTree{Folder: f})
		}
	}
	return roots, nil
}
