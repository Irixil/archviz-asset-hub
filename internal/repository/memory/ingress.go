package memory

import (
	"context"
	"sort"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// IngressMemoryRepo is an in-memory implementation of IngressRepository.
type IngressMemoryRepo struct {
	mu      sync.RWMutex
	sources map[string]repository.IngressSource
	rules   map[string]repository.IngressRule
	log     map[string]repository.IngressLogEntry
}

// NewIngressRepo creates a new in-memory IngressRepository.
func NewIngressRepo() *IngressMemoryRepo {
	return &IngressMemoryRepo{
		sources: map[string]repository.IngressSource{},
		rules:   map[string]repository.IngressRule{},
		log:     map[string]repository.IngressLogEntry{},
	}
}

func (r *IngressMemoryRepo) SeedSource(s repository.IngressSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[s.ID] = s
}

func (r *IngressMemoryRepo) SeedRule(rule repository.IngressRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[rule.ID] = rule
}

func (r *IngressMemoryRepo) SeedLogEntry(e repository.IngressLogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log[e.ID] = e
}

// -- Sources --

func (r *IngressMemoryRepo) ListSources(_ context.Context, workspaceID string) ([]repository.IngressSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.IngressSource{}
	for _, s := range r.sources {
		if s.WorkspaceID == workspaceID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *IngressMemoryRepo) GetSource(_ context.Context, workspaceID, id string) (repository.IngressSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sources[id]
	if !ok || s.WorkspaceID != workspaceID {
		return repository.IngressSource{}, apperr.ErrNotFound
	}
	return s, nil
}

func (r *IngressMemoryRepo) CreateSource(
	_ context.Context,
	params repository.CreateIngressSourceParams,
) (repository.IngressSource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := repository.IngressSource{
		ID:              params.ID,
		WorkspaceID:     params.WorkspaceID,
		CreatedBy:       params.CreatedBy,
		Type:            params.Type,
		Label:           params.Label,
		Config:          params.Config,
		PublicToken:     params.PublicToken,
		DestFolderID:    params.DestFolderID,
		DestProjectID:   params.DestProjectID,
		Enabled:         params.Enabled,
		PollIntervalMin: params.PollIntervalMin,
	}
	r.sources[s.ID] = s
	return s, nil
}

func (r *IngressMemoryRepo) UpdateSource(
	_ context.Context,
	params repository.UpdateIngressSourceParams,
) (repository.IngressSource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sources[params.ID]
	if !ok || s.WorkspaceID != params.WorkspaceID {
		return repository.IngressSource{}, apperr.ErrNotFound
	}
	s.Label = params.Label
	s.Config = params.Config
	s.DestFolderID = params.DestFolderID
	s.DestProjectID = params.DestProjectID
	s.Enabled = params.Enabled
	s.PollIntervalMin = params.PollIntervalMin
	s.ErrorCount = 0
	r.sources[s.ID] = s
	return s, nil
}

func (r *IngressMemoryRepo) DeleteSource(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sources[id]
	if !ok || s.WorkspaceID != workspaceID {
		return apperr.ErrNotFound
	}
	delete(r.sources, id)
	return nil
}

// -- Rules --

func (r *IngressMemoryRepo) ListRules(
	_ context.Context,
	workspaceID, sourceID string,
) ([]repository.IngressRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.sources[sourceID]; !ok || s.WorkspaceID != workspaceID {
		return nil, apperr.ErrNotFound
	}
	out := []repository.IngressRule{}
	for _, rule := range r.rules {
		if rule.SourceID == sourceID {
			out = append(out, rule)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}

func (r *IngressMemoryRepo) GetRule(_ context.Context, workspaceID, id string) (repository.IngressRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rule, ok := r.rules[id]
	if !ok || !r.ruleInWorkspace(rule, workspaceID) {
		return repository.IngressRule{}, apperr.ErrNotFound
	}
	return rule, nil
}

func (r *IngressMemoryRepo) CreateRule(
	_ context.Context,
	params repository.CreateIngressRuleParams,
) (repository.IngressRule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sources[params.SourceID]; !ok || s.WorkspaceID != params.WorkspaceID {
		return repository.IngressRule{}, apperr.ErrNotFound
	}
	rule := repository.IngressRule{
		ID:       params.ID,
		SourceID: params.SourceID,
		Position: params.Position,
		Field:    params.Field,
		Operator: params.Operator,
		Value:    params.Value,
		Action:   params.Action,
	}
	r.rules[rule.ID] = rule
	return rule, nil
}

func (r *IngressMemoryRepo) UpdateRule(
	_ context.Context,
	workspaceID string,
	params repository.UpdateIngressRuleParams,
) (repository.IngressRule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rule, ok := r.rules[params.ID]
	if !ok || !r.ruleInWorkspace(rule, workspaceID) {
		return repository.IngressRule{}, apperr.ErrNotFound
	}
	rule.Position = params.Position
	rule.Field = params.Field
	rule.Operator = params.Operator
	rule.Value = params.Value
	rule.Action = params.Action
	r.rules[rule.ID] = rule
	return rule, nil
}

func (r *IngressMemoryRepo) DeleteRule(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rule, ok := r.rules[id]
	if !ok || !r.ruleInWorkspace(rule, workspaceID) {
		return apperr.ErrNotFound
	}
	delete(r.rules, id)
	return nil
}

// -- Log --

func (r *IngressMemoryRepo) ListSourceLog(
	_ context.Context,
	workspaceID, sourceID string,
	limit, offset int64,
) ([]repository.IngressLogEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.sources[sourceID]; !ok || s.WorkspaceID != workspaceID {
		return nil, apperr.ErrNotFound
	}
	out := []repository.IngressLogEntry{}
	for _, e := range r.log {
		if e.SourceID == sourceID {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ImportedAt.After(out[j].ImportedAt) })
	return paginateLog(out, limit, offset), nil
}

func (r *IngressMemoryRepo) ListWorkspaceLog(
	_ context.Context,
	workspaceID, status string,
	limit, offset int64,
) ([]repository.IngressLogEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.IngressLogEntry{}
	for _, e := range r.log {
		s, ok := r.sources[e.SourceID]
		if !ok || s.WorkspaceID != workspaceID {
			continue
		}
		if status != "" && e.Status != status {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ImportedAt.After(out[j].ImportedAt) })
	return paginateLog(out, limit, offset), nil
}

func (r *IngressMemoryRepo) GetLogEntry(
	_ context.Context,
	workspaceID, id string,
) (repository.IngressLogEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.log[id]
	if !ok || !r.logEntryInWorkspace(e, workspaceID) {
		return repository.IngressLogEntry{}, apperr.ErrNotFound
	}
	return e, nil
}

func (r *IngressMemoryRepo) UpdateLogEntry(
	_ context.Context,
	workspaceID string,
	params repository.UpdateIngressLogEntryParams,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.log[params.ID]
	if !ok || !r.logEntryInWorkspace(e, workspaceID) {
		return apperr.ErrNotFound
	}
	e.Status = params.Status
	e.AssetID = params.AssetID
	e.Error = params.Error
	r.log[e.ID] = e
	return nil
}

func (r *IngressMemoryRepo) DeleteLogEntry(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.log[id]
	if !ok || !r.logEntryInWorkspace(e, workspaceID) {
		return apperr.ErrNotFound
	}
	delete(r.log, id)
	return nil
}

// ruleInWorkspace reports whether rule's parent source belongs to workspaceID.
// Callers must hold r.mu.
func (r *IngressMemoryRepo) ruleInWorkspace(rule repository.IngressRule, workspaceID string) bool {
	s, ok := r.sources[rule.SourceID]
	return ok && s.WorkspaceID == workspaceID
}

// logEntryInWorkspace reports whether e's parent source belongs to workspaceID.
// Callers must hold r.mu.
func (r *IngressMemoryRepo) logEntryInWorkspace(e repository.IngressLogEntry, workspaceID string) bool {
	s, ok := r.sources[e.SourceID]
	return ok && s.WorkspaceID == workspaceID
}

func paginateLog(entries []repository.IngressLogEntry, limit, offset int64) []repository.IngressLogEntry {
	if offset >= int64(len(entries)) {
		return []repository.IngressLogEntry{}
	}
	entries = entries[offset:]
	if limit > 0 && int64(len(entries)) > limit {
		entries = entries[:limit]
	}
	return entries
}
