package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/jobspec"
	"damask/server/internal/queue"
)

// EnqueueExtractExifJob enqueues an extract_exif job for an image asset.
func EnqueueExtractExifJob(ctx context.Context, q queue.JobQueue, workspaceID, assetID, userID string) error {
	payload, _ := json.Marshal(jobspec.ExtractExifPayload{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	_, err := q.Enqueue(ctx, workspaceID, queue.JobTypeExtractExif, string(payload))
	return err
}

func (s *JobServer) jobExtractExif(ctx context.Context, job dbgen.Job) error {
	var p jobspec.ExtractExifPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("exif job: parse payload: %w", err)
	}
	return s.exifSvc.ExtractForAsset(ctx, p.WorkspaceID, p.AssetID, p.UserID)
}
