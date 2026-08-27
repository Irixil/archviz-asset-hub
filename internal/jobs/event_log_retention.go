package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

const (
	eventLogSchedulerCronHour   = 4
	eventLogSchedulerCronMinute = 0
)

// jobPurgeAuditLog runs the event log retention policy for all workspaces.
// For each workspace it:
//  1. Deletes asset_downloaded events older than download_log_retention_days.
//  2. Deletes all other events older than event_log_retention_days.
//  3. Deletes project events older than event_log_retention_days.
func (s *JobServer) jobPurgeAuditLog(ctx context.Context, _ dbgen.Job) error {
	workspaces, err := s.queries.ListWorkspacesForEventRetention(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	for _, ws := range workspaces {
		s.purgeAuditLogForWorkspace(ctx, ws)
	}
	return nil
}

func (s *JobServer) purgeAuditLogForWorkspace(ctx context.Context, ws dbgen.ListWorkspacesForEventRetentionRow) {
	now := time.Now().UTC()

	// Purge download events on their shorter retention cycle.
	if ws.DownloadLogRetentionDays > 0 {
		cutoff := now.AddDate(0, 0, -int(ws.DownloadLogRetentionDays)).Format("2006-01-02 15:04:05")
		if err := s.queries.DeleteDownloadEventsOlderThan(ctx, dbgen.DeleteDownloadEventsOlderThanParams{
			WorkspaceID: ws.ID,
			Cutoff:      cutoff,
		}); err != nil {
			slog.ErrorContext(ctx, "audit-log retention: purge downloads", "workspace_id", ws.ID, "error", err)
		} else {
			slog.InfoContext(
				ctx,
				"audit-log retention: purged download events",
				"workspace_id",
				ws.ID,
				"cutoff",
				cutoff,
			)
		}
	}

	// Purge all asset events beyond the general retention window.
	if ws.EventLogRetentionDays > 0 {
		cutoff := now.AddDate(0, 0, -int(ws.EventLogRetentionDays)).Format("2006-01-02 15:04:05")
		if err := s.queries.DeleteAssetEventsOlderThan(ctx, dbgen.DeleteAssetEventsOlderThanParams{
			WorkspaceID: ws.ID,
			Cutoff:      cutoff,
		}); err != nil {
			slog.ErrorContext(ctx, "audit-log retention: purge asset events", "workspace_id", ws.ID, "error", err)
		} else {
			slog.InfoContext(ctx, "audit-log retention: purged asset events", "workspace_id", ws.ID, "cutoff", cutoff)
		}
		if err := s.queries.DeleteProjectEventsOlderThan(ctx, dbgen.DeleteProjectEventsOlderThanParams{
			WorkspaceID: ws.ID,
			Cutoff:      cutoff,
		}); err != nil {
			slog.ErrorContext(ctx, "audit-log retention: purge project events", "workspace_id", ws.ID, "error", err)
		} else {
			slog.InfoContext(ctx, "audit-log retention: purged project events", "workspace_id", ws.ID, "cutoff", cutoff)
		}
	}
}

// AuditLogRetentionScheduler fires the purge_event_log job nightly at 04:00 UTC.
type AuditLogRetentionScheduler struct {
	queue queue.JobQueue
}

func NewAuditLogRetentionScheduler(q queue.JobQueue) *AuditLogRetentionScheduler {
	return &AuditLogRetentionScheduler{queue: q}
}

func (r *AuditLogRetentionScheduler) Start(ctx context.Context) {
	go func() {
		for {
			next := NextDaily(eventLogSchedulerCronHour, eventLogSchedulerCronMinute)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if _, err := r.queue.Enqueue(ctx, "system", queue.JobTypePurgeAuditLog, "{}"); err != nil {
					slog.ErrorContext(ctx, "audit-log retention scheduler: enqueue", "error", err)
				} else {
					slog.InfoContext(ctx, "audit-log retention scheduler: enqueued purge_event_log")
				}
			}
		}
	}()
}
