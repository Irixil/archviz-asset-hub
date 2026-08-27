package service_test

import (
	"context"
	"testing"

	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"

	"github.com/google/uuid"
)

// sqliteTestEnv holds a real in-memory SQLite DB + seeded workspace/user, for the
// handful of service tests (audit_log, integrations) that still need a real
// *dbgen.Queries / *sql.DB rather than a memory repo.
type sqliteTestEnv struct {
	queries     *dbgen.Queries
	database    *dbpkg.DB
	workspaceID string
	userID      string
}

// newSQLiteEnv opens a fresh in-memory SQLite DB and seeds a workspace and user.
func newSQLiteEnv(t *testing.T) *sqliteTestEnv {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	queries := database.WQ

	ctx := context.Background()
	wsID := uuid.NewString()
	userID := uuid.NewString()

	if _, wsErr := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID: wsID, Name: "test-workspace",
	}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	if _, usrErr := queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID: userID, Email: userID + "@test.com", PasswordHash: "x", Name: "test",
	}); usrErr != nil {
		t.Fatalf("seed user: %v", usrErr)
	}

	return &sqliteTestEnv{queries: queries, database: database, workspaceID: wsID, userID: userID}
}
