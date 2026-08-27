package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/jobspec"
)

func (s *JobServer) jobExtractPDFTextTrack(ctx context.Context, job dbgen.Job) error {
	var p jobspec.ExtractTextPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("jobExtractPDFTextTrack: unmarshal: %w", err)
	}
	trackID, err := s.createExtractTextTrack(ctx, p.WorkspaceID, p.AssetID, "pdf")
	if err != nil {
		return fmt.Errorf("jobExtractPDFTextTrack: create track: %w", err)
	}
	return s.textTrackSvc.RunExtractPDF(ctx, p.WorkspaceID, p.AssetID, trackID, p.StorageKey)
}

func (s *JobServer) jobExtractPlainTextTrack(ctx context.Context, job dbgen.Job) error {
	var p jobspec.ExtractTextPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("jobExtractPlainTextTrack: unmarshal: %w", err)
	}
	trackID, err := s.createExtractTextTrack(ctx, p.WorkspaceID, p.AssetID, "plain")
	if err != nil {
		return fmt.Errorf("jobExtractPlainTextTrack: create track: %w", err)
	}
	return s.textTrackSvc.RunExtractPlain(ctx, p.WorkspaceID, p.AssetID, trackID, p.StorageKey)
}

func (s *JobServer) jobExtractDocumentTextTrack(ctx context.Context, job dbgen.Job) error {
	var p jobspec.ExtractTextPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("jobExtractDocumentTextTrack: unmarshal: %w", err)
	}
	trackID, err := s.createExtractTextTrack(ctx, p.WorkspaceID, p.AssetID, "document")
	if err != nil {
		return fmt.Errorf("jobExtractDocumentTextTrack: create track: %w", err)
	}
	return s.textTrackSvc.RunExtractDocument(ctx, p.WorkspaceID, p.AssetID, trackID, p.StorageKey, p.MimeType)
}

func (s *JobServer) createExtractTextTrack(ctx context.Context, workspaceID, assetID, source string) (string, error) {
	row, err := s.queries.CreateTextTrack(ctx, dbgen.CreateTextTrackParams{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		Source:      source,
		Content:     "",
		Status:      jobStatusPending,
	})
	if err != nil {
		return "", err
	}
	return row.ID, nil
}
