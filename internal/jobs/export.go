package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	dbgen "damask/server/internal/db/gen"
)

type exportRunPayload struct {
	ExportConfigID string `json:"export_config_id"`
	ExportRunID    string `json:"export_run_id"`
}

func (s *JobServer) jobExportRun(ctx context.Context, job dbgen.Job) error {
	var p exportRunPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("export job: parse payload: %w", err)
	}
	return s.exportSvc.ExecuteRun(ctx, job.WorkspaceID, p.ExportConfigID, p.ExportRunID)
}
