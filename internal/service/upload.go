package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"damask/server/internal/apperr"
	"damask/server/internal/assetio"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	apptelemetry "damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
)

// UploadMeta holds caller-supplied metadata for a file upload.
type UploadMeta struct {
	OriginalFilename string
	ProjectID        *string
	FolderID         *string
	UserID           string
	// InheritFields is called after asset creation to copy project field values.
	// May be nil.
	InheritFields assetio.FieldInheritanceFunc
}

type uploadServiceImpl struct {
	ingester   AssetIngester
	audit      audit.Writer
	triggers   WorkflowTriggerPublisher
	invalidate StorageInvalidator
}

// NewUploadService returns an UploadService. Pass a non-nil inv to invalidate
// the storage usage cache after each successful ingest; pass nil to skip.
func NewUploadService(
	ingester AssetIngester,
	aw audit.Writer,
	inv StorageInvalidator,
	triggers ...WorkflowTriggerPublisher,
) UploadService {
	return &uploadServiceImpl{
		ingester:   ingester,
		audit:      aw,
		triggers:   workflowTriggerPublisherOrNop(triggers...),
		invalidate: inv,
	}
}

// Ingest writes r to a temp file, calls AssetIngester.IngestFileFull, then removes the temp file.
// Queue enqueue failures are logged but do not fail the upload (fire-and-forget).
func (s *uploadServiceImpl) Ingest(
	ctx context.Context,
	workspaceID string,
	r io.Reader,
	meta UploadMeta,
) (asset *AssetDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.upload.ingest",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Bool("damask.upload.has_project", meta.ProjectID != nil),
		attribute.Bool("damask.upload.has_folder", meta.FolderID != nil),
	)
	defer func() {
		if asset != nil {
			span.SetAttributes(attribute.String("damask.asset_id", asset.ID))
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"upload ingest failed",
				"workspace_id",
				workspaceID,
				"filename",
				meta.OriginalFilename,
				"error",
				err,
			)
		}
	}()

	if workspaceID == "" {
		return nil, fmt.Errorf("workspaceID is required: %w", apperr.ErrInvalidInput)
	}
	if meta.OriginalFilename == "" {
		return nil, fmt.Errorf("filename is required: %w", apperr.ErrInvalidInput)
	}

	tmpF, err := os.CreateTemp("", "damask-upload-*"+filepath.Ext(meta.OriginalFilename))
	if err != nil {
		return nil, fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmpF.Name()
	defer os.Remove(tmpPath)

	_, copySpan := apptelemetry.StartSpan(ctx, "service.upload.write_temp")
	var written int64
	written, err = io.Copy(tmpF, r)
	copySpan.SetAttributes(attribute.Int64("damask.upload.bytes", written))
	apptelemetry.EndSpan(copySpan, err)
	if err != nil {
		_ = tmpF.Close()
		return nil, fmt.Errorf("cannot write temp file: %w", err)
	}
	_ = tmpF.Close()

	asset, err = s.ingester.IngestFileWithDetails(ctx, workspaceID, tmpPath, assetio.IngestFileOpts{
		ProjectID:     meta.ProjectID,
		FolderID:      meta.FolderID,
		UserID:        meta.UserID,
		InheritFields: meta.InheritFields,
		OriginalName:  meta.OriginalFilename,
	})
	if err != nil {
		return nil, err
	}

	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     asset.ID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetCreated,
		Payload:     audit.AssetCreatedPayload{V: 1, Filename: asset.OriginalFilename, Source: "upload"},
	})
	publishWorkflowTriggerAsync(ctx, s.triggers, "trigger.asset_created", workflowAssetTrigger{
		AssetID:          asset.ID,
		WorkspaceID:      asset.WorkspaceID,
		ProjectID:        ptrStr(asset.ProjectID),
		FolderID:         ptrStr(asset.FolderID),
		MimeType:         asset.MimeType,
		Size:             asset.Size,
		OriginalFilename: asset.OriginalFilename,
		VersionID:        ptrStr(asset.CurrentVersionID),
		StorageKey:       asset.StorageKey,
	}.toMap())
	if s.invalidate != nil {
		s.invalidate.Invalidate(workspaceID)
	}
	return asset, nil
}
