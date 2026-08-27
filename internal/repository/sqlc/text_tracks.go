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

type textTrackRepo struct {
	d *db.DB
}

// NewTextTrackRepo returns a repository.TextTrackRepository backed by sqlc-generated queries.
func NewTextTrackRepo(d *db.DB) repository.TextTrackRepository {
	return &textTrackRepo{d: d}
}

func (r *textTrackRepo) List(ctx context.Context, workspaceID, assetID string) ([]repository.TextTrack, error) {
	rows, err := r.d.RQ.ListTextTracksByAsset(ctx, dbgen.ListTextTracksByAssetParams{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.TextTrack, len(rows))
	for i, row := range rows {
		out[i] = toTextTrack(row)
	}
	return out, nil
}

func (r *textTrackRepo) Get(ctx context.Context, workspaceID, id string) (repository.TextTrack, error) {
	row, err := r.d.RQ.GetTextTrack(ctx, dbgen.GetTextTrackParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.TextTrack{}, apperr.ErrNotFound
		}
		return repository.TextTrack{}, err
	}
	return toTextTrack(row), nil
}

func (r *textTrackRepo) Create(
	ctx context.Context,
	params repository.CreateTextTrackParams,
) (repository.TextTrack, error) {
	row, err := r.d.WQ.CreateTextTrack(ctx, dbgen.CreateTextTrackParams{
		ID:             params.ID,
		WorkspaceID:    params.WorkspaceID,
		AssetID:        params.AssetID,
		AssetVersionID: params.AssetVersionID,
		Source:         params.Source,
		Lang:           params.Lang,
		Content:        params.Content,
		Meta:           params.Meta,
		Status:         params.Status,
		CreatedBy:      params.CreatedBy,
	})
	if err != nil {
		return repository.TextTrack{}, err
	}
	return toTextTrack(row), nil
}

func (r *textTrackRepo) SetReady(
	ctx context.Context,
	workspaceID, id string,
	params repository.SetTextTrackReadyParams,
) error {
	return r.d.WQ.SetTextTrackReady(ctx, dbgen.SetTextTrackReadyParams{
		Content:     params.Content,
		StorageKey:  params.StorageKey,
		ContentType: params.ContentType,
		Meta:        params.Meta,
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (r *textTrackRepo) SetFailed(ctx context.Context, workspaceID, id, errMsg string) error {
	return r.d.WQ.SetTextTrackFailed(ctx, dbgen.SetTextTrackFailedParams{
		ID:          id,
		WorkspaceID: workspaceID,
		Error:       &errMsg,
	})
}

func (r *textTrackRepo) Delete(ctx context.Context, workspaceID, id string) error {
	return r.d.WQ.DeleteTextTrack(ctx, dbgen.DeleteTextTrackParams{ID: id, WorkspaceID: workspaceID})
}

func (r *textTrackRepo) InsertFTS(ctx context.Context, params repository.InsertTextFTSParams) error {
	return r.d.WQ.InsertTextFTS(ctx, dbgen.InsertTextFTSParams{
		TrackID:     params.TrackID,
		AssetID:     params.AssetID,
		WorkspaceID: params.WorkspaceID,
		Source:      params.Source,
		Lang:        params.Lang,
		Content:     params.Content,
	})
}

func (r *textTrackRepo) DeleteFTS(ctx context.Context, trackID string) error {
	return r.d.WQ.DeleteTextFTS(ctx, trackID)
}

func toTextTrack(row dbgen.AssetTextTrack) repository.TextTrack {
	return repository.TextTrack{
		ID:             row.ID,
		WorkspaceID:    row.WorkspaceID,
		AssetID:        row.AssetID,
		AssetVersionID: row.AssetVersionID,
		Source:         row.Source,
		Lang:           row.Lang,
		Content:        row.Content,
		StorageKey:     row.StorageKey,
		ContentType:    row.ContentType,
		Meta:           row.Meta,
		Status:         row.Status,
		Error:          row.Error,
		CreatedBy:      row.CreatedBy,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}
