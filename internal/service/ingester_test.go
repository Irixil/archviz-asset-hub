package service

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	dbpkg "damask/server/internal/db"
	"damask/server/internal/media/ingest"
	"damask/server/internal/repository"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

func newTestIngesterImpl(t *testing.T) (*ingesterImpl, *sql.DB) {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	impl := &ingesterImpl{
		assets:   reposqlc.NewAssetRepo(database),
		versions: reposqlc.NewVersionRepo(database),
		media:    ingest.NewRegistry(transform.NewTransformer()),
	}
	return impl, database.Writer
}

// TestCreateInitialVersionWithNoUser verifies that when createInitialVersion
// is called without a userID, the created_by field is NULL in the database
// (representing a system action, e.g., ingress-created asset).
func TestCreateInitialVersionWithNoUser(t *testing.T) {
	impl, sqlDB := newTestIngesterImpl(t)

	ctx := context.Background()

	workspaceID := uuid.NewString()
	_, err := sqlDB.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, workspaceID, "Test Workspace")
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	assetID := uuid.NewString()
	asset := repository.Asset{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		Size:             100,
		MimeType:         "text/plain",
		OriginalFilename: "test.txt",
	}
	_, err = sqlDB.Exec(
		`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size) VALUES (?, ?, ?, ?, ?, ?)`,
		asset.ID,
		asset.WorkspaceID,
		asset.OriginalFilename,
		"test-key",
		asset.MimeType,
		asset.Size,
	)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if writeErr := os.WriteFile(tmpFile, []byte("hello world"), 0644); writeErr != nil {
		t.Fatalf("write temp file: %v", writeErr)
	}

	versionID, _, err := impl.createInitialVersion(
		ctx,
		asset,
		tmpFile,
		"test-storage-key",
		"text/plain",
		ingest.FileMeta{},
		"",
	)
	if err != nil {
		t.Fatalf("createInitialVersion: %v", err)
	}

	var createdBy *string
	if scanErr := sqlDB.QueryRow(`SELECT created_by FROM asset_versions WHERE id = ?`, versionID).
		Scan(&createdBy); scanErr != nil {
		t.Fatalf("query created_by: %v", scanErr)
	}
	if createdBy != nil {
		t.Errorf("expected created_by to be NULL, got: %v", *createdBy)
	}
}

// TestCreateInitialVersionWithUser verifies that when createInitialVersion
// is called with a userID, it correctly stores that user's ID as created_by.
func TestCreateInitialVersionWithUser(t *testing.T) {
	impl, sqlDB := newTestIngesterImpl(t)

	ctx := context.Background()

	workspaceID := uuid.NewString()
	_, err := sqlDB.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, workspaceID, "Test Workspace")
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	userID := uuid.NewString()
	_, err = sqlDB.Exec(
		`INSERT INTO users (id, email, password_hash, name) VALUES (?, ?, ?, ?)`,
		userID, "user@example.com", "hash", "Test User",
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	assetID := uuid.NewString()
	asset := repository.Asset{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		Size:             100,
		MimeType:         "text/plain",
		OriginalFilename: "test.txt",
	}
	_, err = sqlDB.Exec(
		`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size) VALUES (?, ?, ?, ?, ?, ?)`,
		asset.ID,
		asset.WorkspaceID,
		asset.OriginalFilename,
		"test-key",
		asset.MimeType,
		asset.Size,
	)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if writeErr := os.WriteFile(tmpFile, []byte("hello world"), 0644); writeErr != nil {
		t.Fatalf("write temp file: %v", writeErr)
	}

	versionID, _, err := impl.createInitialVersion(
		ctx,
		asset,
		tmpFile,
		"test-storage-key",
		"text/plain",
		ingest.FileMeta{},
		userID,
	)
	if err != nil {
		t.Fatalf("createInitialVersion: %v", err)
	}

	var createdBy *string
	if scanErr := sqlDB.QueryRow(`SELECT created_by FROM asset_versions WHERE id = ?`, versionID).
		Scan(&createdBy); scanErr != nil {
		t.Fatalf("query created_by: %v", scanErr)
	}
	if createdBy == nil {
		t.Error("expected created_by to not be NULL")
	} else if *createdBy != userID {
		t.Errorf("expected created_by to be %q, got %q", userID, *createdBy)
	}
}
