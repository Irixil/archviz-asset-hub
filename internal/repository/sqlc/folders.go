package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type folderRepo struct {
	d *db.DB
}

// NewFolderRepo returns a repository.FolderRepository backed by sqlc-generated queries.
func NewFolderRepo(d *db.DB) repository.FolderRepository {
	return &folderRepo{d: d}
}

func (r *folderRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Folder, error) {
	row, err := r.d.RQ.GetFolderByID(ctx, dbgen.GetFolderByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Folder{}, apperr.ErrNotFound
		}
		return repository.Folder{}, err
	}
	return toFolder(row), nil
}

func (r *folderRepo) ListByProject(ctx context.Context, workspaceID, projectID string) ([]repository.Folder, error) {
	sqlRows, err := r.d.Reader.QueryContext(ctx,
		`SELECT id, workspace_id, project_id, parent_id, name, slug, position, created_at
		 FROM folders WHERE workspace_id = ? AND project_id = ? ORDER BY position, name`,
		workspaceID, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	var out []repository.Folder
	for sqlRows.Next() {
		var f dbgen.Folder
		if err = sqlRows.Scan(
			&f.ID,
			&f.WorkspaceID,
			&f.ProjectID,
			&f.ParentID,
			&f.Name,
			&f.Slug,
			&f.Position,
			&f.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, toFolder(f))
	}
	return out, sqlRows.Err()
}

func (r *folderRepo) Create(ctx context.Context, f repository.Folder) (repository.Folder, error) {
	row, err := r.d.WQ.CreateFolder(ctx, dbgen.CreateFolderParams{
		ID:          f.ID,
		WorkspaceID: f.WorkspaceID,
		ProjectID:   f.ProjectID,
		ParentID:    f.ParentID,
		Name:        f.Name,
		Slug:        f.Slug,
	})
	if err != nil {
		return repository.Folder{}, err
	}
	return toFolder(row), nil
}

func (r *folderRepo) Update(ctx context.Context, f repository.Folder) (repository.Folder, error) {
	row, err := r.d.WQ.UpdateFolder(ctx, dbgen.UpdateFolderParams{
		ID:          f.ID,
		WorkspaceID: f.WorkspaceID,
		Name:        &f.Name,
		Slug:        f.Slug,
		Position:    &f.Position,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Folder{}, apperr.ErrNotFound
		}
		return repository.Folder{}, err
	}
	return toFolder(row), nil
}

func (r *folderRepo) Delete(ctx context.Context, workspaceID, id string) error {
	return r.d.WQ.DeleteFolder(ctx, dbgen.DeleteFolderParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (r *folderRepo) GetChildren(ctx context.Context, workspaceID, parentID string) ([]repository.Folder, error) {
	rows, err := r.d.RQ.GetFolderChildren(ctx, dbgen.GetFolderChildrenParams{
		ParentID:    &parentID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.Folder, len(rows))
	for i, row := range rows {
		out[i] = toFolder(row)
	}
	return out, nil
}

func (r *folderRepo) NullifyAssets(ctx context.Context, workspaceID, folderID string) error {
	return r.d.WQ.NullifyFolderAssets(ctx, dbgen.NullifyFolderAssetsParams{
		FolderID:    &folderID,
		WorkspaceID: workspaceID,
	})
}

func (r *folderRepo) ListTree(ctx context.Context, workspaceID, projectID string) ([]repository.FolderTree, error) {
	rows, err := r.d.Reader.QueryContext(ctx, `
		WITH RECURSIVE tree AS (
			SELECT *, 0 AS depth FROM folders
			WHERE project_id = ? AND parent_id IS NULL AND workspace_id = ?
			UNION ALL
			SELECT f.*, t.depth + 1 FROM folders f
			JOIN tree t ON f.parent_id = t.id
			WHERE t.depth < 2
		)
		SELECT t.id, t.workspace_id, t.project_id, t.parent_id, t.name, t.slug, t.position, t.created_at, t.depth,
			(SELECT COUNT(*) FROM assets a WHERE a.folder_id = t.id AND a.workspace_id = ?) AS asset_count
		FROM tree t
		ORDER BY t.depth, t.position, t.name
	`, projectID, workspaceID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type flatRow struct {
		folder     repository.Folder
		depth      int64
		assetCount int64
	}

	var flat []flatRow
	for rows.Next() {
		var f dbgen.Folder
		var depth, assetCount int64
		if err = rows.Scan(
			&f.ID, &f.WorkspaceID, &f.ProjectID, &f.ParentID,
			&f.Name, &f.Slug, &f.Position, &f.CreatedAt, &depth, &assetCount,
		); err != nil {
			return nil, err
		}
		flat = append(flat, flatRow{folder: toFolder(f), depth: depth, assetCount: assetCount})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Build tree structure: roots hold children.
	idxByID := make(map[string]int) // id → index in roots
	var roots []repository.FolderTree
	for _, row := range flat {
		node := repository.FolderTree{
			Folder:     row.folder,
			AssetCount: row.assetCount,
			Children:   []repository.FolderTree{},
		}
		if row.folder.ParentID == nil {
			idxByID[row.folder.ID] = len(roots)
			roots = append(roots, node)
		} else {
			parentIdx, ok := idxByID[*row.folder.ParentID]
			if ok {
				roots[parentIdx].Children = append(roots[parentIdx].Children, node)
			}
		}
	}
	return roots, nil
}

func toFolder(f dbgen.Folder) repository.Folder {
	return repository.Folder{
		ID:          f.ID,
		WorkspaceID: f.WorkspaceID,
		ProjectID:   f.ProjectID,
		ParentID:    f.ParentID,
		Name:        f.Name,
		Slug:        f.Slug,
		Position:    f.Position,
		CreatedAt:   parseCreatedAt(f.CreatedAt),
	}
}

// parseCreatedAt parses SQLite time strings stored as text.
func parseCreatedAt(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	return t
}
