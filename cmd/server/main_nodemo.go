//go:build !demo

package main

import (
	"context"

	"database/sql"

	"damask/server/internal/api"
	"damask/server/internal/config"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
)

func initDemoSeeder(
	_ context.Context,
	_ *config.Config,
	_ *sql.DB,
	_ storage.Storage,
	_ transform.Transformer,
	_ transform.Thumbnailer,
) api.DemoSeeder {
	return nil
}
