package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"damask/server/internal/queue"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

// ExportScheduler ticks every minute and enqueues export_run jobs for
// configs whose quiet period has elapsed since the last asset change.
type ExportScheduler struct {
	queue     queue.JobQueue
	exportSvc exportService
}

// NewExportScheduler creates an ExportScheduler backed by the given queue and DB.
func NewExportScheduler(q queue.JobQueue, s *JobServer) *ExportScheduler {
	return &ExportScheduler{
		queue:     q,
		exportSvc: s.exportSvc,
	}
}

// Start launches the scheduler goroutine. It exits when ctx is cancelled.
func (s *ExportScheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

func (s *ExportScheduler) tick(ctx context.Context) {
	due, err := s.exportSvc.ListDueConfigs(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "export scheduler: list due configs", "error", err)
		return
	}
	for _, cfg := range due {
		run := repository.ExportRun{
			ID:             uuid.NewString(),
			ExportConfigID: cfg.ID,
			WorkspaceID:    cfg.WorkspaceID,
			Status:         jobStatusPending,
			CreatedAt:      time.Now(),
		}
		created, runErr := s.exportSvc.CreateRun(ctx, run)
		if runErr != nil {
			slog.WarnContext(ctx, "export scheduler: create run", "config_id", cfg.ID, "error", runErr)
			continue
		}
		payload := fmt.Sprintf(`{"export_config_id":%q,"export_run_id":%q}`, cfg.ID, created.ID)
		if _, e := s.queue.Enqueue(ctx, cfg.WorkspaceID, queue.JobTypeExportRun, payload); e != nil {
			slog.WarnContext(ctx, "export scheduler: enqueue", "config_id", cfg.ID, "error", e)
			continue
		}
		now := time.Now()
		_ = s.exportSvc.SetConfigLastRun(ctx, cfg.ID, repository.ExportRunResult{
			LastRunAt:     now,
			LastRunStatus: jobStatusPending,
		})
	}
}
