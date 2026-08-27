package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"damask/server/internal/ai"
	"damask/server/internal/assetio"
	"damask/server/internal/audit"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/ingress"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
	"damask/server/internal/visualsimilarity"
	"damask/server/internal/workflow"
)

const (
	schedulerCronInterval = 24 * time.Hour
	jobStatusPending      = "pending"
)

// exportService is the narrow subset of service.ExportService used by job
// handlers and the scheduler. Kept narrow (rather than depending on the full
// interface) so a handler's dependency is legible from its type; conformance
// is asserted below so a signature drift in service.ExportService breaks
// this package's build, not just the NewJobServer call site.
type exportService interface {
	ExecuteRun(ctx context.Context, workspaceID, configID, runID string) error
	ListDueConfigs(ctx context.Context) ([]repository.ExportConfig, error)
	CreateRun(ctx context.Context, run repository.ExportRun) (repository.ExportRun, error)
	SetConfigLastRun(ctx context.Context, configID string, p repository.ExportRunResult) error
}

type exifService interface {
	ExtractForAsset(ctx context.Context, workspaceID, assetID, userID string) error
}

// tagService is the narrow subset of service.TagService used by automated
// (non-HTTP) tag application, e.g. silent AI auto-tagging.
type tagService interface {
	ApplyTag(ctx context.Context, workspaceID, assetID, tagName string) error
}

type fieldPurgeService interface {
	PurgeExpiredFields(ctx context.Context) (int, error)
}

// textTrackService is the narrow subset of service.TextTrackService used by job handlers.
type textTrackService interface {
	RunOCR(
		ctx context.Context,
		workspaceID, assetID, trackID, assetVersionID, storageKey, mimeType, lang, outputFormat string,
	) (text string, wordCount int, err error)
	RunExtractPDF(ctx context.Context, workspaceID, assetID, trackID, storageKey string) error
	RunExtractPlain(ctx context.Context, workspaceID, assetID, trackID, storageKey string) error
	RunExtractDocument(ctx context.Context, workspaceID, assetID, trackID, storageKey, mimeType string) error
	CreateAudioTranscript(
		ctx context.Context,
		workspaceID, assetID, assetVersionID, transcript string,
	) (trackID string, err error)
}

// Compile-time conformance assertions: internal/jobs can now import
// internal/service directly (service no longer imports jobs — payload types
// and job-type constants moved to internal/jobspec), so a signature drift in
// any of these real service types fails this package's build instead of
// only surfacing where NewJobServer is called.
var (
	_ exportService     = (service.ExportService)(nil)
	_ exifService       = (*service.ExifService)(nil)
	_ tagService        = (service.TagService)(nil)
	_ fieldPurgeService = (service.FieldService)(nil)
	_ textTrackService  = (service.TextTrackService)(nil)
)

// JobServer holds shared dependencies injected at startup.
type JobServer struct {
	queries           *dbgen.Queries
	sqlDB             *sql.DB
	storage           storage.Storage
	queue             queue.JobQueue
	mailer            mail.Mailer
	hub               events.EventHub
	cfg               *config.Config
	audit             *audit.EventWriter
	handlers          map[string]queue.HandlerFunc
	trf               transform.Transformer
	tmb               transform.Thumbnailer
	ingester          assetio.Ingester
	aiAPIKeyResolver  ai.KeyResolver
	aiProviderFactory ai.ProviderFactory
	workspaceRepo     repository.WorkspaceRepository
	workflowExec      *workflow.Executor
	exportSvc         exportService
	exifSvc           exifService
	tagSvc            tagService
	fieldSvc          fieldPurgeService
	textTrackSvc      textTrackService
	storageSvc        ingress.StorageLimitChecker
	visualSimilarity  *visualsimilarity.Service
}

// Deps holds every dependency required to construct a [JobServer]. Fields
// marked required are validated non-nil by [NewJobServer] so a missing
// wiring bug fails fast at startup instead of panicking or misbehaving deep
// inside a job handler.
type Deps struct {
	Queries           *dbgen.Queries // required
	SQLDB             *sql.DB        // required
	Storage           storage.Storage
	Hub               events.EventHub
	Queue             queue.JobQueue // required
	Mailer            mail.Mailer
	Transformer       transform.Transformer
	Thumbnailer       transform.Thumbnailer
	Config            *config.Config
	Ingester          assetio.Ingester
	AIAPIKeyResolver  ai.KeyResolver // required
	AIProviderFactory ai.ProviderFactory
	WorkspaceRepo     repository.WorkspaceRepository
	WorkflowExec      *workflow.Executor // required
	ExportSvc         exportService
	ExifSvc           exifService
	TagSvc            tagService
	FieldSvc          fieldPurgeService
	TextTrackSvc      textTrackService
	StorageSvc        ingress.StorageLimitChecker
}

// validate checks that every field required to run job handlers is
// non-nil, returning an error naming the first missing field.
func (d Deps) validate() error {
	switch {
	case d.Queries == nil:
		return errors.New("jobs: Deps.Queries is required")
	case d.SQLDB == nil:
		return errors.New("jobs: Deps.SQLDB is required")
	case d.Queue == nil:
		return errors.New("jobs: Deps.Queue is required")
	case d.AIAPIKeyResolver == nil:
		return errors.New("jobs: Deps.AIAPIKeyResolver is required")
	case d.WorkflowExec == nil:
		return errors.New("jobs: Deps.WorkflowExec is required")
	}
	return nil
}

// NewJobServer constructs a JobServer from deps. It panics if a required
// dependency is missing — job wiring is assembled once at startup, so a
// missing dependency is a programming error that should fail fast rather
// than surface as a nil-pointer panic mid-job.
func NewJobServer(deps Deps) *JobServer {
	if err := deps.validate(); err != nil {
		panic(err)
	}
	aiProviderFactory := deps.AIProviderFactory
	if aiProviderFactory == nil {
		aiProviderFactory = ai.NewProvider
	}
	return &JobServer{
		audit:             audit.New(deps.SQLDB),
		cfg:               deps.Config,
		queries:           deps.Queries,
		exportSvc:         deps.ExportSvc,
		exifSvc:           deps.ExifSvc,
		tagSvc:            deps.TagSvc,
		fieldSvc:          deps.FieldSvc,
		textTrackSvc:      deps.TextTrackSvc,
		handlers:          make(map[string]queue.HandlerFunc),
		hub:               deps.Hub,
		aiAPIKeyResolver:  deps.AIAPIKeyResolver,
		aiProviderFactory: aiProviderFactory,
		workspaceRepo:     deps.WorkspaceRepo,
		ingester:          deps.Ingester,
		mailer:            deps.Mailer,
		queue:             deps.Queue,
		sqlDB:             deps.SQLDB,
		storage:           deps.Storage,
		storageSvc:        deps.StorageSvc,
		tmb:               deps.Thumbnailer,
		trf:               deps.Transformer,
		workflowExec:      deps.WorkflowExec,
		visualSimilarity:  visualsimilarity.NewService(deps.Queries, deps.SQLDB),
	}
}

// EnqueueForTest inserts a job directly into the queue. Intended for use in tests only.
func (s *JobServer) EnqueueForTest(ctx context.Context, workspaceID, jobType, payload string) (string, error) {
	job, err := s.queue.Enqueue(ctx, workspaceID, jobType, payload)
	if err != nil {
		return "", err
	}
	return job.ID, nil
}

// DrainForTest registers all handlers and synchronously processes every pending
// job. Intended for use in tests only — not safe for concurrent use.
func (s *JobServer) DrainForTest(ctx context.Context) {
	s.RegisterJobHandlers()
	for {
		// Ignore run_after so jobs parked for a future retry still drain —
		// otherwise tests would pass with work silently left pending.
		job, err := s.queries.ClaimNextJobIgnoreRunAfter(ctx)
		if err != nil {
			return // queue empty
		}
		h, ok := s.handlers[job.Type]
		if !ok {
			continue
		}
		_ = h(ctx, job)
	}
}

// RegisterJobHandlers wires all job handlers into the queue.
// It does not start any scheduler goroutines — call StartSchedulers for that.
func (s *JobServer) RegisterJobHandlers() {
	reg := func(jobType string, h queue.HandlerFunc) {
		s.queue.Register(jobType, h)
		s.handlers[jobType] = h
	}

	// Register ingress job handlers
	ingressWorker := ingress.NewWorker(
		s.queries,
		s.sqlDB,
		s.storage,
		s.queue,
		s.cfg,
		s.audit,
		s.mailer,
		s.ingester,
		s.storageSvc,
	)
	reg(queue.JobTypeIngestPoll, ingressWorker.HandlePoll)
	reg(queue.JobTypeIngestFetch, ingressWorker.HandleFetch)

	reg(queue.JobTypeVersionThumbnail, s.jobVersionThumbnail)
	reg(queue.JobTypeVariantThumbnail, s.jobVariantThumbnail)

	// Variant jobs — registered from the central registry.
	variantHandler := s.wrapVariantJob(s.jobVariant)
	for jobType := range s.variantRegistry() {
		reg(jobType, variantHandler)
	}
	reg(queue.JobTypeOCRTextTrack, s.wrapSimpleJob(s.jobOCRTextTrack))
	reg(queue.JobTypeAIImageDescriptionTextTrack, s.jobAIImageDescriptionTextTrack)
	reg(queue.JobTypeExtractPDFTextTrack, s.jobExtractPDFTextTrack)
	reg(queue.JobTypeExtractPlainTextTrack, s.jobExtractPlainTextTrack)
	reg(queue.JobTypeExtractDocumentTextTrack, s.jobExtractDocumentTextTrack)

	// Rebuild jobs — system-triggered on version upload.
	reg(queue.JobTypeRebuildVariants, s.jobRebuildVariants)
	reg(queue.JobTypeRunWorkflow, s.jobRunWorkflow)

	// EXIF extraction.
	reg(queue.JobTypeExtractExif, s.jobExtractExif)
	reg(queue.JobTypeExtractMediaTags, s.jobExtractMediaTags)

	// Stack merge jobs.
	reg(queue.JobTypeStackMerge, s.jobStackMerge)

	// Variant draft jobs.
	reg(queue.JobTypeCreateVariantDraft, s.jobCreateVariantDraft)

	// Export jobs.
	reg(queue.JobTypeExportRun, s.jobExportRun)

	// Visual similarity.
	reg(queue.JobTypeVisualSimilarityBackfill, s.jobVisualSimilarityBackfill)

	// Auto-tagging.
	reg(queue.JobTypeAutoTag, s.jobAutoTag)

	// Maintenance jobs.
	reg(queue.JobTypePurgeDeletedFields, s.jobPurgeDeletedFields)
	reg(queue.JobTypeEnforceVersionRetention, s.jobEnforceVersionRetention)
	reg(queue.JobTypePurgeVersionStorage, s.jobPurgeVersionStorage)
	reg(queue.JobTypePurgeAuditLog, s.jobPurgeAuditLog)
	reg(queue.JobTypePurgeScratchVariants, s.jobPurgeScratchVariants)
}

// ---- OS helpers ----

func writeToTempFile(_ context.Context, src io.Reader, ext string) (string, func(), error) {
	f, err := os.CreateTemp("", "damask-*"+ext)
	if err != nil {
		return "", nil, fmt.Errorf("create temp: %w", err)
	}
	if _, copyErr := io.Copy(f, src); copyErr != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("copy to temp: %w", copyErr)
	}
	err = f.Close()
	if err != nil {
		return "", nil, fmt.Errorf("close temp: %w", err)
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}

// ---- Helpers ----

// ParseSQLiteTime parses a timestamp string as produced by SQLite's datetime('now').
// It accepts both the SQLite default format ("2006-01-02 15:04:05") and RFC3339.
func ParseSQLiteTime(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognised time format: %q", s)
}

// NextDaily returns the next occurrence of hour:min UTC on or after now.
func NextDaily(hour, minute int) time.Time {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(schedulerCronInterval)
	}
	return next
}

func (s *JobServer) wrapSimpleJob(fn func(context.Context, string) error) queue.HandlerFunc {
	return func(ctx context.Context, job dbgen.Job) error {
		return fn(ctx, job.Payload)
	}
}

func (s *JobServer) jobRunWorkflow(ctx context.Context, job dbgen.Job) error {
	var payload workflow.RunWorkflowPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("parse workflow payload: %w", err)
	}
	return s.workflowExec.Run(ctx, payload.RunID)
}

// failContinuation marks a paused workflow run as failed when the job it
// depends on couldn't complete, so the run doesn't stay "running" forever.
// No-op if cont is nil.
func (s *JobServer) failContinuation(ctx context.Context, cont *workflow.NodeContinuation, cause error) {
	if cont == nil {
		return
	}
	if err := s.workflowExec.FailAt(ctx, *cont, cause); err != nil && !errors.Is(err, cause) {
		slog.ErrorContext(ctx, "workflow continuation fail-at failed",
			"run_id", cont.RunID, "node_id", cont.NodeID, "error", err)
	}
}
