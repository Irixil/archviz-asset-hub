package api

import (
	"context"
	"time"
)

// DemoSeeder is the subset of demo.Seeder used by the API layer.
// Defined as an interface so the api package compiles without importing
// the demo package in non-demo builds.
type DemoSeeder interface {
	GetWorkspaceID(ctx context.Context) (string, bool)
	// GetUsage returns (assetCount, storageUsedBytes, error).
	GetUsage(ctx context.Context, workspaceID string) (int64, int64, error)
	LastResetAt() time.Time
	NextResetAt() time.Time
	IsResetting() bool
	GetDemoUser(ctx context.Context) (userID, workspaceID string, err error)
}
