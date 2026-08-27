// Package main is the server entrypoint.
package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/app"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/ingress"
	"damask/server/internal/jobs"
	"damask/server/internal/mail"
	"damask/server/internal/mailserver"
	"damask/server/internal/queue"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	// Side-effect imports to register ingress source types.
	"damask/server/internal/ingress/sources/canva"
	_ "damask/server/internal/ingress/sources/dav"
	_ "damask/server/internal/ingress/sources/email_api"
	"damask/server/internal/ingress/sources/gdrive"
	_ "damask/server/internal/ingress/sources/imap"
	_ "damask/server/internal/ingress/sources/s3"
	_ "damask/server/internal/ingress/sources/sftp"

	// Side-effect imports to register export destination types.
	exportgdrive "damask/server/internal/export/destinations/gdrive"
	_ "damask/server/internal/export/destinations/sftp"
	"damask/server/internal/oauth"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var uiFS fs.FS // Populated by ui.go (prod) or ui_dev.go (dev)

//nolint:funlen,cyclop // entrypoint, setup, and shutdown logic
func main() {
	logLevel := new(slog.LevelVar)
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG":
		logLevel.Set(slog.LevelDebug)
	case "WARN":
		logLevel.Set(slog.LevelWarn)
	case "ERROR":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}
	slog.Info("log", "level", logLevel.Level())

	slog.Info("database", "path", cfg.DBPath)
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	tokenMaker, err := auth.NewMaker(cfg.JWTSecret)
	if err != nil {
		slog.Error("auth", "error", err)
		os.Exit(1) //nolint: gocritic // Defered database.Close() is not needed on exit.
	}

	eventsHub := events.NewEventHub()

	mailer := mail.NewMailer(&cfg.MailSenderConfig)

	stor, err := initStorageFromConfig(cfg)
	if err != nil {
		slog.Error("storage", "error", err)
		os.Exit(1)
	}

	q := queue.New(database.WQ, cfg.QueueWorkers)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tmConfig := telemetry.Config{
		Enabled:     cfg.Telemetry.Enabled,
		Endpoint:    cfg.Telemetry.Endpoint,
		Token:       cfg.Telemetry.Token,
		ServiceName: cfg.Telemetry.ServiceName,
		Env:         cfg.Telemetry.Env,
	}
	telemetry.SetupMetrics(tmConfig)

	otelTracesShutdown, err := telemetry.SetupTraces(ctx, tmConfig)
	if err != nil {
		slog.WarnContext(ctx, "otel setup failed; continuing without sending traces", "error", err)
	}

	otelLogsShutdown, err := telemetry.SetupLogs(ctx, tmConfig)
	if err != nil {
		slog.WarnContext(ctx, "otel setup failed; continuing without sending logs", "error", err)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if e := otelTracesShutdown(shutdownCtx); e != nil {
			slog.WarnContext(shutdownCtx, "otel shutdown failed", "error", e)
		}
		if e := otelLogsShutdown(shutdownCtx); e != nil {
			slog.WarnContext(shutdownCtx, "otel shutdown failed", "error", e)
		}
	}()

	q.Start(ctx)
	defer q.Stop()

	// --- transform ---
	trf := transform.NewTransformer(cfg.FFmpeg)
	tmb := transform.NewThumbnailer(trf)

	// --- single composition root: builds the entire dependency graph once.
	// Both the HTTP server and the job server consume the same *app.Deps, so
	// a variant produced via a background job goes through identical
	// Invalidate/Workflows wiring as one produced via the API.
	deps, err := app.Build(cfg, database, stor, eventsHub, q, mailer, trf, tmb)
	if err != nil {
		slog.Error("app", "error", err)
		os.Exit(1)
	}

	// --- job server ---
	js := jobs.NewJobServer(jobs.Deps{
		Queries:           database.WQ,
		SQLDB:             database.Writer,
		Storage:           deps.Storage,
		Hub:               deps.Hub,
		Queue:             deps.Queue,
		Mailer:            deps.Mailer,
		Transformer:       deps.Transformer,
		Thumbnailer:       deps.Thumbnailer,
		Config:            deps.Config,
		Ingester:          deps.Ingester,
		AIAPIKeyResolver:  deps.KeyResolver,
		AIProviderFactory: deps.AIProviderFactory,
		WorkspaceRepo:     reposqlc.NewWorkspaceRepo(database),
		WorkflowExec:      deps.WorkflowExec,
		ExportSvc:         deps.Exports,
		ExifSvc:           deps.Exif,
		TagSvc:            deps.Tags,
		FieldSvc:          deps.Fields,
		TextTrackSvc:      deps.TextTracks,
		StorageSvc:        deps.StorageSvc,
	})
	js.RegisterJobHandlers()

	// --- http ---
	// Demo mode: ensure workspace exists on startup, seed if missing, start reset loop.
	// initDemoSeeder is a no-op stub in non-demo builds (main_nodemo.go).
	demoSeeder := initDemoSeeder(ctx, cfg, database.Writer, stor, trf, tmb)

	app := api.NewRouter(deps, tokenMaker, demoSeeder, uiFS)

	mailErr := initMailServer(cfg, database.WQ, q)

	config.InitOIDCProviders(cfg)

	// Wire TokenRefresher into OAuth-backed ingress sources.
	refresher := oauth.NewTokenRefresher(database.WQ, cfg.AppSecret)
	if cfg.Google.ClientID != "" {
		slog.Info("register Google tokens refresher")
		refresher.RegisterProvider("google", &oauth2.Config{
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  cfg.BaseURL.String() + "/integrations/callback/google",
			Scopes:       []string{"openid", "email", "profile", "https://www.googleapis.com/auth/drive.file"},
		})
	}
	if cfg.Canva.ClientID != "" {
		slog.Info("register Canva tokens refresher")
		refresher.RegisterProvider("canva", &oauth2.Config{
			ClientID:     cfg.Canva.ClientID,
			ClientSecret: cfg.Canva.ClientSecret,
			Endpoint: oauth2.Endpoint{ //nolint:gosec // canva api endpoints
				AuthURL:  "https://www.canva.com/api/oauth/authorize",
				TokenURL: "https://api.canva.com/rest/v1/oauth/token",
			},
			RedirectURL: cfg.BaseURL.String() + "/integrations/callback/canva",
			Scopes:      []string{"profile:read", "design:content:read"},
		})
	}
	gdrive.SetRefresher(refresher)
	canva.SetRefresher(refresher)
	exportgdrive.SetRefresher(refresher)

	/// start background services

	if cfg.EnableScheduler {
		initScheduler(ctx, cfg, q, database.WQ, js)
	}

	slog.Info("api server starting", "port", cfg.Port, "env", cfg.AppEnv, "workers", cfg.QueueWorkers)
	appErr := make(chan error, 1)
	go func() {
		if e := app.Listen(":"+cfg.Port, fiber.ListenConfig{
			DisableStartupMessage: true,
		}); e != nil {
			appErr <- e
		}
	}()

	select {
	case e := <-mailErr:
		slog.Error("mail server", "error", e)
		os.Exit(1)
	case e := <-appErr:
		slog.Error("api server", "error", e)
		os.Exit(1)
	case <-ctx.Done():
	}
}

func initMailServer(cfg *config.Config, queries *dbgen.Queries, q *queue.Queue) chan error {
	mail := mailserver.NewMailServer("0.0.0.0:"+cfg.MailServerPort, cfg.BaseURL.Host, queries, q)
	slog.Info("mail server starting", "port", cfg.MailServerPort)
	mailErr := make(chan error, 1)
	go func() {
		if err := mail.Start(); err != nil {
			mailErr <- err
		}
	}()
	return mailErr
}

func initScheduler(
	ctx context.Context,
	cfg *config.Config,
	q *queue.Queue,
	queries *dbgen.Queries,
	js *jobs.JobServer,
) {
	ingress.NewScheduler(queries, q).Start(ctx)
	slog.InfoContext(ctx, "ingress scheduler started")
	jobs.NewFieldCleanupScheduler(queries, q).Start(ctx)
	slog.InfoContext(ctx, "field cleanup scheduler started")
	jobs.NewRetentionScheduler(q).Start(ctx)
	slog.InfoContext(ctx, "retention scheduler started")
	jobs.NewAuditLogRetentionScheduler(q).Start(ctx)
	slog.InfoContext(ctx, "audit-log retention scheduler started")
	jobs.NewScratchPurgeScheduler(q, cfg).Start(ctx)
	slog.InfoContext(ctx, "scratch purge scheduler started")
	jobs.NewExportScheduler(q, js).Start(ctx)
	slog.InfoContext(ctx, "export scheduler started")
}

func initStorageFromConfig(cfg *config.Config) (storage.Storage, error) {
	var err error
	var stor storage.Storage
	switch cfg.StorageType {
	case "memory":
		slog.Info("storage", "using", cfg.StorageType)
		stor, err = storage.NewAferoMemoryStorage()
	case "s3":
		slog.Info("storage", "using", cfg.StorageType, "bucket", cfg.StorageS3.Bucket)
		stor, err = storage.NewAferoS3Storage(cfg.StorageS3)
	case "sftp":
		slog.Info("storage", "using", cfg.StorageType, "host", cfg.StorageSFTP.Host)
		stor, err = storage.NewSFTPStorage(cfg.StorageSFTP)
	default: // "local" and anything unrecognized
		slog.Info("storage", "using", cfg.StorageType, "host", cfg.StoragePath)
		stor, err = storage.NewLocalStorage(cfg.StoragePath)
	}
	return stor, err
}
