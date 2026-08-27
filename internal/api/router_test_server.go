package api

// This file provides NewTestServer, a constructor intended for use in handler-level
// tests only. It wires service interfaces directly without requiring a real database,
// storage backend, or bcrypt. Do not call this from production code.

import (
	"context"
	"net/url"

	"damask/server/internal/auth"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"
	"damask/server/internal/visualsimilarity"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// TestServerConfig holds all injectable dependencies for NewTestServer.
// Any nil service field is replaced with a no-op stub that returns zero values.
// TokenMaker is required; if nil NewTestServer panics.
type TestServerConfig struct {
	// PreMiddleware handlers are registered before auth middleware and routes.
	// Tests use this to manipulate the request context (e.g. simulate cancellation).
	PreMiddleware []fiber.Handler

	// Core infrastructure
	TokenMaker *auth.Maker
	DB         *dbgen.Queries
	Storage    storage.Storage
	Hub        events.EventHub
	Queue      queue.JobQueue
	Mailer     mail.Mailer
	Cfg        *config.Config

	// Services — leave nil to use no-op defaults
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
	VisualSimilarity *visualsimilarity.Service
	EmbedTokens      service.EmbedTokenService
	AutoTag          service.AutoTagService
}

// NewTestServer constructs a Server from explicit service interfaces and returns
// the server together with a fully configured Fiber app. All routes are registered
// identically to NewRouter; the only difference is that real DB/storage constructors
// are replaced with the mocks supplied in cfg.
//
// For use in tests only — do not call from production code.
func NewTestServer(cfg *TestServerConfig) (*Server, *fiber.App) {
	if cfg.TokenMaker == nil {
		panic("testutil: NewTestServer requires a non-nil TokenMaker")
	}

	// Build a minimal config if the caller did not provide one.
	if cfg.Cfg == nil {
		u, _ := url.Parse("http://localhost")
		cfg.Cfg = &config.Config{
			JWTSecret:    "test-secret-key-must-be-32chars!!",
			AppSecret:    "test-secret-key-must-be-32chars!!",
			AppEnv:       appEnvDevelopment,
			BaseURL:      u,
			EnableSignup: true,
			Telemetry: config.TelemetryConfig{
				ServiceName: "damask",
				Env:         appEnvDevelopment,
			},
		}
	}

	// Use no-op stubs for infrastructure that is nil.
	hub := cfg.Hub
	if hub == nil {
		hub = events.NewEventHub()
	}
	mailer := cfg.Mailer
	if mailer == nil {
		mailer = mail.NewMailer(&mail.Config{})
	}
	stor := cfg.Storage
	if stor == nil {
		var err error
		stor, err = storage.NewAferoMemoryStorage()
		if err != nil {
			panic("testutil: failed to create in-memory storage: " + err.Error())
		}
	}
	q := cfg.Queue
	if q == nil {
		q = &noopQueue{}
	}

	s := &Server{
		readQueries:         cfg.DB,
		auth:                cfg.TokenMaker,
		storage:             stor,
		queue:               q,
		mailer:              mailer,
		hub:                 hub,
		previewCache:        NewLRUPreviewCache(100), //nolint:mnd // arbitrary cache size
		cfg:                 cfg.Cfg,
		demo:                nil,
		assets:              cfg.Assets,
		projects:            cfg.Projects,
		folders:             cfg.Folders,
		tags:                cfg.Tags,
		collections:         cfg.Collections,
		shares:              cfg.Shares,
		sharePublic:         cfg.SharePublic,
		fields:              cfg.Fields,
		integrations:        cfg.Integrations,
		assetFields:         cfg.AssetFields,
		projectFields:       cfg.ProjectFields,
		versions:            cfg.Versions,
		variants:            cfg.Variants,
		textTracks:          cfg.TextTracks,
		auditLog:            cfg.AuditLog,
		workspace:           cfg.Workspace,
		users:               cfg.Users,
		ingress:             cfg.Ingress,
		exports:             cfg.Exports,
		stack:               cfg.Stack,
		upload:              cfg.Upload,
		workflows:           cfg.Workflows,
		visualSimilaritySvc: cfg.VisualSimilarity,
		embedTokens:         cfg.EmbedTokens,
		autoTag:             cfg.AutoTag,
		roleCache:           newRoleCache(),
	}

	app := buildTestApp(s, cfg.PreMiddleware...)
	return s, app
}

func (s *Server) SetConfigForTest(cfg *config.Config) {
	s.cfg = cfg
}

// StorageForTest exposes the server's storage backend for use in tests.
func (s *Server) StorageForTest() storage.Storage {
	return s.storage
}

// buildTestApp registers all routes on a fresh Fiber app using server s.
// The route registrations must stay in sync with NewRouter in router.go.
//
//nolint:funlen // tests
func buildTestApp(s *Server, preMiddleware ...fiber.Handler) *fiber.App {
	tokenMaker := s.auth

	bodyLimit := defaultBodyLimitBytes
	if s.cfg.BodyLimit > 0 {
		bodyLimit = s.cfg.BodyLimit
	}
	app := fiber.New(fiber.Config{
		ErrorHandler: createDefaultErrorHandler(bodyLimit),
		BodyLimit:    bodyLimit,
	})

	app.Use(telemetry.FiberMiddleware())
	app.Use(telemetry.FiberStatusMiddleware())

	for _, mw := range preMiddleware {
		app.Use(mw)
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowCredentials: true,
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	}))

	// Health check (public)
	app.Get("/healthz", handleHealthz)

	// Public server config
	app.Get("/config", auth.OptionalAuth(s.auth), s.handleConfig)
	app.Get("/config/auth", s.handleAuthConfig)

	// Demo routes (no-op without demo build tag)
	s.registerDemoRoutes(app, s.cfg)

	// Auth routes (public)
	authGroup := app.Group("/auth")
	authGroup.Post("/register", s.handleRegister)
	authGroup.Post("/login", s.handleLogin)
	authGroup.Post("/logout", s.handleLogout)
	authGroup.Post("/refresh", auth.RequireAuth(tokenMaker), s.handleRefresh)
	authGroup.Post("/forgot-password", s.handleForgotPassword)
	authGroup.Post("/reset-password", s.handleResetPassword)
	authGroup.Patch("/password", auth.RequireAuth(tokenMaker), demoBlockMiddleware(), s.handleChangePassword)
	authGroup.Get("/confirm-email-change", s.handleConfirmEmailChange)
	app.Post("/api/v1/workflows/:id/webhook", s.handleInboundWorkflowWebhook)

	// Protected API routes
	api := app.Group("/api/v1", auth.RequireAuth(tokenMaker))

	// Workspace
	api.Get("/workspace/me", s.handleWorkspaceMe)
	api.Post("/workspace", s.handleCreateWorkspace)
	api.Get("/workspaces", s.handleListWorkspaces)
	api.Post("/workspace/switch", s.handleSwitchWorkspace)

	getRoleFn := func(ctx context.Context, workspaceID, userID string) (auth.Role, error) {
		if s.workspace == nil {
			return auth.Viewer, nil
		}
		return s.getRole(ctx, workspaceID, userID)
	}
	// Workspace settings — owner only
	api.Put(
		"/workspace/settings",
		demoBlockMiddleware(),
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleUpdateWorkspaceSettings,
	)
	api.Get(
		"/workspace/settings/aiproviders/:provider",
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleGetAIProviderKeyStatus,
	)
	api.Put(
		"/workspace/settings/aiproviders/:provider",
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleSetAIProviderKey,
	)
	api.Delete(
		"/workspace/settings/aiproviders/:provider",
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleClearAIProviderKey,
	)
	api.Post(
		"/workspace/settings/aiproviders/:provider/test",
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleTestAIProviderKey,
	)

	// Generic job trigger — owner only
	api.Post("/workspace/jobs/:type/trigger", auth.RequireRole(getRoleFn, auth.Owner), s.handleTriggerWorkspaceJob)

	api.Get("/admin/telemetry", auth.RequireRole(getRoleFn, auth.Owner), s.handleTelemetryStatus)

	// Invites — owner only
	api.Post("/workspace/invites", demoBlockMiddleware(), auth.RequireRole(getRoleFn, auth.Owner), s.handleCreateInvite)
	api.Get("/workspace/invites", auth.RequireRole(getRoleFn, auth.Owner), s.handleListInvites)
	api.Delete("/workspace/invites/:inviteId", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteInvite)

	// Members — owner only
	api.Get("/workspace/members", auth.RequireRole(getRoleFn, auth.Owner), s.handleListMembers)
	api.Delete("/workspace/members/:userId", auth.RequireRole(getRoleFn, auth.Owner), s.handleRemoveMember)
	api.Put("/workspace/members/:userId", auth.RequireRole(getRoleFn, auth.Owner), s.handleUpdateMemberRole)

	// Invite acceptance (public)
	authGroup.Post("/invite/accept", s.handleAcceptInvite)

	// OIDC / Google / Canva (public)
	authGroup.Get("/oidc/login", s.handleOIDCLogin)
	authGroup.Get("/oidc/callback", s.handleOIDCCallback)
	authGroup.Get("/google/login", s.handleGoogleLogin)
	authGroup.Get("/google/callback", s.handleGoogleCallback)
	authGroup.Get("/canva/login", s.handleCanvaLogin)
	authGroup.Get("/canva/callback", s.handleCanvaCallback)

	// Current user profile
	authGroup.Get("/me", auth.RequireAuth(tokenMaker), s.handleGetMe)
	api.Patch("/users/me", s.handleUpdateMe)
	api.Post("/users/me/avatar", s.handleUploadAvatar)
	api.Delete("/users/me/avatar", s.handleDeleteAvatar)
	api.Post("/users/me/email", s.handleRequestEmailChange)
	api.Delete("/users/me/email/pending", s.handleCancelEmailChange)
	api.Delete("/users/me", s.handleDeleteMe)
	app.Get("/api/v1/users/:id/avatar", s.handleGetAvatar)

	// Unlink providers
	authGroup.Delete("/oidc/link", auth.RequireAuth(tokenMaker), s.handleUnlinkOIDC)
	authGroup.Delete("/google/link", auth.RequireAuth(tokenMaker), s.handleUnlinkGoogle)
	authGroup.Delete("/canva/link", auth.RequireAuth(tokenMaker), s.handleUnlinkCanva)

	// OAuth workspace connections
	api.Get("/integrations/connections", s.handleListConnections)
	api.Delete("/integrations/connections/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleDeleteConnection)
	intGroup := app.Group("/integrations", auth.RequireAuth(tokenMaker))
	intGroup.Get("/connect/google", s.handleConnectGoogle)
	intGroup.Get("/callback/google", s.handleCallbackGoogle)
	intGroup.Get("/connect/canva", s.handleConnectCanva)
	intGroup.Get("/callback/canva", s.handleCallbackCanva)

	// Field definitions
	api.Get("/field-definitions", s.handleListFieldDefinitions)
	api.Post("/field-definitions", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateFieldDefinition)
	api.Put("/field-definitions/reorder", auth.RequireRole(getRoleFn, auth.Editor), s.handleReorderFieldDefinitions)
	api.Get("/field-definitions/:id", s.handleGetFieldDefinition)
	api.Get("/field-definitions/:id/stats", s.handleGetFieldDefinitionStats)
	api.Put("/field-definitions/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleUpdateFieldDefinition)
	api.Delete("/field-definitions/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleDeleteFieldDefinition)

	// Projects
	api.Post("/projects", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateProject)
	api.Get("/projects", s.handleListProjects)
	api.Get("/projects/:id", s.handleGetProject)
	api.Put("/projects/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleUpdateProject)
	api.Delete("/projects/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteProject)

	// Project field values
	api.Get("/projects/:id/fields", s.handleGetProjectFields)
	api.Patch("/projects/:id/fields", auth.RequireRole(getRoleFn, auth.Editor), s.handlePatchProjectFields)

	// Tags
	api.Get("/tags", s.handleListTags)
	api.Post("/tags", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateTag)
	api.Patch("/tags/:name", auth.RequireRole(getRoleFn, auth.Editor), s.handlePatchTag)
	api.Delete("/tags", auth.RequireRole(getRoleFn, auth.Editor), s.handleBulkDeleteTags)
	api.Post("/tags/merge", auth.RequireRole(getRoleFn, auth.Editor), s.handleMergeTags)
	api.Get("/tags/suggestions/duplicates", s.handleTagDuplicateSuggestions)

	// Workflows
	api.Get("/workflows", s.handleListWorkflows)
	api.Post("/workflows", auth.RequireRole(getRoleFn, auth.Owner), s.handleCreateWorkflow)
	api.Get("/workflows/node-schemas", s.handleGetWorkflowNodeSchemas)
	api.Get("/workflows/ocr-languages", s.handleListOCRLanguages)
	api.Get("/workflows/templates", s.handleGetWorkflowTemplates)
	api.Get("/workflows/:id", s.handleGetWorkflow)
	api.Put("/workflows/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleUpdateWorkflow)
	api.Patch("/workflows/:id/enabled", auth.RequireRole(getRoleFn, auth.Owner), s.handleToggleWorkflow)
	api.Delete("/workflows/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteWorkflow)
	api.Post("/workflows/:id/runs/bulk", auth.RequireRole(getRoleFn, auth.Editor), s.handleBulkManualWorkflowRun)
	api.Post("/workflows/:id/runs", auth.RequireRole(getRoleFn, auth.Owner), s.handleManualWorkflowRun)
	api.Get("/workflows/:id/runs", s.handleListWorkflowRuns)
	api.Get("/workflows/:id/runs/:rid", s.handleGetWorkflowRun)
	api.Get("/workflows/:id/webhook-token", auth.RequireRole(getRoleFn, auth.Owner), s.handleGetWorkflowWebhookToken)
	api.Post(
		"/workflows/:id/webhook-token/regenerate",
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleRegenerateWorkflowWebhookToken,
	)

	// Folders
	api.Post("/projects/:id/folders", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateFolder)
	api.Get("/projects/:id/folders", s.handleGetFolders)
	api.Put("/folders/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleUpdateFolder)
	api.Delete("/folders/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteFolder)

	// Stack
	api.Post("/stack/export", auth.RequireRole(getRoleFn, auth.Editor), s.handleStackExport)
	api.Post("/stack/merge", auth.RequireRole(getRoleFn, auth.Editor), s.handleStackMerge)

	// Collections
	api.Get("/collections", s.handleListCollections)
	api.Post("/collections", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateCollection)
	api.Get("/collections/:id", s.handleGetCollection)
	api.Put("/collections/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleUpdateCollection)
	api.Delete("/collections/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteCollection)
	api.Post("/collections/:id/assets/:aid", auth.RequireRole(getRoleFn, auth.Editor), s.handleAddCollectionAsset)
	api.Delete("/collections/:id/assets/:aid", auth.RequireRole(getRoleFn, auth.Editor), s.handleRemoveCollectionAsset)

	// Assets — bulk routes first
	api.Post("/assets/bulk/tag", auth.RequireRole(getRoleFn, auth.Editor), s.handleBulkTag)
	api.Post("/assets/bulk/project", auth.RequireRole(getRoleFn, auth.Editor), s.handleBulkProject)
	api.Delete("/assets/bulk", auth.RequireRole(getRoleFn, auth.Owner), s.handleBulkDelete)
	api.Post("/assets/bulk/fields/preview", auth.RequireRole(getRoleFn, auth.Editor), s.handleBulkFieldsPreview)
	api.Patch("/assets/bulk/fields", auth.RequireRole(getRoleFn, auth.Editor), s.handleBulkPatchAssetFields)

	api.Post("/assets", auth.RequireRole(getRoleFn, auth.Editor), s.handleUploadAsset)
	api.Get("/assets", s.handleListAssets)
	api.Post("/assets/:id/classify", auth.RequireRole(getRoleFn, auth.Editor), s.handleClassifyAsset)
	api.Patch("/assets/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleUpdateAssetFolder)
	api.Put("/assets/:id/rename", auth.RequireRole(getRoleFn, auth.Editor), s.handleRenameAsset)
	api.Get("/assets/:id", s.handleGetAsset)
	api.Get("/assets/:id/comments", s.handleGetComments)
	api.Get("/assets/:id/file", s.handleGetAssetFile)
	api.Get("/assets/:id/thumb", s.handleGetAssetThumb)
	api.Delete("/assets/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleDeleteAsset)

	// Asset field values
	api.Get("/assets/:id/fields", s.handleGetAssetFields)
	api.Patch("/assets/:id/fields", auth.RequireRole(getRoleFn, auth.Editor), s.handlePatchAssetFields)

	// Asset collections membership
	api.Get("/assets/:id/collections", s.handleListAssetCollections)

	// Asset tags
	api.Get("/assets/:id/tags", s.handleGetAssetTags)
	api.Post("/assets/:id/tags", auth.RequireRole(getRoleFn, auth.Editor), s.handleAddTagToAsset)
	api.Delete("/assets/:id/tags/:name", auth.RequireRole(getRoleFn, auth.Editor), s.handleRemoveTagFromAsset)

	// AI auto-tagging — accept-all must be registered before /:sid/accept.
	api.Post("/assets/:id/auto-tag", auth.RequireRole(getRoleFn, auth.Editor), s.handleTriggerAutoTag)
	api.Get("/assets/:id/auto-tag/suggestions", s.handleListAutoTagSuggestions)
	api.Post(
		"/assets/:id/auto-tag/suggestions/accept-all",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleAcceptAllAutoTagSuggestions,
	)
	api.Post(
		"/assets/:id/auto-tag/suggestions/:sid/accept",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleAcceptAutoTagSuggestion,
	)
	api.Delete(
		"/assets/:id/auto-tag/suggestions/:sid",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleDismissAutoTagSuggestion,
	)
	api.Delete(
		"/assets/:id/auto-tag/suggestions",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleDismissAllAutoTagSuggestions,
	)

	// Public embed tokens — authenticated CRUD; the public /e/:token routes are below.
	api.Post("/assets/:id/embed-token", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateEmbedToken)
	api.Get("/assets/:id/embed-token", s.handleGetEmbedToken)
	api.Delete("/assets/:id/embed-token", auth.RequireRole(getRoleFn, auth.Editor), s.handleDeleteEmbedToken)

	// AI providers
	api.Get("/aiproviders", s.handleListAIProviders)
	api.Get("/ai/vision-models", s.handleListVisionModels)

	// Variants
	api.Get("/assets/:id/variants", s.handleListVariants)
	api.Get("/assets/:id/variants/watermark", s.handleResolveWatermarkAsset)
	api.Post("/assets/:id/variants", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateVariant)
	api.Post("/assets/:id/variants/automate", auth.RequireRole(getRoleFn, auth.Editor), s.handleAutomateVariants)
	api.Post("/assets/:id/variants/upload", auth.RequireRole(getRoleFn, auth.Editor), s.handleUploadManualVariant)
	// Draft routes — before /:vid
	api.Post("/assets/:id/variants/draft", auth.RequireRole(getRoleFn, auth.Editor), s.handleGenerateDraft)
	api.Get("/assets/:id/variants/draft/:nonce/preview", s.handlePreviewDraft)
	api.Post("/assets/:id/variants/draft/:nonce/commit", auth.RequireRole(getRoleFn, auth.Editor), s.handleCommitDraft)
	api.Delete("/assets/:id/variants/draft/:nonce", auth.RequireRole(getRoleFn, auth.Editor), s.handleDiscardDraft)
	api.Put("/assets/:id/variants/sharing", auth.RequireRole(getRoleFn, auth.Editor), s.handleUpdateVariantsSharing)
	api.Patch("/assets/:id/variants/:vid", auth.RequireRole(getRoleFn, auth.Editor), s.handlePatchVariant)
	api.Post("/assets/:id/variants/:vid/promote", auth.RequireRole(getRoleFn, auth.Editor), s.handlePromoteVariant)
	api.Post(
		"/assets/:id/variants/:vid/set-thumbnail",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleSetVariantThumbnail,
	)
	api.Post("/assets/:id/variants/:vid/rerun", auth.RequireRole(getRoleFn, auth.Editor), s.handleRerunVariant)
	api.Get("/assets/:id/variants/:vid/file", s.handleGetVariantFile)
	api.Delete("/assets/:id/variants/:vid", auth.RequireRole(getRoleFn, auth.Editor), s.handleDeleteVariant)

	api.Get("/assets/:id/text-tracks", s.handleListTextTracks)
	api.Post("/assets/:id/text-tracks", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateTextTrack)
	api.Get("/assets/:id/text-tracks/:tid", s.handleGetTextTrack)
	api.Get("/assets/:id/text-tracks/:tid/download", s.handleDownloadTextTrack)
	api.Delete("/assets/:id/text-tracks/:tid", auth.RequireRole(getRoleFn, auth.Editor), s.handleDeleteTextTrack)

	// Asset versions
	api.Get("/assets/:id/versions", s.handleListAssetVersions)
	api.Post("/assets/:id/versions", auth.RequireRole(getRoleFn, auth.Editor), s.handleUploadAssetVersion)
	api.Post("/assets/:id/versions/:vid/restore", auth.RequireRole(getRoleFn, auth.Editor), s.handleRestoreAssetVersion)
	api.Get("/assets/:id/versions/:vid/file", s.handleGetVersionFile)
	api.Get("/assets/:id/versions/:vid/thumb", s.handleGetVersionThumb)
	api.Delete("/assets/:id/versions/:vid", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteAssetVersion)

	// Transform preview
	api.Get("/assets/:id/preview", s.handlePreviewTransform)

	// SSE events
	api.Get("/events", s.handleSSEEvents)

	// Event logs
	api.Get("/assets/:id/events", s.handleListAssetEvents)
	api.Get("/projects/:id/events", s.handleListProjectEvents)

	// Workspace activity
	api.Get("/activity", s.handleListWorkspaceActivity)
	api.Get("/activity/export", s.handleExportActivity)

	// Ingress
	ingressGroup := api.Group("/ingress")
	ingressGroup.Post("/sources", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateIngressSource)
	ingressGroup.Get("/sources", s.handleListIngressSources)
	ingressGroup.Get("/sources/:id", s.handleGetIngressSource)
	ingressGroup.Put("/sources/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleUpdateIngressSource)
	ingressGroup.Delete("/sources/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteIngressSource)
	ingressGroup.Post("/sources/:id/test", auth.RequireRole(getRoleFn, auth.Editor), s.handleTestIngressSource)
	ingressGroup.Post(
		"/sources/:id/poll",
		demoBlockMiddleware(),
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handlePollIngressSource,
	)
	ingressGroup.Get("/sources/:id/log", s.handleListIngressSourceLog)
	ingressGroup.Get("/log", s.handleListIngressLog)
	ingressGroup.Delete("/log/:entry_id", auth.RequireRole(getRoleFn, auth.Editor), s.handleDeleteIngressLogEntry)
	ingressGroup.Post("/log/:entry_id/retry", auth.RequireRole(getRoleFn, auth.Editor), s.handleRetryIngressLogEntry)

	// Ingress rules
	ingressGroup.Get("/sources/:id/rules", s.handleListIngressRules)
	ingressGroup.Post("/sources/:id/rules", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateIngressRule)
	ingressGroup.Put(
		"/sources/:id/rules/reorder",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleReorderIngressRules,
	)
	ingressGroup.Put("/sources/:id/rules/:rid", auth.RequireRole(getRoleFn, auth.Editor), s.handleUpdateIngressRule)
	ingressGroup.Delete("/sources/:id/rules/:rid", auth.RequireRole(getRoleFn, auth.Editor), s.handleDeleteIngressRule)

	// Exports
	api.Post("/exports", auth.RequireRole(getRoleFn, auth.Owner), s.handleCreateExportConfig)
	api.Get("/exports", s.handleListExportConfigs)
	api.Get("/exports/:id", s.handleGetExportConfig)
	api.Put("/exports/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleUpdateExportConfig)
	api.Delete("/exports/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteExportConfig)
	api.Post("/exports/:id/validate-destination", s.handleValidateExportDestination)
	api.Post("/exports/:id/trigger", s.handleTriggerExport)
	api.Get("/exports/:id/runs", s.handleListExportRuns)
	api.Get("/exports/:id/runs/:runID", s.handleGetExportRun)

	// Shares (authenticated)
	api.Post("/shares", s.handleCreateShare)
	api.Get("/shares", s.handleListShares)
	api.Get("/shares/:id", s.handleGetShare)
	api.Put("/shares/:id", s.handleUpdateShare)
	api.Delete("/shares/:id", s.handleRevokeShare)

	// Share comments
	api.Get("/shares/:id/comments", s.handleOwnerListComments)
	api.Delete("/shares/:id/comments/:cid", s.handleOwnerDeleteComment)

	// Public share routes
	app.Get("/shared/:id/access", s.handleShareInfo)
	app.Post("/shared/:id/access", s.handleShareAccess)

	shareGroup := app.Group("/shared/:id", auth.RequireShareSession(tokenMaker))
	shareGroup.Get("/export", s.handleShareExport)
	shareGroup.Get("/assets", s.handleShareListAssets)
	shareGroup.Get("/assets/:aid", s.handleShareGetAsset)
	shareGroup.Get("/assets/:aid/file", s.handleShareGetAssetFile)
	shareGroup.Get("/assets/:aid/thumb", s.handleShareGetAssetThumb)
	shareGroup.Get("/assets/:aid/variants/:vid/file", s.handleShareGetVariantFile)
	shareGroup.Get("/assets/:aid/variants/:vid/thumb", s.handleShareGetVariantThumb)
	shareGroup.Post("/comments", s.handleShareCreateComment)
	shareGroup.Get("/comments", s.handleShareListComments)
	shareGroup.Get("/assets/:aid/comments", s.handleShareListAssetComments)

	// Public embed token routes
	app.Get("/e/:token", s.handlePublicEmbedFile)
	app.Get("/e/:token/thumb", s.handlePublicEmbedThumb)

	return app
}

// noopQueue is a minimal no-op implementation of queue.JobQueue for tests
// that do not need job enqueueing.
type noopQueue struct{}

func (noopQueue) Register(_ string, _ queue.HandlerFunc) {}

func (noopQueue) Enqueue(_ context.Context, _, _, _ string) (dbgen.Job, error) {
	return dbgen.Job{}, nil
}

func (noopQueue) Start(_ context.Context) {}
func (noopQueue) Stop()                   {}
