package jobs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"damask/server/internal/jobs"
	th "damask/server/internal/testhelpers"
)

// insertVariantForThumbTest inserts a variant row with fake storage content,
// returning the variant ID and its storage key.
func insertVariantForThumbTest(
	t *testing.T,
	env *th.TestEnv,
	workspaceID, assetID string,
) (variantID, storageKey string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var versionID string
	row := env.Database.QueryRowContext(ctx,
		`SELECT id FROM asset_versions WHERE asset_id = ? AND is_current = 1 LIMIT 1`, assetID)
	if err := row.Scan(&versionID); err != nil {
		t.Fatalf("resolve version: %v", err)
	}

	variantID = "thumb-test-variant-id"
	storageKey = fmt.Sprintf("%s/%s/variants/%s.jpg", workspaceID, assetID, variantID)
	_ = env.Storage.Put(t.Context(), storageKey, bytes.NewReader(th.MakeJPEG(10, 10)))

	_, err := env.Database.ExecContext(ctx, `
		INSERT INTO variants (id, workspace_id, asset_version_id, type, storage_key, transform_params, size)
		VALUES (?, ?, ?, 'image_resize', ?, '{"width":100}', 1024)
	`, variantID, workspaceID, versionID, storageKey)
	if err != nil {
		t.Fatalf("insert variant: %v", err)
	}
	return variantID, storageKey
}

// TestVariantThumbnailJobWritesKey enqueues a generate_variant_thumbnail job
// and verifies that thumbnail_key is set on the variant row after draining.
func TestVariantThumbnailJobWritesKey(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	asset := th.UploadAsset(t, env, owner.Cookie)
	variantID, storageKey := insertVariantForThumbTest(t, env, owner.WorkspaceID, asset.ID)

	payload, _ := json.Marshal(jobs.VariantThumbnailJobPayload{
		VariantID:   variantID,
		WorkspaceID: owner.WorkspaceID,
		AssetID:     asset.ID,
		StorageKey:  storageKey,
		MimeType:    "image/jpeg",
	})

	ctx := context.Background()
	_, err := env.Database.ExecContext(
		ctx,
		`INSERT INTO jobs (id, workspace_id, type, payload, status) VALUES (?, ?, 'generate_variant_thumbnail', ?, 'pending')`,
		"thumb-job-id",
		owner.WorkspaceID,
		string(payload),
	)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	th.DrainJobs(t, env)

	var thumbKey *string
	row := env.Database.QueryRowContext(ctx, `SELECT thumbnail_key FROM variants WHERE id = ?`, variantID)
	if e := row.Scan(&thumbKey); e != nil {
		t.Fatalf("query thumbnail_key: %v", e)
	}
	if thumbKey == nil || *thumbKey == "" {
		t.Error("expected thumbnail_key to be set after variant thumbnail job")
	}
}

// TestVariantThumbnailJobEnqueuedAfterVariantCreation verifies that uploading
// a manual variant enqueues a generate_variant_thumbnail job.
func TestVariantThumbnailJobEnqueuedAfterVariantCreation(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	asset := th.UploadAsset(t, env, owner.Cookie)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "variant.jpg")
	_, _ = fw.Write(th.MakeJPEG(50, 50))
	_ = mw.Close()

	uploadURL := fmt.Sprintf("/api/v1/assets/%s/variants/upload", asset.ID)
	req := httptest.NewRequest(http.MethodPost, uploadURL, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload variant: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload variant: expected 201, got %d", resp.StatusCode)
	}

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE type = 'generate_variant_thumbnail' AND workspace_id = ?`,
		owner.WorkspaceID,
	).Scan(&count); e != nil {
		t.Fatalf("query jobs: %v", e)
	}
	if count == 0 {
		t.Error("expected generate_variant_thumbnail job to be enqueued after manual variant upload")
	}
}

// TestVersionThumbnailJobWritesContentType verifies that the version thumbnail
// job writes thumbnail_content_type after processing.
func TestVersionThumbnailJobWritesContentType(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	asset := th.UploadAsset(t, env, owner.Cookie)
	th.DrainJobs(t, env)

	var ct string
	err := env.Database.QueryRow(
		`SELECT COALESCE(thumbnail_content_type, '') FROM asset_versions WHERE asset_id = ? AND is_current = 1`,
		asset.ID,
	).Scan(&ct)
	if err != nil {
		t.Fatalf("query thumbnail_content_type: %v", err)
	}
	if ct == "" {
		t.Error("expected thumbnail_content_type to be non-empty after version thumbnail job")
	}
}
