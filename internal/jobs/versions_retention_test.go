package jobs_test

import (
	"fmt"
	"net/http"
	"testing"

	dbgen "damask/server/internal/db/gen"
	th "damask/server/internal/testhelpers"

	"github.com/gofiber/fiber/v3"
)

// --- Retention policy ---

// TestEnforceRetention_KeepExactN verifies that after enforcement exactly `keep`
// non-current versions survive and the rest are soft-deleted.
func TestEnforceRetention_KeepExactN(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")

	// Set workspace retention to keep=2.
	if _, err := env.Database.Exec(
		`UPDATE workspaces SET version_retention_count = 2 WHERE id = ?`, owner.WorkspaceID,
	); err != nil {
		t.Fatalf("set retention: %v", err)
	}

	asset := th.UploadAsset(t, env, owner.Cookie)

	// Upload 4 more versions (v2–v5), making v5 current.
	jpegData := [5][]byte{}
	for i := range jpegData {
		jpegData[i] = th.MakeJPEG(10+i, 10+i)
	}
	for i := range 4 {
		r := th.BuildVersionUploadRequest(t, asset.ID, fmt.Sprintf("v%d.jpg", i+2), jpegData[i+1], "", owner.Cookie)
		resp, _ := env.App.Test(r, fiber.TestConfig{Timeout: 5000})
		if resp.StatusCode != http.StatusCreated {
			t.Skipf("version upload %d failed", i+2)
		}
	}

	// Confirm we have 5 active versions before enforcement.
	var countBefore int64
	_ = env.Database.QueryRow(
		`SELECT COUNT(*) FROM asset_versions WHERE asset_id = ? AND deleted_at IS NULL`, asset.ID,
	).Scan(&countBefore)
	if countBefore != 5 {
		t.Fatalf("expected 5 active versions before enforcement, got %d", countBefore)
	}

	// Run retention enforcement.
	var wsName string
	var wsRetention int64
	if err := env.Database.QueryRow(
		`SELECT name, version_retention_count FROM workspaces WHERE id = ?`, owner.WorkspaceID,
	).Scan(&wsName, &wsRetention); err != nil {
		t.Fatalf("get workspace: %v", err)
	}
	ws := dbgen.Workspace{ID: owner.WorkspaceID, Name: wsName, VersionRetentionCount: wsRetention}
	if err := env.JobServer.EnforceRetentionForWorkspace(t.Context(), ws); err != nil {
		t.Fatalf("enforce retention: %v", err)
	}

	// Count non-current active versions after enforcement.
	var nonCurrentActive int64
	_ = env.Database.QueryRow(
		`SELECT COUNT(*) FROM asset_versions WHERE asset_id = ? AND deleted_at IS NULL AND is_current = 0`, asset.ID,
	).Scan(&nonCurrentActive)

	if nonCurrentActive != 2 {
		t.Errorf("expected exactly 2 non-current active versions after enforcement (keep=2), got %d", nonCurrentActive)
	}

	// Current version must still be alive.
	var currentActive int64
	_ = env.Database.QueryRow(
		`SELECT COUNT(*) FROM asset_versions WHERE asset_id = ? AND deleted_at IS NULL AND is_current = 1`, asset.ID,
	).Scan(&currentActive)
	if currentActive != 1 {
		t.Errorf("current version was deleted by retention enforcement")
	}
}
