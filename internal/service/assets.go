// Package service implements business logic over repositories.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/jobspec"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	apptelemetry "damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
)

type assetService struct {
	assets     repository.AssetRepository
	versions   repository.VersionRepository
	tags       repository.TagRepository
	fields     repository.FieldRepository
	stor       storage.Storage
	audit      audit.Writer
	q          queue.JobQueue
	invalidate StorageInvalidator
}

// NewAssetService returns an AssetService backed by the given repository.
func NewAssetService(
	assets repository.AssetRepository,
	versions repository.VersionRepository,
	tags repository.TagRepository,
	fields repository.FieldRepository,
	stor storage.Storage,
	aw audit.Writer,
	q queue.JobQueue,
	inv ...StorageInvalidator,
) AssetService {
	var si StorageInvalidator
	if len(inv) > 0 {
		si = inv[0]
	}
	return &assetService{
		assets:     assets,
		versions:   versions,
		tags:       tags,
		fields:     fields,
		stor:       stor,
		audit:      aw,
		q:          q,
		invalidate: si,
	}
}

func (s *assetService) Get(ctx context.Context, workspaceID, assetID string) (*AssetDTO, error) {
	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	return toAssetDTO(asset), nil
}

func (s *assetService) List(ctx context.Context, params ListAssetsParams) (out []*AssetDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.assets.list",
		attribute.String("damask.workspace_id", params.WorkspaceID),
		attribute.Int64("damask.assets.limit", params.Limit),
		attribute.Bool("damask.assets.has_search", params.SearchQuery != ""),
		attribute.Int("damask.assets.tag_filter_count", len(params.TagNames)),
		attribute.Bool("damask.assets.has_cursor", params.CursorID != ""),
		attribute.String("damask.assets.sort_field", params.SortField),
	)
	defer func() {
		span.SetAttributes(attribute.Int("damask.assets.result_count", len(out)))
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "asset list failed", "workspace_id", params.WorkspaceID, "error", err)
		}
	}()

	// For taken_at sort, look up the exif field definition ID.
	exifFieldID := ""
	if params.SortField == "taken_at" {
		fd, fdErr := s.fields.GetByKey(ctx, params.WorkspaceID, "_exif_taken_at")
		if fdErr == nil {
			exifFieldID = fd.ID
		}
		// If the field doesn't exist, proceed with empty exifFieldID (join yields no rows).
	}

	rows, err := s.assets.List(ctx, repository.ListAssetsParams{
		WorkspaceID:  params.WorkspaceID,
		ProjectID:    params.ProjectID,
		FolderID:     params.FolderID,
		FolderIsRoot: params.FolderIsRoot,
		CollectionID: params.CollectionID,
		TagNames:     params.TagNames,
		SearchQuery:  params.SearchQuery,
		MimePrefix:   params.MimePrefix,
		SimilarToIDs: params.SimilarToIDs,
		SortField:    params.SortField,
		SortDesc:     params.SortDesc,
		CursorField:  params.CursorField,
		CursorValue:  params.CursorValue,
		CursorID:     params.CursorID,
		Limit:        params.Limit,
		ExifFieldID:  exifFieldID,
	})
	if err != nil {
		return nil, err
	}
	out = mapDTOs(rows, toAssetDTO)
	return out, nil
}

func (s *assetService) Move(ctx context.Context, workspaceID, assetID string, p MoveAssetParams) (*AssetDTO, error) {
	before, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	updated, err := s.assets.Update(ctx, repository.UpdateAssetParams{
		ID:          assetID,
		WorkspaceID: workspaceID,
		FolderID:    p.FolderID,
		ProjectID:   p.ProjectID,
	})
	if err != nil {
		return nil, err
	}
	dto := toAssetDTO(updated)
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetMoved,
		Payload: audit.AssetMovedPayload{
			V:               1,
			BeforeProjectID: before.ProjectID,
			AfterProjectID:  dto.ProjectID,
			BeforeFolderID:  before.FolderID,
			AfterFolderID:   dto.FolderID,
		},
	})
	return dto, nil
}

func (s *assetService) Rename(ctx context.Context, workspaceID, assetID, newStem string) (*AssetDTO, error) {
	newStem = strings.TrimSpace(newStem)
	if newStem == "" {
		return nil, fmt.Errorf("name cannot be empty: %w", apperr.ErrInvalidInput)
	}
	existing, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	ext := filepath.Ext(existing.OriginalFilename)
	stem := strings.TrimSuffix(newStem, ext)
	newName := stem + ext
	if newName == existing.OriginalFilename {
		return toAssetDTO(existing), nil
	}
	updated, err := s.assets.Update(ctx, repository.UpdateAssetParams{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		OriginalFilename: &newName,
	})
	if err != nil {
		return nil, err
	}
	dto := toAssetDTO(updated)
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetRenamed,
		Payload:     audit.AssetRenamedPayload{V: 1, Before: existing.OriginalFilename, After: dto.OriginalFilename},
	})
	return dto, nil
}

func (s *assetService) CountByIDs(ctx context.Context, workspaceID string, ids []string) (int64, error) {
	return s.assets.CountByIDs(ctx, workspaceID, ids)
}

func (s *assetService) RefreshFTS(ctx context.Context, assetID string) error {
	return s.assets.RefreshFTS(ctx, assetID)
}

func (s *assetService) ListByFields(ctx context.Context, params ListAssetsByFieldsParams) (out []*AssetDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.assets.list_by_fields",
		attribute.String("damask.workspace_id", params.WorkspaceID),
		attribute.Int("damask.fields.filter_count", len(params.FieldFilters)),
		attribute.Int64("damask.assets.limit", params.Limit),
		attribute.Bool("damask.assets.has_cursor", params.CursorID != nil),
	)
	defer func() {
		span.SetAttributes(attribute.Int("damask.assets.result_count", len(out)))
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"asset list by fields failed",
				"workspace_id",
				params.WorkspaceID,
				"filters",
				len(params.FieldFilters),
				"error",
				err,
			)
		}
	}()

	repoFilters := make([]repository.FieldFilter, len(params.FieldFilters))
	for i, f := range params.FieldFilters {
		repoFilters[i] = repository.FieldFilter{Key: f.Key, Operator: f.Operator, Value: f.Value}
	}
	rows, err := s.assets.ListByFields(ctx, repository.ListAssetsByFieldsParams{
		WorkspaceID:  params.WorkspaceID,
		FieldFilters: repoFilters,
		CursorAt:     params.CursorAt,
		CursorID:     params.CursorID,
		Limit:        params.Limit,
	})
	if err != nil {
		return nil, err
	}
	out = mapDTOs(rows, toAssetDTO)
	return out, nil
}

func (s *assetService) Delete(ctx context.Context, workspaceID, assetID string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.assets.delete",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"asset delete failed",
				"workspace_id",
				workspaceID,
				"asset_id",
				assetID,
				"error",
				err,
			)
		}
	}()

	isCover, err := s.assets.IsProjectCover(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if isCover {
		return fmt.Errorf("asset is used as a project cover: %w", apperr.ErrConflict)
	}
	isIcon, err := s.assets.IsWorkspaceIcon(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if isIcon {
		return fmt.Errorf("asset is used as the workspace icon: %w", apperr.ErrConflict)
	}
	return s.assets.SoftDelete(ctx, workspaceID, assetID)
}

func (s *assetService) HardDelete(ctx context.Context, workspaceID, assetID string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.assets.hard_delete",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"asset hard delete failed",
				"workspace_id",
				workspaceID,
				"asset_id",
				assetID,
				"error",
				err,
			)
		}
	}()

	isCover, err := s.assets.IsProjectCover(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if isCover {
		return fmt.Errorf("asset is used as a project cover: %w", apperr.ErrConflict)
	}
	isIcon, err := s.assets.IsWorkspaceIcon(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if isIcon {
		return fmt.Errorf("asset is used as the workspace icon: %w", apperr.ErrConflict)
	}
	keys, err := s.assets.CollectStorageKeys(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetDeleted,
		Payload:     audit.AssetDeletedPayload{V: 1},
	})
	if err = s.assets.HardDelete(ctx, workspaceID, assetID); err != nil {
		return err
	}
	s.deleteStorageKeys(ctx, keys)
	if s.invalidate != nil {
		s.invalidate.Invalidate(workspaceID)
	}
	return nil
}

func (s *assetService) BulkHardDelete(ctx context.Context, workspaceID string, assetIDs []string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.assets.bulk_hard_delete",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Int("damask.assets.requested_count", len(assetIDs)),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"asset bulk hard delete failed",
				"workspace_id",
				workspaceID,
				"asset_count",
				len(assetIDs),
				"error",
				err,
			)
		}
	}()

	type pending struct {
		keys repository.AssetStorageKeys
		id   string
	}
	var todo []pending
	for _, id := range assetIDs {
		keys, keysErr := s.assets.CollectStorageKeys(ctx, workspaceID, id)
		if keysErr != nil {
			continue // skip assets not in this workspace
		}
		todo = append(todo, pending{keys: keys, id: id})
	}
	for _, p := range todo {
		if err = s.assets.HardDelete(ctx, workspaceID, p.id); err != nil {
			return err
		}
		s.deleteStorageKeys(ctx, p.keys)
	}
	if s.invalidate != nil {
		s.invalidate.Invalidate(workspaceID)
	}
	return nil
}

func (s *assetService) deleteStorageKeys(ctx context.Context, keys repository.AssetStorageKeys) {
	_ = s.stor.Delete(ctx, keys.AssetKey)
	if keys.ThumbKey != nil {
		_ = s.stor.Delete(ctx, *keys.ThumbKey)
	}
	for _, vk := range keys.VersionKeys {
		_ = s.stor.Delete(ctx, vk.StorageKey)
		if vk.ThumbnailKey != nil {
			_ = s.stor.Delete(ctx, *vk.ThumbnailKey)
		}
		for _, k := range vk.VariantKeys {
			_ = s.stor.Delete(ctx, k)
		}
	}
	for _, key := range keys.TextTrackKeys {
		_ = s.stor.Delete(ctx, key)
	}
}

func (s *assetService) BulkSetTag(ctx context.Context, workspaceID, tagName string, assetIDs []string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.assets.bulk_tag",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Int("damask.assets.requested_count", len(assetIDs)),
		attribute.String("damask.tag_name", tagName),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"asset bulk tag failed",
				"workspace_id",
				workspaceID,
				"tag",
				tagName,
				"asset_count",
				len(assetIDs),
				"error",
				err,
			)
		}
	}()

	tag, err := s.tags.Upsert(ctx, workspaceID, tagName)
	if err != nil {
		return err
	}
	for _, assetID := range assetIDs {
		if _, getErr := s.assets.GetByID(ctx, workspaceID, assetID); getErr != nil {
			continue // skip assets not in this workspace
		}
		_ = s.tags.AddToAsset(ctx, assetID, tag.ID)
	}
	return nil
}

func (s *assetService) BulkRemoveTag(ctx context.Context, workspaceID, tagName string, assetIDs []string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.assets.bulk_remove_tag",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Int("damask.assets.requested_count", len(assetIDs)),
		attribute.String("damask.tag_name", tagName),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"asset bulk remove tag failed",
				"workspace_id",
				workspaceID,
				"tag",
				tagName,
				"asset_count",
				len(assetIDs),
				"error",
				err,
			)
		}
	}()

	for _, assetID := range assetIDs {
		if _, getErr := s.assets.GetByID(ctx, workspaceID, assetID); getErr != nil {
			continue // skip assets not in this workspace
		}
		if removeErr := s.tags.RemoveFromAsset(ctx, workspaceID, assetID, tagName); removeErr != nil {
			slog.WarnContext(ctx, "bulk remove tag: remove from asset failed", "asset_id", assetID, "error", removeErr)
		}
	}
	return nil
}

func (s *assetService) BulkMoveProject(
	ctx context.Context,
	workspaceID string,
	assetIDs []string,
	projectID *string,
) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.assets.bulk_move_project",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Int("damask.assets.requested_count", len(assetIDs)),
		attribute.Bool("damask.project.clear", projectID == nil),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"asset bulk project move failed",
				"workspace_id",
				workspaceID,
				"asset_count",
				len(assetIDs),
				"error",
				err,
			)
		}
	}()

	for _, assetID := range assetIDs {
		if _, getErr := s.assets.GetByID(ctx, workspaceID, assetID); getErr != nil {
			continue // skip assets not in this workspace
		}
		if err = s.assets.SetProject(ctx, workspaceID, assetID, projectID); err != nil {
			return err
		}
	}
	return nil
}

func (s *assetService) GetComments(ctx context.Context, workspaceID, assetID string) ([]AssetCommentDTO, error) {
	if _, err := s.assets.GetByID(ctx, workspaceID, assetID); err != nil {
		return nil, err
	}
	rows, err := s.assets.ListComments(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]AssetCommentDTO, len(rows))
	for i, c := range rows {
		out[i] = AssetCommentDTO{
			ID:          c.ID,
			AssetID:     c.AssetID,
			ShareID:     c.ShareID,
			AuthorName:  c.AuthorName,
			AuthorEmail: c.AuthorEmail,
			Body:        c.Body,
			CreatedAt:   c.CreatedAt,
		}
	}
	return out, nil
}

func (s *assetService) CountVersionsByAsset(ctx context.Context, assetID string) (int64, error) {
	return s.assets.CountVersionsByAsset(ctx, assetID)
}

func (s *assetService) CountVariantsByCurrentVersion(ctx context.Context, assetID string) (int64, error) {
	return s.assets.CountVariantsByCurrentVersion(ctx, assetID)
}

func (s *assetService) IsRebuildingVariants(ctx context.Context, versionID string) (bool, error) {
	return s.assets.IsRebuildingVariants(ctx, versionID)
}

func (s *assetService) BatchVersionCounts(ctx context.Context, assetIDs []string) (map[string]int64, error) {
	return s.assets.BatchVersionCounts(ctx, assetIDs)
}

func (s *assetService) BatchVariantCounts(ctx context.Context, assetIDs []string) (map[string]int64, error) {
	return s.assets.BatchVariantCounts(ctx, assetIDs)
}

func (s *assetService) WriteAssetDownloadedAsync(workspaceID, assetID, userID string) {
	s.audit.WriteAssetAsync(audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      &userID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetDownloaded,
		Payload:     audit.AssetDownloadedPayload{V: 1, Via: "direct"},
	})
}

// RegenerateThumbnail re-enqueues a version_thumbnail job for the asset's current version.
func (s *assetService) RegenerateThumbnail(
	ctx context.Context,
	workspaceID string,
	assetIDs []string,
) (jobIDs []string, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.assets.regenerate_thumbnail",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Int("damask.assets.requested_count", len(assetIDs)),
	)
	defer func() {
		span.SetAttributes(attribute.Int("damask.jobs.enqueued_count", len(jobIDs)))
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"asset thumbnail regeneration failed",
				"workspace_id",
				workspaceID,
				"asset_count",
				len(assetIDs),
				"error",
				err,
			)
		}
	}()

	for _, assetID := range assetIDs {
		asset, getErr := s.assets.GetByID(ctx, workspaceID, assetID)
		if getErr != nil {
			continue // skip assets not in this workspace
		}
		if asset.CurrentVersionID == nil {
			return jobIDs, fmt.Errorf("asset is has no version yet: %w", apperr.ErrNotFound)
		}

		ver, verErr := s.versions.GetCurrentByAsset(ctx, assetID)
		if verErr != nil {
			return jobIDs, fmt.Errorf("could not load current version: %w", apperr.ErrNotFound)
		}

		payload, _ := json.Marshal(jobspec.VersionThumbnailJobPayload{
			AssetID:     asset.ID,
			VersionID:   ver.ID,
			WorkspaceID: workspaceID,
			StorageKey:  ver.StorageKey,
			MimeType:    ver.MimeType,
		})

		job, enqErr := s.q.Enqueue(ctx, workspaceID, queue.JobTypeVersionThumbnail, string(payload))
		if enqErr != nil {
			return jobIDs, fmt.Errorf("could not enqueue job: %w", apperr.ErrConflict)
		}
		jobIDs = append(jobIDs, job.ID)

		actor := auth.ActorFromCtx(ctx)
		s.audit.WriteAsset(ctx, audit.AssetEvent{
			WorkspaceID: workspaceID,
			AssetID:     assetID,
			UserID:      actor.UserID,
			ActorType:   actor.Type,
			EventType:   audit.EventAssetThumbnailRegen,
		})
	}
	return jobIDs, nil
}

func toAssetDTO(a repository.Asset) *AssetDTO {
	return &AssetDTO{
		ID:                   a.ID,
		WorkspaceID:          a.WorkspaceID,
		ProjectID:            a.ProjectID,
		FolderID:             a.FolderID,
		DerivedFromAssetID:   a.DerivedFromAssetID,
		OriginalFilename:     a.OriginalFilename,
		StorageKey:           a.StorageKey,
		MimeType:             a.MimeType,
		Size:                 a.Size,
		Width:                a.Width,
		Height:               a.Height,
		ThumbnailKey:         a.ThumbnailKey,
		ThumbnailContentType: a.ThumbnailContentType,
		Metadata:             a.Metadata,
		CurrentVersionID:     a.CurrentVersionID,
		CreatedAt:            a.CreatedAt,
		UpdatedAt:            a.UpdatedAt,
	}
}
