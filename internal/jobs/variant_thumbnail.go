package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/transform"
)

// VariantThumbnailJobPayload is the payload for variant thumbnail generation.
type VariantThumbnailJobPayload struct {
	VariantID   string `json:"variant_id"`
	WorkspaceID string `json:"workspace_id"`
	AssetID     string `json:"asset_id"`
	StorageKey  string `json:"storage_key"`
	MimeType    string `json:"mime_type"`
}

// EnqueueVariantThumbnailJob enqueues a variant thumbnail job.
func EnqueueVariantThumbnailJob(ctx context.Context, s *JobServer, p VariantThumbnailJobPayload) error {
	payload, _ := json.Marshal(p)
	_, err := s.queue.Enqueue(ctx, p.WorkspaceID, queue.JobTypeVariantThumbnail, string(payload))
	return err
}

// enqueueVariantThumbRaw is the low-level helper used by rebuild jobs that don't have a VariantJobPayload.
func (s *JobServer) enqueueVariantThumbRaw(
	ctx context.Context,
	workspaceID, assetID, variantID, storageKey, mimeType string,
) {
	_ = EnqueueVariantThumbnailJob(ctx, s, VariantThumbnailJobPayload{
		VariantID:   variantID,
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		StorageKey:  storageKey,
		MimeType:    mimeType,
	})
}

// jobVariantThumbnail generates a thumbnail for a variant and writes it to storage.
func (s *JobServer) jobVariantThumbnail(ctx context.Context, job dbgen.Job) error {
	var p VariantThumbnailJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Workspace guard: the variant must belong to the job's workspace before
	// the bare-ID thumbnail write below.
	if _, err := s.queries.GetVariantByID(ctx, dbgen.GetVariantByIDParams{
		ID:          p.VariantID,
		WorkspaceID: job.WorkspaceID,
	}); err != nil {
		return queue.Permanent(fmt.Errorf("variant %s not found in workspace %s", p.VariantID, job.WorkspaceID))
	}

	thumbData, thumbExt, err := s.tmb.GenerateThumbnailData(ctx, s.storage, p.MimeType, p.StorageKey)
	if err != nil {
		return fmt.Errorf("generate thumbnail: %w", err)
	}
	if thumbData == nil {
		return nil // unsupported or skipped
	}

	thumbKey := fmt.Sprintf("%s/%s/variants/%s/thumb%s", p.WorkspaceID, p.AssetID, p.VariantID, thumbExt)
	if e := s.storage.Put(ctx, thumbKey, bytes.NewReader(thumbData)); e != nil {
		return fmt.Errorf("store variant thumb: %w", e)
	}

	thumbContentType := mime.TypeByExtension(thumbExt)
	if thumbContentType == "" {
		thumbContentType = transform.MimeImageJPEG
	}

	if e := s.queries.SetVariantThumbnail(ctx, dbgen.SetVariantThumbnailParams{
		ThumbnailKey:         &thumbKey,
		ThumbnailContentType: thumbContentType,
		ID:                   p.VariantID,
	}); e != nil {
		return fmt.Errorf("set variant thumbnail: %w", e)
	}

	return nil
}
