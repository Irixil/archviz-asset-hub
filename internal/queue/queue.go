// Package queue implements the in-process job queue backed by SQLite.
package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/jobspec"
	"damask/server/internal/telemetry"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const (
	transcodeSemaphoreLimit    = 2
	rebuildSemaphoreLimit      = 2
	customFFmpegSemaphoreLimit = 2
	tickerInterval             = 2 * time.Second

	// stalledSweepInterval is how often the periodic stalled-job sweep runs.
	stalledSweepInterval = 5 * time.Minute
	// stalledGrace is added to a type's handler timeout before the sweep
	// considers a 'processing' job stalled, leaving room for settle writes.
	stalledGrace = 5 * time.Minute

	// sqliteTimeLayout matches SQLite's datetime('now') output (UTC).
	sqliteTimeLayout = "2006-01-02 15:04:05"
)

// HandlerFunc processes a job payload and returns an error on failure.
type HandlerFunc func(ctx context.Context, job dbgen.Job) error

// JobQueue is the interface implemented by queue backends.
// The SQLite-backed Queue satisfies this interface; future backends (e.g. asynq) can too.
type JobQueue interface {
	Register(jobType string, handler HandlerFunc)
	Enqueue(ctx context.Context, workspaceID, jobType, payload string) (dbgen.Job, error)
	Start(ctx context.Context)
	Stop()
}

// Queue is an in-process job queue backed by SQLite with a configurable worker pool.
type Queue struct {
	queries  *dbgen.Queries
	workers  int
	handlers map[string]HandlerFunc

	notify chan struct{}
	done   chan struct{}
	wg     sync.WaitGroup

	// semGroups limits concurrency per job-type family. A worker reserves a
	// slot before claiming and excludes saturated families from the claim, so
	// it never holds a job it can't immediately run.
	semGroups []semGroup
}

// semGroup couples a concurrency semaphore with the job types it limits.
type semGroup struct {
	sem   chan struct{}
	types []string
}

func (g *semGroup) covers(jobType string) bool {
	return slices.Contains(g.types, jobType)
}

// New creates a new Queue. Call Start() to begin processing.
func New(queries *dbgen.Queries, workers int) *Queue {
	if workers <= 0 {
		workers = 4
	}
	return &Queue{
		queries:  queries,
		workers:  workers,
		handlers: make(map[string]HandlerFunc),
		notify:   make(chan struct{}, workers),
		done:     make(chan struct{}),
		semGroups: []semGroup{
			// Video transcode/watermark share one semaphore.
			{
				sem:   make(chan struct{}, transcodeSemaphoreLimit),
				types: []string{JobTypeVideoTranscode, JobTypeVideoWatermark},
			},
			// custom_ffmpeg runs user-supplied commands and gets its own
			// semaphore so a slow/hanging one can't starve the real
			// transcode/watermark jobs above.
			{
				sem:   make(chan struct{}, customFFmpegSemaphoreLimit),
				types: []string{JobTypeCustomFFmpeg},
			},
			{
				sem:   make(chan struct{}, rebuildSemaphoreLimit),
				types: []string{JobTypeRebuildVariants},
			},
		},
	}
}

// Register adds a handler for the given job type.
func (q *Queue) Register(jobType string, h HandlerFunc) {
	q.handlers[jobType] = h
}

// Start re-queues any stalled jobs and launches worker goroutines.
func (q *Queue) Start(ctx context.Context) {
	// Re-queue jobs that were 'processing' when the server last crashed.
	if err := q.queries.RequeueStalledJobs(ctx); err != nil {
		slog.ErrorContext(ctx, "queue: requeue stalled", "error", err)
	}

	for range q.workers {
		q.wg.Add(1)
		go q.worker(ctx)
	}

	q.wg.Add(1)
	go q.stalledSweeper(ctx)
}

// stalledSweeper periodically requeues jobs stuck in 'processing' past their
// type's handler timeout (plus grace). Handler timeouts bound legitimate
// processing time, so anything older lost its worker (crash, kill -9).
func (q *Queue) stalledSweeper(ctx context.Context) {
	defer q.wg.Done()

	ticker := time.NewTicker(stalledSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.SweepStalledJobs(ctx)
		}
	}
}

// SweepStalledJobs requeues 'processing' jobs whose updated_at predates their
// type's timeout plus grace, and terminally fails those whose attempt budget
// is already spent — without the cap, a job that kills its worker (e.g. an
// OOM'd ffmpeg) would requeue forever. Exported for tests and manual triggers.
func (q *Queue) SweepStalledJobs(ctx context.Context) {
	now := time.Now().UTC()

	// Bucket tuned job types by their (timeout, attempt budget) pair so each
	// bucket needs exactly one requeue and one terminal-fail UPDATE.
	type sweepBudget struct {
		timeout     time.Duration
		maxAttempts int64
	}
	buckets := make(map[sweepBudget][]string)
	typed := tunedJobTypes()
	for _, jobType := range typed {
		b := sweepBudget{timeout: TimeoutFor(jobType), maxAttempts: MaxAttemptsFor(jobType)}
		buckets[b] = append(buckets[b], jobType)
	}

	for b, types := range buckets {
		cutoff := now.Add(-(b.timeout + stalledGrace)).Format(sqliteTimeLayout)
		err := q.queries.RequeueStalledJobsOfTypes(ctx, dbgen.RequeueStalledJobsOfTypesParams{
			Cutoff:      cutoff,
			MaxAttempts: b.maxAttempts,
			Types:       typeList(types),
		})
		if err != nil {
			slog.ErrorContext(ctx, "queue: sweep stalled", "error", err)
		}
		err = q.queries.FailStalledJobsOfTypes(ctx, dbgen.FailStalledJobsOfTypesParams{
			Cutoff:      cutoff,
			MaxAttempts: b.maxAttempts,
			Types:       typeList(types),
		})
		if err != nil {
			slog.ErrorContext(ctx, "queue: sweep stalled", "error", err)
		}
	}

	// Everything else runs under the default timeout and attempt budget.
	cutoff := now.Add(-(defaultJobTimeout + stalledGrace)).Format(sqliteTimeLayout)
	err := q.queries.RequeueStalledJobsNotOfTypes(ctx, dbgen.RequeueStalledJobsNotOfTypesParams{
		Cutoff:      cutoff,
		MaxAttempts: defaultMaxAttempts,
		Types:       typeList(typed),
	})
	if err != nil {
		slog.ErrorContext(ctx, "queue: sweep stalled", "error", err)
	}
	err = q.queries.FailStalledJobsNotOfTypes(ctx, dbgen.FailStalledJobsNotOfTypesParams{
		Cutoff:      cutoff,
		MaxAttempts: defaultMaxAttempts,
		Types:       typeList(typed),
	})
	if err != nil {
		slog.ErrorContext(ctx, "queue: sweep stalled", "error", err)
	}
}

// Stop gracefully shuts down all workers.
func (q *Queue) Stop() {
	close(q.done)
	q.wg.Wait()
}

// Enqueue persists a new job and notifies an idle worker.
func (q *Queue) Enqueue(ctx context.Context, workspaceID, jobType, payload string) (job dbgen.Job, err error) {
	_, span := telemetry.StartSpan(ctx, "service.queue.enqueue",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.job.type", jobType),
	)
	defer telemetry.EndSpan(span, err)

	job, err = q.queries.CreateJob(ctx, dbgen.CreateJobParams{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Type:        jobType,
		Payload:     payload,
	})
	if err != nil {
		return dbgen.Job{}, err
	}

	span.SetAttributes(attribute.String("damask.job.id", job.ID))
	slog.DebugContext(ctx, "queue: created job", "job", job)

	// Best-effort wake up a worker; non-blocking.
	select {
	case q.notify <- struct{}{}:
	default:
	}

	return job, nil
}

// worker loops, claiming and processing one job at a time.
func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()

	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.done:
			return
		case <-ctx.Done():
			return
		case <-q.notify:
			q.processNext(ctx)
		case <-ticker.C:
			q.processNext(ctx)
		}
	}
}

// processNext claims the oldest pending job and runs its handler.
//
// Invariant: ClaimNextJob/CompleteJob/FailJob are independent, non-transactional
// statements — no BeginTx spans a poll tick. This must stay true: holding a
// transaction open here would pin the single writer connection (see
// ROADMAP.68) between polls and starve every other write in the process.
func (q *Queue) processNext(ctx context.Context) {
	var err error
	ctx, span := telemetry.StartSpan(ctx, "service.queue.next")
	defer telemetry.EndSpan(span, err)

	// Reserve a slot in every non-saturated semaphore group up front and
	// exclude saturated ones from the claim, so this worker never holds a
	// claimed job it can't immediately run (head-of-line blocking).
	//
	// Note: idle workers transiently reserve a slot in *every* group on each
	// claim, so concurrent claims of unrelated jobs can briefly saturate a
	// group and make other tickers skip its pending jobs. Bounded by the 2s
	// ticker — acceptable.
	var reserved []*semGroup
	var excludedTypes []string
	for i := range q.semGroups {
		g := &q.semGroups[i]
		select {
		case g.sem <- struct{}{}:
			reserved = append(reserved, g)
		default:
			excludedTypes = append(excludedTypes, g.types...)
		}
	}
	releaseUnused := func(keep *semGroup) {
		for _, g := range reserved {
			if g != keep {
				<-g.sem
			}
		}
	}

	job, err := q.queries.ClaimNextJob(ctx, typeList(excludedTypes))
	if err != nil {
		releaseUnused(nil)
		if !errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Error, err.Error())
			slog.ErrorContext(ctx, "queue: claim job", "error", err)
		}
		return
	}

	// Keep only the reservation covering the claimed type (if any).
	var held *semGroup
	for _, g := range reserved {
		if g.covers(job.Type) {
			held = g
			break
		}
	}
	releaseUnused(held)
	if held != nil {
		defer func() { <-held.sem }()
	}

	defer slog.DebugContext(
		ctx,
		"end background job",
		"job",
		"service.queue.job."+job.Type,
		"job_id",
		job.ID,
		"workspace_id",
		job.WorkspaceID,
		"attempt",
		job.Attempts,
	)

	span.SetAttributes(attribute.String("damask.job.id", job.ID))
	span.SetAttributes(attribute.String("damask.job.type", job.Type))
	span.SetAttributes(attribute.String("damask.job.workspace_id", job.WorkspaceID))

	h, ok := q.handlers[job.Type]
	if !ok {
		slog.ErrorContext(ctx, "queue: no handler for job type", "job_type", job.Type, "job_id", job.ID)
		span.SetStatus(codes.Error, "queue: no handler registered")
		errMsg := "no handler registered"
		_, _ = q.queries.FailJob(ctx, dbgen.FailJobParams{
			Error:    &errMsg,
			ID:       job.ID,
			Attempts: job.Attempts,
		})
		return
	}

	jobCtx := auth.WithActor(ctx, auth.Actor{Type: "system"})
	jobCtx, cancel := context.WithTimeout(jobCtx, TimeoutFor(job.Type))
	defer cancel()
	jobCtx, jobSpan := telemetry.StartBackgroundSpan(jobCtx, "service.queue.job."+job.Type,
		attribute.String("job.id", job.ID),
		attribute.String("job.type", job.Type),
		attribute.String("job.params", job.Payload),
		attribute.Int64("job.attempt", job.Attempts),
		attribute.String("damask.workspace_id", job.WorkspaceID),
	)

	slog.DebugContext(
		ctx,
		"start background job",
		"job",
		"service.queue.job."+job.Type,
		"job_id",
		job.ID,
		"workspace_id",
		job.WorkspaceID,
		"attempt",
		job.Attempts,
	)

	var jobFailed bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				jobFailed = true
				panicErr := fmt.Errorf("panic: %v", r)
				telemetry.RecordError(jobSpan, panicErr)
				span.SetStatus(codes.Error, "queue: job panicked")
				slog.ErrorContext(
					ctx,
					"queue: job panicked",
					"job_id",
					job.ID,
					"job_type",
					job.Type,
					"panic",
					r,
					"stack",
					string(debug.Stack()),
				)
				// Panics retry like transient errors, bounded by the
				// per-type attempt budget.
				q.settleFailure(ctx, job, panicErr)
			}
			jobSpan.End()
		}()

		if err = h(jobCtx, job); err != nil {
			jobFailed = true
			telemetry.RecordError(jobSpan, err)
			slog.ErrorContext(ctx, "queue: job failed", "job_id", job.ID, "job_type", job.Type, "error", err)
			q.settleFailure(ctx, job, err)
			return
		}
		jobSpan.SetStatus(codes.Ok, "")
	}()

	if jobFailed {
		return
	}

	span.SetStatus(codes.Ok, "")

	q.settleSuccess(ctx, job)

	// Signal other workers to check for more work.
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// settleSuccess marks a job done. The attempts guard makes it a no-op if the
// sweep requeued the job and another worker re-claimed it while this one
// overran its timeout.
func (q *Queue) settleSuccess(ctx context.Context, job dbgen.Job) {
	n, err := q.queries.CompleteJob(ctx, dbgen.CompleteJobParams{ID: job.ID, Attempts: job.Attempts})
	switch {
	case err != nil:
		slog.ErrorContext(ctx, "queue: complete job", "job_id", job.ID, "error", err)
	case n == 0:
		slog.WarnContext(ctx, "queue: stale complete skipped", "job_id", job.ID, "attempt", job.Attempts)
	}
}

// settleFailure records a job failure: permanent errors and exhausted attempt
// budgets fail terminally; anything else requeues with exponential backoff.
// job.Attempts is post-claim, i.e. the number of attempts made so far; it also
// fences the writes — a stale worker whose job was sweep-requeued and
// re-claimed (attempts bumped) matches zero rows and must not settle.
func (q *Queue) settleFailure(ctx context.Context, job dbgen.Job, jobErr error) {
	errMsg := jobErr.Error()

	if IsPermanent(jobErr) || job.Attempts >= MaxAttemptsFor(job.Type) {
		n, err := q.queries.FailJob(ctx, dbgen.FailJobParams{Error: &errMsg, ID: job.ID, Attempts: job.Attempts})
		switch {
		case err != nil:
			slog.ErrorContext(ctx, "queue: fail job", "job_id", job.ID, "error", err)
		case n == 0:
			slog.WarnContext(ctx, "queue: stale fail skipped", "job_id", job.ID, "attempt", job.Attempts)
		}
		return
	}

	delay := retryBackoff(job.Attempts)
	runAfter := time.Now().UTC().Add(delay)
	n, err := q.queries.FailJobRetry(ctx, dbgen.FailJobRetryParams{
		Error:    &errMsg,
		RunAfter: &runAfter,
		ID:       job.ID,
		Attempts: job.Attempts,
	})
	if err != nil {
		slog.ErrorContext(ctx, "queue: requeue job", "job_id", job.ID, "error", err)
		return
	}
	if n == 0 {
		slog.WarnContext(ctx, "queue: stale retry skipped", "job_id", job.ID, "attempt", job.Attempts)
		return
	}
	slog.WarnContext(ctx, "queue: job requeued for retry",
		"job_id", job.ID,
		"job_type", job.Type,
		"attempt", job.Attempts,
		"max_attempts", MaxAttemptsFor(job.Type),
		"retry_in", delay,
		"error", errMsg,
	)
}

// Job type constants used throughout the application. The canonical values
// live in internal/jobspec; these names are kept so the many existing call
// sites across the codebase don't need to change.
const (
	JobTypeVersionThumbnail            = jobspec.JobTypeVersionThumbnail
	JobTypeVariantThumbnail            = jobspec.JobTypeVariantThumbnail
	JobTypeOCRTextTrack                = jobspec.JobTypeOCRTextTrack
	JobTypeAIImageDescriptionTextTrack = jobspec.JobTypeAIImageDescriptionTextTrack
	JobTypeExtractPDFTextTrack         = jobspec.JobTypeExtractPDFTextTrack
	JobTypeExtractPlainTextTrack       = jobspec.JobTypeExtractPlainTextTrack
	JobTypeExtractDocumentTextTrack    = jobspec.JobTypeExtractDocumentTextTrack

	JobTypeVideoCaptureImage = jobspec.JobTypeVideoCaptureImage
	JobTypeVideoTranscode    = jobspec.JobTypeVideoTranscode
	JobTypeVideoWatermark    = jobspec.JobTypeVideoWatermark
	JobTypeImageResize       = jobspec.JobTypeImageResize
	JobTypeImageConvert      = jobspec.JobTypeImageConvert
	JobTypeImageCrop         = jobspec.JobTypeImageCrop
	JobTypeImageWatermark    = jobspec.JobTypeImageWatermark
	JobTypeImageBgRemove     = jobspec.JobTypeImageBgRemove
	JobTypeImageWithPrompt   = jobspec.JobTypeImageWithPrompt
	JobTypeImageSmartCrop    = jobspec.JobTypeImageSmartCrop
	JobTypeExtractAudio      = jobspec.JobTypeExtractAudio
	JobTypeTranscodeAudio    = jobspec.JobTypeTranscodeAudio
	JobTypeNormalizeAudio    = jobspec.JobTypeNormalizeAudio
	JobTypeCustomFFmpeg      = jobspec.JobTypeCustomFFmpeg

	JobTypeIngestPoll  = jobspec.JobTypeIngestPoll
	JobTypeIngestFetch = jobspec.JobTypeIngestFetch

	JobTypeRebuildVariants = jobspec.JobTypeRebuildVariants
	JobTypeRunWorkflow     = jobspec.JobTypeRunWorkflow

	JobTypeExtractExif      = jobspec.JobTypeExtractExif
	JobTypeExtractMediaTags = jobspec.JobTypeExtractMediaTags

	JobTypeStackMerge         = jobspec.JobTypeStackMerge
	JobTypeCreateVariantDraft = jobspec.JobTypeCreateVariantDraft

	JobTypeExportRun = jobspec.JobTypeExportRun

	JobTypePurgeDeletedFields      = jobspec.JobTypePurgeDeletedFields
	JobTypeEnforceVersionRetention = jobspec.JobTypeEnforceVersionRetention
	JobTypePurgeVersionStorage     = jobspec.JobTypePurgeVersionStorage
	JobTypePurgeAuditLog           = jobspec.JobTypePurgeAuditLog
	JobTypePurgeScratchVariants    = jobspec.JobTypePurgeScratchVariants

	JobTypeVisualSimilarityBackfill = jobspec.JobTypeVisualSimilarityBackfill

	JobTypeAutoTag = jobspec.JobTypeAutoTag
)
