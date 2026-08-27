//go:build integration

package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	th "damask/server/internal/testhelpers"

	"github.com/gofiber/fiber/v3"
)

// --- AV-1.4: List versions ---

func TestListVersions_InitiallyOneVersion(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")

	asset := th.UploadAsset(t, env, owner.Cookie)

	req := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var versions []api.VersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version after upload, got %d", len(versions))
	}
	if !versions[0].IsCurrent {
		t.Error("v1 should be current")
	}
	if versions[0].VersionNum != 1 {
		t.Errorf("expected version_num=1, got %d", versions[0].VersionNum)
	}
}

func TestListVersions_AssetNotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/nonexistent/versions", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListVersions_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	req := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// --- AV-1.3: Upload new version ---

func TestUploadVersion_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")

	asset := th.UploadAsset(t, env, owner.Cookie)
	time.Sleep(5 * time.Millisecond)

	req := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", th.MakeJPEG(200, 200), "second version", owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload version: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Version api.VersionResponse `json:"version"`
		Asset   api.AssetResponse   `json:"asset"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Version.VersionNum != 2 {
		t.Errorf("expected version_num=2, got %d", result.Version.VersionNum)
	}
	if !result.Version.IsCurrent {
		t.Error("new version should be current")
	}
	if result.Version.Comment == nil || *result.Version.Comment != "second version" {
		t.Errorf("expected comment 'second version', got %v", result.Version.Comment)
	}
	if result.Asset.ID != asset.ID {
		t.Errorf("asset ID mismatch: got %q", result.Asset.ID)
	}

	// List should now show 2 versions.
	listReq := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.App.Test(listReq)
	var versions []api.VersionResponse
	_ = json.NewDecoder(listResp.Body).Decode(&versions)
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
	// Most recent first
	if versions[0].VersionNum != 2 {
		t.Errorf("expected v2 first, got v%d", versions[0].VersionNum)
	}
}

func TestUploadVersion_DuplicateCurrentRejected(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Re-upload the exact same bytes as the first version — the data migration
	// uses a placeholder hash, so we need to upload a real version first.
	jpegData := th.MakeJPEG(50, 50)

	// First real upload as v2.
	r1 := th.BuildVersionUploadRequest(t, asset.ID, "dup.jpg", jpegData, "", owner.Cookie)
	resp1, _ := env.App.Test(r1, fiber.TestConfig{Timeout: 5000})
	if resp1.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp1.Body)
		t.Skipf("v2 upload failed: %d %s", resp1.StatusCode, b)
	}

	// Upload the exact same bytes again — should 409.
	r2 := th.BuildVersionUploadRequest(t, asset.ID, "dup.jpg", jpegData, "", owner.Cookie)
	resp2, _ := env.App.Test(r2, fiber.TestConfig{Timeout: 5000})
	if resp2.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp2.Body)
		t.Errorf("expected 409 for duplicate current version, got %d: %s", resp2.StatusCode, b)
	}
}

func TestUploadVersion_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	req := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", th.MakeJPEG(10, 10), "", nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestUploadVersion_ViewerForbidden(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)
	viewerToken := th.MintEditorToken(t, env, owner.WorkspaceID, auth.Viewer)

	req := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", th.MakeJPEG(10, 10), "", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestUploadVersion_AssetNotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")

	req := th.BuildVersionUploadRequest(t, "nonexistent", "v2.jpg", th.MakeJPEG(10, 10), "", owner.Cookie)
	resp, _ := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUploadVersion_CommentTooLong(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	longComment := string(make([]byte, 501))
	req := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", th.MakeJPEG(10, 10), longComment, owner.Cookie)
	resp, _ := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for long comment, got %d", resp.StatusCode)
	}
}

// --- AV-1.5: Restore ---

func TestRestoreVersion_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Upload v2.
	r2 := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", th.MakeJPEG(200, 200), "", owner.Cookie)
	resp2, _ := env.App.Test(r2, fiber.TestConfig{Timeout: 5000})
	if resp2.StatusCode != http.StatusCreated {
		t.Skip("v2 upload failed")
	}

	// Get v1's ID from the list.
	listReq := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.App.Test(listReq)
	var versions []api.VersionResponse
	_ = json.NewDecoder(listResp.Body).Decode(&versions)

	var v1ID string
	for _, v := range versions {
		if v.VersionNum == 1 {
			v1ID = v.ID
		}
	}
	if v1ID == "" {
		t.Fatal("could not find v1 ID")
	}

	// Restore v1.
	restoreURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s/restore", asset.ID, v1ID)
	restoreReq := th.AuthRequest(http.MethodPost, restoreURL, nil, owner.Cookie)
	resp, err := env.App.Test(restoreReq)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Version api.VersionResponse `json:"version"`
		Asset   api.AssetResponse   `json:"asset"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if !result.Version.IsCurrent {
		t.Error("restored version should be current")
	}
	if result.Version.VersionNum != 1 {
		t.Errorf("expected version_num=1, got %d", result.Version.VersionNum)
	}
}

func TestRestoreVersion_AlreadyCurrent(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Get the single version ID.
	listReq := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.App.Test(listReq)
	var versions []api.VersionResponse
	_ = json.NewDecoder(listResp.Body).Decode(&versions)
	if len(versions) == 0 {
		t.Fatal("no versions")
	}

	restoreURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s/restore", asset.ID, versions[0].ID)
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPost, restoreURL, nil, owner.Cookie))
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 for already-current, got %d", resp.StatusCode)
	}
}

func TestRestoreVersion_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	restoreURL := fmt.Sprintf("/api/v1/assets/%s/versions/nonexistent/restore", asset.ID)
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPost, restoreURL, nil, owner.Cookie))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- AV-1.6: Delete version ---

func TestDeleteVersion_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Upload v2 so we can delete v1 later.
	r2 := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", th.MakeJPEG(200, 200), "", owner.Cookie)
	resp2, _ := env.App.Test(r2, fiber.TestConfig{Timeout: 5000})
	if resp2.StatusCode != http.StatusCreated {
		t.Skip("v2 upload failed")
	}

	// Restore v1 to make v2 non-current so we can delete it.
	listReq := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.App.Test(listReq)
	var versions []api.VersionResponse
	_ = json.NewDecoder(listResp.Body).Decode(&versions)

	// Find v2 (non-current after we restore v1).
	var v2ID, v1ID string
	for _, v := range versions {
		if v.VersionNum == 2 {
			v2ID = v.ID
		}
		if v.VersionNum == 1 {
			v1ID = v.ID
		}
	}

	// First restore v1 to make v2 non-current.
	restoreURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s/restore", asset.ID, v1ID)
	env.App.Test(th.AuthRequest(http.MethodPost, restoreURL, nil, owner.Cookie)) //nolint:errcheck

	// Now delete v2 (non-current).
	deleteURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s", asset.ID, v2ID)
	deleteResp, err := env.App.Test(th.AuthRequest(http.MethodDelete, deleteURL, nil, owner.Cookie))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleteResp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(deleteResp.Body)
		t.Fatalf("expected 204, got %d: %s", deleteResp.StatusCode, b)
	}

	// Deleted version should not appear in active list.
	listResp2, _ := env.App.Test(th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie))
	var versions2 []api.VersionResponse
	_ = json.NewDecoder(listResp2.Body).Decode(&versions2)
	for _, v := range versions2 {
		if v.ID == v2ID {
			t.Error("deleted version still appears in active list")
		}
	}
}

func TestDeleteVersion_CannotDeleteCurrent(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	listReq := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.App.Test(listReq)
	var versions []api.VersionResponse
	_ = json.NewDecoder(listResp.Body).Decode(&versions)
	if len(versions) == 0 {
		t.Fatal("no versions")
	}

	deleteURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s", asset.ID, versions[0].ID)
	resp, _ := env.App.Test(th.AuthRequest(http.MethodDelete, deleteURL, nil, owner.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for deleting current version, got %d", resp.StatusCode)
	}
}

func TestDeleteVersion_ViewerForbidden(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)
	viewerToken := th.MintEditorToken(t, env, owner.WorkspaceID, auth.Viewer)

	listReq := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.App.Test(listReq)
	var versions []api.VersionResponse
	_ = json.NewDecoder(listResp.Body).Decode(&versions)

	deleteURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s", asset.ID, versions[0].ID)
	req := httptest.NewRequest(http.MethodDelete, deleteURL, nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

// --- AV-1.7: File + thumb endpoints ---

func TestGetVersionFile_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	listReq := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.App.Test(listReq)
	var versions []api.VersionResponse
	_ = json.NewDecoder(listResp.Body).Decode(&versions)
	if len(versions) == 0 {
		t.Fatal("no versions")
	}

	fileURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s/file", asset.ID, versions[0].ID)
	resp, err := env.App.Test(th.AuthRequest(http.MethodGet, fileURL, nil, owner.Cookie))
	if err != nil {
		t.Fatalf("get version file: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected non-empty file content")
	}
}

func TestGetVersionFile_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	fileURL := fmt.Sprintf("/api/v1/assets/%s/versions/nonexistent/file", asset.ID)
	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, fileURL, nil, owner.Cookie))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetVersionThumb_NotReady(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	listReq := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.App.Test(listReq)
	var versions []api.VersionResponse
	_ = json.NewDecoder(listResp.Body).Decode(&versions)

	thumbURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s/thumb", asset.ID, versions[0].ID)
	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, thumbURL, nil, owner.Cookie))
	// Thumbnail won't be ready immediately (job is async) — expect 404.
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 (thumb not ready), got %d", resp.StatusCode)
	}
}

// --- VV: Variant count in version list ---

func TestListVersions_VariantCountZeroInitially(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	req := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var versions []api.VersionResponse
	_ = json.NewDecoder(resp.Body).Decode(&versions)

	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0].VariantCount != 0 {
		t.Errorf("expected variant_count=0, got %d", versions[0].VariantCount)
	}
}

func TestListVersions_VariantCountReflectsInsertedVariant(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Insert a variant directly on the current version.
	insertVariantDirectly(t, env, asset.ID, asset.WorkspaceID)

	req := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var versions []api.VersionResponse
	_ = json.NewDecoder(resp.Body).Decode(&versions)

	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0].VariantCount != 1 {
		t.Errorf("expected variant_count=1, got %d", versions[0].VariantCount)
	}
}

func TestListVersions_VariantCountPerVersion(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Insert a variant on v1.
	insertVariantDirectly(t, env, asset.ID, asset.WorkspaceID)

	// Upload v2 (new current version — v1 becomes non-current).
	r2 := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", th.MakeJPEG(200, 200), "", owner.Cookie)
	resp2, _ := env.App.Test(r2, fiber.TestConfig{Timeout: 5000})
	if resp2.StatusCode != http.StatusCreated {
		t.Skip("v2 upload failed")
	}

	req := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var versions []api.VersionResponse
	_ = json.NewDecoder(resp.Body).Decode(&versions)

	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}

	// versions[0] = v2 (newest first), versions[1] = v1
	var v1, v2 api.VersionResponse
	for _, v := range versions {
		if v.VersionNum == 1 {
			v1 = v
		} else {
			v2 = v
		}
	}
	if v1.VariantCount != 1 {
		t.Errorf("v1 expected variant_count=1, got %d", v1.VariantCount)
	}
	if v2.VariantCount != 0 {
		t.Errorf("v2 expected variant_count=0, got %d", v2.VariantCount)
	}
}

// --- Workspace isolation ---

func TestVersions_WorkspaceIsolation(t *testing.T) {
	env := th.SetupTestApp(t)
	alice := th.Register(t, env, "Alice", "alice@example.com", "password123")
	bob := th.Register(t, env, "Bob", "bob@example.com", "password123")

	aliceAsset := th.UploadAsset(t, env, alice.Cookie)

	// Bob cannot list Alice's asset's versions.
	listURL := fmt.Sprintf("/api/v1/assets/%s/versions", aliceAsset.ID)
	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, listURL, nil, bob.Cookie))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	// Bob cannot upload a version to Alice's asset.
	uploadReq := th.BuildVersionUploadRequest(t, aliceAsset.ID, "v2.jpg", th.MakeJPEG(10, 10), "", bob.Cookie)
	uploadResp, _ := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	if uploadResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", uploadResp.StatusCode)
	}
}

// TestListVersions_InitialVersionHasThumbnailURL verifies that after the thumbnail
// job runs, the initial v1 version returned by GET /versions has a non-nil thumbnail_url.
// This is a regression test for the bug where CreateAsset enqueued asset_thumbnail
// (which only updated assets.thumbnail_key) instead of version_thumbnail (which also
// updates asset_versions.thumbnail_key).
func TestListVersions_InitialVersionHasThumbnailURL(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")

	asset := th.UploadAsset(t, env, owner.Cookie)

	// Run the enqueued version_thumbnail job synchronously.
	th.DrainJobs(t, env)

	req := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var versions []api.VersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0].ThumbnailURL == nil {
		t.Error("expected thumbnail_url to be set on v1 after thumbnail job ran, got nil")
	}
}
