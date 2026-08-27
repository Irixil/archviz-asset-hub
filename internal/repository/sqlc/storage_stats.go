package reposqlc

import (
	"context"

	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type storageStatsRepo struct {
	d *db.DB
}

// NewStorageStatsRepo returns a repository.StorageStatsRepository backed by sqlc-generated queries.
func NewStorageStatsRepo(d *db.DB) repository.StorageStatsRepository {
	return &storageStatsRepo{d: d}
}

func (r *storageStatsRepo) GetByProjectAndType(
	ctx context.Context,
	workspaceID string,
) ([]repository.StorageProjectTypeRow, error) {
	rows, err := r.d.RQ.GetStorageByProjectAndType(ctx, dbgen.GetStorageByProjectAndTypeParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.StorageProjectTypeRow, len(rows))
	for i, row := range rows {
		out[i] = repository.StorageProjectTypeRow{
			ProjectID:     row.ProjectID,
			ProjectName:   row.ProjectName,
			AssetType:     row.AssetType,
			VersionsBytes: anyToInt64(row.VersionsBytes),
			VariantsBytes: anyToInt64(row.VariantsBytes),
		}
	}
	return out, nil
}

func (r *storageStatsRepo) GetFolderCountsByProject(
	ctx context.Context,
	workspaceID string,
) ([]repository.StorageFolderCountRow, error) {
	rows, err := r.d.RQ.GetFolderCountsByProject(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.StorageFolderCountRow, len(rows))
	for i, row := range rows {
		out[i] = repository.StorageFolderCountRow{ProjectID: row.ProjectID, FolderCount: row.FolderCount}
	}
	return out, nil
}

func (r *storageStatsRepo) GetStorageLimitBytes(ctx context.Context, workspaceID string) (*int64, error) {
	return r.d.RQ.GetWorkspaceStorageLimitBytes(ctx, workspaceID)
}

func (r *storageStatsRepo) GetByFolder(
	ctx context.Context,
	workspaceID, projectID string,
) ([]repository.StorageFolderRow, error) {
	rows, err := r.d.RQ.GetStorageByFolder(ctx, dbgen.GetStorageByFolderParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
		ProjectID:     &projectID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.StorageFolderRow, len(rows))
	for i, row := range rows {
		out[i] = repository.StorageFolderRow{
			FolderID:      anyToStringPtr(row.FolderID),
			FolderName:    row.FolderName,
			VersionsBytes: anyToInt64(row.VersionsBytes),
			VariantsBytes: anyToInt64(row.VariantsBytes),
		}
	}
	return out, nil
}

// anyToStringPtr converts a SQLite CASE expression `any` result to *string.
func anyToStringPtr(v any) *string {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

// anyToInt64 converts a SQLite COALESCE/SUM `any` result to int64.
func anyToInt64(v any) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}
