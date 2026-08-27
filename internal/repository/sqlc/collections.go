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

type collectionRepo struct {
	d *db.DB
}

// NewCollectionRepo returns a repository.CollectionRepository backed by sqlc-generated queries.
func NewCollectionRepo(d *db.DB) repository.CollectionRepository {
	return &collectionRepo{d: d}
}

func (r *collectionRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Collection, error) {
	row, err := r.d.RQ.GetCollection(ctx, dbgen.GetCollectionParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Collection{}, apperr.ErrNotFound
		}
		return repository.Collection{}, err
	}
	col := toCollection(row)
	count, err := r.CountAssets(ctx, id)
	if err != nil {
		return repository.Collection{}, err
	}
	col.AssetCount = count
	return col, nil
}

func (r *collectionRepo) List(ctx context.Context, workspaceID string) ([]repository.Collection, error) {
	rows, err := r.d.RQ.ListCollections(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.Collection, len(rows))
	for i, row := range rows {
		out[i] = repository.Collection{
			ID:          row.ID,
			WorkspaceID: row.WorkspaceID,
			Name:        row.Name,
			Description: row.Description,
			CreatedBy:   row.CreatedBy,
			AssetCount:  row.AssetCount,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}
	}
	return out, nil
}

func (r *collectionRepo) Create(ctx context.Context, c repository.Collection) (repository.Collection, error) {
	row, err := r.d.WQ.CreateCollection(ctx, dbgen.CreateCollectionParams{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Description: c.Description,
		CreatedBy:   c.CreatedBy,
	})
	if err != nil {
		return repository.Collection{}, err
	}
	return toCollection(row), nil
}

func (r *collectionRepo) Update(ctx context.Context, c repository.Collection) (repository.Collection, error) {
	row, err := r.d.WQ.UpdateCollection(ctx, dbgen.UpdateCollectionParams{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Description: c.Description,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Collection{}, apperr.ErrNotFound
		}
		return repository.Collection{}, err
	}
	col := toCollection(row)
	count, err := r.CountAssets(ctx, c.ID)
	if err != nil {
		return repository.Collection{}, err
	}
	col.AssetCount = count
	return col, nil
}

func (r *collectionRepo) Delete(ctx context.Context, workspaceID, id string) error {
	return r.d.WQ.DeleteCollection(ctx, dbgen.DeleteCollectionParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (r *collectionRepo) AddAsset(ctx context.Context, collectionID, assetID string) error {
	return r.d.WQ.AddCollectionAsset(ctx, dbgen.AddCollectionAssetParams{
		CollectionID:   collectionID,
		AssetID:        assetID,
		CollectionID_2: collectionID,
	})
}

func (r *collectionRepo) RemoveAsset(ctx context.Context, collectionID, assetID string) error {
	return r.d.WQ.RemoveCollectionAsset(ctx, dbgen.RemoveCollectionAssetParams{
		CollectionID: collectionID,
		AssetID:      assetID,
	})
}

func (r *collectionRepo) ListForAsset(
	ctx context.Context,
	workspaceID, assetID string,
) ([]repository.Collection, error) {
	rows, err := r.d.RQ.ListCollectionsForAsset(ctx, dbgen.ListCollectionsForAssetParams{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.Collection, len(rows))
	for i, row := range rows {
		out[i] = repository.Collection{
			ID:          row.ID,
			WorkspaceID: row.WorkspaceID,
			Name:        row.Name,
			Description: row.Description,
			CreatedBy:   row.CreatedBy,
			AssetCount:  row.AssetCount,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}
	}
	return out, nil
}

func (r *collectionRepo) CountAssets(ctx context.Context, collectionID string) (int64, error) {
	row := r.d.Reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM collection_assets WHERE collection_id = ?`,
		collectionID,
	)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *collectionRepo) ListAssetIDs(ctx context.Context, collectionID string) ([]string, error) {
	rows, err := r.d.RQ.ListCollectionAssets(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, a := range rows {
		out[i] = a.ID
	}
	return out, nil
}

func toCollection(c dbgen.Collection) repository.Collection {
	return repository.Collection{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Description: c.Description,
		CreatedBy:   c.CreatedBy,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}
