package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/jobspec"
	"damask/server/internal/queue"
)

func (s *JobServer) publishVariantReady(ctx context.Context, workspaceID, assetID, variantID string) {
	s.hub.Publish(ctx, workspaceID, events.VariantReady(assetID, variantID))
}

func (s *JobServer) publishVariantFailed(ctx context.Context, workspaceID, assetID, jobID, errMsg string) {
	s.hub.Publish(ctx, workspaceID, events.VariantFailed(assetID, jobID, errMsg))
}

func (s *JobServer) wrapVariantJob(h queue.HandlerFunc) queue.HandlerFunc {
	return func(ctx context.Context, job dbgen.Job) error {
		var p jobspec.VariantJobPayload
		if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
			return fmt.Errorf("parse variant job payload: %w", err)
		}

		err := h(ctx, job)
		if err != nil {
			slog.ErrorContext(ctx, "variant generation failed", "error", err.Error())
			if p.VariantID != "" {
				if _, setErr := s.sqlDB.ExecContext(
					ctx,
					`UPDATE variants SET status = 'failed' WHERE id = ? AND workspace_id = ?`,
					p.VariantID,
					p.WorkspaceID,
				); setErr != nil {
					slog.WarnContext(ctx, "variant status update failed", "variant_id", p.VariantID, "error", setErr)
				}
			}
			s.publishVariantFailed(ctx, p.WorkspaceID, p.AssetID, job.ID, err.Error())
		}
		return err
	}
}
