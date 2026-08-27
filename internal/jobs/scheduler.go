package jobs

import (
	"context"
	"log/slog"
	"time"

	"damask/server/internal/queue"
)

const runsInterval = 24 * time.Hour

// RetentionScheduler fires the enforce_version_retention job once per night
// at approximately 02:00 UTC.
type RetentionScheduler struct {
	queue queue.JobQueue
}

// NewRetentionScheduler creates a RetentionScheduler.
func NewRetentionScheduler(q queue.JobQueue) *RetentionScheduler {
	return &RetentionScheduler{queue: q}
}

// Start launches the scheduler goroutine. It exits when ctx is cancelled.
func (r *RetentionScheduler) Start(ctx context.Context) {
	go func() {
		for {
			next := NextRunAt(time.Now().UTC())
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if _, err := r.queue.Enqueue(ctx, "system", queue.JobTypeEnforceVersionRetention, "{}"); err != nil {
					slog.ErrorContext(ctx, "retention scheduler: enqueue", "error", err)
				} else {
					slog.InfoContext(ctx, "retention scheduler: enqueued enforce_version_retention")
				}
			}
		}
	}()
}

// NextRunAt returns the next 02:00 UTC after t.
func NextRunAt(t time.Time) time.Time {
	next := time.Date(t.Year(), t.Month(), t.Day(), 2, 0, 0, 0, time.UTC)
	if !next.After(t) {
		next = next.Add(runsInterval)
	}
	return next
}
