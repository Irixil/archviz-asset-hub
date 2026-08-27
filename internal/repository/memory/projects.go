package memory

import (
	"context"

	"damask/server/internal/repository"
)

type ProjectRepo struct {
	mapStore[repository.Project]
}

func NewProjectRepo() *ProjectRepo {
	return &ProjectRepo{mapStore: newMapStore[repository.Project]()}
}

func (r *ProjectRepo) Seed(projects ...repository.Project) {
	r.mapStore.seed(projects, func(p repository.Project) string { return p.ID })
}

func (r *ProjectRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Project, error) {
	return r.mapStore.get("project", id, workspaceID, func(p repository.Project) string { return p.WorkspaceID })
}

func (r *ProjectRepo) List(_ context.Context, workspaceID string) ([]repository.ProjectWithCount, error) {
	var out []repository.ProjectWithCount
	for _, p := range r.mapStore.all() {
		if p.WorkspaceID == workspaceID {
			out = append(out, repository.ProjectWithCount{Project: p})
		}
	}
	return out, nil
}

func (r *ProjectRepo) NullifyAssets(_ context.Context, _, _ string) error { return nil }

func (r *ProjectRepo) Create(_ context.Context, p repository.Project) (repository.Project, error) {
	r.mapStore.put(p.ID, p)
	return p, nil
}

func (r *ProjectRepo) Update(_ context.Context, p repository.Project) (repository.Project, error) {
	err := r.mapStore.putChecked("project", p.ID, p.WorkspaceID,
		func(x repository.Project) string { return x.WorkspaceID }, p)
	return p, err
}

func (r *ProjectRepo) Delete(_ context.Context, workspaceID, id string) error {
	return r.mapStore.del("project", id, workspaceID, func(p repository.Project) string { return p.WorkspaceID })
}
