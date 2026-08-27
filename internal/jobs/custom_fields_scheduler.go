// Package jobs defines background job types and scheduling logic.
package jobs

import (
	"context"
	"log/slog"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

const (
	schedulerCronHour   = 3
	schedulerCronMinute = 0
)

// FieldCleanupScheduler enqueues a purge_deleted_fields job once per day at 03:00 UTC.
type FieldCleanupScheduler struct {
	queries *dbgen.Queries
	queue   queue.JobQueue
}

// NewFieldCleanupScheduler creates a FieldCleanupScheduler.
func NewFieldCleanupScheduler(queries *dbgen.Queries, q queue.JobQueue) *FieldCleanupScheduler {
	return &FieldCleanupScheduler{queries: queries, queue: q}
}

// Start launches the scheduler goroutine. Returns immediately; exits when ctx is cancelled.
func (s *FieldCleanupScheduler) Start(ctx context.Context) {
	go func() {
		for {
			next := NextDaily(schedulerCronHour, schedulerCronMinute)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if _, err := s.queue.Enqueue(ctx, "system", queue.JobTypePurgeDeletedFields, "{}"); err != nil {
					slog.ErrorContext(ctx, "field cleanup scheduler: enqueue purge", "error", err)
				}
			}
		}
	}()
}

// jobPurgeDeletedFields hard-deletes field_definitions (and their values) that
// have been soft-deleted for more than 30 days.
func (s *JobServer) jobPurgeDeletedFields(ctx context.Context, _ dbgen.Job) error {
	_, err := s.fieldSvc.PurgeExpiredFields(ctx)
	return err
}
