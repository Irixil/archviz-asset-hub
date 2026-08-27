package api

import (
	"errors"
	"fmt"
	"io/fs"

	appcomp "damask/server/internal/app"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/mail"
	"damask/server/internal/media/ingest"
	"damask/server/internal/queue"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"
	"damask/server/internal/visualsimilarity"

	"golang.org/x/crypto/bcrypt"

	"github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	_ "damask/server/internal/docs" //nolint:nolintlint // to register swaggo docs
)

const defaultBodyLimitBytes = 100 * 1024 * 1024 // 100 MB

// Server holds shared dependencies injected at startup.
type Server struct {
	readQueries         *dbgen.Queries
	auth                *auth.Maker
	storage             storage.Storage
	queue               queue.JobQueue
	mailer              mail.Mailer
	hub                 events.EventHub
	previewCache        *LruPreviewCache
	cfg                 *config.Config
	trf                 transform.Transformer
	media               *ingest.Registry
	demo                DemoSeeder // nil when demo build tag is not set
	assets              service.AssetService
	projects            service.ProjectService
	folders             service.FolderService
	tags                service.TagService
	collections         service.CollectionService
	shares              service.ShareService
	sharePublic         service.SharePublicService
	fields              service.FieldService
	integrations        service.IntegrationService
	assetFields         service.AssetFieldService
	projectFields       service.ProjectFieldService
	versions            service.VersionService
	variants            service.VariantService
	textTracks          service.TextTrackService
	auditLog            service.AuditLogService
	workspace           service.WorkspaceService
	users               service.UserService
	ingress             service.IngressService
	exports             service.ExportService
	stack               service.StackService
	upload              service.UploadService
	workflows           service.WorkflowService
	storageSvc          service.StorageService
	visualSimilaritySvc *visualsimilarity.Service
	embedTokens         service.EmbedTokenService
	autoTag             service.AutoTagService
	bcryptCost          int
	roleCache           *roleCache
}

// NewHTTPServer builds a Server that reads all of its dependencies from the
// single composition root (app.Deps) instead of constructing repositories
// and services itself — deps here are the same instances the job server
// uses, so API-driven and background-job mutations behave identically.
func NewHTTPServer(
	deps *appcomp.Deps,
	tokenMaker *auth.Maker,
	demoSeeder DemoSeeder,
	bcryptCost int,
) *Server {
	return &Server{
		readQueries:         deps.DB.RQ,
		auth:                tokenMaker,
		storage:             deps.Storage,
		queue:               deps.Queue,
		mailer:              deps.Mailer,
		hub:                 deps.Hub,
		previewCache:        NewLRUPreviewCache(100), //nolint:mnd // arbitrary cache size
		cfg:                 deps.Config,
		trf:                 deps.Transformer,
		media:               deps.Media,
		demo:                demoSeeder,
		assets:              deps.Assets,
		projects:            deps.Projects,
		folders:             deps.Folders,
		tags:                deps.Tags,
		collections:         deps.Collections,
		shares:              deps.Shares,
		sharePublic:         deps.SharePublic,
		fields:              deps.Fields,
		integrations:        deps.Integrations,
		assetFields:         deps.AssetFields,
		projectFields:       deps.ProjectFields,
		versions:            deps.Versions,
		variants:            deps.Variants,
		textTracks:          deps.TextTracks,
		auditLog:            deps.AuditLog,
		workspace:           deps.Workspace,
		users:               deps.Users,
		ingress:             deps.Ingress,
		exports:             deps.Exports,
		stack:               deps.Stack,
		upload:              deps.Upload,
		workflows:           deps.Workflows,
		storageSvc:          deps.StorageSvc,
		visualSimilaritySvc: deps.VisualSimilarity,
		embedTokens:         deps.EmbedTokens,
		autoTag:             deps.AutoTag,
		bcryptCost:          bcryptCost,
		roleCache:           newRoleCache(),
	}
}

// NewRouter creates a configured Fiber app with all routes registered.
// @title Damask Swagger API
// @version 1.0
// @description This is a Damask server.
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@getdamask.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath /
// @schemes http.
//
// NOTE: when adding, removing, or re-annotating a route below, update the route
// table in CLAUDE.md ("Route layout") in the same change — it doubles as the
// access-control spec for agent-driven development.
//
//nolint:funlen // router
func NewRouter(
	deps *appcomp.Deps,
	tokenMaker *auth.Maker,
	demoSeeder DemoSeeder,
	uiFS fs.FS,
) *fiber.App {
	s := NewHTTPServer(deps, tokenMaker, demoSeeder, bcrypt.DefaultCost)

	cfg := deps.Config
	bodyLimit := defaultBodyLimitBytes
	if cfg.BodyLimit > 0 {
		bodyLimit = cfg.BodyLimit
	}
	app := fiber.New(fiber.Config{
		ErrorHandler: createDefaultErrorHandler(bodyLimit),
		BodyLimit:    bodyLimit,
	})

	app.Use(telemetry.FiberMiddleware())
	app.Use(telemetry.FiberStatusMiddleware())

	allowOrigins := []string{s.cfg.BaseURL.String()}
	if cfg.AppEnv == appEnvDevelopment {
		allowOrigins = append(allowOrigins, "http://localhost:5173")
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowOrigins,
		AllowCredentials: true,
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	}))

	// Health check (public)
	app.Get("/healthz", handleHealthz)

	// Public server config (demo flag, etc.)
	app.Get("/config", auth.OptionalAuth(s.auth), s.handleConfig)
	app.Get("/config/auth", s.handleAuthConfig)

	// Demo routes — only compiled and registered with -tags=demo
	s.registerDemoRoutes(app, cfg)

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
	app.Get("/api/v1/users/:id/avatar", s.handleGetAvatar)
	app.Post("/api/v1/workflows/:id/webhook", s.handleInboundWorkflowWebhook)

	// Protected API routes
	api := app.Group("/api/v1", auth.RequireAuth(tokenMaker))

	// Workspace
	api.Get("/workspace/me", s.handleWorkspaceMe)
	api.Post("/workspace", s.handleCreateWorkspace)
	api.Get("/workspaces", s.handleListWorkspaces)
	api.Post("/workspace/switch", s.handleSwitchWorkspace)

	getRoleFn := s.getRole

	// AI providers
	api.Get("/aiproviders", s.handleListAIProviders)
	api.Get("/ai/vision-models", s.handleListVisionModels)

	// Workspace settings — owner only; blocked in demo mode
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
	api.Post(
		"/workspace/jobs/:type/trigger",
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleTriggerWorkspaceJob,
	)

	api.Get("/admin/telemetry", auth.RequireRole(getRoleFn, auth.Owner), s.handleTelemetryStatus)

	// Invites — owner only; blocked in demo mode
	api.Post(
		"/workspace/invites",
		demoBlockMiddleware(),
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleCreateInvite,
	)
	api.Get("/workspace/invites", auth.RequireRole(getRoleFn, auth.Owner), s.handleListInvites)
	api.Delete(
		"/workspace/invites/:inviteId",
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleDeleteInvite,
	)

	// Members — owner only
	api.Get("/workspace/members", auth.RequireRole(getRoleFn, auth.Owner), s.handleListMembers)
	api.Delete("/workspace/members/:userId", auth.RequireRole(getRoleFn, auth.Owner), s.handleRemoveMember)
	api.Put("/workspace/members/:userId", auth.RequireRole(getRoleFn, auth.Owner), s.handleUpdateMemberRole)

	api.Get("/workspace/storage", s.handleGetWorkspaceStorage)
	api.Get("/workspace/storage/projects/:project_id/folders", s.handleGetProjectFolderStorage)

	// Invite acceptance is public — the caller has no account yet
	authGroup.Post("/invite/accept", s.handleAcceptInvite)

	// OIDC / Google / Canva login flows (public)
	authGroup.Get("/oidc/login", s.handleOIDCLogin)
	authGroup.Get("/oidc/callback", s.handleOIDCCallback)
	authGroup.Get("/google/login", s.handleGoogleLogin)
	authGroup.Get("/google/callback", s.handleGoogleCallback)
	authGroup.Get("/canva/login", s.handleCanvaLogin)
	authGroup.Get("/canva/callback", s.handleCanvaCallback)

	// Current user profile + linked identities
	authGroup.Get("/me", auth.RequireAuth(tokenMaker), s.handleGetMe)
	api.Patch("/users/me", s.handleUpdateMe)
	api.Post("/users/me/avatar", s.handleUploadAvatar)
	api.Delete("/users/me/avatar", s.handleDeleteAvatar)
	api.Post("/users/me/email", s.handleRequestEmailChange)
	api.Delete("/users/me/email/pending", s.handleCancelEmailChange)
	api.Delete("/users/me", s.handleDeleteMe)

	// Unlink identity providers (authenticated)
	authGroup.Delete("/oidc/link", auth.RequireAuth(tokenMaker), s.handleUnlinkOIDC)
	authGroup.Delete("/google/link", auth.RequireAuth(tokenMaker), s.handleUnlinkGoogle)
	authGroup.Delete("/canva/link", auth.RequireAuth(tokenMaker), s.handleUnlinkCanva)

	// OAuth workspace connections (authenticated)
	api.Get("/integrations/connections", s.handleListConnections)
	api.Delete(
		"/integrations/connections/:id",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleDeleteConnection,
	)
	intGroup := app.Group("/integrations", auth.RequireAuth(tokenMaker))
	intGroup.Get("/connect/google", s.handleConnectGoogle)
	intGroup.Get("/callback/google", s.handleCallbackGoogle)
	intGroup.Get("/connect/canva", s.handleConnectCanva)
	intGroup.Get("/callback/canva", s.handleCallbackCanva)

	// Field definitions — reorder must be registered before /:id to avoid conflict
	api.Get("/field-definitions", s.handleListFieldDefinitions)
	api.Post("/field-definitions", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateFieldDefinition)
	api.Put(
		"/field-definitions/reorder",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleReorderFieldDefinitions,
	)
	api.Get("/field-definitions/:id", s.handleGetFieldDefinition)
	api.Get("/field-definitions/:id/stats", s.handleGetFieldDefinitionStats)
	api.Put(
		"/field-definitions/:id",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleUpdateFieldDefinition,
	)
	api.Delete(
		"/field-definitions/:id",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleDeleteFieldDefinition,
	)

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
	api.Get("/workflows/runs", s.handleListAllWorkflowRuns)
	api.Get("/workflows/:id", s.handleGetWorkflow)
	api.Put("/workflows/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleUpdateWorkflow)
	api.Patch("/workflows/:id/enabled", auth.RequireRole(getRoleFn, auth.Owner), s.handleToggleWorkflow)
	api.Delete("/workflows/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteWorkflow)
	api.Post("/workflows/:id/runs/bulk", auth.RequireRole(getRoleFn, auth.Editor), s.handleBulkManualWorkflowRun)
	api.Post("/workflows/:id/runs", auth.RequireRole(getRoleFn, auth.Owner), s.handleManualWorkflowRun)
	api.Get("/workflows/:id/runs", s.handleListWorkflowRuns)
	api.Get("/workflows/:id/runs/:rid", s.handleGetWorkflowRun)
	api.Get(
		"/workflows/:id/webhook-token",
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleGetWorkflowWebhookToken,
	)
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

	// Stack — export and merge
	api.Post("/stack/export", auth.RequireRole(getRoleFn, auth.Editor), s.handleStackExport)
	api.Post("/stack/merge", auth.RequireRole(getRoleFn, auth.Editor), s.handleStackMerge)

	// Collections
	api.Get("/collections", s.handleListCollections)
	api.Post("/collections", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateCollection)
	api.Get("/collections/:id", s.handleGetCollection)
	api.Put("/collections/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleUpdateCollection)
	api.Delete("/collections/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteCollection)
	api.Post(
		"/collections/:id/assets/:aid",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleAddCollectionAsset,
	)
	api.Delete(
		"/collections/:id/assets/:aid",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleRemoveCollectionAsset,
	)

	// Assets — bulk routes must be registered before /:id to avoid conflict
	api.Post("/assets/bulk/tag", auth.RequireRole(getRoleFn, auth.Editor), s.handleBulkTag)
	api.Post("/assets/bulk/project", auth.RequireRole(getRoleFn, auth.Editor), s.handleBulkProject)
	api.Delete("/assets/bulk", auth.RequireRole(getRoleFn, auth.Owner), s.handleBulkDelete)
	api.Post(
		"/assets/bulk/fields/preview",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleBulkFieldsPreview,
	)
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
	api.Post(
		"/assets/:id/thumb/regenerate",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleRegenerateThumbnail,
	)
	api.Delete("/assets/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleDeleteAsset)

	// Asset field values
	api.Get("/assets/:id/fields", s.handleGetAssetFields)
	api.Patch("/assets/:id/fields", auth.RequireRole(getRoleFn, auth.Editor), s.handlePatchAssetFields)

	// Asset collections membership
	api.Get("/assets/:id/collections", s.handleListAssetCollections)

	// Asset tags
	api.Get("/assets/:id/tags", s.handleGetAssetTags)
	api.Post("/assets/:id/tags", auth.RequireRole(getRoleFn, auth.Editor), s.handleAddTagToAsset)
	api.Delete(
		"/assets/:id/tags/:name",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleRemoveTagFromAsset,
	)

	// AI auto-tagging — accept-all must be registered before /:sid/accept to
	// prevent Fiber from matching the literal "accept-all" as a :sid param.
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

	// Variants
	api.Get("/variants/validate-command", s.handleValidateCustomFFmpegCommand)
	api.Get("/variant-param-history", s.handleGetVariantParamHistory)
	api.Get("/assets/:id/variants", s.handleListVariants)
	api.Get("/assets/:id/variants/watermark", s.handleResolveWatermarkAsset)
	api.Post("/assets/:id/variants", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateVariant)
	api.Post(
		"/assets/:id/variants/automate",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleAutomateVariants,
	)
	api.Post(
		"/assets/:id/variants/upload",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleUploadManualVariant,
	)
	// Draft routes — must be registered before /:vid to prevent "draft" being captured as a vid.
	api.Post("/assets/:id/variants/draft", auth.RequireRole(getRoleFn, auth.Editor), s.handleGenerateDraft)
	api.Get("/assets/:id/variants/draft/:nonce/preview", auth.RequireRole(getRoleFn, auth.Editor), s.handlePreviewDraft)
	api.Post(
		"/assets/:id/variants/draft/:nonce/commit",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleCommitDraft,
	)
	api.Delete(
		"/assets/:id/variants/draft/:nonce",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleDiscardDraft,
	)
	api.Put(
		"/assets/:id/variants/sharing",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleUpdateVariantsSharing,
	)
	api.Patch("/assets/:id/variants/:vid", auth.RequireRole(getRoleFn, auth.Editor), s.handlePatchVariant)
	api.Post(
		"/assets/:id/variants/:vid/promote",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handlePromoteVariant,
	)
	api.Post(
		"/assets/:id/variants/:vid/set-thumbnail",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleSetVariantThumbnail,
	)
	api.Post(
		"/assets/:id/variants/:vid/rerun",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleRerunVariant,
	)
	api.Get("/assets/:id/variants/:vid/file", s.handleGetVariantFile)
	api.Get("/assets/:id/variants/:vid/thumb", s.handleGetVariantThumb)
	api.Delete("/assets/:id/variants/:vid", auth.RequireRole(getRoleFn, auth.Editor), s.handleDeleteVariant)

	// Text tracks
	api.Get("/assets/:id/text-tracks", s.handleListTextTracks)
	api.Post("/assets/:id/text-tracks", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateTextTrack)
	api.Get("/assets/:id/text-tracks/:tid", s.handleGetTextTrack)
	api.Get("/assets/:id/text-tracks/:tid/download", s.handleDownloadTextTrack)
	api.Delete(
		"/assets/:id/text-tracks/:tid",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleDeleteTextTrack,
	)

	// Asset versions
	api.Get("/assets/:id/versions", s.handleListAssetVersions)
	api.Post("/assets/:id/versions", auth.RequireRole(getRoleFn, auth.Editor), s.handleUploadAssetVersion)
	api.Post(
		"/assets/:id/versions/:vid/restore",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleRestoreAssetVersion,
	)
	api.Get("/assets/:id/versions/:vid/file", s.handleGetVersionFile)
	api.Get("/assets/:id/versions/:vid/thumb", s.handleGetVersionThumb)
	api.Delete(
		"/assets/:id/versions/:vid",
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleDeleteAssetVersion,
	)

	// Visual similarity search
	api.Get("/assets/:id/similar", s.handleGetSimilarAssets)

	// Transform preview (in-memory, no storage write)
	api.Get("/assets/:id/preview", s.handlePreviewTransform)

	// Server-Sent Events (workspace-scoped)
	api.Get("/events", s.handleSSEEvents)

	// Asset event log
	api.Get("/assets/:id/events", s.handleListAssetEvents)

	// Project event log
	api.Get("/projects/:id/events", s.handleListProjectEvents)

	// Workspace activity feed
	api.Get("/activity", s.handleListWorkspaceActivity)
	api.Get("/activity/export", s.handleExportActivity)

	// Ingress sources
	ingressGroup := api.Group("/ingress")
	ingressGroup.Post("/sources", auth.RequireRole(getRoleFn, auth.Editor), s.handleCreateIngressSource)
	ingressGroup.Get("/sources", s.handleListIngressSources)
	ingressGroup.Get("/sources/:id", s.handleGetIngressSource)
	ingressGroup.Put("/sources/:id", auth.RequireRole(getRoleFn, auth.Editor), s.handleUpdateIngressSource)
	ingressGroup.Delete(
		"/sources/:id",
		auth.RequireRole(getRoleFn, auth.Owner),
		s.handleDeleteIngressSource,
	)
	ingressGroup.Post(
		"/sources/:id/test",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleTestIngressSource,
	)
	ingressGroup.Post(
		"/sources/:id/poll",
		demoBlockMiddleware(),
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handlePollIngressSource,
	)
	ingressGroup.Get("/sources/:id/log", s.handleListIngressSourceLog)
	ingressGroup.Get("/log", s.handleListIngressLog)
	ingressGroup.Delete(
		"/log/:entry_id",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleDeleteIngressLogEntry,
	)
	ingressGroup.Post(
		"/log/:entry_id/retry",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleRetryIngressLogEntry,
	)

	// Ingress rules — reorder must be registered before /:rid to avoid conflict
	ingressGroup.Get("/sources/:id/rules", s.handleListIngressRules)
	ingressGroup.Post(
		"/sources/:id/rules",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleCreateIngressRule,
	)
	ingressGroup.Put(
		"/sources/:id/rules/reorder",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleReorderIngressRules,
	)
	ingressGroup.Put(
		"/sources/:id/rules/:rid",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleUpdateIngressRule,
	)
	ingressGroup.Delete(
		"/sources/:id/rules/:rid",
		auth.RequireRole(getRoleFn, auth.Editor),
		s.handleDeleteIngressRule,
	)

	// Export configs
	api.Post("/exports", auth.RequireRole(getRoleFn, auth.Owner), s.handleCreateExportConfig)
	api.Get("/exports", s.handleListExportConfigs)
	api.Post("/exports/validate-destination", s.handleValidateExportDestination)
	api.Post("/exports/:id/trigger", s.handleTriggerExport)
	api.Get("/exports/runs/:runID", s.handleGetExportRun)
	api.Get("/exports/:id/runs/:runID", s.handleGetExportRun)
	api.Get("/exports/:id/runs", s.handleListExportRuns)
	api.Get("/exports/:id", s.handleGetExportConfig)
	api.Put("/exports/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleUpdateExportConfig)
	api.Delete("/exports/:id", auth.RequireRole(getRoleFn, auth.Owner), s.handleDeleteExportConfig)

	// Shares — authenticated, workspace-scoped
	api.Post("/shares", s.handleCreateShare)
	api.Get("/shares", s.handleListShares)
	api.Get("/shares/:id", s.handleGetShare)
	api.Put("/shares/:id", s.handleUpdateShare)
	api.Delete("/shares/:id", s.handleRevokeShare)

	// Share comments — owner moderation (S-7)
	api.Get("/shares/:id/comments", s.handleOwnerListComments)
	api.Delete("/shares/:id/comments/:cid", s.handleOwnerDeleteComment)

	// Public share routes — unauthenticated or share-session-authenticated
	// S-4: access endpoints (no auth required)
	app.Get("/shared/:id/access", s.handleShareInfo)
	app.Post("/shared/:id/access", s.handleShareAccess)

	// S-5 + S-6: content and comment endpoints require a valid share session token
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

	// Public embed token routes — durable, unauthenticated links to an asset's
	// current version. Always serve whatever is_current points to at request time.
	app.Get("/e/:token", s.handlePublicEmbedFile)        // public by design — no auth
	app.Get("/e/:token/thumb", s.handlePublicEmbedThumb) // public by design — no auth

	// Mount the UI with the default configuration under /swagger
	app.Get("/swagger/*", swaggo.HandlerDefault)

	// Serve the SvelteKit SPA
	if uiFS != nil {
		// Production: serve embedded SPA
		app.Use("/", newSPAHandler(uiFS))
	} else {
		// Development: proxy to Vite dev server
		app.Use("/", newViteProxy())
	}

	return app
}

func createDefaultErrorHandler(bodyLimitBytes int) func(c fiber.Ctx, err error) error {
	bodyLimitMb := fmt.Sprintf("%.2f", float32(bodyLimitBytes)/(1<<20)) //nolint:mnd // string formatting

	return func(c fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		e := &fiber.Error{}
		if errors.As(err, &e) {
			code = e.Code
		}
		if code == fiber.StatusRequestEntityTooLarge {
			return c.Status(code).
				JSON(fiber.Map{apiErrorKey: "file too large: maximum upload size is " + bodyLimitMb + " MB"})
		}
		return c.Status(code).JSON(fiber.Map{apiErrorKey: err.Error()})
	}
}
