package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// ExportConfigMemoryRepo is an in-memory implementation of ExportConfigRepository.
type ExportConfigMemoryRepo struct {
	mu      sync.RWMutex
	configs map[string]repository.ExportConfig
}

// NewExportConfigRepo creates a new in-memory ExportConfigRepository.
func NewExportConfigRepo() *ExportConfigMemoryRepo {
	return &ExportConfigMemoryRepo{configs: map[string]repository.ExportConfig{}}
}

func (r *ExportConfigMemoryRepo) Seed(cfgs ...repository.ExportConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range cfgs {
		r.configs[c.ID] = c
	}
}

func (r *ExportConfigMemoryRepo) Create(_ context.Context, p repository.ExportConfig) (repository.ExportConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	r.configs[p.ID] = p
	return p, nil
}

func (r *ExportConfigMemoryRepo) Get(_ context.Context, workspaceID, id string) (repository.ExportConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.configs[id]
	if !ok || c.WorkspaceID != workspaceID {
		return repository.ExportConfig{}, apperr.ErrNotFound
	}
	return c, nil
}

func (r *ExportConfigMemoryRepo) List(_ context.Context, workspaceID string) ([]repository.ExportConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.ExportConfig{}
	for _, c := range r.configs {
		if c.WorkspaceID == workspaceID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *ExportConfigMemoryRepo) ListByProject(
	_ context.Context,
	workspaceID, projectID string,
) ([]repository.ExportConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.ExportConfig{}
	for _, c := range r.configs {
		if c.WorkspaceID == workspaceID && c.ProjectID == projectID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *ExportConfigMemoryRepo) Update(_ context.Context, p repository.ExportConfig) (repository.ExportConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.configs[p.ID]
	if !ok || existing.WorkspaceID != p.WorkspaceID {
		return repository.ExportConfig{}, apperr.ErrNotFound
	}
	p.CreatedAt = existing.CreatedAt
	p.UpdatedAt = time.Now().UTC()
	r.configs[p.ID] = p
	return p, nil
}

func (r *ExportConfigMemoryRepo) Delete(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.configs[id]
	if !ok || c.WorkspaceID != workspaceID {
		return apperr.ErrNotFound
	}
	delete(r.configs, id)
	return nil
}

func (r *ExportConfigMemoryRepo) SetLastRun(_ context.Context, id string, p repository.ExportRunResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.configs[id]
	if !ok {
		return apperr.ErrNotFound
	}
	c.LastRunAt = &p.LastRunAt
	c.LastRunStatus = &p.LastRunStatus
	c.LastError = p.LastError
	c.UpdatedAt = time.Now().UTC()
	r.configs[id] = c
	return nil
}

// ListDue returns configs scheduled after_quiet where the project's last touch was
// at least quiet_minutes ago. In-memory implementation assumes touched_at = now for simplicity.
func (r *ExportConfigMemoryRepo) ListDue(_ context.Context) ([]repository.ExportConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now().UTC()
	out := []repository.ExportConfig{}
	for _, c := range r.configs {
		if c.ScheduleType != "after_quiet" || !c.Enabled {
			continue
		}
		if c.QuietMinutes == nil {
			continue
		}
		quietDur := time.Duration(*c.QuietMinutes) * time.Minute
		if c.LastRunAt != nil && now.Sub(*c.LastRunAt) < quietDur {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// ExportRunMemoryRepo is an in-memory implementation of ExportRunRepository.
type ExportRunMemoryRepo struct {
	mu   sync.RWMutex
	runs map[string]repository.ExportRun
}

// NewExportRunRepo creates a new in-memory ExportRunRepository.
func NewExportRunRepo() *ExportRunMemoryRepo {
	return &ExportRunMemoryRepo{runs: map[string]repository.ExportRun{}}
}

func (r *ExportRunMemoryRepo) Seed(runs ...repository.ExportRun) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, run := range runs {
		r.runs[run.ID] = run
	}
}

func (r *ExportRunMemoryRepo) Create(_ context.Context, p repository.ExportRun) (repository.ExportRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p.CreatedAt = time.Now().UTC()
	p.Status = "pending"
	r.runs[p.ID] = p
	return p, nil
}

func (r *ExportRunMemoryRepo) Get(_ context.Context, workspaceID, id string) (repository.ExportRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	run, ok := r.runs[id]
	if !ok || run.WorkspaceID != workspaceID {
		return repository.ExportRun{}, apperr.ErrNotFound
	}
	return run, nil
}

func (r *ExportRunMemoryRepo) List(
	_ context.Context,
	configID string,
	limit, offset int,
) ([]repository.ExportRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.ExportRun{}
	for _, run := range r.runs {
		if run.ExportConfigID == configID {
			out = append(out, run)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if offset >= len(out) {
		return []repository.ExportRun{}, nil
	}
	out = out[offset:]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *ExportRunMemoryRepo) Start(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return apperr.ErrNotFound
	}
	now := time.Now().UTC()
	run.Status = "running"
	run.StartedAt = &now
	r.runs[id] = run
	return nil
}

func (r *ExportRunMemoryRepo) UpdateProgress(_ context.Context, id string, p repository.ExportProgress) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return apperr.ErrNotFound
	}
	run.AssetsExported = p.AssetsExported
	run.AssetsSkipped = p.AssetsSkipped
	run.BytesWritten = p.BytesWritten
	r.runs[id] = run
	return nil
}

func (r *ExportRunMemoryRepo) Finish(_ context.Context, id string, p repository.ExportFinish) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return apperr.ErrNotFound
	}
	now := time.Now().UTC()
	run.Status = p.Status
	run.AssetsTotal = p.AssetsTotal
	run.AssetsExported = p.AssetsExported
	run.AssetsSkipped = p.AssetsSkipped
	run.BytesWritten = p.BytesWritten
	run.Error = p.Error
	run.CompletedAt = &now
	r.runs[id] = run
	return nil
}
