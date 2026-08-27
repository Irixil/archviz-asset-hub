//go:build integration

package api_test

// Tests for HTTP caching headers (Cache-Control, ETag, Last-Modified) and
// conditional request handling (If-None-Match, If-Modified-Since) across all
// binary-serving endpoints.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"damask/server/internal/api"
	th "damask/server/internal/testhelpers"

	"github.com/gofiber/fiber/v3"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// uploadAndSeedThumb uploads a JPEG asset, drains jobs to generate a thumbnail,
// and returns the asset ID and owner cookie.
func uploadAndSeedThumb(t *testing.T, env *th.TestEnv, owner th.AuthResult) string {
	t.Helper()
	asset := th.UploadAsset(t, env, owner.Cookie)
	th.DrainJobs(t, env)
	return asset.ID
}

// assertCacheHeaders checks that all three caching headers are present and
// non-empty on the given response.
func assertCacheHeaders(t *testing.T, resp *http.Response, label string) (etag, lastMod string) {
	t.Helper()
	etag = resp.Header.Get("ETag")
	lastMod = resp.Header.Get("Last-Modified")
	cc := resp.Header.Get("Cache-Control")
	if etag == "" {
		t.Errorf("%s: missing ETag header", label)
	}
	if lastMod == "" {
		t.Errorf("%s: missing Last-Modified header", label)
	}
	if cc == "" {
		t.Errorf("%s: missing Cache-Control header", label)
	}
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("%s: ETag must be double-quoted, got %q", label, etag)
	}
	if !strings.Contains(cc, "max-age=") {
		t.Errorf("%s: Cache-Control must contain max-age, got %q", label, cc)
	}
	return etag, lastMod
}

// do304 sends the same request with If-None-Match set to the given ETag and
// asserts a 304 response with no body.
func do304WithETag(t *testing.T, env *th.TestEnv, req *http.Request, etag string, label string) {
	t.Helper()
	req.Header.Set("If-None-Match", etag)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("%s: 304 request: %v", label, err)
	}
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("%s: expected 304, got %d", label, resp.StatusCode)
	}
}

// do304WithLastMod sends the same request with If-Modified-Since set and
// asserts a 304.
func do304WithLastMod(t *testing.T, env *th.TestEnv, req *http.Request, lastMod string, label string) {
	t.Helper()
	req.Header.Set("If-Modified-Since", lastMod)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("%s: IMS request: %v", label, err)
	}
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("%s: expected 304, got %d", label, resp.StatusCode)
	}
}

// ── asset file ────────────────────────────────────────────────────────────────

func TestCaching_AssetFile_HeadersPresent(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadAndSeedThumb(t, env, owner)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/file", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("file request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	assertCacheHeaders(t, resp, "asset file")
}

func TestCaching_AssetFile_ETag304(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadAndSeedThumb(t, env, owner)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/file", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("file request: %v", err)
	}
	etag := resp.Header.Get("ETag")

	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/file", nil, owner.Cookie)
	do304WithETag(t, env, req2, etag, "asset file ETag")
}

func TestCaching_AssetFile_LastModified304(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadAndSeedThumb(t, env, owner)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/file", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("file request: %v", err)
	}
	lastMod := resp.Header.Get("Last-Modified")

	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/file", nil, owner.Cookie)
	do304WithLastMod(t, env, req2, lastMod, "asset file Last-Modified")
}

// ── asset thumb ───────────────────────────────────────────────────────────────

func TestCaching_AssetThumb_HeadersPresent(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadAndSeedThumb(t, env, owner)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/thumb", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("thumb request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	assertCacheHeaders(t, resp, "asset thumb")
}

func TestCaching_AssetThumb_ETag304(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadAndSeedThumb(t, env, owner)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/thumb", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("thumb request: %v", err)
	}
	etag := resp.Header.Get("ETag")

	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/thumb", nil, owner.Cookie)
	do304WithETag(t, env, req2, etag, "asset thumb ETag")
}

// ── variant file ──────────────────────────────────────────────────────────────

func TestCaching_VariantFile_HeadersPresent(t *testing.T) {
	env2, owner2 := th.SetupWithOwner(t)
	asset2 := th.UploadAsset(t, env2, owner2.Cookie)

	// Insert variant directly into DB (mirrors variants_test.go pattern).
	variantID := insertVariantFile(t, env2, asset2.ID, owner2.WorkspaceID)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset2.ID+"/variants/"+variantID+"/file", nil, owner2.Cookie)
	resp, err := env2.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("variant file request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	etag, lastMod := assertCacheHeaders(t, resp, "variant file")

	cc := resp.Header.Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Errorf("variant file: Cache-Control must contain 'immutable', got %q", cc)
	}

	// 304 on ETag
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset2.ID+"/variants/"+variantID+"/file", nil, owner2.Cookie)
	do304WithETag(t, env2, req2, etag, "variant file ETag")

	// 304 on Last-Modified
	req3 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset2.ID+"/variants/"+variantID+"/file", nil, owner2.Cookie)
	do304WithLastMod(t, env2, req3, lastMod, "variant file Last-Modified")

}

// insertVariantFile seeds a variant row and copies the asset's stored file as
// the variant's storage key so the file endpoint can actually stream it.
func insertVariantFile(t *testing.T, env *th.TestEnv, assetID, workspaceID string) string {
	t.Helper()

	// Look up the current version's storage key to use as the variant's file.
	var storageKey string
	if err := env.Database.QueryRow(
		`SELECT storage_key FROM assets WHERE id = ?`, assetID,
	).Scan(&storageKey); err != nil {
		t.Fatalf("lookup storage key: %v", err)
	}

	// Look up current version ID.
	var versionID string
	if err := env.Database.QueryRow(
		`SELECT id FROM asset_versions WHERE asset_id = ? AND is_current = 1`, assetID,
	).Scan(&versionID); err != nil {
		t.Fatalf("lookup version: %v", err)
	}

	variantID := "test-cache-variant-" + assetID[:8]
	variantKey := storageKey + ".variant.jpg"

	// Copy the file in storage so the variant key is resolvable.
	rc, err := env.Storage.Get(t.Context(), storageKey)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	if err := env.Storage.Put(t.Context(), variantKey, rc); err != nil {
		rc.Close() //nolint:errcheck
		t.Fatalf("put variant copy: %v", err)
	}
	rc.Close() //nolint:errcheck

	_, err = env.Database.Exec(`
		INSERT INTO variants (id, workspace_id, asset_version_id, type, storage_key, size, created_at)
		VALUES (?, ?, ?, 'manual', ?, 1024, datetime('now'))
	`, variantID, workspaceID, versionID, variantKey)
	if err != nil {
		t.Fatalf("insert variant: %v", err)
	}
	return variantID
}

// ── preview transform ─────────────────────────────────────────────────────────

func TestCaching_Preview_HeadersPresent(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadAndSeedThumb(t, env, owner)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/preview?w=100&h=100&format=jpeg", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 10000})
	if err != nil {
		t.Fatalf("preview request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	assertCacheHeaders(t, resp, "preview")
}

func TestCaching_Preview_ETag304_SkipsTransform(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadAndSeedThumb(t, env, owner)

	// First request — fills cache and returns headers.
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/preview?w=100&h=100&format=jpeg", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 10000})
	if err != nil {
		t.Fatalf("preview request: %v", err)
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag on first preview response")
	}

	// Second request with ETag — must be 304.
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/preview?w=100&h=100&format=jpeg", nil, owner.Cookie)
	do304WithETag(t, env, req2, etag, "preview ETag")
}

func TestCaching_Preview_NotPublic(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadAndSeedThumb(t, env, owner)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/preview?w=50&h=50", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 10000})
	if err != nil {
		t.Fatalf("preview request: %v", err)
	}
	cc := resp.Header.Get("Cache-Control")
	if strings.Contains(cc, "public") {
		t.Errorf("preview Cache-Control must not be public (auth-gated), got %q", cc)
	}
}

// ── version file ──────────────────────────────────────────────────────────────

func TestCaching_VersionFile_HeadersAndImmutable(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Get the version ID via the versions list endpoint.
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/versions", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	var versions []api.VersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		t.Fatalf("decode versions: %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("expected at least one version")
	}
	versionID := versions[0].ID

	fileReq := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/versions/"+versionID+"/file", nil, owner.Cookie)
	fileResp, err := env.App.Test(fileReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("version file request: %v", err)
	}
	if fileResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", fileResp.StatusCode)
	}

	etag, lastMod := assertCacheHeaders(t, fileResp, "version file")
	cc := fileResp.Header.Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Errorf("version file: Cache-Control must contain 'immutable', got %q", cc)
	}

	// 304 on ETag
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/versions/"+versionID+"/file", nil, owner.Cookie)
	do304WithETag(t, env, req2, etag, "version file ETag")

	// 304 on Last-Modified
	req3 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/versions/"+versionID+"/file", nil, owner.Cookie)
	do304WithLastMod(t, env, req3, lastMod, "version file Last-Modified")
}

// ── version thumb ─────────────────────────────────────────────────────────────

func TestCaching_VersionThumb_HeadersAndImmutable(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)
	th.DrainJobs(t, env) // generate thumbnails

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/versions", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	var versions []api.VersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		t.Fatalf("decode versions: %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("expected at least one version")
	}
	versionID := versions[0].ID

	thumbReq := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/versions/"+versionID+"/thumb", nil, owner.Cookie)
	thumbResp, err := env.App.Test(thumbReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("version thumb request: %v", err)
	}
	if thumbResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", thumbResp.StatusCode)
	}

	etag, _ := assertCacheHeaders(t, thumbResp, "version thumb")
	cc := thumbResp.Header.Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Errorf("version thumb: Cache-Control must contain 'immutable', got %q", cc)
	}

	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/versions/"+versionID+"/thumb", nil, owner.Cookie)
	do304WithETag(t, env, req2, etag, "version thumb ETag")
}

// ── shared asset file ─────────────────────────────────────────────────────────

func assignAssetToProject(t *testing.T, env *th.TestEnv, cookie *http.Cookie, assetID, projectID string) {
	t.Helper()
	body := `{"asset_ids":["` + assetID + `"],"project_id":"` + projectID + `"}`
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/project", th.JSONStr(body), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("assign project: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("assign project: expected 204, got %d", resp.StatusCode)
	}
}

func TestCaching_ShareAssetFile_HeadersPresent(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	p := createProject(t, env, owner.Cookie, "CacheProj", "#abc")
	assignAssetToProject(t, env, owner.Cookie, asset.ID, p.ID)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "project",
		TargetID:      p.ID,
		AllowDownload: boolPtr(true),
	})
	token := accessShare(t, env, sh.ID, "")

	fileReq := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+asset.ID+"/file", "", token)
	fileResp, err := env.App.Test(fileReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("shared file request: %v", err)
	}
	if fileResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", fileResp.StatusCode)
	}
	assertCacheHeaders(t, fileResp, "shared asset file")
}

func TestCaching_ShareAssetFile_ETag304(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	p := createProject(t, env, owner.Cookie, "CacheProj2", "#abc")
	assignAssetToProject(t, env, owner.Cookie, asset.ID, p.ID)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "project",
		TargetID:      p.ID,
		AllowDownload: boolPtr(true),
	})
	token := accessShare(t, env, sh.ID, "")

	fileReq := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+asset.ID+"/file", "", token)
	fileResp, err := env.App.Test(fileReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("shared file request: %v", err)
	}
	etag := fileResp.Header.Get("ETag")

	req2 := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+asset.ID+"/file", "", token)
	do304WithETag(t, env, req2, etag, "shared asset file ETag")
}

// ── shared asset thumb ────────────────────────────────────────────────────────

func TestCaching_ShareAssetThumb_HeadersPresent(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)
	th.DrainJobs(t, env)

	p := createProject(t, env, owner.Cookie, "CacheThumbProj", "#abc")
	assignAssetToProject(t, env, owner.Cookie, asset.ID, p.ID)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
	})
	token := accessShare(t, env, sh.ID, "")

	thumbReq := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+asset.ID+"/thumb", "", token)
	thumbResp, err := env.App.Test(thumbReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("shared thumb request: %v", err)
	}
	if thumbResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", thumbResp.StatusCode)
	}
	etag, _ := assertCacheHeaders(t, thumbResp, "shared asset thumb")

	req2 := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+asset.ID+"/thumb", "", token)
	do304WithETag(t, env, req2, etag, "shared asset thumb ETag")
}

func boolPtr(b bool) *bool { return &b }
