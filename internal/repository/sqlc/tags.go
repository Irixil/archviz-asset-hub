package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"damask/server/internal/apperr"
	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type tagRepo struct {
	d *db.DB
}

// NewTagRepo returns a repository.TagRepository backed by sqlc-generated queries.
func NewTagRepo(d *db.DB) repository.TagRepository {
	return &tagRepo{d: d}
}

func (r *tagRepo) GetByName(ctx context.Context, workspaceID, name string) (repository.Tag, error) {
	row, err := r.d.RQ.GetTagByWorkspaceAndName(ctx, dbgen.GetTagByWorkspaceAndNameParams{
		WorkspaceID: workspaceID,
		Name:        name,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Tag{}, apperr.ErrNotFound
		}
		return repository.Tag{}, err
	}
	return toTag(row), nil
}

func (r *tagRepo) List(ctx context.Context, workspaceID string, includeSystem bool) ([]repository.Tag, error) {
	rows, err := r.d.RQ.ListTagsWithCount(ctx, dbgen.ListTagsWithCountParams{
		WorkspaceID:   workspaceID,
		IncludeSystem: includeSystem,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.Tag, len(rows))
	for i, row := range rows {
		out[i] = repository.Tag{
			ID:          row.ID,
			WorkspaceID: row.WorkspaceID,
			Name:        row.Name,
			Color:       row.Color,
			GroupName:   row.GroupName,
			AssetCount:  row.AssetCount,
			CreatedAt:   row.CreatedAt,
			LastUsedAt:  row.LastUsedAt,
		}
	}
	return out, nil
}

func (r *tagRepo) EnsureSystemTag(ctx context.Context, workspaceID, name string) error {
	return r.d.WQ.EnsureSystemTag(ctx, dbgen.EnsureSystemTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        name,
	})
}

func (r *tagRepo) Upsert(ctx context.Context, workspaceID, name string) (repository.Tag, error) {
	row, err := r.d.WQ.GetOrCreateTag(ctx, dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        name,
	})
	if err != nil {
		return repository.Tag{}, err
	}
	return toTag(row), nil
}

func (r *tagRepo) UpdateMetadata(ctx context.Context, workspaceID, name string, color, groupName *string) error {
	return r.d.WQ.UpdateTagMetadata(ctx, dbgen.UpdateTagMetadataParams{
		WorkspaceID: workspaceID,
		Name:        name,
		Color:       color,
		GroupName:   groupName,
	})
}

func (r *tagRepo) Rename(ctx context.Context, workspaceID, oldName, newName string) error {
	return r.d.WQ.UpdateTagName(ctx, dbgen.UpdateTagNameParams{
		WorkspaceID: workspaceID,
		Name_2:      oldName,
		Name:        newName,
	})
}

func (r *tagRepo) Delete(ctx context.Context, workspaceID string, names []string) error {
	for _, name := range names {
		if err := r.d.WQ.DeleteTag(ctx, dbgen.DeleteTagParams{
			WorkspaceID: workspaceID,
			Name:        name,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *tagRepo) ListForAsset(ctx context.Context, assetID string) ([]repository.Tag, error) {
	rows, err := r.d.RQ.GetTagsForAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.Tag, len(rows))
	for i, row := range rows {
		out[i] = repository.Tag{
			ID:          row.ID,
			WorkspaceID: row.WorkspaceID,
			Name:        row.Name,
		}
	}
	return out, nil
}

func (r *tagRepo) AddToAsset(ctx context.Context, assetID, tagID string) error {
	return r.d.WQ.AddTagToAsset(ctx, dbgen.AddTagToAssetParams{
		AssetID: assetID,
		TagID:   tagID,
	})
}

func (r *tagRepo) RemoveFromAsset(ctx context.Context, workspaceID, assetID, tagName string) error {
	return r.d.WQ.RemoveTagFromAsset(ctx, dbgen.RemoveTagFromAssetParams{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		Name:        tagName,
	})
}

func (r *tagRepo) BatchTagsForAssets(ctx context.Context, assetIDs []string) (map[string][]string, error) {
	if len(assetIDs) == 0 {
		return map[string][]string{}, nil
	}
	placeholders := make([]string, len(assetIDs))
	args := make([]any, len(assetIDs))
	for i, id := range assetIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	q := fmt.Sprintf( //nolint:gosec // query is built with validated inputs and parameter placeholders
		`SELECT at.asset_id, t.name FROM asset_tags at JOIN tags t ON t.id = at.tag_id WHERE at.asset_id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := r.d.Reader.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]string, len(assetIDs))
	for rows.Next() {
		var assetID, name string
		if err = rows.Scan(&assetID, &name); err != nil {
			return nil, err
		}
		out[assetID] = append(out[assetID], name)
	}
	return out, rows.Err()
}

func (r *tagRepo) CountAssets(ctx context.Context, tagID string) (int64, error) {
	return r.d.RQ.CountTagAssets(ctx, tagID)
}

func (r *tagRepo) ReassignAssets(ctx context.Context, fromTagID, toTagID string) error {
	return r.d.WQ.ReassignTagAssets(ctx, dbgen.ReassignTagAssetsParams{
		TagID:   toTagID,
		TagID_2: fromTagID,
	})
}

func (r *tagRepo) TouchLastUsed(ctx context.Context, workspaceID, name string) error {
	return r.d.WQ.TouchTagLastUsed(ctx, dbgen.TouchTagLastUsedParams{
		WorkspaceID: workspaceID,
		Name:        name,
	})
}

func (r *tagRepo) FindAssetBySystemTagInFolder(
	ctx context.Context,
	workspaceID, tagName, folderID string,
) (repository.Asset, error) {
	row, err := r.d.RQ.FindAssetBySystemTagInFolder(ctx, dbgen.FindAssetBySystemTagInFolderParams{
		WorkspaceID: workspaceID,
		Name:        tagName,
		FolderID:    &folderID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Asset{}, apperr.ErrNotFound
		}
		return repository.Asset{}, err
	}
	return toAsset(row), nil
}

func (r *tagRepo) FindAssetBySystemTagInProject(
	ctx context.Context,
	workspaceID, tagName, projectID string,
) (repository.Asset, error) {
	row, err := r.d.RQ.FindAssetBySystemTagInProject(ctx, dbgen.FindAssetBySystemTagInProjectParams{
		WorkspaceID: workspaceID,
		Name:        tagName,
		ProjectID:   &projectID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Asset{}, apperr.ErrNotFound
		}
		return repository.Asset{}, err
	}
	return toAsset(row), nil
}

func (r *tagRepo) FindAssetBySystemTagInWorkspace(
	ctx context.Context,
	workspaceID, tagName string,
) (repository.Asset, error) {
	row, err := r.d.RQ.FindAssetBySystemTagInWorkspace(ctx, dbgen.FindAssetBySystemTagInWorkspaceParams{
		WorkspaceID: workspaceID,
		Name:        tagName,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Asset{}, apperr.ErrNotFound
		}
		return repository.Asset{}, err
	}
	return toAsset(row), nil
}

func (r *tagRepo) RunInTx(ctx context.Context, fn func(tx repository.TagRepository) error) error {
	tx, err := r.d.Writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after read-only queries or commit.
	txRepo := &tagRepo{d: r.d.WithTx(tx)}
	if err = fn(txRepo); err != nil {
		return err
	}
	return tx.Commit()
}

func toTag(t dbgen.Tag) repository.Tag {
	return repository.Tag{
		ID:          t.ID,
		WorkspaceID: t.WorkspaceID,
		Name:        t.Name,
		Color:       t.Color,
		GroupName:   t.GroupName,
		CreatedAt:   t.CreatedAt,
		LastUsedAt:  t.LastUsedAt,
	}
}
