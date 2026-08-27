package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/jobspec"
	"damask/server/internal/queue"
	"damask/server/internal/transform"
)

// EnqueueRebuildVariantsJob enqueues a rebuild_variants job when a new version is uploaded.
// sourceVersionID is the version that was current before the upload — its variant params are copied.
// If sourceVersionID is empty (first upload), this is a no-op.
func EnqueueRebuildVariantsJob(
	ctx context.Context,
	q queue.JobQueue,
	workspaceID, assetID, newVersionID, sourceVersionID string,
) error {
	if sourceVersionID == "" {
		return nil
	}
	payload, _ := json.Marshal(RebuildVariantsPayload{
		AssetID:         assetID,
		NewVersionID:    newVersionID,
		SourceVersionID: sourceVersionID,
	})
	_, err := q.Enqueue(ctx, workspaceID, queue.JobTypeRebuildVariants, string(payload))
	return err
}

// jobVersionThumbnail generates a thumbnail for a specific asset version.
// It writes the thumbnail to storage and updates asset_versions.thumbnail_key.
// If this is the current version, it also updates assets.thumbnail_key.
func (s *JobServer) jobVersionThumbnail(ctx context.Context, job dbgen.Job) error {
	var p jobspec.VersionThumbnailJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Workspace guard: the version (and its writes below) must belong to the
	// job's workspace.
	if ver, verErr := s.queries.GetVersionByIDUnchecked(ctx, p.VersionID); verErr != nil ||
		ver.WorkspaceID != job.WorkspaceID {
		return queue.Permanent(fmt.Errorf("version %s not found in workspace %s", p.VersionID, job.WorkspaceID))
	}

	slog.DebugContext(ctx, "generate thumbnail", "mime_type", p.MimeType, "storage_key", p.StorageKey)

	thumbData, thumbExt, err := s.tmb.GenerateThumbnailData(ctx, s.storage, p.MimeType, p.StorageKey)
	if err != nil {
		return fmt.Errorf("generate thumbnail: %w", err)
	}
	if thumbData == nil {
		slog.DebugContext(ctx, "generate thumbnail: unsupported file", "format", p.MimeType, "storedKey", p.StorageKey)
		return nil // unsupported or skipped (e.g. no ffmpeg)
	}

	thumbKey := fmt.Sprintf("%s/%s/versions/%s/thumb%s", p.WorkspaceID, p.AssetID, p.VersionID, thumbExt)
	slog.DebugContext(ctx, "generate thumbnail: store in", "thumbKey", thumbKey)

	if err = s.storage.Put(ctx, thumbKey, bytes.NewReader(thumbData)); err != nil {
		return fmt.Errorf("store thumb: %w", err)
	}

	thumbContentType := mime.TypeByExtension(thumbExt)
	if thumbContentType == "" {
		thumbContentType = transform.MimeImageJPEG
	}

	if err = s.queries.SetVersionThumbnail(ctx, dbgen.SetVersionThumbnailParams{
		ThumbnailKey:         &thumbKey,
		ThumbnailContentType: thumbContentType,
		ID:                   p.VersionID,
	}); err != nil {
		return fmt.Errorf("set version thumbnail: %w", err)
	}

	// Compute perceptual hash for image assets (non-fatal).
	if transform.IsImageMime(p.MimeType) {
		s.computeAndStoreVisualSimilarity(ctx, p.WorkspaceID, p.VersionID, p.StorageKey)
	}

	// If this version is still current, sync the asset thumbnail too.
	ver, err := s.queries.GetVersionByIDUnchecked(ctx, p.VersionID)
	if err == nil && ver.IsCurrent == 1 {
		slog.DebugContext(
			ctx,
			"generate thumbnail: update current thumbnail",
			"assetID",
			p.AssetID,
			"thumbKey",
			thumbKey,
		)
		if err = s.queries.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
			ThumbnailKey:         &thumbKey,
			ThumbnailContentType: thumbContentType,
			ID:                   p.AssetID,
		}); err == nil {
			s.hub.Publish(ctx, p.WorkspaceID, events.ThumbnailReady(p.AssetID, thumbKey))
		}
	}

	return nil
}
