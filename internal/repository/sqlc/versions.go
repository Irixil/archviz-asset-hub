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

type versionRepo struct {
	d *db.DB
	// writer is nil on the tx-scoped clone produced by RunInTx, which signals
	// SetCurrent to run its statements on the existing tx instead of opening
	// a nested one (SQLite's writer pool only allows a single connection, so
	// a nested BeginTx on it would deadlock waiting for a connection to free up).
	writer *sql.DB
}

// NewVersionRepo returns a repository.VersionRepository backed by sqlc-generated queries.
func NewVersionRepo(d *db.DB) repository.VersionRepository {
	return &versionRepo{d: d, writer: d.Writer}
}

func (r *versionRepo) GetByID(ctx context.Context, id string) (repository.AssetVersion, error) {
	row, err := r.d.RQ.GetVersionByIDUnchecked(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetVersion{}, apperr.ErrNotFound
		}
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) ListByAsset(ctx context.Context, assetID string) ([]repository.AssetVersion, error) {
	rows, err := r.d.RQ.ListVersions(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.AssetVersion, len(rows))
	for i, row := range rows {
		out[i] = toVersion(row)
	}
	return out, nil
}

func (r *versionRepo) Create(ctx context.Context, v repository.AssetVersion) (repository.AssetVersion, error) {
	row, err := r.d.WQ.CreateAssetVersion(ctx, dbgen.CreateAssetVersionParams{
		ID:           v.ID,
		AssetID:      v.AssetID,
		WorkspaceID:  v.WorkspaceID,
		VersionNum:   v.VersionNum,
		StorageKey:   v.StorageKey,
		ContentHash:  v.ContentHash,
		MimeType:     v.MimeType,
		Size:         v.Size,
		Width:        v.Width,
		Height:       v.Height,
		DurationSec:  v.DurationSec,
		ThumbnailKey: v.ThumbnailKey,
		Comment:      v.Comment,
		CreatedBy:    v.CreatedBy,
	})
	if err != nil {
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) GetCurrentByAsset(ctx context.Context, assetID string) (repository.AssetVersion, error) {
	row, err := r.d.RQ.GetCurrentVersion(ctx, assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetVersion{}, apperr.ErrNotFound
		}
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) GetFirstByAsset(ctx context.Context, assetID string) (repository.AssetVersion, error) {
	row, err := r.d.RQ.GetFirstVersion(ctx, assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetVersion{}, apperr.ErrNotFound
		}
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) GetByIDForWorkspace(
	ctx context.Context,
	workspaceID, id string,
) (repository.AssetVersion, error) {
	row, err := r.d.RQ.GetVersionByID(ctx, dbgen.GetVersionByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetVersion{}, apperr.ErrNotFound
		}
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) SoftDelete(ctx context.Context, id string) error {
	return r.d.WQ.SoftDeleteVersion(ctx, id)
}

func (r *versionRepo) Delete(ctx context.Context, id string) error {
	return r.d.WQ.HardDeleteVersion(ctx, id)
}

func (r *versionRepo) IsReferencedAsCover(ctx context.Context, versionID string) (bool, error) {
	count, err := r.d.RQ.IsVersionReferencedAsCover(ctx, dbgen.IsVersionReferencedAsCoverParams{
		CoverVersionID: &versionID,
		IconVersionID:  &versionID,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *versionRepo) CountByAsset(ctx context.Context, assetID string) (int64, error) {
	return r.d.RQ.CountActiveVersions(ctx, assetID)
}

func (r *versionRepo) GetByHash(ctx context.Context, assetID, contentHash string) (repository.AssetVersion, error) {
	row, err := r.d.RQ.GetVersionByHash(ctx, dbgen.GetVersionByHashParams{
		AssetID:     assetID,
		ContentHash: contentHash,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetVersion{}, apperr.ErrNotFound
		}
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) NextVersionNum(ctx context.Context, assetID string) (int64, error) {
	var maxNum sql.NullInt64
	err := r.d.Reader.QueryRowContext(ctx,
		`SELECT MAX(version_num) FROM asset_versions WHERE asset_id = ?`, assetID,
	).Scan(&maxNum)
	if err != nil {
		return 0, err
	}
	return maxNum.Int64 + 1, nil
}

func (r *versionRepo) SetCurrent(ctx context.Context, assetID, versionID string) error {
	if r.writer == nil {
		// Already running inside RunInTx's transaction — reuse it instead of
		// nesting a BeginTx on the (single-connection) writer pool.
		return setCurrentVersion(ctx, r.d.WQ, assetID, versionID)
	}

	tx, err := r.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after read-only queries or commit.
	if err = setCurrentVersion(ctx, r.d.WQ.WithTx(tx), assetID, versionID); err != nil {
		return err
	}
	return tx.Commit()
}

func setCurrentVersion(ctx context.Context, q *dbgen.Queries, assetID, versionID string) error {
	if err := q.ClearCurrentVersionFlags(ctx, assetID); err != nil {
		return err
	}
	if err := q.SetCurrentVersionFlag(ctx, versionID); err != nil {
		return err
	}
	return q.UpdateAssetCurrentVersion(ctx, dbgen.UpdateAssetCurrentVersionParams{
		CurrentVersionID: &versionID,
		ID:               assetID,
	})
}

func (r *versionRepo) RunInTx(ctx context.Context, fn func(tx repository.VersionRepository) error) error {
	tx, err := r.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after read-only queries or commit.
	txRepo := &versionRepo{d: r.d.WithTx(tx), writer: nil}
	if err = fn(txRepo); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *versionRepo) SetAssetThumbnail(ctx context.Context, assetID string, key *string) error {
	return r.d.WQ.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: key,
		ID:           assetID,
	})
}

func (r *versionRepo) ListWithVariantCount(
	ctx context.Context,
	assetID string,
) ([]repository.AssetVersionWithCount, error) {
	rows, err := r.d.RQ.ListVersionsWithVariantCount(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.AssetVersionWithCount, len(rows))
	for i, row := range rows {
		out[i] = repository.AssetVersionWithCount{
			AssetVersion: repository.AssetVersion{
				ID:           row.ID,
				AssetID:      row.AssetID,
				WorkspaceID:  row.WorkspaceID,
				VersionNum:   row.VersionNum,
				StorageKey:   row.StorageKey,
				ContentHash:  row.ContentHash,
				MimeType:     row.MimeType,
				Size:         row.Size,
				Width:        row.Width,
				Height:       row.Height,
				DurationSec:  row.DurationSec,
				ThumbnailKey: row.ThumbnailKey,
				Comment:      row.Comment,
				CreatedBy:    row.CreatedBy,
				CreatedAt:    parseVersionTime(row.CreatedAt),
				IsCurrent:    row.IsCurrent != 0,
				DeletedAt:    row.DeletedAt,
			},
			VariantCount: row.VariantCount,
		}
	}
	return out, nil
}

func (r *versionRepo) FindDuplicateVersions(
	ctx context.Context,
	workspaceID, contentHash, excludeAssetID string,
) ([]repository.DuplicateVersionMatch, error) {
	rows, err := r.d.RQ.FindVersionsByContentHash(ctx, dbgen.FindVersionsByContentHashParams{
		WorkspaceID:    workspaceID,
		ContentHash:    contentHash,
		ExcludeAssetID: excludeAssetID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.DuplicateVersionMatch, len(rows))
	for i, row := range rows {
		out[i] = repository.DuplicateVersionMatch{
			VersionID:  row.VersionID,
			AssetID:    row.AssetID,
			VersionNum: row.VersionNum,
			StorageKey: row.StorageKey,
			IsCurrent:  row.IsCurrent != 0,
			DeletedAt:  row.DeletedAt,
			CreatedAt:  parseVersionTime(row.CreatedAt),
		}
	}
	return out, nil
}

func toVersion(v dbgen.AssetVersion) repository.AssetVersion {
	return repository.AssetVersion{
		ID:           v.ID,
		AssetID:      v.AssetID,
		WorkspaceID:  v.WorkspaceID,
		VersionNum:   v.VersionNum,
		StorageKey:   v.StorageKey,
		ContentHash:  v.ContentHash,
		MimeType:     v.MimeType,
		Size:         v.Size,
		Width:        v.Width,
		Height:       v.Height,
		DurationSec:  v.DurationSec,
		ThumbnailKey: v.ThumbnailKey,
		Comment:      v.Comment,
		CreatedBy:    v.CreatedBy,
		CreatedAt:    parseVersionTime(v.CreatedAt),
		IsCurrent:    v.IsCurrent != 0,
		DeletedAt:    v.DeletedAt,
	}
}

func parseVersionTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	return t
}
