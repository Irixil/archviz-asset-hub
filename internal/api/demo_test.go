//go:build demo

package api_test

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/app"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	dbpkg "damask/server/internal/db"
	"damask/server/internal/demo"
	"damask/server/internal/events"
	"damask/server/internal/jobs"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/storage"
	th "damask/server/internal/testhelpers"
	"damask/server/internal/transform"

	"github.com/gofiber/fiber/v3"
)

type demoEnv struct {
	App      *fiber.App
	Maker    *auth.Maker
	Database *sql.DB
	Seeder   *demo.Seeder
}

func setupDemoTestApp(t *testing.T) *demoEnv {
	t.Helper()

	demoCfg := config.DemoConfig{
		DemoMode:           true,
		ResetIntervalHours: 6,
		UserEmail:          "demo@damask.studio",
		WorkspaceName:      "Demo Agency",
		ShowBanner:         true,
		SignupURL:          "https://damask.studio/signup",
	}
	u, _ := url.Parse("http://localhost")
	cfg := &config.Config{
		JWTSecret:    "test-app-secret-for-tests!!",
		AppSecret:    "test-app-secret-for-tests!!",
		AppEnv:       "development",
		BaseURL:      u,
		Demo:         demoCfg,
		EnableSignup: true,
	}

	database, err := dbpkg.Open(":memory:")
	queries, rawDB := database.WQ, database.Writer
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	maker, err := auth.NewMaker("test-secret-key-must-be-32chars!!")
	if err != nil {
		t.Fatalf("auth maker: %v", err)
	}

	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	q := queue.New(queries, 1)
	hub := events.NewEventHub()

	trf := transform.NewTransformer()
	tmb := transform.NewThumbnailer(trf)
	noopMailer := mail.NewMailer(&mail.Config{})

	seeder := demo.New(rawDB, stor, demoCfg, trf, tmb)
	if err := seeder.EnsureWorkspace(t.Context()); err != nil {
		t.Fatalf("ensure demo workspace: %v", err)
	}

	deps, err := app.Build(cfg, database, stor, hub, q, noopMailer, trf, tmb)
	if err != nil {
		t.Fatalf("app.Build: %v", err)
	}

	_ = jobs.NewJobServer(jobs.Deps{
		Queries:           queries,
		SQLDB:             rawDB,
		Storage:           deps.Storage,
		Hub:               deps.Hub,
		Queue:             deps.Queue,
		Mailer:            deps.Mailer,
		Transformer:       deps.Transformer,
		Thumbnailer:       deps.Thumbnailer,
		Config:            deps.Config,
		Ingester:          deps.Ingester,
		AIAPIKeyResolver:  deps.KeyResolver,
		AIProviderFactory: deps.AIProviderFactory,
		WorkspaceRepo:     reposqlc.NewWorkspaceRepo(database),
		WorkflowExec:      deps.WorkflowExec,
		ExportSvc:         deps.Exports,
		ExifSvc:           deps.Exif,
		TagSvc:            deps.Tags,
		FieldSvc:          deps.Fields,
		TextTrackSvc:      deps.TextTracks,
		StorageSvc:        deps.StorageSvc,
	})
	router := api.NewRouter(deps, maker, seeder, nil)

	return &demoEnv{App: router, Maker: maker, Database: rawDB, Seeder: seeder}
}

func mintDemoToken(t *testing.T, env *demoEnv) string {
	t.Helper()
	userID, workspaceID, err := env.Seeder.GetDemoUser(t.Context())
	if err != nil {
		t.Fatalf("get demo user: %v", err)
	}
	token, err := env.Maker.CreateDemoToken(userID, workspaceID, time.Hour)
	if err != nil {
		t.Fatalf("mint demo token: %v", err)
	}
	return token
}

// --- DM-4.1: Demo middleware blocks restricted endpoints ---

func TestDemoMiddleware_BlocksWorkspaceSettings(t *testing.T) {
	env := setupDemoTestApp(t)
	token := mintDemoToken(t, env)

	req := th.BearerRequest(http.MethodPut, "/api/v1/workspace/settings",
		strings.NewReader(`{"name":"Hacked"}`), token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "not_available_in_demo" {
		t.Errorf("expected not_available_in_demo error, got %v", body["error"])
	}
}

func TestDemoMiddleware_BlocksCreateInvite(t *testing.T) {
	env := setupDemoTestApp(t)
	token := mintDemoToken(t, env)

	req := th.BearerRequest(http.MethodPost, "/api/v1/workspace/invites",
		strings.NewReader(`{"email":"attacker@evil.com","role":"editor"}`), token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "not_available_in_demo" {
		t.Errorf("expected not_available_in_demo error, got %v", body["error"])
	}
}

func TestDemoMiddleware_BlocksIngressPoll(t *testing.T) {
	env := setupDemoTestApp(t)
	token := mintDemoToken(t, env)

	req := th.BearerRequest(http.MethodPost, "/api/v1/ingress/sources/some-source-id/poll", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "not_available_in_demo" {
		t.Errorf("expected not_available_in_demo error, got %v", body["error"])
	}
}

func TestDemoMiddleware_BlockedResponse_HasSignupURLAndMessage(t *testing.T) {
	env := setupDemoTestApp(t)
	token := mintDemoToken(t, env)

	req := th.BearerRequest(http.MethodPut, "/api/v1/workspace/settings",
		strings.NewReader(`{"name":"x"}`), token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["signup_url"] == nil {
		t.Error("expected signup_url in demo 403 response")
	}
	if body["message"] == nil {
		t.Error("expected message in demo 403 response")
	}
}

// Non-demo tokens must NOT be blocked by demoBlock.
func TestDemoMiddleware_AllowsRegularUser(t *testing.T) {
	env := setupDemoTestApp(t)

	regReq := httptest.NewRequest(http.MethodPost, "/auth/register",
		strings.NewReader(`{"name":"Alice","email":"alice@example.com","password":"password123"}`))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := env.App.Test(regReq)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", regResp.StatusCode)
	}
	cookie := th.FindCookie(regResp, "auth_token")

	req := th.AuthRequest(http.MethodPut, "/api/v1/workspace/settings",
		strings.NewReader(`{"name":"My Workspace"}`), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode == http.StatusForbidden {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if body["error"] == "not_available_in_demo" {
			t.Fatal("regular user must not be blocked by demo middleware")
		}
	}
}

// --- DM-4.2: Upload cap ---

func TestDemoUploadCap_AssetLimit(t *testing.T) {
	env := setupDemoTestApp(t)
	token := mintDemoToken(t, env)

	_, workspaceID, err := env.Seeder.GetDemoUser(t.Context())
	if err != nil {
		t.Fatalf("get demo user: %v", err)
	}

	for i := 0; i < api.DemoMaxAssets; i++ {
		assetID := "cap-asset-" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		_, err := env.Database.Exec(`
			INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size, created_at, updated_at)
			VALUES (?, ?, 'fake.jpg', ?, 'image/jpeg', 1000, datetime('now'), datetime('now'))
		`, assetID, workspaceID, "fake/key/"+assetID)
		if err != nil {
			t.Fatalf("insert fake asset %d: %v", i, err)
		}
	}

	req := th.BuildUploadRequest(t, "new.jpg", th.MakeJPEG(10, 10), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 429, got %d: %s", resp.StatusCode, b)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if errMsg, ok := body["error"].(string); !ok || !strings.Contains(errMsg, "limit") {
		t.Errorf("expected limit message in error, got %v", body["error"])
	}
}

func TestDemoUploadCap_AllowsUploadUnderLimit(t *testing.T) {
	env := setupDemoTestApp(t)
	token := mintDemoToken(t, env)

	req := th.BuildUploadRequest(t, "test.jpg", th.MakeJPEG(10, 10), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}
}

// --- DM-3.1: POST /demo/session ---

func TestDemoSession_IssuesToken(t *testing.T) {
	env := setupDemoTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/demo/session", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["token"] == nil || body["token"] == "" {
		t.Error("expected token in response")
	}
	if body["is_demo"] != true {
		t.Errorf("expected is_demo=true, got %v", body["is_demo"])
	}
	tokenStr, _ := body["token"].(string)
	claims, err := env.Maker.VerifyToken(tokenStr)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if !claims.IsDemo {
		t.Error("token claims should have IsDemo=true")
	}
}

func TestDemoSession_SetsHttpOnlyCookie(t *testing.T) {
	env := setupDemoTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/demo/session", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	cookie := th.FindCookie(resp, "auth_token")
	if cookie == nil {
		t.Fatal("expected auth_token cookie to be set")
	}
	if !cookie.HttpOnly {
		t.Error("auth_token cookie must be HttpOnly")
	}
}

func TestDemoSession_TokenGrantsWorkspaceAccess(t *testing.T) {
	env := setupDemoTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/demo/session", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("session request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var sessionBody struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&sessionBody)

	meReq := th.BearerRequest(http.MethodGet, "/api/v1/workspace/me", nil, sessionBody.Token)
	meResp, err := env.App.Test(meReq)
	if err != nil {
		t.Fatalf("me request: %v", err)
	}
	if meResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(meResp.Body)
		t.Fatalf("expected 200 from /workspace/me, got %d: %s", meResp.StatusCode, b)
	}
}

// --- DM-5.5: GET /demo/status ---

func TestDemoStatus_ReturnsExpectedFields(t *testing.T) {
	env := setupDemoTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/demo/status", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["available"] != true {
		t.Errorf("expected available=true, got %v", body["available"])
	}
	if _, ok := body["asset_count"]; !ok {
		t.Error("expected asset_count field")
	}
	if body["asset_limit"] != float64(api.DemoMaxAssets) {
		t.Errorf("expected asset_limit=%d, got %v", api.DemoMaxAssets, body["asset_limit"])
	}
	if body["storage_limit_mb"] != float64(api.DemoMaxStorageBytes)/(1024*1024) {
		t.Errorf("expected storage_limit_mb=100, got %v", body["storage_limit_mb"])
	}
}

// --- DM-3.2: is_demo claim round-trip ---

func TestDemoToken_ClaimsRoundTrip(t *testing.T) {
	maker, _ := auth.NewMaker("test-secret-key-must-be-32chars!!")

	token, err := maker.CreateDemoToken("user1", "ws1", time.Hour)
	if err != nil {
		t.Fatalf("create demo token: %v", err)
	}
	claims, err := maker.VerifyToken(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if !claims.IsDemo {
		t.Error("IsDemo should be true")
	}
	if claims.UserID != "user1" {
		t.Errorf("UserID: got %q", claims.UserID)
	}

	regularToken, _ := maker.CreateToken("user1", "ws1", time.Hour)
	regularClaims, err := maker.VerifyToken(regularToken)
	if err != nil {
		t.Fatalf("verify regular token: %v", err)
	}
	if regularClaims.IsDemo {
		t.Error("regular token should not have IsDemo=true")
	}
}
