package memory

import (
	"context"
	"sort"
	"sync"

	"damask/server/internal/repository"
)

// AuditLogMemoryRepo is an in-memory implementation of AuditLogRepository.
type AuditLogMemoryRepo struct {
	mu          sync.RWMutex
	assetEvents []repository.AuditEvent
	assetWS     map[string]string // event ID -> workspace ID
	projEvents  []repository.AuditEvent
	projWS      map[string]string // event ID -> workspace ID
}

// NewAuditLogRepo creates a new in-memory AuditLogRepository.
func NewAuditLogRepo() *AuditLogMemoryRepo {
	return &AuditLogMemoryRepo{
		assetWS: map[string]string{},
		projWS:  map[string]string{},
	}
}

// SeedAssetEvent adds an asset event scoped to workspaceID (EntityID is the asset ID).
func (r *AuditLogMemoryRepo) SeedAssetEvent(workspaceID string, e repository.AuditEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assetEvents = append(r.assetEvents, e)
	r.assetWS[e.ID] = workspaceID
}

// SeedProjectEvent adds a project event scoped to workspaceID (EntityID is the project ID).
func (r *AuditLogMemoryRepo) SeedProjectEvent(workspaceID string, e repository.AuditEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.projEvents = append(r.projEvents, e)
	r.projWS[e.ID] = workspaceID
}

func filterEvents(
	events []repository.AuditEvent,
	inScope func(repository.AuditEvent) bool,
	params repository.ListAuditEventsParams,
) []repository.AuditEvent {
	out := []repository.AuditEvent{}
	for _, e := range events {
		if !inScope(e) {
			continue
		}
		if params.Cursor != "" && e.CreatedAt >= params.Cursor {
			continue
		}
		if params.EventType != "" && e.EventType != params.EventType {
			continue
		}
		if params.UserID != "" && (e.UserID == nil || *e.UserID != params.UserID) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if params.Limit > 0 && int64(len(out)) > params.Limit {
		out = out[:params.Limit]
	}
	return out
}

func (r *AuditLogMemoryRepo) ListAssetEvents(
	_ context.Context,
	params repository.ListAuditEventsParams,
) ([]repository.AuditEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return filterEvents(r.assetEvents, func(e repository.AuditEvent) bool {
		return e.EntityID == params.EntityID && r.assetWS[e.ID] == params.WorkspaceID
	}, params), nil
}

func (r *AuditLogMemoryRepo) ListProjectEvents(
	_ context.Context,
	params repository.ListAuditEventsParams,
) ([]repository.AuditEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return filterEvents(r.projEvents, func(e repository.AuditEvent) bool {
		return e.EntityID == params.EntityID && r.projWS[e.ID] == params.WorkspaceID
	}, params), nil
}

func (r *AuditLogMemoryRepo) ListWorkspaceAssetEvents(
	_ context.Context,
	params repository.ListAuditEventsParams,
) ([]repository.AuditEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return filterEvents(r.assetEvents, func(e repository.AuditEvent) bool {
		return r.assetWS[e.ID] == params.WorkspaceID
	}, params), nil
}

func (r *AuditLogMemoryRepo) ListWorkspaceProjectEvents(
	_ context.Context,
	params repository.ListAuditEventsParams,
) ([]repository.AuditEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return filterEvents(r.projEvents, func(e repository.AuditEvent) bool {
		return r.projWS[e.ID] == params.WorkspaceID
	}, params), nil
}
