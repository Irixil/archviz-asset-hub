package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// fiberApp builds a minimal Fiber app with the given middleware and a 200 handler.
func fiberApp(middleware ...fiber.Handler) *fiber.App {
	app := fiber.New()
	for _, m := range middleware {
		app.Use(m)
	}
	app.Get("/*", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Post("/*", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	return app
}

// fiberAppCapture builds a Fiber app that captures claims via a final handler.
func fiberAppCapture(middleware fiber.Handler, capture func(fiber.Ctx) error) *fiber.App {
	app := fiber.New()
	app.Get("/", middleware, capture)
	app.Post("/", middleware, capture)
	return app
}

func newTestMakerMiddleware(t *testing.T) *Maker {
	t.Helper()
	m, err := NewMaker("test-secret-key-must-be-32chars!!")
	if err != nil {
		t.Fatalf("NewMaker: %v", err)
	}
	return m
}

func validToken(t *testing.T, m *Maker) string {
	t.Helper()
	tok, err := m.CreateToken("usr_1", "ws_1", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	return tok
}

func doGet(app *fiber.App, setup func(*http.Request)) *http.Response {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if setup != nil {
		setup(req)
	}
	resp, err := app.Test(req)
	if err != nil {
		panic(err)
	}
	return resp
}

// --- RequireAuth ---

func TestRequireAuth_MissingToken(t *testing.T) {
	m := newTestMakerMiddleware(t)
	app := fiberApp(RequireAuth(m))

	resp := doGet(app, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRequireAuth_BearerToken(t *testing.T) {
	m := newTestMakerMiddleware(t)
	tok := validToken(t, m)

	var captured *Claims
	app := fiberAppCapture(RequireAuth(m), func(c fiber.Ctx) error {
		captured = GetClaims(c)
		return c.SendStatus(200)
	})

	resp := doGet(app, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if captured == nil || captured.UserID != "usr_1" {
		t.Fatalf("expected claims with UserID usr_1, got %+v", captured)
	}
}

func TestRequireAuth_Cookie(t *testing.T) {
	m := newTestMakerMiddleware(t)
	tok := validToken(t, m)

	var captured *Claims
	app := fiberAppCapture(RequireAuth(m), func(c fiber.Ctx) error {
		captured = GetClaims(c)
		return c.SendStatus(200)
	})

	resp := doGet(app, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: tok})
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if captured == nil || captured.WorkspaceID != "ws_1" {
		t.Fatalf("expected claims with WorkspaceID ws_1, got %+v", captured)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	m := newTestMakerMiddleware(t)
	app := fiberApp(RequireAuth(m))

	resp := doGet(app, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer not-a-token")
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	m := newTestMakerMiddleware(t)
	tok, err := m.CreateToken("usr_1", "ws_1", -time.Minute)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	app := fiberApp(RequireAuth(m))
	resp := doGet(app, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// --- OptionalAuth ---

func TestOptionalAuth_NoToken(t *testing.T) {
	m := newTestMakerMiddleware(t)

	var captured *Claims
	app := fiberAppCapture(OptionalAuth(m), func(c fiber.Ctx) error {
		captured = GetClaims(c)
		return c.SendStatus(200)
	})

	resp := doGet(app, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if captured != nil {
		t.Fatalf("expected nil claims without token, got %+v", captured)
	}
}

func TestOptionalAuth_ValidToken(t *testing.T) {
	m := newTestMakerMiddleware(t)
	tok := validToken(t, m)

	var captured *Claims
	app := fiberAppCapture(OptionalAuth(m), func(c fiber.Ctx) error {
		captured = GetClaims(c)
		return c.SendStatus(200)
	})

	resp := doGet(app, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if captured == nil || captured.UserID != "usr_1" {
		t.Fatalf("expected non-nil claims, got %+v", captured)
	}
}

// --- RequireRole ---

func makeRoleFn(role Role, err error) func(context.Context, string, string) (Role, error) {
	return func(_ context.Context, _, _ string) (Role, error) { return role, err }
}

func TestRequireRole_InsufficientRole(t *testing.T) {
	m := newTestMakerMiddleware(t)
	tok := validToken(t, m)

	app := fiber.New()
	app.Get("/", RequireAuth(m), RequireRole(makeRoleFn(Viewer, nil), Editor), func(c fiber.Ctx) error {
		return c.SendStatus(200)
	})

	resp := doGet(app, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRequireRole_SufficientRole(t *testing.T) {
	m := newTestMakerMiddleware(t)
	tok := validToken(t, m)

	app := fiber.New()
	app.Get("/", RequireAuth(m), RequireRole(makeRoleFn(Owner, nil), Editor), func(c fiber.Ctx) error {
		return c.SendStatus(200)
	})

	resp := doGet(app, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- RequireShareSession ---

func validShareToken(t *testing.T, m *Maker, shareID string) string {
	t.Helper()
	tok, err := m.CreateShareToken(shareID, "asset", "ast_1", true, false, "Alice", time.Hour)
	if err != nil {
		t.Fatalf("CreateShareToken: %v", err)
	}
	return tok
}

func TestRequireShareSession_MissingToken(t *testing.T) {
	m := newTestMakerMiddleware(t)

	app := fiber.New()
	app.Get("/shared/:id/assets", RequireShareSession(m), func(c fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest(http.MethodGet, "/shared/sh_1/assets", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRequireShareSession_ShareIDMismatch(t *testing.T) {
	m := newTestMakerMiddleware(t)
	tok := validShareToken(t, m, "sh_1")

	app := fiber.New()
	app.Get("/shared/:id/assets", RequireShareSession(m), func(c fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest(http.MethodGet, "/shared/sh_2/assets", nil)
	req.Header.Set("X-Share-Token", tok)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRequireShareSession_Valid(t *testing.T) {
	m := newTestMakerMiddleware(t)
	tok := validShareToken(t, m, "sh_1")

	var captured *ShareClaims
	app := fiber.New()
	app.Get("/shared/:id/assets", RequireShareSession(m), func(c fiber.Ctx) error {
		captured = GetShareClaims(c)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest(http.MethodGet, "/shared/sh_1/assets", nil)
	req.Header.Set("X-Share-Token", tok)
	resp, _ := app.Test(req)

	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if captured == nil || captured.ShareID != "sh_1" {
		t.Fatalf("expected ShareClaims with ShareID sh_1, got %+v", captured)
	}
}
