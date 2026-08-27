package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"damask/server/internal/apperr"
	"damask/server/internal/db"
	"damask/server/internal/repository"
)

type variantRepo struct {
	writerDB rawDB
	readerDB rawDB
}

// NewVariantRepo returns a repository.VariantRepository backed by sqlc-generated queries.
func NewVariantRepo(d *db.DB) repository.VariantRepository {
	return &variantRepo{writerDB: d.Writer, readerDB: d.Reader}
}

func (r *variantRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Variant, error) {
	row := r.readerDB.QueryRowContext(ctx, `
		SELECT id, workspace_id, asset_version_id, type, storage_key, transform_params, size,
		       status, thumbnail_key, thumbnail_content_type, title, is_shared, content_hash, created_at
		FROM variants
		WHERE id = ? AND workspace_id = ?`, id, workspaceID)
	return scanVariant(row)
}

func (r *variantRepo) ListByAsset(ctx context.Context, _ string, assetID string) ([]repository.Variant, error) {
	rows, err := r.readerDB.QueryContext(ctx, `
		SELECT v.id, v.workspace_id, v.asset_version_id, v.type, v.storage_key, v.transform_params, v.size,
		       v.status, v.thumbnail_key, v.thumbnail_content_type, v.title, v.is_shared, v.content_hash, v.created_at
		FROM variants v
		JOIN asset_versions av ON av.id = v.asset_version_id
		WHERE av.asset_id = ? AND av.is_current = 1
		ORDER BY v.created_at ASC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []repository.Variant
	for rows.Next() {
		v, scanErr := scanVariant(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *variantRepo) Create(ctx context.Context, v repository.Variant) (repository.Variant, error) {
	status := v.Status
	if status == "" {
		status = "ready"
	}
	row := r.writerDB.QueryRowContext(
		ctx,
		`
		INSERT INTO variants (id, workspace_id, asset_version_id, type, storage_key, transform_params, size, status, title, is_shared, content_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, workspace_id, asset_version_id, type, storage_key, transform_params, size,
		          status, thumbnail_key, thumbnail_content_type, title, is_shared, content_hash, created_at`,
		v.ID,
		v.WorkspaceID,
		v.AssetVersionID,
		v.Type,
		v.StorageKey,
		v.TransformParams,
		v.Size,
		status,
		v.Title,
		v.IsShared,
		v.ContentHash,
	)
	return scanVariant(row)
}

func (r *variantRepo) Delete(ctx context.Context, workspaceID, id string) error {
	_, err := r.writerDB.ExecContext(ctx, `DELETE FROM variants WHERE id = ? AND workspace_id = ?`, id, workspaceID)
	return err
}

func (r *variantRepo) UpdateTitle(ctx context.Context, workspaceID, variantID string, title *string) error {
	res, err := r.writerDB.ExecContext(
		ctx,
		`UPDATE variants SET title = ? WHERE id = ? AND workspace_id = ?`,
		title,
		variantID,
		workspaceID,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func (r *variantRepo) UpdateSharedBatch(ctx context.Context, workspaceID string, ids []string, isShared bool) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+2) //nolint:mnd // isShared and workspaceID
	args = append(args, isShared)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, workspaceID)
	query := fmt.Sprintf(
		`UPDATE variants SET is_shared = ? WHERE id IN (%s) AND workspace_id = ?`, placeholders)
	_, err := r.writerDB.ExecContext(ctx, query, args...)
	return err
}

func (r *variantRepo) ListSharedByAssetIDs(
	ctx context.Context,
	assetIDs []string,
) ([]repository.VariantWithAssetID, error) {
	if len(assetIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(assetIDs)), ",")
	args := make([]any, 0, len(assetIDs))
	for _, id := range assetIDs {
		args = append(args, id)
	}
	query := fmt.Sprintf(
		`SELECT v.id, v.workspace_id, v.asset_version_id, v.type, v.storage_key, v.transform_params, v.size,
		       v.status, v.thumbnail_key, v.thumbnail_content_type, v.title, v.is_shared, v.content_hash, v.created_at,
		       av.asset_id AS asset_id
		FROM variants v
		JOIN asset_versions av ON av.id = v.asset_version_id
		WHERE av.asset_id IN (%s)
		  AND av.is_current = 1
		  AND v.is_shared = 1
		ORDER BY av.asset_id, v.created_at ASC`, placeholders)
	rows, err := r.readerDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []repository.VariantWithAssetID
	for rows.Next() {
		var item repository.VariantWithAssetID
		if err = rows.Scan(
			&item.ID,
			&item.WorkspaceID,
			&item.AssetVersionID,
			&item.Type,
			&item.StorageKey,
			&item.TransformParams,
			&item.Size,
			&item.Status,
			&item.ThumbnailKey,
			&item.ThumbnailContentType,
			&item.Title,
			&item.IsShared,
			&item.ContentHash,
			&item.CreatedAt,
			&item.AssetID,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *variantRepo) GetSharedByVariantAndAsset(
	ctx context.Context,
	variantID, assetID string,
) (repository.Variant, error) {
	row := r.readerDB.QueryRowContext(ctx, `
		SELECT v.id, v.workspace_id, v.asset_version_id, v.type, v.storage_key, v.transform_params, v.size,
		       v.status, v.thumbnail_key, v.thumbnail_content_type, v.title, v.is_shared, v.content_hash, v.created_at
		FROM variants v
		JOIN asset_versions av ON av.id = v.asset_version_id
		WHERE v.id = ?
		  AND av.asset_id = ?
		  AND av.is_current = 1
		  AND v.is_shared = 1`, variantID, assetID)
	return scanVariant(row)
}

// ListVariantParamHistory returns the most recent non-empty transform_params rows for the
// given workspace + variant type, capped at `limit`. Distinctness across rows with the same
// canonical content is computed by the caller, not here — see VariantRepository docs.
func (r *variantRepo) ListVariantParamHistory(
	ctx context.Context,
	workspaceID, variantType string,
	limit int,
) ([]string, error) {
	rows, err := r.readerDB.QueryContext(ctx, `
		SELECT transform_params
		FROM variants
		WHERE workspace_id = ?
		  AND type = ?
		  AND transform_params IS NOT NULL
		  AND transform_params != ''
		  AND transform_params != '{}'
		ORDER BY created_at DESC
		LIMIT ?`, workspaceID, variantType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanVariant(row interface {
	Scan(dest ...any) error
}) (repository.Variant, error) {
	var v repository.Variant
	err := row.Scan(
		&v.ID, &v.WorkspaceID, &v.AssetVersionID, &v.Type, &v.StorageKey, &v.TransformParams, &v.Size,
		&v.Status, &v.ThumbnailKey, &v.ThumbnailContentType, &v.Title, &v.IsShared, &v.ContentHash, &v.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.Variant{}, apperr.ErrNotFound
	}
	return v, err
}
