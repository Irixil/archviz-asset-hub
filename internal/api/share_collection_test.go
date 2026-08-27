//go:build integration

package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"damask/server/internal/api"
	th "damask/server/internal/testhelpers"

	"github.com/gofiber/fiber/v3"
)

// createShare is a test helper for tests still using testhelpers (th).
// Remove this once share_collection_test.go and shares_public_test.go are migrated to testutil.
func createShare(t *testing.T, env *th.TestEnv, cookie *http.Cookie, req api.CreateShareRequest) api.ShareResponse {
	t.Helper()
	httpReq := th.AuthRequest(http.MethodPost, "/api/v1/shares", th.JSONBody(req), cookie)
	resp, err := env.App.Test(httpReq)
	if err != nil {
		t.Fatalf("create share request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var s api.ShareResponse
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		t.Fatalf("decode share: %v", err)
	}
	return s
}

// ── WS-5.1: POST /api/v1/shares with target_type = "collection" ──────────────

func Test_ShareCollection_Creates(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Create a collection first.
	col := createCollection(t, env, owner.Cookie, "Q3 Selects", []string{})

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "collection",
		TargetID:   col.ID,
		Label:      "Q3 Selects Share",
	})

	if sh.TargetType != "collection" {
		t.Errorf("expected target_type collection, got %q", sh.TargetType)
	}
	if sh.TargetID != col.ID {
		t.Errorf("expected target_id %q, got %q", col.ID, sh.TargetID)
	}
	if sh.PublicURL == "" {
		t.Error("expected non-empty public_url")
	}
}

func Test_ShareCollection_InvalidCollectionID(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodPost, "/api/v1/shares",
		th.JSONBody(api.CreateShareRequest{
			TargetType: "collection",
			TargetID:   "nonexistent-collection-id",
			Label:      "bad",
		}), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func Test_ShareCollection_WrongWorkspace(t *testing.T) {
	env1, owner1 := th.SetupWithOwner(t)
	env2, owner2 := th.SetupWithOwner(t)

	// Collection created in workspace 2.
	col := createCollection(t, env2, owner2.Cookie, "Other WS Collection", []string{})

	// Try to share it from workspace 1 — collection not found in ws1.
	req := th.AuthRequest(http.MethodPost, "/api/v1/shares",
		th.JSONBody(api.CreateShareRequest{
			TargetType: "collection",
			TargetID:   col.ID,
			Label:      "cross-workspace",
		}), owner1.Cookie)
	resp, err := env1.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// ── WS-5.2: GET /shared/:id/assets returns collection assets ─────────────────

func Test_PublicGallery_ListsCollectionAssets(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	asset1 := th.UploadAsset(t, env, owner.Cookie)
	asset2 := th.UploadAsset(t, env, owner.Cookie)

	col := createCollection(t, env, owner.Cookie, "Gallery", []string{asset1.ID, asset2.ID})

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "collection",
		TargetID:   col.ID,
		Label:      "Gallery Share",
	})

	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets", "", token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Assets []map[string]any `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Assets) != 2 {
		t.Errorf("expected 2 assets, got %d", len(result.Assets))
	}
}

func Test_PublicGallery_RevokedShare_Returns404(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	col := createCollection(t, env, owner.Cookie, "Gallery", []string{})
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "collection",
		TargetID:   col.ID,
		Label:      "To Revoke",
	})

	// Obtain token before revocation.
	token := accessShare(t, env, sh.ID, "")

	// Revoke the share.
	revokeResp, err := env.App.Test(th.AuthRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID, nil, owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer revokeResp.Body.Close()
	if revokeResp.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke: expected 204, got %d", revokeResp.StatusCode)
	}

	// Re-access with the old token — should be 410 Gone.
	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets", "", token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusGone {
		t.Errorf("expected 410, got %d", resp.StatusCode)
	}
}

func Test_PublicGallery_PasswordProtected_NoToken_Returns401(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	col := createCollection(t, env, owner.Cookie, "Secret", []string{})
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "collection",
		TargetID:   col.ID,
		Label:      "Secret Gallery",
		Password:   strPtr("hunter2"),
	})

	// Try to access without correct password — no session token.
	req := httptest.NewRequest(http.MethodPost, "/shared/"+sh.ID+"/access", bytes.NewReader([]byte(`{"visitor_name":"Visitor","password":"wrong"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// ── WS-5.3: GET /shared/:id/export — anonymous ZIP scoped to share ───────────

func Test_ShareExport_CollectionZip(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	asset := th.UploadAsset(t, env, owner.Cookie)

	col := createCollection(t, env, owner.Cookie, "Exports", []string{asset.ID})

	allowDL := true
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "collection",
		TargetID:      col.ID,
		Label:         "My Export",
		AllowDownload: &allowDL,
	})

	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/export", "", token)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type: got %q, want application/zip", ct)
	}

	data, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	if len(zr.File) == 0 {
		t.Error("expected at least one file in ZIP")
	}
}

func Test_ShareExport_DisallowedDownload_Returns403(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	col := createCollection(t, env, owner.Cookie, "No DL", []string{})

	allowDL := false
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "collection",
		TargetID:      col.ID,
		Label:         "No Download",
		AllowDownload: &allowDL,
	})

	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/export", "", token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func Test_ShareExport_ScopedToSharedCollection(t *testing.T) {
	// Share token for collection A (1 asset) must not produce more than 1 file in ZIP,
	// even though collection B (1 other asset) also exists in the same workspace.
	env, owner := th.SetupWithOwner(t)

	assetA := th.UploadAsset(t, env, owner.Cookie)
	assetB := th.UploadAsset(t, env, owner.Cookie)

	colA := createCollection(t, env, owner.Cookie, "Col A", []string{assetA.ID})
	colB := createCollection(t, env, owner.Cookie, "Col B", []string{assetB.ID})
	_ = colB

	allowDL := true
	shA := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "collection",
		TargetID:      colA.ID,
		Label:         "Share A",
		AllowDownload: &allowDL,
	})

	tokenA := accessShare(t, env, shA.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+shA.ID+"/export", "", tokenA)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	data, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}

	// colA has exactly 1 asset — ZIP should have exactly 1 entry (no _missing_files.txt).
	assetFiles := 0
	for _, f := range zr.File {
		if f.Name != "_missing_files.txt" {
			assetFiles++
		}
	}
	if assetFiles != 1 {
		t.Errorf("expected 1 asset file in ZIP, got %d — colB assets may have leaked", assetFiles)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

type testCollection struct {
	ID string `json:"id"`
}

func createCollection(t *testing.T, env *th.TestEnv, cookie *http.Cookie, name string, assetIDs []string) testCollection {
	t.Helper()
	body := map[string]any{"name": name}
	if len(assetIDs) > 0 {
		body["asset_ids"] = assetIDs
	}
	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JSONBody(body), cookie))
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}
	var col testCollection
	if err := json.NewDecoder(resp.Body).Decode(&col); err != nil {
		t.Fatalf("decode collection: %v", err)
	}
	return col
}
