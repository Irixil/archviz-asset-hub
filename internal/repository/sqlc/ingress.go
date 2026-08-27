package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"damask/server/internal/apperr"
	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type ingressRepo struct {
	d *db.DB
}

// NewIngressRepo returns a repository.IngressRepository backed by sqlc-generated queries.
func NewIngressRepo(d *db.DB) repository.IngressRepository {
	return &ingressRepo{d: d}
}

func (r *ingressRepo) ListSources(ctx context.Context, workspaceID string) ([]repository.IngressSource, error) {
	rows, err := r.d.RQ.ListIngressSources(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.IngressSource, len(rows))
	for i, row := range rows {
		out[i] = toIngressSource(row)
	}
	return out, nil
}

func (r *ingressRepo) GetSource(ctx context.Context, workspaceID, id string) (repository.IngressSource, error) {
	row, err := r.d.RQ.GetIngressSource(ctx, dbgen.GetIngressSourceParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.IngressSource{}, apperr.ErrNotFound
		}
		return repository.IngressSource{}, err
	}
	return toIngressSource(row), nil
}

func (r *ingressRepo) CreateSource(
	ctx context.Context,
	params repository.CreateIngressSourceParams,
) (repository.IngressSource, error) {
	row, err := r.d.WQ.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID:              params.ID,
		WorkspaceID:     params.WorkspaceID,
		CreatedBy:       params.CreatedBy,
		Type:            params.Type,
		Label:           params.Label,
		Config:          params.Config,
		PublicToken:     params.PublicToken,
		DestFolderID:    params.DestFolderID,
		DestProjectID:   params.DestProjectID,
		Enabled:         boolToInt64(params.Enabled),
		PollIntervalMin: params.PollIntervalMin,
	})
	if err != nil {
		return repository.IngressSource{}, err
	}
	return toIngressSource(row), nil
}

func (r *ingressRepo) UpdateSource(
	ctx context.Context,
	params repository.UpdateIngressSourceParams,
) (repository.IngressSource, error) {
	row, err := r.d.WQ.UpdateIngressSource(ctx, dbgen.UpdateIngressSourceParams{
		Label:           params.Label,
		Config:          params.Config,
		DestFolderID:    params.DestFolderID,
		DestProjectID:   params.DestProjectID,
		Enabled:         boolToInt64(params.Enabled),
		PollIntervalMin: params.PollIntervalMin,
		ID:              params.ID,
		WorkspaceID:     params.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.IngressSource{}, apperr.ErrNotFound
		}
		return repository.IngressSource{}, err
	}
	return toIngressSource(row), nil
}

func (r *ingressRepo) DeleteSource(ctx context.Context, workspaceID, id string) error {
	return r.d.WQ.DeleteIngressSource(ctx, dbgen.DeleteIngressSourceParams{ID: id, WorkspaceID: workspaceID})
}

func (r *ingressRepo) ListRules(ctx context.Context, workspaceID, sourceID string) ([]repository.IngressRule, error) {
	rows, err := r.d.RQ.ListIngressRulesForWorkspace(ctx, dbgen.ListIngressRulesForWorkspaceParams{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.IngressRule, len(rows))
	for i, row := range rows {
		out[i] = toIngressRule(row)
	}
	return out, nil
}

func (r *ingressRepo) GetRule(ctx context.Context, workspaceID, id string) (repository.IngressRule, error) {
	row, err := r.d.RQ.GetIngressRuleForWorkspace(ctx, dbgen.GetIngressRuleForWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.IngressRule{}, apperr.ErrNotFound
		}
		return repository.IngressRule{}, err
	}
	return toIngressRule(row), nil
}

func (r *ingressRepo) CreateRule(
	ctx context.Context,
	params repository.CreateIngressRuleParams,
) (repository.IngressRule, error) {
	row, err := r.d.WQ.CreateIngressRuleForWorkspace(ctx, dbgen.CreateIngressRuleForWorkspaceParams{
		ID:          params.ID,
		SourceID:    params.SourceID,
		Position:    params.Position,
		Field:       params.Field,
		Operator:    params.Operator,
		Value:       params.Value,
		Action:      params.Action,
		WorkspaceID: params.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.IngressRule{}, apperr.ErrNotFound
		}
		return repository.IngressRule{}, err
	}
	return toIngressRule(row), nil
}

func (r *ingressRepo) UpdateRule(
	ctx context.Context,
	workspaceID string,
	params repository.UpdateIngressRuleParams,
) (repository.IngressRule, error) {
	row, err := r.d.WQ.UpdateIngressRuleForWorkspace(ctx, dbgen.UpdateIngressRuleForWorkspaceParams{
		Position:    params.Position,
		Field:       params.Field,
		Operator:    params.Operator,
		Value:       params.Value,
		Action:      params.Action,
		ID:          params.ID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.IngressRule{}, apperr.ErrNotFound
		}
		return repository.IngressRule{}, err
	}
	return toIngressRule(row), nil
}

func (r *ingressRepo) DeleteRule(ctx context.Context, workspaceID, id string) error {
	rows, err := r.d.WQ.DeleteIngressRuleForWorkspace(ctx, dbgen.DeleteIngressRuleForWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func (r *ingressRepo) ListSourceLog(
	ctx context.Context,
	workspaceID, sourceID string,
	limit, offset int64,
) ([]repository.IngressLogEntry, error) {
	rows, err := r.d.RQ.ListIngressSourceLogForWorkspace(ctx, dbgen.ListIngressSourceLogForWorkspaceParams{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.IngressLogEntry, len(rows))
	for i, row := range rows {
		out[i] = toIngressLogEntry(row)
	}
	return out, nil
}

func (r *ingressRepo) ListWorkspaceLog(
	ctx context.Context,
	workspaceID, status string,
	limit, offset int64,
) ([]repository.IngressLogEntry, error) {
	var statusArg any
	if status != "" {
		statusArg = status
	}
	rows, err := r.d.RQ.ListWorkspaceIngressLog(ctx, dbgen.ListWorkspaceIngressLogParams{
		WorkspaceID: workspaceID,
		Status:      statusArg,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.IngressLogEntry, len(rows))
	for i, row := range rows {
		out[i] = toIngressLogEntry(row)
	}
	return out, nil
}

func (r *ingressRepo) GetLogEntry(ctx context.Context, workspaceID, id string) (repository.IngressLogEntry, error) {
	row, err := r.d.RQ.GetIngressLogEntryForWorkspace(ctx, dbgen.GetIngressLogEntryForWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.IngressLogEntry{}, apperr.ErrNotFound
		}
		return repository.IngressLogEntry{}, err
	}
	return toIngressLogEntry(row), nil
}

func (r *ingressRepo) UpdateLogEntry(
	ctx context.Context,
	workspaceID string,
	params repository.UpdateIngressLogEntryParams,
) error {
	rows, err := r.d.WQ.UpdateIngressLogEntryForWorkspace(ctx, dbgen.UpdateIngressLogEntryForWorkspaceParams{
		Status:      params.Status,
		AssetID:     params.AssetID,
		Error:       params.Error,
		ID:          params.ID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func (r *ingressRepo) DeleteLogEntry(ctx context.Context, workspaceID, id string) error {
	rows, err := r.d.WQ.DeleteIngressLogEntryForWorkspace(ctx, dbgen.DeleteIngressLogEntryForWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func toIngressSource(src dbgen.IngressSource) repository.IngressSource {
	return repository.IngressSource{
		ID:              src.ID,
		WorkspaceID:     src.WorkspaceID,
		CreatedBy:       src.CreatedBy,
		Type:            src.Type,
		Label:           src.Label,
		Config:          src.Config,
		PublicToken:     src.PublicToken,
		DestFolderID:    src.DestFolderID,
		DestProjectID:   src.DestProjectID,
		Enabled:         src.Enabled != 0,
		PollIntervalMin: src.PollIntervalMin,
		LastPolledAt:    src.LastPolledAt,
		LastError:       src.LastError,
		ErrorCount:      src.ErrorCount,
		CreatedAt:       src.CreatedAt,
		UpdatedAt:       src.UpdatedAt,
	}
}

func toIngressRule(r dbgen.IngressRule) repository.IngressRule {
	return repository.IngressRule{
		ID:       r.ID,
		SourceID: r.SourceID,
		Position: r.Position,
		Field:    r.Field,
		Operator: r.Operator,
		Value:    r.Value,
		Action:   r.Action,
	}
}

func toIngressLogEntry(e dbgen.IngressLog) repository.IngressLogEntry {
	return repository.IngressLogEntry{
		ID:         e.ID,
		SourceID:   e.SourceID,
		RemoteID:   e.RemoteID,
		Filename:   e.Filename,
		AssetID:    e.AssetID,
		Status:     e.Status,
		Error:      e.Error,
		ImportedAt: e.ImportedAt,
	}
}
