// Package testutil provides a lightweight test harness for Damask handler tests.
// It wires mock services into a real Fiber app without touching a database or
// running migrations, so tests start in milliseconds.
//
// Usage:
//
//	env := testutil.NewTestEnv(t)
//	env.Assets.GetFn = func(...) (*service.AssetDTO, error) { return fixtures.Asset(), nil }
//	token := env.MintToken(t, "user_1", "ws_1")
//	req := testutil.BearerRequest(http.MethodGet, "/api/v1/assets/ast_1", nil, token)
//	resp, _ := env.App.Test(req)
package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	"damask/server/internal/testutil/mockservice"

	"github.com/gofiber/fiber/v3"
)

const testSecret = "test-secret-key-must-be-32chars!!"

// TestEnv holds the Fiber app and all mock services for handler tests.
// Override the Fn fields on individual mocks before calling App.Test.
type TestEnv struct {
	App    *fiber.App
	Maker  *auth.Maker
	Server *api.Server

	Assets        *mockservice.MockAssetService
	Projects      *mockservice.MockProjectService
	Folders       *mockservice.MockFolderService
	Tags          *mockservice.MockTagService
	Collections   *mockservice.MockCollectionService
	Shares        *mockservice.MockShareService
	SharePublic   *mockservice.MockSharePublicService
	Fields        *mockservice.MockFieldService
	Integrations  *mockservice.MockIntegrationService
	AssetFields   *mockservice.MockAssetFieldService
	ProjectFields *mockservice.MockProjectFieldService
	Versions      *mockservice.MockVersionService
	Variants      *mockservice.MockVariantService
	TextTracks    *mockservice.MockTextTrackService
	AuditLog      *mockservice.MockAuditLogService
	Workspace     *mockservice.MockWorkspaceService
	Users         *mockservice.MockUserService
	Ingress       *mockservice.MockIngressService
	Stack         *mockservice.MockStackService
	Upload        *mockservice.MockUploadService
	Workflows     *mockservice.MockWorkflowService
	EmbedTokens   *mockservice.MockEmbedTokenService
	AutoTag       *mockservice.MockAutoTagService
}

// TestEnvOption customises the api.TestServerConfig used by NewTestEnv.
type TestEnvOption func(*api.TestServerConfig)

// WithPreMiddleware registers a Fiber handler before auth middleware and routes,
// letting a test manipulate the request context (e.g. simulate cancellation).
func WithPreMiddleware(h fiber.Handler) TestEnvOption {
	return func(cfg *api.TestServerConfig) {
		cfg.PreMiddleware = append(cfg.PreMiddleware, h)
	}
}

// NewTestEnv creates a TestEnv with all mock services wired into a Fiber app.
// No database, no migrations, no bcrypt — tests start instantly.
func NewTestEnv(t *testing.T, opts ...TestEnvOption) *TestEnv {
	t.Helper()

	maker, err := auth.NewMaker(testSecret)
	if err != nil {
		t.Fatalf("testutil: auth.NewMaker: %v", err)
	}

	assets := mockservice.NewAssetService()
	projects := mockservice.NewProjectService()
	folders := mockservice.NewFolderService()
	tags := mockservice.NewTagService()
	collections := mockservice.NewCollectionService()
	shares := mockservice.NewShareService()
	sharePublic := mockservice.NewSharePublicService()
	fields := mockservice.NewFieldService()
	integrations := mockservice.NewIntegrationService()
	assetFields := mockservice.NewAssetFieldService()
	projectFields := mockservice.NewProjectFieldService()
	versions := mockservice.NewVersionService()
	variants := mockservice.NewVariantService()
	textTracks := mockservice.NewTextTrackService()
	auditLog := mockservice.NewAuditLogService()
	workspace := mockservice.NewWorkspaceService()
	users := mockservice.NewUserService()
	ingress := mockservice.NewIngressService()
	stack := mockservice.NewStackService()
	upload := mockservice.NewUploadService()
	workflows := mockservice.NewWorkflowService()
	embedTokens := mockservice.NewEmbedTokenService()
	autoTag := mockservice.NewAutoTagService()

	cfg := &api.TestServerConfig{
		TokenMaker:    maker,
		Assets:        assets,
		Projects:      projects,
		Folders:       folders,
		Tags:          tags,
		Collections:   collections,
		Shares:        shares,
		SharePublic:   sharePublic,
		Fields:        fields,
		Integrations:  integrations,
		AssetFields:   assetFields,
		ProjectFields: projectFields,
		Versions:      versions,
		Variants:      variants,
		TextTracks:    textTracks,
		AuditLog:      auditLog,
		Workspace:     workspace,
		Users:         users,
		Ingress:       ingress,
		Stack:         stack,
		Upload:        upload,
		Workflows:     workflows,
		EmbedTokens:   embedTokens,
		AutoTag:       autoTag,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	srv, app := api.NewTestServer(cfg)

	return &TestEnv{
		App:    app,
		Maker:  maker,
		Server: srv,

		Assets:        assets,
		Projects:      projects,
		Folders:       folders,
		Tags:          tags,
		Collections:   collections,
		Shares:        shares,
		SharePublic:   sharePublic,
		Fields:        fields,
		Integrations:  integrations,
		AssetFields:   assetFields,
		ProjectFields: projectFields,
		Versions:      versions,
		Variants:      variants,
		TextTracks:    textTracks,
		AuditLog:      auditLog,
		Workspace:     workspace,
		Users:         users,
		Ingress:       ingress,
		Stack:         stack,
		Upload:        upload,
		Workflows:     workflows,
		EmbedTokens:   embedTokens,
		AutoTag:       autoTag,
	}
}

// MintToken issues a signed JWT for (userID, workspaceID) valid for 1 hour.
// Use the returned string as a Bearer token or build a cookie with it.
func (e *TestEnv) MintToken(t *testing.T, userID, workspaceID string) string {
	t.Helper()
	tok, err := e.Maker.CreateToken(userID, workspaceID, time.Hour)
	if err != nil {
		t.Fatalf("testutil: MintToken: %v", err)
	}
	return tok
}

// MintCookie issues a signed JWT and wraps it in an [http.Cookie] named "auth_token".
func (e *TestEnv) MintCookie(t *testing.T, userID, workspaceID string) *http.Cookie {
	t.Helper()
	tok := e.MintToken(t, userID, workspaceID)
	return &http.Cookie{Name: "auth_token", Value: tok, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}
}

// AuthRequest builds an HTTP request carrying the given cookie.
// body ([io.Reader]) may be nil.
func AuthRequest(method, path string, body io.Reader, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

// BearerRequest builds an HTTP request with an Authorization: Bearer <token> header.
// body ([io.Reader]) may be nil.
func BearerRequest(method, path string, body io.Reader, token string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// JSONStr returns an [io.Reader] over a raw JSON string literal.
func JSONStr(s string) io.Reader {
	return strings.NewReader(s)
}

// JSONBody marshals v to JSON and returns it as an [io.Reader].
func JSONBody(v any) io.Reader {
	b, err := json.Marshal(v)
	if err != nil {
		panic("testutil.JSONBody: " + err.Error())
	}
	return bytes.NewReader(b)
}

// FindCookie returns the first cookie with the given name from an HTTP response,
// or nil if not found.
func FindCookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// DecodeJSON decodes the JSON body of resp into dst.
func DecodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("testutil.DecodeJSON: %v", err)
	}
}

// AssertStatus calls t.Fatalf if resp.StatusCode != want.
func AssertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("expected status %d, got %d", want, resp.StatusCode)
	}
}

// Compile-time guard: ensure fiber.App is imported.
var _ *fiber.App
