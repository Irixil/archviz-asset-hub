package reposqlc

import (
	"context"

	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type auditLogRepo struct {
	d *db.DB
}

// NewAuditLogRepo returns a repository.AuditLogRepository backed by sqlc-generated queries.
func NewAuditLogRepo(d *db.DB) repository.AuditLogRepository {
	return &auditLogRepo{d: d}
}

func anyArg(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *auditLogRepo) ListAssetEvents(
	ctx context.Context,
	params repository.ListAuditEventsParams,
) ([]repository.AuditEvent, error) {
	rows, err := r.d.RQ.ListAssetEvents(ctx, dbgen.ListAssetEventsParams{
		AssetID:     params.EntityID,
		WorkspaceID: params.WorkspaceID,
		Cursor:      anyArg(params.Cursor),
		EventType:   anyArg(params.EventType),
		Limit:       params.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.AuditEvent, len(rows))
	for i, row := range rows {
		out[i] = repository.AuditEvent{
			ID:        row.ID,
			EntityID:  row.AssetID,
			EventType: row.EventType,
			Payload:   row.Payload,
			CreatedAt: row.CreatedAt,
			UserID:    row.UserID,
			UserName:  row.UserName,
			ActorType: row.ActorType,
		}
	}
	return out, nil
}

func (r *auditLogRepo) ListProjectEvents(
	ctx context.Context,
	params repository.ListAuditEventsParams,
) ([]repository.AuditEvent, error) {
	rows, err := r.d.RQ.ListProjectEvents(ctx, dbgen.ListProjectEventsParams{
		ProjectID:   params.EntityID,
		WorkspaceID: params.WorkspaceID,
		Cursor:      anyArg(params.Cursor),
		EventType:   anyArg(params.EventType),
		Limit:       params.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.AuditEvent, len(rows))
	for i, row := range rows {
		out[i] = repository.AuditEvent{
			ID:        row.ID,
			EntityID:  row.ProjectID,
			EventType: row.EventType,
			Payload:   row.Payload,
			CreatedAt: row.CreatedAt,
			UserID:    row.UserID,
			UserName:  row.UserName,
			ActorType: row.ActorType,
		}
	}
	return out, nil
}

//nolint:dupl // Asset and project event listing differ only by sqlc-generated row and param types.
func (r *auditLogRepo) ListWorkspaceAssetEvents(
	ctx context.Context,
	params repository.ListAuditEventsParams,
) ([]repository.AuditEvent, error) {
	rows, err := r.d.RQ.ListWorkspaceAssetEvents(ctx, dbgen.ListWorkspaceAssetEventsParams{
		WorkspaceID: params.WorkspaceID,
		Cursor:      anyArg(params.Cursor),
		UserID:      anyArg(params.UserID),
		EventType:   anyArg(params.EventType),
		Limit:       params.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.AuditEvent, len(rows))
	for i, row := range rows {
		out[i] = repository.AuditEvent{
			ID:        row.ID,
			EntityID:  row.AssetID,
			EventType: row.EventType,
			Payload:   row.Payload,
			CreatedAt: row.CreatedAt,
			UserID:    row.UserID,
			UserName:  row.UserName,
			ActorType: row.ActorType,
		}
	}
	return out, nil
}

//nolint:dupl // Asset and project event listing differ only by sqlc-generated row and param types.
func (r *auditLogRepo) ListWorkspaceProjectEvents(
	ctx context.Context,
	params repository.ListAuditEventsParams,
) ([]repository.AuditEvent, error) {
	rows, err := r.d.RQ.ListWorkspaceProjectEvents(ctx, dbgen.ListWorkspaceProjectEventsParams{
		WorkspaceID: params.WorkspaceID,
		Cursor:      anyArg(params.Cursor),
		UserID:      anyArg(params.UserID),
		EventType:   anyArg(params.EventType),
		Limit:       params.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.AuditEvent, len(rows))
	for i, row := range rows {
		out[i] = repository.AuditEvent{
			ID:        row.ID,
			EntityID:  row.ProjectID,
			EventType: row.EventType,
			Payload:   row.Payload,
			CreatedAt: row.CreatedAt,
			UserID:    row.UserID,
			UserName:  row.UserName,
			ActorType: row.ActorType,
		}
	}
	return out, nil
}
