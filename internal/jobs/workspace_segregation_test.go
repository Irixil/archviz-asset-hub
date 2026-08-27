package jobs_test

import (
	"context"
	"encoding/json"
	"testing"

	"damask/server/internal/jobs"
	th "damask/server/internal/testhelpers"
)

// TestJobWorkspaceSegregation_CrossWorkspacePayloadRejected enqueues a job in
// workspace A whose payload references a variant that only exists in
// workspace B. The handler must treat it as not-found and never write to B's
// rows.
func TestJobWorkspaceSegregation_CrossWorkspacePayloadRejected(t *testing.T) {
	env := th.SetupTestApp(t)
	alice := th.Register(t, env, "Alice", "alice@example.com", "password123")
	bob := th.Register(t, env, "Bob", "bob@example.com", "password123")

	bobAsset := th.UploadAsset(t, env, bob.Cookie)
	variantID, storageKey := insertVariantForThumbTest(t, env, bob.WorkspaceID, bobAsset.ID)

	ctx := context.Background()
	// Snapshot B's variant state before the hostile job runs.
	var before *string
	if err := env.Database.QueryRowContext(ctx,
		`SELECT thumbnail_key FROM variants WHERE id = ?`, variantID).Scan(&before); err != nil {
		t.Fatalf("query thumbnail_key: %v", err)
	}

	payload, _ := json.Marshal(jobs.VariantThumbnailJobPayload{
		VariantID:   variantID,
		WorkspaceID: alice.WorkspaceID,
		AssetID:     bobAsset.ID,
		StorageKey:  storageKey,
		MimeType:    "image/jpeg",
	})
	if _, err := env.Database.ExecContext(ctx,
		`INSERT INTO jobs (id, workspace_id, type, payload, status)
		 VALUES ('cross-ws-job', ?, 'generate_variant_thumbnail', ?, 'pending')`,
		alice.WorkspaceID, string(payload),
	); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	th.DrainJobs(t, env)

	var after *string
	if err := env.Database.QueryRowContext(ctx,
		`SELECT thumbnail_key FROM variants WHERE id = ?`, variantID).Scan(&after); err != nil {
		t.Fatalf("query thumbnail_key after: %v", err)
	}
	if (before == nil) != (after == nil) || (before != nil && *before != *after) {
		t.Fatalf("workspace B's variant was modified by a workspace A job: before=%v after=%v", before, after)
	}
}
