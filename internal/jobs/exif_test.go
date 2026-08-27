package jobs_test

import (
	"context"
	"net/http"
	"testing"

	th "damask/server/internal/testhelpers"
)

// TestExtractExif_EnqueuedOnImageUpload verifies that uploading an image asset
// results in an extract_exif job being created in the jobs table.
func TestExtractExif_EnqueuedOnImageUpload(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	th.UploadAsset(t, env, owner.Cookie)

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE type = 'extract_exif' AND workspace_id = ?`,
		owner.WorkspaceID,
	).Scan(&count); e != nil {
		t.Fatalf("query jobs: %v", e)
	}
	if count == 0 {
		t.Error("expected extract_exif job to be enqueued after image upload")
	}
}

// TestExtractExif_NotEnqueuedForNonImage verifies that uploading a non-image file
// does not enqueue an extract_exif job.
func TestExtractExif_NotEnqueuedForNonImage(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Upload a text file (not an image)
	req := th.BuildUploadRequest(t, "readme.txt", []byte("hello world"), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var count int
	if err = env.Database.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE type = 'extract_exif' AND workspace_id = ?`,
		owner.WorkspaceID,
	).Scan(&count); err != nil {
		t.Fatalf("query jobs: %v", err)
	}
	if count != 0 {
		t.Errorf("expected no extract_exif job for non-image, got %d", count)
	}
}

// TestExtractExif_HandlerDisabledWhenExifKeepOff verifies that the job handler
// is a no-op when exif_keep = 0 (default).
func TestExtractExif_HandlerDisabledWhenExifKeepOff(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	th.UploadAsset(t, env, owner.Cookie)

	// exif_keep defaults to 0 — drain jobs, verify no field values created
	th.DrainJobs(t, env)

	var count int
	if err := env.Database.QueryRow(
		`SELECT COUNT(*) FROM asset_field_values afv
		 JOIN field_definitions fd ON fd.id = afv.field_id
		 WHERE fd.key LIKE '_exif_%' AND fd.workspace_id = ?`,
		owner.WorkspaceID,
	).Scan(&count); err != nil {
		t.Fatalf("query field values: %v", err)
	}
	if count != 0 {
		t.Errorf("expected no _exif_ field values when exif_keep=0, got %d", count)
	}
}

// TestExtractExif_HandlerWritesTombstoneForJPEGWithNoExif verifies that a JPEG
// with no EXIF data gets a tombstone (empty _exif_make value) so it is not re-queued.
func TestExtractExif_HandlerWritesTombstoneForJPEGWithNoExif(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Enable EXIF extraction
	_, err := env.Database.Exec(
		`UPDATE workspaces SET exif_keep = 1 WHERE id = ?`, owner.WorkspaceID,
	)
	if err != nil {
		t.Fatalf("enable exif_keep: %v", err)
	}

	// Upload a synthetic JPEG (no real EXIF tags — just valid JPEG header bytes)
	th.UploadAsset(t, env, owner.Cookie)
	th.DrainJobs(t, env)

	// The _exif_make field definition should now exist
	var fieldCount int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM field_definitions WHERE key = '_exif_make' AND workspace_id = ?`,
		owner.WorkspaceID,
	).Scan(&fieldCount); e != nil {
		t.Fatalf("query field defs: %v", e)
	}
	if fieldCount == 0 {
		t.Fatal("expected _exif_make field definition to be created")
	}

	var source string
	if e := env.Database.QueryRow(
		`SELECT source FROM field_definitions WHERE key = '_exif_make' AND workspace_id = ?`,
		owner.WorkspaceID,
	).Scan(&source); e != nil {
		t.Fatalf("query field source: %v", e)
	}
	if source != "exif" {
		t.Fatalf("source = %q, want exif", source)
	}

	// A tombstone value row should exist for _exif_make
	var valueCount int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM asset_field_values afv
		 JOIN field_definitions fd ON fd.id = afv.field_id
		 WHERE fd.key = '_exif_make' AND fd.workspace_id = ?`,
		owner.WorkspaceID,
	).Scan(&valueCount); e != nil {
		t.Fatalf("query tombstone: %v", e)
	}
	if valueCount == 0 {
		t.Error("expected tombstone row for _exif_make after processing no-EXIF JPEG")
	}
}

// TestExtractExif_EnsureExifFields_Idempotent verifies that calling ensureExifFields
// twice does not create duplicate field definitions.
func TestExtractExif_EnsureExifFields_Idempotent(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	_, err := env.Database.Exec(`UPDATE workspaces SET exif_keep = 1 WHERE id = ?`, owner.WorkspaceID)
	if err != nil {
		t.Fatalf("enable exif: %v", err)
	}

	// Upload and drain twice
	th.UploadAsset(t, env, owner.Cookie)
	th.DrainJobs(t, env)
	th.UploadAsset(t, env, owner.Cookie)
	th.DrainJobs(t, env)

	// Each _exif_ field key must appear exactly once per workspace
	rows, err := env.Database.QueryContext(context.Background(),
		`SELECT key, COUNT(*) AS cnt FROM field_definitions
		 WHERE workspace_id = ? AND key LIKE '_exif_%'
		 GROUP BY key HAVING cnt > 1`,
		owner.WorkspaceID,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var cnt int
		_ = rows.Scan(&key, &cnt)
		t.Errorf("duplicate field definition: key=%s count=%d", key, cnt)
	}
}
