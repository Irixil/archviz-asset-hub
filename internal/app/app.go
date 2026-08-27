// Package app is the single composition root for Damask's dependency graph.
// Build constructs every repository and service exactly once; both the HTTP
// server (internal/api) and the job server (internal/jobs) consume the
// resulting *Deps instead of each wiring their own copy. This guarantees
// that any given asset/variant/tag mutation goes through the same service
// instance regardless of whether it originated from an HTTP request or a
// background job — no divergent behavior between the two entry points.
package app

import (
	"errors"

	"damask/server/internal/ai"
	"damask/server/internal/audit"
	"damask/server/internal/config"
	"damask/server/internal/db"
	"damask/server/internal/events"
	"damask/server/internal/mail"
	"damask/server/internal/media/ingest"
	"damask/server/internal/queue"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
	"damask/server/internal/visualsimilarity"
	"damask/server/internal/workflow"
)

// Deps is the fully constructed dependency graph shared by every server
// entry point (HTTP API, job workers). Every field is populated exactly
// once by Build; nothing here is workspace-scoped state — services here
// must derive any per-workspace behavior from the workspaceID argument on
// each call, never from server-side mutable state keyed on anything else.
type Deps struct {
	// Infra
	Config      *config.Config
	DB          *db.DB
	Storage     storage.Storage
	Queue       queue.JobQueue
	Hub         events.EventHub
	Mailer      mail.Mailer
	Transformer transform.Transformer
	Thumbnailer transform.Thumbnailer
	Media       *ingest.Registry
	Audit       *audit.EventWriter

	// Cross-cutting infra
	KeyResolver       ai.KeyResolver
	AIProviderFactory ai.ProviderFactory
	TriggerDispatcher *workflow.TriggerDispatcher
	WorkflowExec      *workflow.Executor

	// Services — single instance shared by API handlers and job handlers.
	Assets           service.AssetService
	Projects         service.ProjectService
	Folders          service.FolderService
	Tags             service.TagService
	Collections      service.CollectionService
	Shares           service.ShareService
	SharePublic      service.SharePublicService
	Fields           service.FieldService
	Integrations     service.IntegrationService
	AssetFields      service.AssetFieldService
	ProjectFields    service.ProjectFieldService
	Versions         service.VersionService
	Variants         service.VariantService
	TextTracks       service.TextTrackService
	AuditLog         service.AuditLogService
	Workspace        service.WorkspaceService
	Users            service.UserService
	Ingress          service.IngressService
	Exports          service.ExportService
	Stack            service.StackService
	Upload           service.UploadService
	Workflows        service.WorkflowService
	StorageSvc       service.StorageService
	AutoTag          service.AutoTagService
	EmbedTokens      service.EmbedTokenService
	VisualSimilarity *visualsimilarity.Service
	Exif             *service.ExifService
	Ingester         service.AssetIngester
}

// Build constructs the entire dependency graph exactly once. cfg, database,
// stor, hub, q, mailer, trf and tmb must already be initialized by the
// caller (they involve I/O — opening the DB, connecting to storage — that
// callers may want to sequence or fail fast on independently).
func Build(
	cfg *config.Config,
	database *db.DB,
	stor storage.Storage,
	hub events.EventHub,
	q queue.JobQueue,
	mailer mail.Mailer,
	trf transform.Transformer,
	tmb transform.Thumbnailer,
) (*Deps, error) {
	if cfg == nil || database == nil || stor == nil || hub == nil || q == nil {
		return nil, errors.New("app: Build requires non-nil cfg, database, storage, hub and queue")
	}

	media := ingest.NewRegistry(trf)
	auditWriter := audit.New(database.Writer)
	r := buildRepos(database)

	// --- cross-cutting infra ---
	keyResolver := ai.NewKeyResolver(r.workspace, *cfg)
	triggerDispatcher := workflow.NewTriggerDispatcher(r.workflow, r.workflowRun, q)
	storageSvc := service.NewStorageService(r.storageStats)

	// --- single TagService instance, real dispatcher wired in. ---
	// Both API-driven tag mutations and workflow "set tag" actions go
	// through this instance; workflow.TriggerDispatcher's depth guard
	// (workflow.WithTriggerDepth / TriggerDepthFrom) prevents a workflow's
	// own tag mutation from re-triggering itself.
	tagSvc := service.NewTagService(r.tag, auditWriter, service.TagServiceDeps{
		Assets:   r.asset,
		Triggers: triggerDispatcher,
	})

	autoTagSvc := service.NewAutoTagService(r.asset, r.workspace, r.autoTagSug, q, tagSvc, keyResolver)
	duplicateSvc := service.NewDuplicateService(r.version, r.asset, r.workspace, stor)
	ingester := service.NewAssetIngester(r.asset, r.version, stor, q, media, autoTagSvc, duplicateSvc, tagSvc)

	// variantSvc always carries Workflows + Invalidate: previously main.go's
	// job-server copy omitted them, so background-job variant operations
	// skipped storage-size invalidation and workflow lookups performed by
	// API-driven ones. The single instance here converges both call paths.
	variantSvc := service.NewVariantServiceWithDeps(
		r.variant,
		r.asset,
		tagSvc,
		auditWriter,
		service.VariantServiceDeps{
			Actions:    service.NewSQLVariantActionsStore(database.Writer),
			Queue:      q,
			Storage:    stor,
			Workflows:  r.workflow,
			Invalidate: storageSvc,
		},
	)

	fieldSvc := service.NewFieldService(r.field)
	assetSvc := service.NewAssetService(
		r.asset, r.version, r.tag, r.field, stor, auditWriter, q, storageSvc,
	)
	assetFieldSvc := service.NewAssetFieldService(r.asset, r.field, r.assetField, auditWriter)
	shareSvc := service.NewShareService(r.share, auditWriter)
	workspaceSvc := service.NewWorkspaceService(r.workspace, r.user, cfg.AppSecret, keyResolver)
	exportSvc := service.NewExportService(database, stor, cfg.AppSecret, q, r.project)
	exifSvc := service.NewExifService(r.workspace, r.asset, r.field, r.assetField, stor)
	textTrackSvc := service.NewTextTrackService(r.textTrack, q, stor)

	versionSvc := service.NewVersionService(r.version, auditWriter, service.VersionServiceDeps{
		Assets:     r.asset,
		Storage:    stor,
		Queue:      q,
		Media:      media,
		Triggers:   triggerDispatcher,
		Invalidate: storageSvc,
		AutoTag:    autoTagSvc,
	})

	// --- workflow executor ---
	// Built after the services it wraps so its adapters close over the same
	// single instances used everywhere else in the graph.
	workflowExec := workflow.NewExecutor(workflow.Deps{
		Workflows:   r.workflow,
		Runs:        r.workflowRun,
		Queue:       q,
		Storage:     stor,
		Mailer:      mailer,
		Hub:         hub,
		Audit:       auditWriter,
		Assets:      newAssetManager(assetSvc),
		Variants:    newVariantManager(variantSvc),
		Versions:    newVersionManager(r.version),
		Shares:      newShareManager(shareSvc),
		Tags:        newTagManager(tagSvc),
		AssetFields: newAssetFieldManager(assetFieldSvc),
		Workspace:   newWorkspaceManager(workspaceSvc),
		TextTracks:  newTextTrackManager(textTrackSvc),
		AutoTag:     newAutoTagManager(autoTagSvc),
		Config:      cfg,
	})

	return &Deps{
		Config:            cfg,
		DB:                database,
		Storage:           stor,
		Queue:             q,
		Hub:               hub,
		Mailer:            mailer,
		Transformer:       trf,
		Thumbnailer:       tmb,
		Media:             media,
		Audit:             auditWriter,
		KeyResolver:       keyResolver,
		AIProviderFactory: ai.NewProvider,
		TriggerDispatcher: triggerDispatcher,
		WorkflowExec:      workflowExec,

		Assets:        assetSvc,
		Projects:      service.NewProjectService(r.project, auditWriter),
		Folders:       service.NewFolderService(r.folder),
		Tags:          tagSvc,
		Collections:   service.NewCollectionService(r.collection, r.asset),
		Shares:        shareSvc,
		SharePublic:   service.NewSharePublicService(r.share, r.user, r.variant, mailer),
		Fields:        fieldSvc,
		Integrations:  service.NewIntegrationService(reposqlc.NewOAuthRepo(database)),
		AssetFields:   assetFieldSvc,
		ProjectFields: service.NewProjectFieldService(r.project, r.field, r.projectField, auditWriter),
		Versions:      versionSvc,
		Variants:      variantSvc,
		TextTracks:    textTrackSvc,
		AuditLog:      service.NewAuditLogService(r.auditLog),
		Workspace:     workspaceSvc,
		Users:         service.NewUserService(r.user, r.workspace, stor),
		Ingress:       service.NewIngressService(r.ingress, r.user, cfg.AppSecret, q, mailer),
		Exports:       exportSvc,
		Stack:         service.NewStackService(r.asset, r.version, r.variant, stor, q),
		Upload: service.NewUploadService(
			ingester,
			auditWriter,
			storageSvc,
			triggerDispatcher,
		),
		Workflows: service.NewWorkflowService(
			r.workflow,
			r.workflowRun,
			r.workflowWebhook,
			q,
			service.WorkflowServiceDeps{Assets: r.asset, Variants: r.variant, Versions: r.version},
		),
		StorageSvc:       storageSvc,
		AutoTag:          autoTagSvc,
		EmbedTokens:      service.NewEmbedTokenService(r.embedToken, r.asset, r.version, cfg.BaseURL.String()),
		VisualSimilarity: visualsimilarity.NewService(database.WQ, database.Writer),
		Exif:             exifSvc,
		Ingester:         ingester,
	}, nil
}
