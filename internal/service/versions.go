package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
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

const maxCommentLength = 500

// VersionDTO is the output of VersionService methods.
type VersionDTO struct {
	ID           string
	AssetID      string
	WorkspaceID  string
	VersionNum   int64
	StorageKey   string
	ContentHash  string
	MimeType     string
	Size         int64
	Width        *int64
	Height       *int64
	DurationSec  *float64
	ThumbnailKey *string
	Comment      *string
	CreatedBy    *string
	CreatedAt    time.Time
	IsCurrent    bool
	DeletedAt    *string
}

type versionService struct {
	versions   repository.VersionRepository
	assets     repository.AssetRepository
	storage    storage.Storage
	queue      queue.JobQueue
	media      *ingest.Registry
	audit      audit.Writer
	triggers   WorkflowTriggerPublisher
	invalidate StorageInvalidator
	autoTag    AutoTagService
}

type VersionServiceDeps struct {
	Assets     repository.AssetRepository
	Storage    storage.Storage
	Queue      queue.JobQueue
	Media      *ingest.Registry
	Triggers   WorkflowTriggerPublisher
	Invalidate StorageInvalidator
	// AutoTag, when set, is used to dismiss stale tag suggestions and
	// re-trigger auto-tagging when a new version is uploaded for an asset.
	AutoTag AutoTagService
}

// NewVersionService returns a VersionService.
func NewVersionService(
	versions repository.VersionRepository,
	aw audit.Writer,
	deps ...VersionServiceDeps,
) VersionService {
	cfg := VersionServiceDeps{}
	if len(deps) > 0 {
		cfg = deps[0]
	}
	return &versionService{
		versions:   versions,
		assets:     cfg.Assets,
		storage:    cfg.Storage,
		queue:      cfg.Queue,
		media:      cfg.Media,
		audit:      aw,
		triggers:   workflowTriggerPublisherOrNop(cfg.Triggers),
		invalidate: cfg.Invalidate,
		autoTag:    cfg.AutoTag,
	}
}

func (s *versionService) List(ctx context.Context, assetID string) ([]*VersionDTO, error) {
	rows, err := s.versions.ListByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toVersionDTO)
	return out, nil
}

func (s *versionService) Get(ctx context.Context, workspaceID, id string) (*VersionDTO, error) {
	v, err := s.versions.GetByIDForWorkspace(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

func (s *versionService) GetCurrentByAsset(ctx context.Context, assetID string) (*VersionDTO, error) {
	v, err := s.versions.GetCurrentByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

func (s *versionService) GetFirstByAsset(ctx context.Context, assetID string) (*VersionDTO, error) {
	v, err := s.versions.GetFirstByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

func (s *versionService) ListWithVariantCount(ctx context.Context, assetID string) ([]*VersionWithCountDTO, error) {
	rows, err := s.versions.ListWithVariantCount(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]*VersionWithCountDTO, len(rows))
	for i, r := range rows {
		out[i] = &VersionWithCountDTO{
			VersionDTO:   *toVersionDTO(r.AssetVersion),
			VariantCount: r.VariantCount,
		}
	}
	return out, nil
}

func (s *versionService) GetByHash(ctx context.Context, assetID, contentHash string) (*VersionDTO, error) {
	v, err := s.versions.GetByHash(ctx, assetID, contentHash)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

func (s *versionService) NextVersionNum(ctx context.Context, assetID string) (int64, error) {
	return s.versions.NextVersionNum(ctx, assetID)
}

func (s *versionService) Create(ctx context.Context, v *VersionDTO) (out *VersionDTO, err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.versions.create",
		attribute.String("damask.workspace_id", v.WorkspaceID),
		attribute.String("damask.asset_id", v.AssetID),
		attribute.Int64("damask.version.number", v.VersionNum),
		attribute.Int64("damask.version.size", v.Size),
	)
	defer func() {
		if out != nil {
			span.SetAttributes(attribute.String("damask.version_id", out.ID))
		}
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"version create failed",
				"workspace_id",
				v.WorkspaceID,
				"asset_id",
				v.AssetID,
				"error",
				err,
			)
		}
	}()

	created, err := s.versions.Create(ctx, repository.AssetVersion{
		ID:          v.ID,
		AssetID:     v.AssetID,
		WorkspaceID: v.WorkspaceID,
		VersionNum:  v.VersionNum,
		StorageKey:  v.StorageKey,
		ContentHash: v.ContentHash,
		MimeType:    v.MimeType,
		Size:        v.Size,
		Width:       v.Width,
		Height:      v.Height,
		DurationSec: v.DurationSec,
		Comment:     v.Comment,
		CreatedBy:   v.CreatedBy,
	})
	if err != nil {
		return nil, err
	}
	return toVersionDTO(created), nil
}

func (s *versionService) UploadNewVersion(
	ctx context.Context,
	p UploadAssetVersionParams,
) (out *UploadAssetVersionResult, err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.versions.upload_new",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.String("damask.asset_id", p.AssetID),
		attribute.String("damask.filename", p.Filename),
	)
	defer func() {
		if out != nil && out.Version != nil {
			span.SetAttributes(attribute.String("damask.version_id", out.Version.ID))
		}
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "version upload failed",
				"workspace_id", p.WorkspaceID, "asset_id", p.AssetID,
				"filename", p.Filename, "error", err)
		}
	}()

	comment, err := s.validateUploadNewVersionParams(p)
	if err != nil {
		return nil, err
	}

	asset, err := s.assets.GetByID(ctx, p.WorkspaceID, p.AssetID)
	if err != nil {
		return nil, err
	}

	tmpPath, cleanup, err := writeTempFile(p.Reader, filepath.Ext(p.Filename))
	if err != nil {
		return nil, err
	}
	defer cleanup()

	nextNum, err := s.versions.NextVersionNum(ctx, p.AssetID)
	if err != nil {
		return nil, fmt.Errorf("could not determine version number: %w", err)
	}

	storageKey, hash, size, err := s.resolveStorageKey(ctx, p.AssetID, p.WorkspaceID, nextNum, p.Filename, tmpPath)
	if err != nil {
		return nil, err
	}

	mimeType := resolveVersionMimeType(tmpPath, p.ContentType, p.Filename)
	meta := s.extractVersionMeta(ctx, tmpPath, mimeType, p.AssetID)

	created, err := s.createAndPromoteVersion(ctx, p, nextNum, storageKey, hash, mimeType, size, comment, meta)
	if err != nil {
		return nil, err
	}

	newVersion := toVersionDTO(created)
	newVersion.IsCurrent = true

	s.handleNewVersionSideEffects(ctx, p.WorkspaceID, p.AssetID, newVersion.ID)

	updatedAsset, err := s.assets.GetByID(ctx, p.WorkspaceID, p.AssetID)
	if err != nil {
		return nil, fmt.Errorf("could not reload asset: %w", err)
	}

	s.publishVersionEvent(ctx, asset, updatedAsset, newVersion, comment, mimeType)

	return &UploadAssetVersionResult{
		Asset:   toAssetDTO(updatedAsset),
		Version: newVersion,
	}, nil
}

// createAndPromoteVersion creates the new version row and promotes it to
// current within a single transaction, avoiding an orphaned version row (and
// asset.current_version_id left unset) if SetCurrent fails.
func (s *versionService) createAndPromoteVersion(
	ctx context.Context,
	p UploadAssetVersionParams,
	nextNum int64,
	storageKey, hash, mimeType string,
	size int64,
	comment string,
	meta ingest.FileMeta,
) (repository.AssetVersion, error) {
	var commentPtr *string
	if comment != "" {
		commentPtr = &comment
	}
	createdBy := p.UserID
	newVersionID := uuid.NewString()

	var created repository.AssetVersion
	txErr := s.versions.RunInTx(ctx, func(tx repository.VersionRepository) error {
		var createErr error
		created, createErr = tx.Create(ctx, repository.AssetVersion{
			ID:          newVersionID,
			AssetID:     p.AssetID,
			WorkspaceID: p.WorkspaceID,
			VersionNum:  nextNum,
			StorageKey:  storageKey,
			ContentHash: hash,
			MimeType:    mimeType,
			Size:        size,
			Width:       meta.Width,
			Height:      meta.Height,
			DurationSec: meta.DurationSec,
			Comment:     commentPtr,
			CreatedBy:   &createdBy,
		})
		if createErr != nil {
			return fmt.Errorf("could not create version: %w", createErr)
		}
		if setCurrErr := tx.SetCurrent(ctx, p.AssetID, newVersionID); setCurrErr != nil {
			return fmt.Errorf("could not promote version: %w", setCurrErr)
		}
		return nil
	})
	return created, txErr
}

// handleNewVersionSideEffects clears the stale thumbnail and refreshes
// auto-tag suggestions after a new version is promoted to current. Both are
// best-effort: failures are logged, not returned, since the version itself
// was already committed successfully.
func (s *versionService) handleNewVersionSideEffects(ctx context.Context, workspaceID, assetID, versionID string) {
	if thumbErr := s.SetAssetThumbnail(ctx, assetID, nil); thumbErr != nil {
		slog.ErrorContext(ctx, "clear asset thumbnail",
			"asset_id", assetID, "version_id", versionID, "error", thumbErr)
	}

	if s.autoTag == nil {
		return
	}
	// Suggestions from the previous version are stale; clear them and
	// let a fresh auto_tag run (if enabled) populate suggestions for
	// the new version.
	if dismissErr := s.autoTag.DismissAll(ctx, workspaceID, assetID); dismissErr != nil {
		slog.WarnContext(ctx, "dismiss stale auto-tag suggestions",
			"asset_id", assetID, "version_id", versionID, "error", dismissErr)
	}
	if enqErr := s.autoTag.Enqueue(ctx, workspaceID, assetID, false); enqErr != nil {
		slog.WarnContext(ctx, "enqueue auto_tag for new version",
			"asset_id", assetID, "version_id", versionID, "error", enqErr)
	}
}

func writeTempFile(r io.Reader, ext string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "damask-version-*"+ext)
	if err != nil {
		return "", func() {}, fmt.Errorf("cannot create temp file: %w", err)
	}
	name := f.Name()
	remove := func() { _ = os.Remove(name) }
	if _, err = io.Copy(f, r); err != nil {
		_ = f.Close()
		remove()
		return "", func() {}, fmt.Errorf("cannot write temp file: %w", err)
	}
	if err = f.Close(); err != nil {
		remove()
		return "", func() {}, fmt.Errorf("cannot close temp file: %w", err)
	}
	return name, remove, nil
}

func (s *versionService) resolveStorageKey(
	ctx context.Context,
	assetID, workspaceID string,
	nextNum int64,
	filename, tmpPath string,
) (storageKey, hash string, size int64, err error) {
	hashFile, err := os.Open(tmpPath)
	if err != nil {
		return "", "", 0, fmt.Errorf("could not open uploaded file: %w", err)
	}
	hash, size, err = versioning.HashReader(hashFile)
	_ = hashFile.Close()
	if err != nil {
		return "", "", 0, fmt.Errorf("could not hash file: %w", err)
	}

	existing, hashErr := s.versions.GetByHash(ctx, assetID, hash)
	if hashErr == nil {
		if existing.IsCurrent {
			return "", "", 0, fmt.Errorf("this file is identical to the current version: %w", apperr.ErrConflict)
		}
		return existing.StorageKey, hash, size, nil
	}

	key := fmt.Sprintf("%s/%s/v%d/%s", workspaceID, assetID, nextNum, filename)
	storeFile, err := os.Open(tmpPath)
	if err != nil {
		return "", "", 0, fmt.Errorf("could not reopen uploaded file: %w", err)
	}
	putErr := s.storage.Put(ctx, key, storeFile)
	_ = storeFile.Close()
	if putErr != nil {
		return "", "", 0, fmt.Errorf("could not store file: %w", putErr)
	}
	return key, hash, size, nil
}

func (s *versionService) publishVersionEvent(
	ctx context.Context,
	asset, updatedAsset repository.Asset,
	version *VersionDTO,
	comment, mimeType string,
) {
	s.enqueueVersionThumbnail(ctx, asset, version)
	s.enqueueVersionMediaTags(ctx, updatedAsset.WorkspaceID, updatedAsset.ID, version.ID, mimeType)
	s.WriteVersionUploaded(ctx, updatedAsset.WorkspaceID, updatedAsset.ID, version, comment)
	publishWorkflowTriggerAsync(ctx, s.triggers, "trigger.version_uploaded", workflowAssetTrigger{
		AssetID:          updatedAsset.ID,
		WorkspaceID:      updatedAsset.WorkspaceID,
		ProjectID:        ptrStr(updatedAsset.ProjectID),
		FolderID:         ptrStr(updatedAsset.FolderID),
		MimeType:         version.MimeType,
		Size:             version.Size,
		OriginalFilename: updatedAsset.OriginalFilename,
		VersionID:        version.ID,
		VersionNum:       version.VersionNum,
		StorageKey:       version.StorageKey,
	}.toMap())
	if s.invalidate != nil {
		s.invalidate.Invalidate(updatedAsset.WorkspaceID)
	}
}

// validateUploadNewVersionParams checks required deps and fields for
// UploadNewVersion, and returns the trimmed comment to persist.
func (s *versionService) validateUploadNewVersionParams(p UploadAssetVersionParams) (string, error) {
	if valErr := s.validateUploadNewVersionDeps(); valErr != nil {
		return "", valErr
	}
	if p.WorkspaceID == "" || p.AssetID == "" || p.Filename == "" || p.UserID == "" || p.Reader == nil {
		return "", fmt.Errorf(
			"workspace_id, asset_id, filename, user_id, and reader are required: %w",
			apperr.ErrInvalidInput,
		)
	}

	comment := strings.TrimSpace(p.Comment)
	if len(comment) > maxCommentLength {
		return "", fmt.Errorf("comment must be 500 characters or fewer: %w", apperr.ErrInvalidInput)
	}
	return comment, nil
}

func (s *versionService) validateUploadNewVersionDeps() error {
	if s.assets == nil {
		return errors.New("version upload requires asset repository (misconfigured service)")
	}
	if s.storage == nil {
		return errors.New("version upload requires storage (misconfigured service)")
	}
	if s.queue == nil {
		return errors.New("version upload requires queue (misconfigured service)")
	}
	return nil
}

func (s *versionService) enqueueVersionThumbnail(ctx context.Context, asset repository.Asset, version *VersionDTO) {
	payload, _ := json.Marshal(jobspec.VersionThumbnailJobPayload{
		AssetID:     asset.ID,
		VersionID:   version.ID,
		WorkspaceID: asset.WorkspaceID,
		StorageKey:  version.StorageKey,
		MimeType:    version.MimeType,
	})
	if _, err := s.queue.Enqueue(ctx, asset.WorkspaceID, queue.JobTypeVersionThumbnail, string(payload)); err != nil {
		slog.ErrorContext(
			ctx,
			"enqueue version thumbnail",
			"asset_id",
			asset.ID,
			"version_id",
			version.ID,
			"error",
			err,
		)
	}
}

func resolveVersionMimeType(tmpPath, contentType, filename string) string {
	if mt, _ := transform.DetectMimeType(tmpPath); mt != "" {
		return mt
	}
	if contentType != "" {
		return contentType
	}
	return mime.TypeByExtension(filepath.Ext(filename))
}

func (s *versionService) extractVersionMeta(ctx context.Context, tmpPath, mimeType, assetID string) ingest.FileMeta {
	if s.media == nil {
		return ingest.FileMeta{}
	}
	extracted, metaErr := s.media.ExtractMeta(ctx, tmpPath, mimeType)
	if metaErr != nil {
		slog.WarnContext(ctx, "version metadata extraction failed",
			"asset_id", assetID, "mime_type", mimeType, "error", metaErr)
		return ingest.FileMeta{}
	}
	return extracted
}

func (s *versionService) enqueueVersionMediaTags(
	ctx context.Context,
	workspaceID, assetID, versionID, mimeType string,
) {
	if !strings.HasPrefix(mimeType, "audio/") && !strings.HasPrefix(mimeType, "video/") {
		return
	}
	payload, _ := json.Marshal(jobspec.ExtractMediaTagsPayload{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
	})
	if _, err := s.queue.Enqueue(ctx, workspaceID, queue.JobTypeExtractMediaTags, string(payload)); err != nil {
		slog.ErrorContext(ctx, "enqueue extract_media_tags",
			"asset_id", assetID, "version_id", versionID, "error", err)
	}
}

func (s *versionService) SetCurrent(ctx context.Context, assetID, versionID string) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.versions.set_current",
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.version_id", versionID),
	)
	defer func() { telemetry.EndSpan(span, err) }()
	return s.versions.SetCurrent(ctx, assetID, versionID)
}

func (s *versionService) SetAssetThumbnail(ctx context.Context, assetID string, key *string) error {
	return s.versions.SetAssetThumbnail(ctx, assetID, key)
}

// Delete soft-deletes a non-current version that is not in use as a cover.
func (s *versionService) Delete(ctx context.Context, workspaceID, assetID, versionID string) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.versions.delete",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.version_id", versionID),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"version delete failed",
				"workspace_id",
				workspaceID,
				"asset_id",
				assetID,
				"version_id",
				versionID,
				"error",
				err,
			)
		}
	}()

	v, err := s.versions.GetByIDForWorkspace(ctx, workspaceID, versionID)
	if err != nil {
		return err
	}
	if v.AssetID != assetID {
		return fmt.Errorf("version %q: %w", versionID, apperr.ErrNotFound)
	}
	if v.IsCurrent {
		return fmt.Errorf("cannot delete the current version: %w", apperr.ErrInvalidInput)
	}
	isCover, err := s.versions.IsReferencedAsCover(ctx, versionID)
	if err != nil {
		return err
	}
	if isCover {
		return fmt.Errorf("version is in use as a project cover or workspace icon: %w", apperr.ErrConflict)
	}
	if delErr := s.versions.SoftDelete(ctx, versionID); delErr != nil {
		return delErr
	}
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVersionDeleted,
		Payload:     audit.AssetVersionDeletedPayload{V: 1, VersionNum: v.VersionNum},
	})
	if s.invalidate != nil {
		s.invalidate.Invalidate(workspaceID)
	}
	return nil
}

// WriteVersionUploaded emits an asset_version_uploaded audit event.
// Called by handlers that orchestrate the multi-step upload flow.
func (s *versionService) WriteVersionUploaded(
	ctx context.Context,
	workspaceID, assetID string,
	v *VersionDTO,
	comment string,
) {
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVersionUploaded,
		Payload:     audit.AssetVersionUploadedPayload{V: 1, VersionNum: v.VersionNum, Size: v.Size, Comment: comment},
	})
}

// WriteVersionRestored emits an asset_version_restored audit event.
// Called by handlers after SetCurrent succeeds.
func (s *versionService) WriteVersionRestored(
	ctx context.Context,
	workspaceID, assetID string,
	fromVersionNum, toVersionNum int64,
) {
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVersionRestored,
		Payload: audit.AssetVersionRestoredPayload{
			V:              1,
			FromVersionNum: fromVersionNum,
			ToVersionNum:   toVersionNum,
		},
	})
}

func toVersionDTO(v repository.AssetVersion) *VersionDTO {
	return &VersionDTO{
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
		CreatedAt:    v.CreatedAt,
		IsCurrent:    v.IsCurrent,
		DeletedAt:    v.DeletedAt,
	}
}
