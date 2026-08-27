package app_test

import (
	"context"
	"testing"

	"damask/server/internal/app"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"

	"github.com/google/uuid"
)

// TestBuild_VariantConvergence is the CR-3.4 regression test: before the
// single composition root, cmd/server/main.go built variantSvc without
// Workflows/Invalidate while internal/api/router.go built its own copy with
// them — so a variant produced via a background job (which only had access
// to main.go's instance, e.g. through the workflow executor) silently
// skipped storage-size cache invalidation and workflow-coverage lookups
// that an API-driven variant mutation performed. Now there is exactly one
// service.VariantService instance in *app.Deps, so both call paths are
// provably identical. This test builds the graph once and exercises the
// same deps.Variants instance the way both the API handler and the
// workflow-run job handler would, asserting both behaviors fire.
func TestBuild_VariantConvergence(t *testing.T) {
	a := validBuildArgs(t)
	deps, err := a.build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	userID := "usr_1"
	ctx := auth.WithActor(context.Background(), auth.Actor{Type: "user", UserID: &userID})
	const workspaceID = "ws_conv"
	const projectID = "prj_conv"
	const assetID = "ast_conv"
	const versionID = "ver_conv"

	seedConvergenceFixtures(t, deps, workspaceID, projectID, assetID, versionID)

	// --- Workflows lookup convergence ---
	// A workflow covering this project must be visible through
	// deps.Variants.List regardless of whether the caller is the API's
	// GET /assets/:id/variants handler or a workflow node running inside a
	// background job (both go through the same instance now).
	result, err := deps.Variants.List(ctx, service.ListVariantsParams{
		WorkspaceID:    workspaceID,
		AssetID:        assetID,
		AssetProjectID: projectID,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result.CoveringWorkflow == nil {
		t.Fatal("expected List to report a covering workflow — deps.Variants was built without Workflows wired")
	}

	// --- Storage-invalidation convergence ---
	// Prime the usage cache, then create a variant through the exact same
	// deps.Variants instance. If Invalidate isn't wired, the cache would
	// still report the pre-creation snapshot.
	usageBefore, err := deps.StorageSvc.GetUsage(ctx, workspaceID)
	if err != nil {
		t.Fatalf("GetUsage (prime cache): %v", err)
	}
	_ = usageBefore

	if _, err = deps.Variants.Create(ctx, service.CreateVariantParams{
		ID:             uuid.NewString(),
		WorkspaceID:    workspaceID,
		AssetID:        assetID,
		AssetVersionID: versionID,
		Type:           "thumbnail",
		StorageKey:     "k/variant1",
	}); err != nil {
		t.Fatalf("Create variant: %v", err)
	}

	// A fresh GetUsage call must recompute (not return the primed value) —
	// the only way that happens is if Create called s.invalidate.Invalidate,
	// proving deps.Variants was built with Invalidate wired.
	usageAfter, err := deps.StorageSvc.GetUsage(ctx, workspaceID)
	if err != nil {
		t.Fatalf("GetUsage (after create): %v", err)
	}
	if !usageAfter.ComputedAt.After(usageBefore.ComputedAt) {
		t.Fatal("expected GetUsage to recompute after variant Create — " +
			"deps.Variants was built without Invalidate wired, so background-job " +
			"variant creation would leave the storage-usage cache stale")
	}
}

func seedConvergenceFixtures(
	t *testing.T,
	deps *app.Deps,
	workspaceID, projectID, assetID, versionID string,
) {
	t.Helper()
	ctx := context.Background()
	queries := deps.DB.WQ

	if _, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID: workspaceID, Name: "Convergence WS",
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := queries.CreateProject(ctx, dbgen.CreateProjectParams{
		ID: projectID, WorkspaceID: workspaceID, Name: "Convergence Project",
	}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := queries.CreateAsset(ctx, dbgen.CreateAssetParams{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		ProjectID:        &projectID,
		OriginalFilename: "photo.jpg",
		StorageKey:       "k/" + assetID,
		MimeType:         "image/jpeg",
	}); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	if _, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID: "usr_1", Email: "conv@example.com", PasswordHash: "x", Name: "Convergence User",
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := queries.CreateAssetVersion(ctx, dbgen.CreateAssetVersionParams{
		ID:          versionID,
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		VersionNum:  1,
		StorageKey:  "k/" + assetID + "/v1",
		ContentHash: "hash1",
		MimeType:    "image/jpeg",
		Size:        1024,
		IsCurrent:   1,
	}); err != nil {
		t.Fatalf("seed asset version: %v", err)
	}

	graph := `{"nodes":[{"id":"trigger","type":"trigger.version_uploaded","config":{"project_id":"` +
		projectID + `"}}],"edges":[]}`
	workflowRepo := reposqlc.NewWorkflowRepo(deps.DB)
	if _, err := workflowRepo.Create(ctx, repository.CreateWorkflowParams{
		ID:            "wf_conv",
		WorkspaceID:   workspaceID,
		Name:          "Convergence workflow",
		Enabled:       true,
		TriggerType:   "trigger.version_uploaded",
		TriggerConfig: `{"project_id":"` + projectID + `"}`,
		Graph:         graph,
		CreatedBy:     "usr_1",
	}); err != nil {
		t.Fatalf("seed workflow: %v", err)
	}
}
