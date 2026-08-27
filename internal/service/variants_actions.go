package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"mime"
	"path/filepath"
	"strings"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/jobspec"
	"damask/server/internal/queue"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

var rerunnableTypes = map[string]bool{
	queue.JobTypeImageBgRemove:   true,
	queue.JobTypeImageWithPrompt: true,
}

type VariantActionsStore interface {
	Promote(ctx context.Context, p promoteVariantDBParams) (promoteVariantDBResult, error)
	SetAsThumbnail(ctx context.Context, p setVariantThumbnailDBParams) error
	MarkVariantPending(ctx context.Context, workspaceID, variantID string, transformParams *string) error
	GetVersion(ctx context.Context, versionID string) (variantSourceVersion, error)
	SetVariantStatus(ctx context.Context, variantID, workspaceID, status string) error
}

type promoteVariantDBParams struct {
	NewAssetID           string
	NewVersionID         string
	WorkspaceID          string
	ProjectID            *string
	FolderID             *string
	OriginalFilename     string
	StorageKey           string
	MimeType             string
	Size                 int64
	Width                *int64
	Height               *int64
	ThumbnailKey         *string
	ThumbnailContentType string
	DerivedFromAssetID   string
	ContentHash          string
	CreatedBy            *string
	SourceAssetID        string
	SourceVariantID      string
}

type promoteVariantDBResult struct {
	NewAssetID   string
	NewVersionID string
}

type setVariantThumbnailDBParams struct {
	AssetID              string
	CurrentVersionID     string
	ThumbnailKey         string
	ThumbnailContentType string
}

type sqlVariantActionsStore struct {
	db *sql.DB
}

type variantSourceVersion struct {
	VersionNum int64
	StorageKey string
	MimeType   string
}

func NewSQLVariantActionsStore(db *sql.DB) VariantActionsStore {
	return &sqlVariantActionsStore{db: db}
}

func (s *sqlVariantActionsStore) Promote(
	ctx context.Context,
	p promoteVariantDBParams,
) (promoteVariantDBResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return promoteVariantDBResult{}, err
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after read-only queries or commit.

	if _, execErr := tx.ExecContext(ctx, `
		INSERT INTO assets (
			id, workspace_id, project_id, folder_id, original_filename,
			storage_key, mime_type, size, width, height,
			thumbnail_key, thumbnail_content_type, derived_from_asset_id,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		p.NewAssetID, p.WorkspaceID, p.ProjectID, p.FolderID, p.OriginalFilename,
		p.StorageKey, p.MimeType, p.Size, p.Width, p.Height,
		p.ThumbnailKey, p.ThumbnailContentType, p.DerivedFromAssetID,
	); execErr != nil {
		return promoteVariantDBResult{}, execErr
	}

	if _, execErr := tx.ExecContext(ctx, `
		INSERT INTO asset_versions (
			id, asset_id, workspace_id, version_num, storage_key, content_hash,
			mime_type, size, width, height, thumbnail_key, thumbnail_content_type,
			created_by, is_current, created_at
		) VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, datetime('now'))`,
		p.NewVersionID, p.NewAssetID, p.WorkspaceID, p.StorageKey, p.ContentHash,
		p.MimeType, p.Size, p.Width, p.Height, p.ThumbnailKey, p.ThumbnailContentType,
		p.CreatedBy,
	); execErr != nil {
		return promoteVariantDBResult{}, execErr
	}

	if _, execErr := tx.ExecContext(ctx, `
		UPDATE assets SET current_version_id = ?, updated_at = datetime('now') WHERE id = ?`,
		p.NewVersionID, p.NewAssetID,
	); execErr != nil {
		return promoteVariantDBResult{}, execErr
	}

	if _, execErr := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO asset_tags (asset_id, tag_id)
		SELECT ?, tag_id FROM asset_tags WHERE asset_id = ?`,
		p.NewAssetID, p.SourceAssetID,
	); execErr != nil {
		return promoteVariantDBResult{}, execErr
	}

	if _, execErr := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO asset_field_values (
			id, asset_id, field_id, value_text, value_number, value_date, value_boolean,
			created_by, created_at, updated_at
		)
		SELECT lower(hex(randomblob(16))), ?, field_id, value_text, value_number, value_date, value_boolean,
		       created_by, datetime('now'), datetime('now')
		FROM asset_field_values
		WHERE asset_id = ?`,
		p.NewAssetID, p.SourceAssetID,
	); execErr != nil {
		return promoteVariantDBResult{}, execErr
	}

	if _, execErr := tx.ExecContext(
		ctx,
		`DELETE FROM variants WHERE id = ? AND workspace_id = ?`,
		p.SourceVariantID,
		p.WorkspaceID,
	); execErr != nil {
		return promoteVariantDBResult{}, execErr
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return promoteVariantDBResult{}, commitErr
	}
	return promoteVariantDBResult{NewAssetID: p.NewAssetID, NewVersionID: p.NewVersionID}, nil
}

func (s *sqlVariantActionsStore) SetAsThumbnail(ctx context.Context, p setVariantThumbnailDBParams) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after read-only queries or commit.

	if _, execErr := tx.ExecContext(ctx, `
		UPDATE assets
		SET thumbnail_key = ?, thumbnail_content_type = ?, updated_at = datetime('now')
		WHERE id = ?`,
		p.ThumbnailKey, p.ThumbnailContentType, p.AssetID,
	); execErr != nil {
		return execErr
	}

	if _, execErr := tx.ExecContext(ctx, `
		UPDATE asset_versions
		SET thumbnail_key = ?, thumbnail_content_type = ?
		WHERE id = ?`,
		p.ThumbnailKey, p.ThumbnailContentType, p.CurrentVersionID,
	); execErr != nil {
		return execErr
	}

	return tx.Commit()
}

func (s *sqlVariantActionsStore) MarkVariantPending(
	ctx context.Context,
	workspaceID, variantID string,
	transformParams *string,
) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE variants
		SET storage_key = '',
		    transform_params = ?,
		    size = NULL,
		    status = 'pending',
		    thumbnail_key = NULL,
		    thumbnail_content_type = 'image/jpeg'
		WHERE id = ? AND workspace_id = ?`,
		transformParams, variantID, workspaceID,
	)
	return err
}

func (s *sqlVariantActionsStore) GetVersion(ctx context.Context, versionID string) (variantSourceVersion, error) {
	var version variantSourceVersion
	err := s.db.QueryRowContext(ctx, `SELECT version_num, storage_key, mime_type FROM asset_versions WHERE id = ?`, versionID).
		Scan(&version.VersionNum, &version.StorageKey, &version.MimeType)
	if errors.Is(err, sql.ErrNoRows) {
		return variantSourceVersion{}, apperr.ErrNotFound
	}
	return version, err
}

func (s *sqlVariantActionsStore) SetVariantStatus(ctx context.Context, variantID, workspaceID, status string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE variants SET status = ? WHERE id = ? AND workspace_id = ?`,
		status,
		variantID,
		workspaceID,
	)
	return err
}

func (s *variantService) Promote(ctx context.Context, p PromoteVariantParams) (PromoteVariantResult, error) {
	if s.actions == nil || s.storage == nil || s.queue == nil {
		return PromoteVariantResult{}, fmt.Errorf("variant promote unavailable: %w", apperr.ErrConflict)
	}

	name := strings.TrimSpace(p.Name)
	if name == "" || len(name) > 255 {
		return PromoteVariantResult{}, fmt.Errorf(
			"name is required and must be 255 characters or fewer: %w",
			apperr.ErrInvalidInput,
		)
	}

	asset, err := s.assets.GetByID(ctx, p.WorkspaceID, p.AssetID)
	if err != nil {
		return PromoteVariantResult{}, err
	}
	variant, err := s.variants.GetByID(ctx, p.WorkspaceID, p.VariantID)
	if err != nil {
		return PromoteVariantResult{}, err
	}
	if asset.CurrentVersionID == nil || variant.AssetVersionID != *asset.CurrentVersionID {
		return PromoteVariantResult{}, fmt.Errorf(
			"cannot promote a variant from a previous version: %w",
			apperr.ErrConflict,
		)
	}
	if strings.TrimSpace(variant.StorageKey) == "" {
		return PromoteVariantResult{}, fmt.Errorf("variant output is not ready: %w", apperr.ErrConflict)
	}

	mimeType := inferMimeTypeFromKey(variant.StorageKey)
	filename := promotedFilename(name, mimeType, variant.StorageKey)
	width, height := promotedDimensions(asset, variant)
	contentHash, err := hashStorageObject(ctx, s.storage, variant.StorageKey)
	if err != nil {
		return PromoteVariantResult{}, err
	}

	actor := auth.ActorFromCtx(ctx)
	newAssetID := uuid.NewString()
	newVersionID := uuid.NewString()
	result, err := s.actions.Promote(ctx, promoteVariantDBParams{
		NewAssetID:           newAssetID,
		NewVersionID:         newVersionID,
		WorkspaceID:          p.WorkspaceID,
		ProjectID:            asset.ProjectID,
		FolderID:             asset.FolderID,
		OriginalFilename:     filename,
		StorageKey:           variant.StorageKey,
		MimeType:             mimeType,
		Size:                 derefInt64(variant.Size),
		Width:                width,
		Height:               height,
		ThumbnailKey:         variant.ThumbnailKey,
		ThumbnailContentType: coalesceString(variant.ThumbnailContentType, "image/jpeg"),
		DerivedFromAssetID:   asset.ID,
		ContentHash:          contentHash,
		CreatedBy:            actor.UserID,
		SourceAssetID:        asset.ID,
		SourceVariantID:      variant.ID,
	})
	if err != nil {
		return PromoteVariantResult{}, err
	}

	if variant.ThumbnailKey == nil {
		thumbPayload, _ := json.Marshal(jobspec.VersionThumbnailJobPayload{
			AssetID:     result.NewAssetID,
			VersionID:   result.NewVersionID,
			WorkspaceID: p.WorkspaceID,
			StorageKey:  variant.StorageKey,
			MimeType:    mimeType,
		})
		if _, enqErr := s.queue.Enqueue(
			ctx,
			p.WorkspaceID,
			queue.JobTypeVersionThumbnail,
			string(thumbPayload),
		); enqErr != nil {
			slog.WarnContext(
				ctx,
				"enqueue thumbnail for promoted variant",
				keyAssetID,
				result.NewAssetID,
				"error",
				enqErr,
			)
		}
	}

	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: p.WorkspaceID,
		AssetID:     result.NewAssetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetCreated,
		Payload: audit.AssetCreatedPayload{
			V:        1,
			Filename: filename,
			Source:   "derived",
			SourceID: asset.ID,
		},
	})

	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: p.WorkspaceID,
		AssetID:     asset.ID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVariantPromoted,
		Payload:     audit.AssetVariantPromotedPayload{V: 1, VariantID: variant.ID, NewAssetID: result.NewAssetID},
	})

	return PromoteVariantResult{
		NewAssetID:  result.NewAssetID,
		NewAssetURL: fmt.Sprintf("/library?asset=%s", result.NewAssetID),
	}, nil
}

func (s *variantService) SetAsThumbnail(ctx context.Context, workspaceID, assetID, variantID string) error {
	if s.actions == nil {
		return fmt.Errorf("set thumbnail unavailable: %w", apperr.ErrConflict)
	}

	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	variant, err := s.variants.GetByID(ctx, workspaceID, variantID)
	if err != nil {
		return err
	}
	if asset.CurrentVersionID == nil || variant.AssetVersionID != *asset.CurrentVersionID {
		return fmt.Errorf("cannot set thumbnail from a previous version: %w", apperr.ErrConflict)
	}
	if variant.ThumbnailKey == nil {
		return fmt.Errorf("variant thumbnail is still generating: %w", apperr.ErrConflict)
	}

	if setErr := s.actions.SetAsThumbnail(ctx, setVariantThumbnailDBParams{
		AssetID:              assetID,
		CurrentVersionID:     variant.AssetVersionID,
		ThumbnailKey:         variant.StorageKey,
		ThumbnailContentType: inferMimeTypeFromKey(variant.StorageKey),
	}); setErr != nil {
		return setErr
	}

	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetThumbnailSetFromVariant,
		Payload:     audit.AssetThumbnailSetFromVariantPayload{V: 1, VariantID: variantID},
	})
	return nil
}

func (s *variantService) Rerun(ctx context.Context, p RerunVariantParams) error {
	if s.actions == nil || s.storage == nil || s.queue == nil {
		return fmt.Errorf("rerun unavailable: %w", apperr.ErrConflict)
	}

	asset, err := s.assets.GetByID(ctx, p.WorkspaceID, p.AssetID)
	if err != nil {
		return err
	}
	variant, err := s.variants.GetByID(ctx, p.WorkspaceID, p.VariantID)
	if err != nil {
		return err
	}
	if asset.CurrentVersionID == nil || variant.AssetVersionID != *asset.CurrentVersionID {
		return fmt.Errorf("cannot re-run a variant from a previous version: %w", apperr.ErrConflict)
	}
	if !rerunnableTypes[variant.Type] {
		return fmt.Errorf("this variant type does not support re-run: %w", apperr.ErrInvalidInput)
	}

	merged, err := mergeVariantParams(variant.TransformParams, p.NewParams)
	if err != nil {
		return fmt.Errorf("invalid rerun params: %w", apperr.ErrInvalidInput)
	}
	paramsBytes, _ := json.Marshal(merged)
	paramsStr := string(paramsBytes)

	if pendingErr := s.actions.MarkVariantPending(ctx, p.WorkspaceID, p.VariantID, &paramsStr); pendingErr != nil {
		return pendingErr
	}

	if strings.TrimSpace(variant.StorageKey) != "" {
		if delErr := s.storage.Delete(ctx, variant.StorageKey); delErr != nil {
			slog.WarnContext(ctx, "delete old variant storage before rerun", "variant_id", variant.ID, "error", delErr)
		}
	}
	if variant.ThumbnailKey != nil {
		if delErr := s.storage.Delete(ctx, *variant.ThumbnailKey); delErr != nil {
			slog.WarnContext(
				ctx,
				"delete old variant thumbnail before rerun",
				"variant_id",
				variant.ID,
				"error",
				delErr,
			)
		}
	}

	sourceVersion, err := s.actions.GetVersion(ctx, variant.AssetVersionID)
	if err != nil {
		return err
	}

	payload, _ := json.Marshal(jobspec.VariantJobPayload{
		AssetID:     asset.ID,
		WorkspaceID: p.WorkspaceID,
		VersionID:   variant.AssetVersionID,
		VersionNum:  sourceVersion.VersionNum,
		StorageKey:  sourceVersion.StorageKey,
		MimeType:    sourceVersion.MimeType,
		Type:        variant.Type,
		Params:      paramsBytes,
		VariantID:   variant.ID,
	})
	if _, enqErr := s.queue.Enqueue(ctx, p.WorkspaceID, variant.Type, string(payload)); enqErr != nil {
		return fmt.Errorf("could not enqueue variant rerun: %w", apperr.ErrConflict)
	}

	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: p.WorkspaceID,
		AssetID:     asset.ID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVariantRerun,
		Payload:     audit.AssetVariantRerunPayload{V: 1, VariantID: variant.ID, Params: merged},
	})
	return nil
}

func mergeVariantParams(existing *string, overrides map[string]any) (map[string]any, error) {
	merged := map[string]any{}
	if existing != nil && strings.TrimSpace(*existing) != "" {
		if err := json.Unmarshal([]byte(*existing), &merged); err != nil {
			return nil, err
		}
	}
	maps.Copy(merged, overrides)
	return merged, nil
}

func inferMimeTypeFromKey(storageKey string) string {
	ext := strings.ToLower(filepath.Ext(storageKey))
	if ext == "" {
		return "application/octet-stream"
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func promotedFilename(name, mimeType, storageKey string) string {
	base := strings.TrimSpace(name)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" {
		base = "variant"
	}
	ext := filepath.Ext(storageKey)
	if ext == "" {
		if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}
	return base + ext
}

func promotedDimensions(asset repository.Asset, variant repository.Variant) (*int64, *int64) {
	switch variant.Type {
	case queue.JobTypeImageResize:
		var params struct {
			Width  *int64 `json:"width"`
			Height *int64 `json:"height"`
		}
		if variant.TransformParams != nil && json.Unmarshal([]byte(*variant.TransformParams), &params) == nil {
			return params.Width, params.Height
		}
	case queue.JobTypeImageCrop:
		var params struct {
			Width  *int64 `json:"width"`
			Height *int64 `json:"height"`
		}
		if variant.TransformParams != nil && json.Unmarshal([]byte(*variant.TransformParams), &params) == nil {
			return params.Width, params.Height
		}
	}

	if strings.HasPrefix(inferMimeTypeFromKey(variant.StorageKey), "image/") {
		return asset.Width, asset.Height
	}
	return nil, nil
}

func hashStorageObject(ctx context.Context, stor interface {
	Get(ctx context.Context, key string) (io.ReadCloser, error)
}, key string) (string, error) {
	rc, err := stor.Get(ctx, key)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	h := sha256.New()
	if _, copyErr := io.Copy(h, rc); copyErr != nil {
		return "", copyErr
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func coalesceString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
