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

type autoTagSuggestionRepo struct {
	d *db.DB
}

// NewAutoTagSuggestionRepo returns a repository.AutoTagSuggestionRepository backed by sqlc-generated queries.
func NewAutoTagSuggestionRepo(d *db.DB) repository.AutoTagSuggestionRepository {
	return &autoTagSuggestionRepo{d: d}
}

func (r *autoTagSuggestionRepo) List(
	ctx context.Context,
	workspaceID, assetID string,
) ([]repository.AutoTagSuggestion, error) {
	rows, err := r.d.RQ.ListAutoTagSuggestions(ctx, dbgen.ListAutoTagSuggestionsParams{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.AutoTagSuggestion, len(rows))
	for i, row := range rows {
		out[i] = toAutoTagSuggestion(row)
	}
	return out, nil
}

func (r *autoTagSuggestionRepo) Get(
	ctx context.Context,
	workspaceID, id string,
) (repository.AutoTagSuggestion, error) {
	row, err := r.d.RQ.GetAutoTagSuggestion(ctx, dbgen.GetAutoTagSuggestionParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AutoTagSuggestion{}, apperr.ErrNotFound
		}
		return repository.AutoTagSuggestion{}, err
	}
	return toAutoTagSuggestion(row), nil
}

func (r *autoTagSuggestionRepo) Create(
	ctx context.Context,
	params repository.CreateAutoTagSuggestionParams,
) (repository.AutoTagSuggestion, error) {
	row, err := r.d.WQ.CreateAutoTagSuggestion(ctx, dbgen.CreateAutoTagSuggestionParams{
		ID:             params.ID,
		WorkspaceID:    params.WorkspaceID,
		AssetID:        params.AssetID,
		AssetVersionID: params.AssetVersionID,
		TagName:        params.TagName,
	})
	if err != nil {
		return repository.AutoTagSuggestion{}, err
	}
	return toAutoTagSuggestion(row), nil
}

func (r *autoTagSuggestionRepo) Delete(ctx context.Context, workspaceID, id string) error {
	return r.d.WQ.DeleteAutoTagSuggestion(ctx, dbgen.DeleteAutoTagSuggestionParams{ID: id, WorkspaceID: workspaceID})
}

func (r *autoTagSuggestionRepo) DeleteByAsset(ctx context.Context, workspaceID, assetID string) error {
	return r.d.WQ.DeleteAutoTagSuggestionsByAsset(ctx, dbgen.DeleteAutoTagSuggestionsByAssetParams{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
	})
}

func toAutoTagSuggestion(row dbgen.AutoTagSuggestion) repository.AutoTagSuggestion {
	return repository.AutoTagSuggestion{
		ID:             row.ID,
		WorkspaceID:    row.WorkspaceID,
		AssetID:        row.AssetID,
		AssetVersionID: row.AssetVersionID,
		TagName:        row.TagName,
		CreatedAt:      row.CreatedAt,
	}
}
