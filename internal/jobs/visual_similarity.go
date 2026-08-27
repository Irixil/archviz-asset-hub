package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"  //nolint:gci // register GIF format
	_ "image/jpeg" //nolint:gci // register JPEG format
	_ "image/png"  //nolint:gci // register PNG format
	"log/slog"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"
	"damask/server/internal/visualsimilarity"

	"go.opentelemetry.io/otel/attribute"
)

const visualSimilarityBackfillSleep = 100 * time.Millisecond

// computeAndStoreVisualSimilarity decodes the image at storageKey and stores the perceptual hash
// for assetVersionID. Non-fatal: errors are logged and the function returns nil.
func (s *JobServer) computeAndStoreVisualSimilarity(
	ctx context.Context,
	workspaceID, assetVersionID, storageKey string,
) {
	if ctx.Err() != nil {
		return
	}

	ctx, span := telemetry.StartBackgroundSpan(ctx, "visual_similarity.compute",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_version_id", assetVersionID),
	)
	defer telemetry.EndSpan(span, nil)

	rc, err := s.storage.Get(ctx, storageKey)
	if err != nil {
		slog.WarnContext(ctx, "visual similarity: get from storage", "version_id", assetVersionID, "error", err)
		return
	}
	defer rc.Close()

	img, _, err := image.Decode(rc)
	if err != nil {
		slog.WarnContext(ctx, "visual similarity: decode image", "version_id", assetVersionID, "error", err)
		return
	}

	hashes, err := visualsimilarity.Compute(img)
	if err != nil {
		slog.WarnContext(ctx, "visual similarity: compute hashes", "version_id", assetVersionID, "error", err)
		return
	}

	if err = s.visualSimilarity.Store(ctx, workspaceID, assetVersionID, hashes); err != nil {
		slog.WarnContext(ctx, "visual similarity: store hashes", "version_id", assetVersionID, "error", err)
	}
}

// jobVisualSimilarityBackfill iterates all image versions in the workspace without a hash row,
// decodes and stores their perceptual hash. Throttled to one version at a time.
func (s *JobServer) jobVisualSimilarityBackfill(ctx context.Context, job dbgen.Job) error {
	var payload struct {
		WorkspaceID string `json:"workspace_id"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	ctx, span := telemetry.StartBackgroundSpan(ctx, "visual_similarity.backfill",
		attribute.String("damask.workspace_id", payload.WorkspaceID),
	)
	defer func() { telemetry.EndSpan(span, nil) }()

	versions, err := s.queries.ListVersionsWithoutVisualSimilarityHash(ctx, payload.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list versions without hash: %w", err)
	}

	slog.InfoContext(
		ctx,
		"visual similarity backfill: starting",
		"workspace_id",
		payload.WorkspaceID,
		"count",
		len(versions),
	)

	for _, v := range versions {
		if !transform.IsImageMime(v.MimeType) {
			continue
		}
		s.computeAndStoreVisualSimilarity(ctx, v.WorkspaceID, v.ID, v.StorageKey)
		time.Sleep(visualSimilarityBackfillSleep)
	}

	slog.InfoContext(
		ctx,
		"visual similarity backfill: done",
		"workspace_id",
		payload.WorkspaceID,
		"processed",
		len(versions),
	)
	return nil
}

// EnqueueVisualSimilarityBackfill enqueues a workspace-wide backfill job.
func EnqueueVisualSimilarityBackfill(ctx context.Context, q queue.JobQueue, workspaceID string) error {
	payload, _ := json.Marshal(map[string]string{"workspace_id": workspaceID})
	_, err := q.Enqueue(ctx, workspaceID, queue.JobTypeVisualSimilarityBackfill, string(payload))
	return err
}
