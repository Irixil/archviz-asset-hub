package memory

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// WorkflowRepo is a map-backed WorkflowRepository for unit tests.
type WorkflowRepo struct {
	mapStore[repository.Workflow]
}

func NewWorkflowRepo() *WorkflowRepo {
	return &WorkflowRepo{mapStore: newMapStore[repository.Workflow]()}
}

func (r *WorkflowRepo) Seed(wfs ...repository.Workflow) {
	r.mapStore.seed(wfs, func(wf repository.Workflow) string { return wf.ID })
}

func (r *WorkflowRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Workflow, error) {
	return r.mapStore.get("workflow", id, workspaceID, func(wf repository.Workflow) string { return wf.WorkspaceID })
}

func (r *WorkflowRepo) List(_ context.Context, workspaceID string) ([]repository.Workflow, error) {
	var out []repository.Workflow
	for _, wf := range r.mapStore.all() {
		if wf.WorkspaceID == workspaceID {
			out = append(out, wf)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *WorkflowRepo) ListByTrigger(_ context.Context, triggerType string) ([]repository.Workflow, error) {
	var out []repository.Workflow
	for _, wf := range r.mapStore.all() {
		if wf.Enabled && wf.TriggerType == triggerType {
			out = append(out, wf)
		}
	}
	return out, nil
}

func (r *WorkflowRepo) ListEnabledByTrigger(
	_ context.Context,
	workspaceID, triggerType string,
) ([]repository.Workflow, error) {
	var out []repository.Workflow
	for _, wf := range r.mapStore.all() {
		if wf.WorkspaceID == workspaceID && wf.Enabled && wf.TriggerType == triggerType {
			out = append(out, wf)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (r *WorkflowRepo) Create(_ context.Context, p repository.CreateWorkflowParams) (repository.Workflow, error) {
	now := time.Now().UTC()
	wf := repository.Workflow{
		ID:                   p.ID,
		WorkspaceID:          p.WorkspaceID,
		Name:                 p.Name,
		Description:          p.Description,
		Enabled:              p.Enabled,
		TriggerType:          p.TriggerType,
		TriggerConfig:        defaultTriggerConfig(p.TriggerConfig),
		Graph:                p.Graph,
		NotifyOnFailureEmail: p.NotifyOnFailureEmail,
		CreatedBy:            p.CreatedBy,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	r.mapStore.put(wf.ID, wf)
	return wf, nil
}

func (r *WorkflowRepo) Update(_ context.Context, p repository.UpdateWorkflowParams) (repository.Workflow, error) {
	var result repository.Workflow
	err := r.mapStore.mutate("workflow", p.ID, p.WorkspaceID,
		func(wf repository.Workflow) string { return wf.WorkspaceID },
		func(wf repository.Workflow) (repository.Workflow, error) {
			if p.Name != nil {
				wf.Name = *p.Name
			}
			if p.Description != nil {
				wf.Description = *p.Description
			}
			if p.TriggerType != nil {
				wf.TriggerType = *p.TriggerType
			}
			if p.TriggerConfig != nil {
				wf.TriggerConfig = defaultTriggerConfig(*p.TriggerConfig)
			}
			if p.Graph != nil {
				wf.Graph = *p.Graph
			}
			if p.NotifyOnFailureEmail != nil {
				wf.NotifyOnFailureEmail = *p.NotifyOnFailureEmail
			}
			wf.UpdatedAt = time.Now().UTC()
			result = wf
			return wf, nil
		},
	)
	return result, err
}

func (r *WorkflowRepo) FindCoveringWorkflow(
	_ context.Context,
	workspaceID, assetID, assetProjectID, assetFolderID string,
) (*repository.CoveringWorkflow, error) {
	type candidate struct {
		wf    repository.Workflow
		score int
	}
	var candidates []candidate
	for _, wf := range r.mapStore.all() {
		if wf.WorkspaceID != workspaceID || wf.TriggerType != "trigger.version_uploaded" || !wf.Enabled {
			continue
		}
		var cfg struct {
			ProjectID string `json:"project_id"`
			FolderID  string `json:"folder_id"`
			AssetID   string `json:"asset_id"`
		}
		_ = json.Unmarshal([]byte(defaultTriggerConfig(wf.TriggerConfig)), &cfg)
		switch {
		case cfg.AssetID != "" && cfg.AssetID == assetID:
			candidates = append(candidates, candidate{wf: wf, score: 0})
		case cfg.FolderID != "" && cfg.FolderID == assetFolderID:
			candidates = append(candidates, candidate{wf: wf, score: 1})
		case cfg.ProjectID != "" && cfg.ProjectID == assetProjectID:
			candidates = append(candidates, candidate{wf: wf, score: 2})
		case cfg.AssetID == "" && cfg.FolderID == "" && cfg.ProjectID == "":
			candidates = append(candidates, candidate{wf: wf, score: 3})
		}
	}
	if len(candidates) == 0 {
		return nil, apperr.ErrNotFound
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score < candidates[j].score })
	best := candidates[0].wf
	return &repository.CoveringWorkflow{
		ID:            best.ID,
		Name:          best.Name,
		TriggerType:   best.TriggerType,
		TriggerConfig: defaultTriggerConfig(best.TriggerConfig),
		Enabled:       best.Enabled,
	}, nil
}

func (r *WorkflowRepo) SetEnabled(_ context.Context, workspaceID, id string, enabled bool) error {
	return r.mapStore.mutate("workflow", id, workspaceID,
		func(wf repository.Workflow) string { return wf.WorkspaceID },
		func(wf repository.Workflow) (repository.Workflow, error) {
			wf.Enabled = enabled
			wf.UpdatedAt = time.Now().UTC()
			return wf, nil
		},
	)
}

func (r *WorkflowRepo) Delete(_ context.Context, workspaceID, id string) error {
	return r.mapStore.del("workflow", id, workspaceID, func(wf repository.Workflow) string { return wf.WorkspaceID })
}

func (r *WorkflowRepo) TouchLastRunAt(_ context.Context, id string) error {
	r.mapStore.mu.Lock()
	defer r.mapStore.mu.Unlock()
	wf, ok := r.mapStore.items[id]
	if !ok {
		return apperr.ErrNotFound
	}
	now := time.Now().UTC()
	wf.LastRunAt = &now
	wf.UpdatedAt = now
	r.mapStore.items[id] = wf
	return nil
}

func (r *WorkflowRepo) RunInTx(_ context.Context, fn func(repository.WorkflowRepository) error) error {
	return fn(r)
}

func defaultTriggerConfig(v string) string {
	if v == "" {
		return "{}"
	}
	return v
}

// WorkflowRunRepo is a map-backed WorkflowRunRepository for unit tests.
// Keyed by run ID (no workspace scope on lookup), so it does not embed mapStore.
type WorkflowRunRepo struct {
	mu    sync.RWMutex
	runs  map[string]repository.WorkflowRun
	steps map[string]repository.WorkflowRunStep
}

func NewWorkflowRunRepo() *WorkflowRunRepo {
	return &WorkflowRunRepo{
		runs:  map[string]repository.WorkflowRun{},
		steps: map[string]repository.WorkflowRunStep{},
	}
}

func (r *WorkflowRunRepo) GetByID(_ context.Context, id string) (repository.WorkflowRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	run, ok := r.runs[id]
	if !ok {
		return repository.WorkflowRun{}, apperr.ErrNotFound
	}
	return run, nil
}

func (r *WorkflowRunRepo) List(
	_ context.Context,
	workspaceID, workflowID string,
	limit int,
	cursor string,
) ([]repository.WorkflowRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.WorkflowRun
	for _, run := range r.runs {
		if run.WorkspaceID != workspaceID {
			continue
		}
		if workflowID != "" && run.WorkflowID != workflowID {
			continue
		}
		if cursor != "" && run.CreatedAt.Format(time.RFC3339Nano)+"|"+run.ID >= cursor {
			continue
		}
		out = append(out, run)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *WorkflowRunRepo) Create(
	_ context.Context,
	p repository.CreateWorkflowRunParams,
) (repository.WorkflowRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run := repository.WorkflowRun{
		ID:          p.ID,
		WorkflowID:  p.WorkflowID,
		WorkspaceID: p.WorkspaceID,
		Status:      p.Status,
		TriggerData: p.TriggerData,
		Context:     p.Context,
		Error:       p.Error,
		StartedAt:   p.StartedAt,
		CompletedAt: p.CompletedAt,
		CreatedAt:   time.Now().UTC(),
	}
	r.runs[run.ID] = run
	return run, nil
}

func (r *WorkflowRunRepo) SetStatus(_ context.Context, id, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return apperr.ErrNotFound
	}
	run.Status = status
	if status == "running" {
		now := time.Now().UTC()
		run.StartedAt = &now
	}
	r.runs[id] = run
	return nil
}

func (r *WorkflowRunRepo) SetFinal(_ context.Context, p repository.SetWorkflowRunFinalParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[p.ID]
	if !ok {
		return apperr.ErrNotFound
	}
	run.Status = p.Status
	run.Context = p.Context
	run.Error = p.Error
	if p.CompletedAt != nil {
		run.CompletedAt = p.CompletedAt
	}
	r.runs[p.ID] = run
	return nil
}

func (r *WorkflowRunRepo) ListSteps(_ context.Context, runID string) ([]repository.WorkflowRunStep, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.WorkflowRunStep
	for _, step := range r.steps {
		if step.RunID == runID {
			out = append(out, step)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(*out[j].StartedAt) })
	return out, nil
}

func (r *WorkflowRunRepo) CreateStep(
	_ context.Context,
	p repository.CreateWorkflowRunStepParams,
) (repository.WorkflowRunStep, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	started := p.StartedAt
	if started == nil {
		now := time.Now().UTC()
		started = &now
	}
	step := repository.WorkflowRunStep{
		ID:          p.ID,
		RunID:       p.RunID,
		NodeID:      p.NodeID,
		NodeType:    p.NodeType,
		Status:      p.Status,
		Attempt:     p.Attempt,
		InputCtx:    p.InputCtx,
		OutputCtx:   p.OutputCtx,
		Error:       p.Error,
		StartedAt:   started,
		CompletedAt: p.CompletedAt,
	}
	r.steps[step.ID] = step
	return step, nil
}

func (r *WorkflowRunRepo) SetStepStatus(_ context.Context, id, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	step, ok := r.steps[id]
	if !ok {
		return apperr.ErrNotFound
	}
	step.Status = status
	r.steps[id] = step
	return nil
}

func (r *WorkflowRunRepo) SetStepFailed(_ context.Context, id, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	step, ok := r.steps[id]
	if !ok {
		return apperr.ErrNotFound
	}
	now := time.Now().UTC()
	step.Status = "failed"
	step.Error = &errMsg
	step.CompletedAt = &now
	r.steps[id] = step
	return nil
}

func (r *WorkflowRunRepo) SetStepCompleted(_ context.Context, id, outputCtx string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	step, ok := r.steps[id]
	if !ok {
		return apperr.ErrNotFound
	}
	now := time.Now().UTC()
	step.Status = "completed"
	step.OutputCtx = &outputCtx
	step.CompletedAt = &now
	r.steps[id] = step
	return nil
}

func (r *WorkflowRunRepo) IncrementStepAttempt(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	step, ok := r.steps[id]
	if !ok {
		return apperr.ErrNotFound
	}
	step.Attempt++
	r.steps[id] = step
	return nil
}

// WorkflowWebhookRepo is a map-backed WorkflowWebhookRepository for unit tests.
// Keyed by workflowID (not an ID+workspaceID pair), so it does not embed mapStore.
type WorkflowWebhookRepo struct {
	mu     sync.RWMutex
	tokens map[string]string
}

func NewWorkflowWebhookRepo() *WorkflowWebhookRepo {
	return &WorkflowWebhookRepo{tokens: map[string]string{}}
}

func (r *WorkflowWebhookRepo) GetTokenHash(_ context.Context, workflowID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tokenHash, ok := r.tokens[workflowID]
	if !ok {
		return "", apperr.ErrNotFound
	}
	return tokenHash, nil
}

func (r *WorkflowWebhookRepo) Upsert(_ context.Context, workflowID, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[workflowID] = tokenHash
	return nil
}

func (r *WorkflowWebhookRepo) Delete(_ context.Context, workflowID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tokens, workflowID)
	return nil
}
