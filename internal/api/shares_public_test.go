//go:build integration

package api_test

import (
	"damask/server/internal/api"
	th "damask/server/internal/testhelpers"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// accessShare calls POST /shared/:id/access with an optional password and returns
// the share session token on success.
func accessShare(t *testing.T, env *th.TestEnv, shareID, password string) string {
	t.Helper()
	var body string
	if password != "" {
		body = fmt.Sprintf(`{"visitor_name":"Visitor","password":%q}`, password)
	} else {
		body = `{"visitor_name":"Visitor"}`
	}
	req := httptest.NewRequest(http.MethodPost, "/shared/"+shareID+"/access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("access share: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var res api.ShareAccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode access response: %v", err)
	}
	return res.Token
}

// shareRequest builds an HTTP request with a Bearer share session token.
func shareRequest(method, path string, body string, token string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("X-Share-Token", token)
	}
	return req
}

// ── S-4: POST /shared/:id/access ───────────────────────────────────────────────────

func TestShareAccess_NoPassword(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
	})

	token := accessShare(t, env, sh.ID, "")
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestShareAccess_WithCorrectPassword(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	password := "s3cr3t"
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
		Password:   &password,
	})

	token := accessShare(t, env, sh.ID, "s3cr3t")
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestShareAccess_WrongPassword(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	password := "s3cr3t"
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
		Password:   &password,
	})

	req := httptest.NewRequest(http.MethodPost, "/shared/"+sh.ID+"/access", th.JSONBody(api.ShareAccessRequest{VisitorName: "Visitor", Password: "password"}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestShareAccess_MissingPassword(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	password := "s3cr3t"
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
		Password:   &password,
	})

	req := httptest.NewRequest(http.MethodPost, "/shared/"+sh.ID+"/access", th.JSONBody(api.ShareAccessRequest{VisitorName: "Visitor", Password: ""}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestShareAccess_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/shared/nonexistent/access", th.JSONBody(api.ShareAccessRequest{VisitorName: "Visitor", Password: ""}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestShareAccess_Revoked(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
	})

	// Revoke the share
	req := th.AuthRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID, nil, owner.Cookie)
	env.App.Test(req) //nolint:errcheck

	req2 := httptest.NewRequest(http.MethodPost, "/shared/"+sh.ID+"/access", th.JSONBody(api.ShareAccessRequest{VisitorName: "Visitor", Password: ""}))
	req2.Header.Set("Content-Type", "application/json")
	resp, _ := env.App.Test(req2)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for revoked share, got %d", resp.StatusCode)
	}
}

// ── S-4b: GET /shared/:id/access ─────────────────────────────────────────────

func TestShareInfo_NoPassword(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		Label:      "My Gallery",
		TargetType: "project",
		TargetID:   p.ID,
	})

	req := httptest.NewRequest(http.MethodGet, "/shared/"+sh.ID+"/access", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("GET /shared/:id/access: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var info api.ShareInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.HasPassword {
		t.Error("expected has_password = false")
	}
	if info.Label != "My Gallery" {
		t.Errorf("expected label = 'My Gallery', got %q", info.Label)
	}
}

func TestShareInfo_WithPassword(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	password := "s3cr3t"
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
		Password:   &password,
	})

	req := httptest.NewRequest(http.MethodGet, "/shared/"+sh.ID+"/access", nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var info api.ShareInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !info.HasPassword {
		t.Error("expected has_password = true")
	}
}

func TestShareInfo_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/shared/nonexistent/access", nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestShareAccess_Expired(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	expiresInDays := 7
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "project",
		TargetID:      p.ID,
		ExpiresInDays: &expiresInDays,
	})

	// Force expiry
	env.Database.Exec(`UPDATE shares SET expires_at = datetime('now', '-1 day') WHERE id = ?`, sh.ID) //nolint:errcheck

	req := httptest.NewRequest(http.MethodPost, "/shared/"+sh.ID+"/access", th.JSONBody(api.ShareAccessRequest{VisitorName: "Visitor", Password: ""}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusGone {
		t.Errorf("expected 410 for expired share, got %d", resp.StatusCode)
	}
}

func TestShareAccess_IncrementsViewCount(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
	})

	if sh.ViewCount != 0 {
		t.Fatalf("expected view_count = 0, got %d", sh.ViewCount)
	}

	accessShare(t, env, sh.ID, "")

	// Check via owner API
	req := th.AuthRequest(http.MethodGet, "/api/v1/shares/"+sh.ID, nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var got api.ShareResponse
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.ViewCount != 1 {
		t.Errorf("expected view_count = 1 after access, got %d", got.ViewCount)
	}
}

// ── S-5: public content endpoints ────────────────────────────────────────────

func TestShareListAssets_Project(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	assetID := uploadTestAsset(t, env, owner)
	// Assign asset to project directly via SQL
	if _, err := env.Database.Exec(`UPDATE assets SET project_id = ? WHERE id = ?`, p.ID, assetID); err != nil {
		t.Fatalf("assign asset to project: %v", err)
	}

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
	})
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets", "", token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestShareListAssets_SingleAsset(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "asset",
		TargetID:   assetID,
	})
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets", "", token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Assets []api.AssetResponse `json:"assets"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Assets) != 1 || body.Assets[0].ID != assetID {
		t.Errorf("expected 1 asset with id %s, got %v", assetID, body.Assets)
	}
}

func TestShareListAssets_NoToken(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
	})

	req := httptest.NewRequest(http.MethodGet, "/shared/"+sh.ID+"/assets", nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestShareListAssets_WrongShareToken(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p1 := createProject(t, env, owner.Cookie, "P1", "#000")
	p2 := createProject(t, env, owner.Cookie, "P2", "#000")
	sh1 := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p1.ID,
	})
	sh2 := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p2.ID,
	})
	token1 := accessShare(t, env, sh1.ID, "")

	// Use sh1 token on sh2 path
	req := shareRequest(http.MethodGet, "/shared/"+sh2.ID+"/assets", "", token1)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for mismatched share token, got %d", resp.StatusCode)
	}
}

func TestShareGetAsset_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "asset",
		TargetID:   assetID,
	})
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+assetID, "", token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.ID != assetID {
		t.Errorf("asset id mismatch: got %s", got.ID)
	}
}

func TestShareGetAsset_NotInShare(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID1 := uploadTestAsset(t, env, owner)
	assetID2 := uploadTestAsset(t, env, owner)

	// Share only targets assetID1
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "asset",
		TargetID:   assetID1,
	})
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+assetID2, "", token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for asset not in share, got %d", resp.StatusCode)
	}
}

func TestShareGetAssetFile_AllowDownloadFalse(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	allowDownload := false
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      assetID,
		AllowDownload: &allowDownload,
	})
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+assetID+"/file", "", token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 when allow_download=false, got %d", resp.StatusCode)
	}
}

func TestShareGetAssetFile_AllowDownloadTrue(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	allowDownload := true
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      assetID,
		AllowDownload: &allowDownload,
	})
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+assetID+"/file", "", token)
	resp, _ := env.App.Test(req)
	// 200 = file streamed successfully
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestShareGetAssetFile_ServesCurrentVersion(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")

	// Upload original asset (100×100) then a second version (200×200).
	asset := th.UploadAsset(t, env, owner.Cookie)
	v2Data := th.MakeJPEG(200, 200)
	vReq := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", v2Data, "", owner.Cookie)
	vResp, err := env.App.Test(vReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload v2: %v", err)
	}
	if vResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(vResp.Body)
		t.Fatalf("expected 201 for v2 upload, got %d: %s", vResp.StatusCode, b)
	}

	allowDownload := true
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      asset.ID,
		AllowDownload: &allowDownload,
	})
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+asset.ID+"/file", "", token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("shared file: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	fileBytes, _ := io.ReadAll(resp.Body)
	if len(fileBytes) == 0 {
		t.Fatal("expected non-empty file content")
	}
	v1Bytes := th.MakeJPEG(100, 100)
	if len(fileBytes) <= len(v1Bytes) {
		t.Errorf("expected v2 file (%d bytes) to be larger than v1 (%d bytes)", len(fileBytes), len(v1Bytes))
	}
}

func TestShareContentRecheck_RevokedMidSession(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
	})

	// Obtain a valid token
	token := accessShare(t, env, sh.ID, "")

	// Owner revokes the share
	revokeReq := th.AuthRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID, nil, owner.Cookie)
	env.App.Test(revokeReq) //nolint:errcheck

	// Subsequent content request should be rejected
	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets", "", token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusGone {
		t.Errorf("expected 410 after share revoked mid-session, got %d", resp.StatusCode)
	}
}

// ── S-6: public comment endpoints ────────────────────────────────────────────

func TestShareCreateComment_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      assetID,
		AllowComments: true,
	})
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Sarah","body":"Looks great!"}`, assetID)
	req := shareRequest(http.MethodPost, "/shared/"+sh.ID+"/comments", body, token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var got api.CommentResponse
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.AuthorName != "Sarah" {
		t.Errorf("author_name = %q, want Sarah", got.AuthorName)
	}
	if got.Body != "Looks great!" {
		t.Errorf("body = %q, want 'Looks great!'", got.Body)
	}
	if got.AssetID != assetID {
		t.Errorf("asset_id mismatch")
	}
}

func TestShareCreateComment_CommentsNotAllowed(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	// allow_comments defaults to false
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType: "asset",
		TargetID:   assetID,
	})
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Sarah","body":"Hi"}`, assetID)
	req := shareRequest(http.MethodPost, "/shared/"+sh.ID+"/comments", body, token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 when comments disabled, got %d", resp.StatusCode)
	}
}

func TestShareCreateComment_MissingAuthorName(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      assetID,
		AllowComments: true,
	})
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"body":"Hi"}`, assetID)
	req := shareRequest(http.MethodPost, "/shared/"+sh.ID+"/comments", body, token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestShareCreateComment_AssetNotInShare(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID1 := uploadTestAsset(t, env, owner)
	assetID2 := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      assetID1,
		AllowComments: true,
	})
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Sarah","body":"Hi"}`, assetID2)
	req := shareRequest(http.MethodPost, "/shared/"+sh.ID+"/comments", body, token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for asset not in share, got %d", resp.StatusCode)
	}
}

func TestShareListComments_Grouped(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")

	// Upload two assets and assign them to the project directly via SQL.
	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)
	if _, err := env.Database.Exec(`UPDATE assets SET project_id = ? WHERE id IN (?, ?)`, p.ID, a1, a2); err != nil {
		t.Fatalf("assign assets to project: %v", err)
	}

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "project",
		TargetID:      p.ID,
		AllowComments: true,
	})
	token := accessShare(t, env, sh.ID, "")

	// Post one comment on each asset
	postComment := func(assetID, author, body string) {
		t.Helper()
		b := fmt.Sprintf(`{"asset_id":%q,"author_name":%q,"body":%q}`, assetID, author, body)
		req := shareRequest(http.MethodPost, "/shared/"+sh.ID+"/comments", b, token)
		env.App.Test(req) //nolint:errcheck
	}
	postComment(a1, "Alice", "Nice!")
	postComment(a2, "Bob", "Cool")

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/comments", "", token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	type group struct {
		AssetID  string                `json:"asset_id"`
		Comments []api.CommentResponse `json:"comments"`
	}
	var groups []group
	_ = json.NewDecoder(resp.Body).Decode(&groups)
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestShareListAssetComments(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      assetID,
		AllowComments: true,
	})
	token := accessShare(t, env, sh.ID, "")

	// Post two comments
	for i := 0; i < 2; i++ {
		body := fmt.Sprintf(`{"asset_id":%q,"author_name":"User%d","body":"Comment %d"}`, assetID, i, i)
		req := shareRequest(http.MethodPost, "/shared/"+sh.ID+"/comments", body, token)
		env.App.Test(req) //nolint:errcheck
	}

	req := shareRequest(http.MethodGet, "/shared/"+sh.ID+"/assets/"+assetID+"/comments", "", token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var items []api.CommentResponse
	_ = json.NewDecoder(resp.Body).Decode(&items)
	if len(items) != 2 {
		t.Errorf("expected 2 comments, got %d", len(items))
	}
}

// ── S-7: owner moderation ─────────────────────────────────────────────────────

func TestOwnerListComments_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      assetID,
		AllowComments: true,
	})
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Client","body":"Feedback"}`, assetID)
	req := shareRequest(http.MethodPost, "/shared/"+sh.ID+"/comments", body, token)
	env.App.Test(req) //nolint:errcheck

	req2 := th.AuthRequest(http.MethodGet, "/api/v1/shares/"+sh.ID+"/comments", nil, owner.Cookie)
	resp, _ := env.App.Test(req2)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var items []api.CommentResponse
	_ = json.NewDecoder(resp.Body).Decode(&items)
	if len(items) != 1 {
		t.Errorf("expected 1 comment, got %d", len(items))
	}
}

func TestOwnerListComments_WrongWorkspace(t *testing.T) {
	env := th.SetupTestApp(t)
	owner1 := th.Register(t, env, "Owner1", "owner1@example.com", "password123")
	owner2 := th.Register(t, env, "Owner2", "owner2@example.com", "password123")
	p := createProject(t, env, owner1.Cookie, "P", "#000")
	sh := createShare(t, env, owner1.Cookie, api.CreateShareRequest{
		TargetType: "project",
		TargetID:   p.ID,
	})

	req := th.AuthRequest(http.MethodGet, "/api/v1/shares/"+sh.ID+"/comments", nil, owner2.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestOwnerDeleteComment_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      assetID,
		AllowComments: true,
	})
	token := accessShare(t, env, sh.ID, "")

	// Post a comment
	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Client","body":"Delete me"}`, assetID)
	req := shareRequest(http.MethodPost, "/shared/"+sh.ID+"/comments", body, token)
	resp, _ := env.App.Test(req)
	var comment api.CommentResponse
	_ = json.NewDecoder(resp.Body).Decode(&comment)

	// Owner deletes the comment
	delReq := th.AuthRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID+"/comments/"+comment.ID, nil, owner.Cookie)
	delResp, _ := env.App.Test(delReq)
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", delResp.StatusCode)
	}

	// Confirm deleted
	listReq := th.AuthRequest(http.MethodGet, "/api/v1/shares/"+sh.ID+"/comments", nil, owner.Cookie)
	listResp, _ := env.App.Test(listReq)
	var items []api.CommentResponse
	_ = json.NewDecoder(listResp.Body).Decode(&items)
	if len(items) != 0 {
		t.Errorf("expected 0 comments after delete, got %d", len(items))
	}
}

func TestOwnerDeleteComment_WrongWorkspace(t *testing.T) {
	env := th.SetupTestApp(t)
	owner1 := th.Register(t, env, "Owner1", "owner1@example.com", "password123")
	owner2 := th.Register(t, env, "Owner2", "owner2@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner1)

	sh := createShare(t, env, owner1.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      assetID,
		AllowComments: true,
	})
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Client","body":"Feedback"}`, assetID)
	req := shareRequest(http.MethodPost, "/shared/"+sh.ID+"/comments", body, token)
	resp, _ := env.App.Test(req)
	var comment api.CommentResponse
	_ = json.NewDecoder(resp.Body).Decode(&comment)

	// owner2 tries to delete it
	delReq := th.AuthRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID+"/comments/"+comment.ID, nil, owner2.Cookie)
	delResp, _ := env.App.Test(delReq)
	if delResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for cross-workspace delete, got %d", delResp.StatusCode)
	}
}
