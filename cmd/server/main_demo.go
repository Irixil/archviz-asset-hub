//go:build demo

package main

import (
	"context"
	"log/slog"
	"os"

	"damask/server/internal/config"
	"damask/server/internal/demo"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
	"database/sql"
)

func initDemoSeeder(ctx context.Context, cfg *config.Config, sqlDB *sql.DB, stor storage.Storage, trf transform.Transformer, tmb transform.Thumbnailer) *demo.Seeder {
	if !cfg.Demo.DemoMode {
		return nil
	}
	seeder := demo.New(sqlDB, stor, cfg.Demo, trf, tmb)
	if err := seeder.EnsureWorkspace(ctx); err != nil {
		slog.Error("demo: ensure workspace", "error", err)
		os.Exit(1)
	}
	if err := seeder.SeedIfEmpty(ctx); err != nil {
		slog.WarnContext(ctx, "demo: initial seed failed (non-fatal)", "error", err)
	}
	seeder.StartResetLoop(ctx)
	slog.InfoContext(ctx, "demo: mode enabled", "reset_interval_hours", cfg.Demo.ResetIntervalHours)
	return seeder
}
