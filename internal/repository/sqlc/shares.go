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

type shareRepo struct {
	d *db.DB
}

// NewShareRepo returns a repository.ShareRepository backed by sqlc-generated queries.
func NewShareRepo(d *db.DB) repository.ShareRepository {
	return &shareRepo{d: d}
}

func (r *shareRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Share, error) {
	row, err := r.d.RQ.GetShareByIDAndWorkspace(ctx, dbgen.GetShareByIDAndWorkspaceParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Share{}, apperr.ErrNotFound
		}
		return repository.Share{}, err
	}
	return toShare(row), nil
}

func (r *shareRepo) GetPublic(ctx context.Context, id string) (repository.Share, error) {
	row, err := r.d.RQ.GetShareByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Share{}, apperr.ErrNotFound
		}
		return repository.Share{}, err
	}
	return toShare(row), nil
}

func (r *shareRepo) GetByIDAndWorkspace(ctx context.Context, workspaceID, id string) (repository.Share, error) {
	return r.GetByID(ctx, workspaceID, id)
}

func (r *shareRepo) List(ctx context.Context, workspaceID string) ([]repository.Share, error) {
	rows, err := r.d.RQ.ListSharesByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.Share, len(rows))
	for i, row := range rows {
		out[i] = toShare(row)
	}
	return out, nil
}

func (r *shareRepo) Create(ctx context.Context, s repository.Share) (repository.Share, error) {
	row, err := r.d.WQ.CreateShare(ctx, dbgen.CreateShareParams{
		ID:            s.ID,
		WorkspaceID:   s.WorkspaceID,
		CreatedBy:     s.CreatedBy,
		Label:         s.Label,
		TargetType:    s.TargetType,
		TargetID:      s.TargetID,
		PasswordHash:  s.PasswordHash,
		ExpiresAt:     s.ExpiresAt,
		AllowComments: boolToInt(s.AllowComments),
		AllowDownload: boolToInt(s.AllowDownload),
	})
	if err != nil {
		return repository.Share{}, err
	}
	return toShare(row), nil
}

func (r *shareRepo) Update(ctx context.Context, s repository.Share) (repository.Share, error) {
	row, err := r.d.WQ.UpdateShare(ctx, dbgen.UpdateShareParams{
		ID:            s.ID,
		WorkspaceID:   s.WorkspaceID,
		Label:         s.Label,
		PasswordHash:  s.PasswordHash,
		ExpiresAt:     s.ExpiresAt,
		AllowComments: boolToInt(s.AllowComments),
		AllowDownload: boolToInt(s.AllowDownload),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Share{}, apperr.ErrNotFound
		}
		return repository.Share{}, err
	}
	return toShare(row), nil
}

func (r *shareRepo) Revoke(ctx context.Context, workspaceID, id string) error {
	return r.d.WQ.RevokeShare(ctx, dbgen.RevokeShareParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (r *shareRepo) IncrementViewCount(ctx context.Context, id string) error {
	return r.d.WQ.IncrementShareViewCount(ctx, id)
}

func (r *shareRepo) ListAssetsByTarget(
	ctx context.Context,
	targetType, targetID string,
) ([]repository.PublicAsset, error) {
	var query string
	switch targetType {
	case "asset":
		query = `SELECT id, workspace_id, project_id, folder_id, original_filename, storage_key,
			mime_type, size, width, height, thumbnail_key, metadata, created_at, updated_at
			FROM assets WHERE id = ?`
	case "project":
		query = `SELECT id, workspace_id, project_id, folder_id, original_filename, storage_key,
			mime_type, size, width, height, thumbnail_key, metadata, created_at, updated_at
			FROM assets WHERE project_id = ? ORDER BY created_at DESC, id DESC`
	case "collection":
		query = `SELECT a.id, a.workspace_id, a.project_id, a.folder_id, a.original_filename, a.storage_key,
			a.mime_type, a.size, a.width, a.height, a.thumbnail_key, a.metadata, a.created_at, a.updated_at
			FROM assets a
			JOIN collection_assets ca ON ca.asset_id = a.id
			WHERE ca.collection_id = ? ORDER BY ca.position ASC, ca.added_at ASC`
	default:
		return nil, nil
	}

	rows, err := r.d.Reader.QueryContext(ctx, query, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []repository.PublicAsset
	for rows.Next() {
		var a repository.PublicAsset
		var createdAt, updatedAt string
		if err = rows.Scan(
			&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.OriginalFilename, &a.StorageKey,
			&a.MimeType, &a.Size, &a.Width, &a.Height, &a.ThumbnailKey, &a.Metadata,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		a.CreatedAt = parseShareTime(createdAt)
		a.UpdatedAt = parseShareTime(updatedAt)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *shareRepo) GetPublicAsset(ctx context.Context, assetID string) (repository.PublicAsset, error) {
	row := r.d.Reader.QueryRowContext(ctx, `
		SELECT id, workspace_id, project_id, folder_id, original_filename, storage_key,
		       mime_type, size, width, height, thumbnail_key, metadata, created_at, updated_at
		FROM assets WHERE id = ?`, assetID)
	var a repository.PublicAsset
	var createdAt, updatedAt string
	if err := row.Scan(
		&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.OriginalFilename, &a.StorageKey,
		&a.MimeType, &a.Size, &a.Width, &a.Height, &a.ThumbnailKey, &a.Metadata,
		&createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.PublicAsset{}, apperr.ErrNotFound
		}
		return repository.PublicAsset{}, err
	}
	a.CreatedAt = parseShareTime(createdAt)
	a.UpdatedAt = parseShareTime(updatedAt)
	return a, nil
}

func (r *shareRepo) GetPublicAssetFile(ctx context.Context, assetID string) (repository.PublicAssetFile, error) {
	row := r.d.Reader.QueryRowContext(ctx, `
		SELECT a.mime_type, a.original_filename, v.storage_key, v.content_hash, v.size, v.created_at
		FROM assets a
		JOIN asset_versions v ON v.asset_id = a.id AND v.is_current = 1 AND v.deleted_at IS NULL
		WHERE a.id = ?`, assetID)
	var f repository.PublicAssetFile
	if err := row.Scan(
		&f.MimeType,
		&f.OriginalFilename,
		&f.StorageKey,
		&f.ContentHash,
		&f.Size,
		&f.VersionCreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.PublicAssetFile{}, apperr.ErrNotFound
		}
		return repository.PublicAssetFile{}, err
	}
	return f, nil
}

func (r *shareRepo) GetPublicAssetThumb(ctx context.Context, assetID string) (*string, time.Time, error) {
	row := r.d.Reader.QueryRowContext(ctx, `SELECT thumbnail_key, updated_at FROM assets WHERE id = ?`, assetID)
	var thumbKey *string
	var updatedAtStr string
	if err := row.Scan(&thumbKey, &updatedAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, time.Time{}, apperr.ErrNotFound
		}
		return nil, time.Time{}, err
	}
	return thumbKey, parseShareTime(updatedAtStr), nil
}

func (r *shareRepo) IsAssetInTarget(ctx context.Context, targetType, targetID, assetID string) (bool, error) {
	var query string
	var args []any
	switch targetType {
	case "asset":
		return assetID == targetID, nil
	case "project":
		query = `SELECT COUNT(1) FROM assets WHERE id = ? AND project_id = ?`
		args = []any{assetID, targetID}
	case "collection":
		query = `SELECT COUNT(1) FROM collection_assets WHERE collection_id = ? AND asset_id = ?`
		args = []any{targetID, assetID}
	default:
		return false, nil
	}
	row := r.d.Reader.QueryRowContext(ctx, query, args...)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *shareRepo) CreateComment(ctx context.Context, c repository.ShareComment) (repository.ShareComment, error) {
	row, err := r.d.WQ.CreateComment(ctx, dbgen.CreateCommentParams{
		ID:          c.ID,
		ShareID:     c.ShareID,
		AssetID:     c.AssetID,
		AuthorName:  c.AuthorName,
		AuthorEmail: c.AuthorEmail,
		Body:        c.Body,
	})
	if err != nil {
		return repository.ShareComment{}, err
	}
	return toComment(row), nil
}

func (r *shareRepo) ListCommentsByShare(ctx context.Context, shareID string) ([]repository.ShareComment, error) {
	rows, err := r.d.RQ.ListCommentsByShare(ctx, shareID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.ShareComment, len(rows))
	for i, row := range rows {
		out[i] = toComment(row)
	}
	return out, nil
}

func (r *shareRepo) ListCommentsByShareAndAsset(
	ctx context.Context,
	shareID, assetID string,
) ([]repository.ShareComment, error) {
	rows, err := r.d.RQ.ListCommentsByShareAndAsset(ctx, dbgen.ListCommentsByShareAndAssetParams{
		ShareID: shareID,
		AssetID: assetID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.ShareComment, len(rows))
	for i, row := range rows {
		out[i] = toComment(row)
	}
	return out, nil
}

func (r *shareRepo) DeleteComment(ctx context.Context, shareID, commentID string) error {
	return r.d.WQ.DeleteComment(ctx, dbgen.DeleteCommentParams{
		ID:      commentID,
		ShareID: shareID,
	})
}

func toShare(s dbgen.Share) repository.Share {
	return repository.Share{
		ID:            s.ID,
		WorkspaceID:   s.WorkspaceID,
		CreatedBy:     s.CreatedBy,
		Label:         s.Label,
		TargetType:    s.TargetType,
		TargetID:      s.TargetID,
		PasswordHash:  s.PasswordHash,
		ExpiresAt:     s.ExpiresAt,
		AllowComments: s.AllowComments != 0,
		AllowDownload: s.AllowDownload != 0,
		ViewCount:     s.ViewCount,
		CreatedAt:     parseShareTime(s.CreatedAt),
		RevokedAt:     s.RevokedAt,
	}
}

func toComment(c dbgen.ShareComment) repository.ShareComment {
	return repository.ShareComment{
		ID:          c.ID,
		ShareID:     c.ShareID,
		AssetID:     c.AssetID,
		AuthorName:  c.AuthorName,
		AuthorEmail: c.AuthorEmail,
		Body:        c.Body,
		CreatedAt:   c.CreatedAt,
	}
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func parseShareTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	return t
}
