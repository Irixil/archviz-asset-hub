package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"damask/server/internal/assetio"
	"damask/server/internal/jobspec"
	"damask/server/internal/media/ingest"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"
	"damask/server/internal/versioning"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

// AssetIngester extends assetio.Ingester with a richer return used within the service layer.
type AssetIngester interface {
	assetio.Ingester
	IngestFileWithDetails(
		ctx context.Context,
		workspaceID, filePath string,
		opts assetio.IngestFileOpts,
	) (*AssetDTO, error)
}

type ingesterImpl struct {
	assets     repository.AssetRepository
	versions   repository.VersionRepository
	stor       storage.Storage
	q          queue.JobQueue
	media      *ingest.Registry
	autoTag    AutoTagService
	duplicates DuplicateService
	tags       TagService
}

// NewAssetIngester returns an AssetIngester backed by the given dependencies.
func NewAssetIngester(
	assets repository.AssetRepository,
	versions repository.VersionRepository,
	stor storage.Storage,
	q queue.JobQueue,
	media *ingest.Registry,
	autoTag AutoTagService,
	duplicates DuplicateService,
	tags ...TagService,
) AssetIngester {
	var tagSvc TagService
	if len(tags) > 0 {
		tagSvc = tags[0]
	}
	return &ingesterImpl{
		assets: assets, versions: versions, stor: stor, q: q, media: media, autoTag: autoTag, duplicates: duplicates,
		tags: tagSvc,
	}
}

func (s *ingesterImpl) IngestFile(
	ctx context.Context,
	workspaceID, filePath string,
	opts assetio.IngestFileOpts,
) (assetio.AssetSummary, error) {
	asset, dup, err := s.ingest(ctx, workspaceID, filePath, opts)
	if err != nil {
		return assetio.AssetSummary{}, err
	}
	return assetio.AssetSummary{
		ID:               asset.ID,
		WorkspaceID:      asset.WorkspaceID,
		StorageKey:       asset.StorageKey,
		MimeType:         asset.MimeType,
		OriginalFilename: asset.OriginalFilename,
		DuplicateOf:      dup,
	}, nil
}

func (s *ingesterImpl) IngestFileWithDetails(
	ctx context.Context,
	workspaceID, filePath string,
	opts assetio.IngestFileOpts,
) (*AssetDTO, error) {
	asset, dup, err := s.ingest(ctx, workspaceID, filePath, opts)
	if err != nil {
		return nil, err
	}
	return &AssetDTO{
		ID:               asset.ID,
		WorkspaceID:      asset.WorkspaceID,
		ProjectID:        asset.ProjectID,
		FolderID:         asset.FolderID,
		OriginalFilename: asset.OriginalFilename,
		StorageKey:       asset.StorageKey,
		MimeType:         asset.MimeType,
		Size:             asset.Size,
		Width:            asset.Width,
		Height:           asset.Height,
		ThumbnailKey:     asset.ThumbnailKey,
		Metadata:         asset.Metadata,
		CurrentVersionID: asset.CurrentVersionID,
		CreatedAt:        asset.CreatedAt,
		UpdatedAt:        asset.UpdatedAt,
		DuplicateOf:      dup,
	}, nil
}

// ingest is the shared implementation called by IngestFile and IngestFileFull.
//

func (s *ingesterImpl) ingest(
	ctx context.Context,
	workspaceID, filePath string,
	opts assetio.IngestFileOpts,
) (asset repository.Asset, dup *assetio.DuplicateMatch, err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.ingester.ingest",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Bool("damask.upload.has_project", opts.ProjectID != nil),
		attribute.Bool("damask.upload.has_folder", opts.FolderID != nil),
	)
	defer func() {
		if asset.ID != "" {
			span.SetAttributes(
				attribute.String("damask.asset_id", asset.ID),
				attribute.String("damask.mime_type", asset.MimeType),
				attribute.Int64("damask.asset.size", asset.Size),
			)
		}
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "asset ingest failed", keyWorkspaceID, workspaceID, "error", err)
		}
	}()

	slog.DebugContext(
		ctx,
		"starting asset ingest",
		keyWorkspaceID, workspaceID,
		"file_path", filePath,
		"project_id", opts.ProjectID,
		"folder_id", opts.FolderID,
		"user_id", opts.UserID,
		"original_name", opts.OriginalName,
	)

	stat, err := os.Stat(filePath)
	if err != nil {
		return repository.Asset{}, nil, fmt.Errorf("could not stat uploaded file: %w", err)
	}
	span.SetAttributes(attribute.Int64("damask.upload.bytes", stat.Size()))

	mimeType, err := transform.DetectMimeType(filePath)
	if err != nil {
		return repository.Asset{}, nil, fmt.Errorf("could not detect MIME type: %w", err)
	}
	span.SetAttributes(attribute.String("damask.mime_type", mimeType))

	assetID := uuid.New().String()
	originalFilename := filepath.Base(filePath)
	if opts.OriginalName != "" {
		originalFilename = opts.OriginalName
	}
	storageKey := fmt.Sprintf("%s/%s/%s", workspaceID, assetID, originalFilename)

	f, err := os.Open(filePath)
	if err != nil {
		return repository.Asset{}, nil, fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()
	_, storeSpan := telemetry.StartSpan(ctx, "service.ingester.storage_put",
		attribute.String("damask.storage.key", storageKey),
		attribute.Int64("damask.upload.bytes", stat.Size()),
	)
	err = s.stor.Put(ctx, storageKey, f)
	telemetry.EndSpan(storeSpan, err)
	if err != nil {
		return repository.Asset{}, nil, fmt.Errorf("could not store file: %w", err)
	}

	meta := ingest.FileMeta{}
	if s.media.Supports(mimeType) {
		metaCtx, metaSpan := telemetry.StartSpan(ctx, "service.ingester.extract_metadata",
			attribute.String("damask.mime_type", mimeType),
		)
		if m, merr := s.media.ExtractMeta(ctx, filePath, mimeType); merr == nil {
			meta = m
		} else {
			telemetry.RecordError(metaSpan, merr)
			slog.WarnContext(metaCtx, "metadata extraction failed", keyMimeType, mimeType, "error", merr)
		}
		metaSpan.End()
	} else {
		slog.DebugContext(
			ctx,
			"no handler for MIME type, skipping metadata extraction",
			keyMimeType,
			mimeType,
		)
	}

	_, createSpan := telemetry.StartSpan(ctx, "service.ingester.create_asset")
	asset, err = s.assets.Create(ctx, repository.CreateAssetParams{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		ProjectID:        opts.ProjectID,
		FolderID:         opts.FolderID,
		OriginalFilename: originalFilename,
		StorageKey:       storageKey,
		MimeType:         mimeType,
		Size:             stat.Size(),
		Width:            meta.Width,
		Height:           meta.Height,
	})
	telemetry.EndSpan(createSpan, err)
	if err != nil {
		return repository.Asset{}, nil, fmt.Errorf("could not save asset: %w", err)
	}

	slog.DebugContext(ctx, "created asset", keyAssetID, asset.ID, keyMimeType, asset.MimeType, keySize, asset.Size)

	initialVersionID, contentHash, vErr := s.createInitialVersion(
		ctx, asset, filePath, storageKey, mimeType, meta, opts.UserID,
	)
	if vErr != nil {
		slog.ErrorContext(ctx, "create initial version", keyAssetID, asset.ID, "error", vErr)
	}

	if vErr == nil && contentHash != "" {
		if dup, err = s.checkDuplicate(ctx, workspaceID, asset, storageKey, contentHash); err != nil {
			return repository.Asset{}, nil, err
		}
	}

	s.applyPrimaryCategory(ctx, asset)

	if opts.InheritFields != nil && opts.ProjectID != nil && opts.UserID != "" {
		inheritCtx, inheritSpan := telemetry.StartSpan(ctx, "service.ingester.inherit_project_fields",
			attribute.String("damask.asset_id", asset.ID),
			attribute.String("damask.project_id", *opts.ProjectID),
		)
		opts.InheritFields(inheritCtx, workspaceID, asset.ID, *opts.ProjectID, opts.UserID)
		inheritSpan.End()
	}

	slog.DebugContext(
		ctx,
		"asset ingest completed",
		keyAssetID,
		asset.ID,
		keyWorkspaceID,
		workspaceID,
		keyMimeType,
		asset.MimeType,
		keySize,
		asset.Size,
		"supported_media",
		s.media.Supports(mimeType),
	)

	// once created, we can enqueue specialized jobs for this asset
	s.enqueueIngestionJobs(ctx, asset, workspaceID, mimeType, initialVersionID, opts.UserID)

	return asset, dup, nil
}

func (s *ingesterImpl) applyPrimaryCategory(ctx context.Context, asset repository.Asset) {
	if s.tags == nil {
		return
	}
	category := PrimaryAssetCategory(asset.OriginalFilename, asset.MimeType)
	if category == "" {
		return
	}
	if err := s.tags.UpsertForAsset(ctx, asset.WorkspaceID, asset.ID, category); err != nil {
		// The file is already durable. Keep it visible as "needs review" so an
		// editor can retry classification without uploading a duplicate.
		slog.WarnContext(ctx, "automatic asset classification failed",
			keyAssetID, asset.ID, "category", category, "error", err)
	}
}

// checkDuplicate runs content-hash duplicate detection for a just-created
// asset+version. In "block" mode with a match found, it rolls back the asset
// (and its cascade-deleted version row) plus the just-written storage object,
// and returns a *DuplicateConflictError. In "warn" mode it returns the match
// for the caller to attach to the response. Any failure in the dedup
// plumbing itself (mode lookup, FindDuplicate) is logged and treated as if
// detection were off, so a broken dedup path never blocks or drops an upload.
func (s *ingesterImpl) checkDuplicate(
	ctx context.Context,
	workspaceID string,
	asset repository.Asset,
	storageKey, contentHash string,
) (*assetio.DuplicateMatch, error) {
	if s.duplicates == nil {
		return nil, nil //nolint:nilnil // no dedup service wired means detection is off
	}

	mode, err := s.duplicates.Mode(ctx, workspaceID)
	if err != nil {
		slog.WarnContext(ctx, "duplicate mode lookup failed, defaulting to warn", "error", err)
		mode = DuplicateModeWarn
	}
	if mode == DuplicateModeOff {
		return nil, nil //nolint:nilnil // detection disabled for this workspace
	}

	dup, err := s.duplicates.FindDuplicate(ctx, workspaceID, contentHash, asset.ID)
	if err != nil {
		slog.WarnContext(ctx, "duplicate check failed, skipping", keyAssetID, asset.ID, "error", err)
		return nil, nil //nolint:nilnil // never block/lose an upload over a lookup bug
	}
	if dup == nil {
		return nil, nil //nolint:nilnil // no match found
	}

	if mode != DuplicateModeBlock {
		return dup, nil
	}

	if delErr := s.assets.HardDelete(ctx, workspaceID, asset.ID); delErr != nil {
		slog.ErrorContext(ctx, "failed to roll back blocked duplicate upload", keyAssetID, asset.ID, "error", delErr)
	}
	if delErr := s.stor.Delete(ctx, storageKey); delErr != nil {
		slog.ErrorContext(ctx, "failed to remove storage for blocked duplicate upload",
			keyAssetID, asset.ID, "storage_key", storageKey, "error", delErr)
	}
	return nil, &DuplicateConflictError{Match: *dup}
}

func (s *ingesterImpl) enqueueIngestionJobs(
	ctx context.Context,
	asset repository.Asset,
	workspaceID, mimeType, initialVersionID, userID string,
) {
	enqueue := func(spanName, logMsg, jobType string, payload any) {
		data, _ := json.Marshal(payload)
		_, span := telemetry.StartSpan(ctx, spanName,
			attribute.String("damask.asset_id", asset.ID),
			attribute.String("damask.job.type", jobType),
		)
		_, err := s.q.Enqueue(ctx, workspaceID, jobType, string(data))
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, logMsg, keyAssetID, asset.ID, "error", err)
		}
	}

	if s.media.Supports(mimeType) && initialVersionID != "" {
		enqueue("service.ingester.enqueue_thumbnail", "enqueue version thumbnail",
			queue.JobTypeVersionThumbnail, jobspec.VersionThumbnailJobPayload{
				AssetID:     asset.ID,
				VersionID:   initialVersionID,
				WorkspaceID: asset.WorkspaceID,
				StorageKey:  asset.StorageKey,
				MimeType:    asset.MimeType,
			})
	}
	if transform.IsImageMime(mimeType) {
		enqueue("service.ingester.enqueue_exif", "enqueue extract_exif",
			queue.JobTypeExtractExif, jobspec.ExtractExifPayload{
				AssetID:     asset.ID,
				WorkspaceID: workspaceID,
				UserID:      userID,
			})
	}
	if strings.HasPrefix(mimeType, "audio/") || strings.HasPrefix(mimeType, "video/") {
		enqueue("service.ingester.enqueue_media_tags", "enqueue extract_media_tags",
			queue.JobTypeExtractMediaTags, jobspec.ExtractMediaTagsPayload{
				AssetID:     asset.ID,
				WorkspaceID: workspaceID,
			})
	}
	if transform.IsPdfMime(mimeType) {
		enqueue("service.ingester.enqueue_extract_text", "enqueue extract_text",
			queue.JobTypeExtractPDFTextTrack, jobspec.ExtractTextPayload{
				AssetID:     asset.ID,
				WorkspaceID: workspaceID,
				StorageKey:  asset.StorageKey,
			})
	}
	if transform.IsTextMime(mimeType) {
		enqueue("service.ingester.enqueue_extract_text", "enqueue extract_text",
			queue.JobTypeExtractPlainTextTrack, jobspec.ExtractTextPayload{
				AssetID:     asset.ID,
				WorkspaceID: workspaceID,
				StorageKey:  asset.StorageKey,
			})
	}
	if transform.IsDocumentMime(mimeType) {
		enqueue("service.ingester.enqueue_extract_text", "enqueue extract_document_text",
			queue.JobTypeExtractDocumentTextTrack, jobspec.ExtractTextPayload{
				AssetID:     asset.ID,
				WorkspaceID: workspaceID,
				StorageKey:  asset.StorageKey,
				MimeType:    mimeType,
			})
	}
	if err := s.autoTag.Enqueue(ctx, workspaceID, asset.ID, false); err != nil {
		slog.WarnContext(ctx, "auto_tag: enqueue failed", keyAssetID, asset.ID, "error", err)
	}
}

func (s *ingesterImpl) createInitialVersion(
	ctx context.Context,
	asset repository.Asset,
	filePath, storageKey, mimeType string,
	meta ingest.FileMeta,
	userID string,
) (versionID, contentHash string, err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.ingester.create_initial_version",
		attribute.String("damask.workspace_id", asset.WorkspaceID),
		attribute.String("damask.asset_id", asset.ID),
		attribute.Int64("damask.asset.size", asset.Size),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "create initial version failed", keyAssetID, asset.ID, "error", err)
		}
	}()

	hash, err := versioning.HashFile(filePath)
	if err != nil {
		return "", "", fmt.Errorf("hash file: %w", err)
	}

	versionID = uuid.NewString()

	var createdByPtr *string
	if userID != "" {
		createdByPtr = &userID
	}

	if err = s.versions.RunInTx(ctx, func(tx repository.VersionRepository) error {
		if _, createErr := tx.Create(ctx, repository.AssetVersion{
			ID:          versionID,
			AssetID:     asset.ID,
			WorkspaceID: asset.WorkspaceID,
			VersionNum:  1,
			StorageKey:  storageKey,
			ContentHash: hash,
			MimeType:    mimeType,
			Size:        asset.Size,
			Width:       meta.Width,
			Height:      meta.Height,
			DurationSec: meta.DurationSec,
			CreatedBy:   createdByPtr,
		}); createErr != nil {
			return fmt.Errorf(
				"create version row (asset_id, workspace_id, created_by) (%s, %s, %v): %w",
				asset.ID,
				asset.WorkspaceID,
				createdByPtr,
				createErr,
			)
		}

		// SetCurrent atomically flips the version's is_current flag and the asset's
		// current_version_id (see repository.VersionRepository.SetCurrent). Wrapping it
		// in the same tx as Create avoids an orphaned version row if this step fails.
		if setErr := tx.SetCurrent(ctx, asset.ID, versionID); setErr != nil {
			return fmt.Errorf("set current_version_id: %w", setErr)
		}
		return nil
	}); err != nil {
		return "", "", err
	}

	return versionID, hash, nil
}
