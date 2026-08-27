package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"damask/server/internal/jobspec"
	"damask/server/internal/workflow"
)

type OCRTextTrackPayload struct {
	WorkspaceID    string `json:"workspace_id"`
	AssetID        string `json:"asset_id"`
	TrackID        string `json:"track_id"`
	AssetVersionID string `json:"asset_version_id"`
	StorageKey     string `json:"storage_key"`
	MimeType       string `json:"mime_type"`
	Lang           string `json:"lang"`
	OutputFormat   string `json:"output_format"`
	// Continuation, when set, resumes a suspended workflow run once the OCR
	// text is ready (see action.ocr workflow node).
	Continuation *workflow.NodeContinuation `json:"continuation,omitempty"`
}

func (s *JobServer) jobOCRTextTrack(ctx context.Context, rawPayload string) error {
	var p OCRTextTrackPayload
	if err := json.Unmarshal([]byte(rawPayload), &p); err != nil {
		return fmt.Errorf("jobOCRTextTrack: unmarshal: %w", err)
	}
	text, wordCount, err := s.textTrackSvc.RunOCR(
		ctx,
		p.WorkspaceID,
		p.AssetID,
		p.TrackID,
		p.AssetVersionID,
		p.StorageKey,
		p.MimeType,
		p.Lang,
		p.OutputFormat,
	)
	if err != nil {
		s.failContinuation(ctx, p.Continuation, err)
		return err
	}

	if p.Continuation == nil {
		return nil
	}

	if resumeErr := s.workflowExec.ResumeAt(ctx, *p.Continuation, map[string]any{
		"text":                   text,
		"track_id":               p.TrackID,
		jobspec.MetaKeyWordCount: wordCount,
	}); resumeErr != nil {
		slog.ErrorContext(ctx, "workflow continuation failed after ocr ready",
			"run_id", p.Continuation.RunID,
			"node_id", p.Continuation.NodeID,
			"error", resumeErr,
		)
		s.failContinuation(ctx, p.Continuation, resumeErr)
		return fmt.Errorf("jobOCRTextTrack: resume workflow: %w", resumeErr)
	}
	return nil
}
