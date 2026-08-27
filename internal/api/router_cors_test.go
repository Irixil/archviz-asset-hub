//go:build integration

package api_test

import (
	"net/http"
	"testing"

	th "damask/server/internal/testhelpers"
)

// preflight sends a CORS preflight (OPTIONS) request for origin against path and
// returns the Access-Control-Allow-Origin response header (empty if absent).
func preflight(t *testing.T, env *th.TestEnv, origin, path string) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", "GET")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("preflight request: %v", err)
	}
	defer resp.Body.Close()
	return resp.Header.Get("Access-Control-Allow-Origin")
}

func TestCORS_ProdConfig_DeniesLocalhostOrigin(t *testing.T) {
	env := th.SetupTestApp(t, th.WithAppEnv("production"))

	if got := preflight(t, env, "http://localhost:5173", "/api/v1/projects"); got == "http://localhost:5173" {
		t.Fatalf("expected localhost origin to be denied in production, got Access-Control-Allow-Origin=%q", got)
	}
}

func TestCORS_ProdConfig_AllowsBaseURL(t *testing.T) {
	env := th.SetupTestApp(t, th.WithAppEnv("production"))

	baseOrigin := env.Config.BaseURL.Scheme + "://" + env.Config.BaseURL.Host
	if got := preflight(t, env, baseOrigin, "/api/v1/projects"); got != baseOrigin {
		t.Fatalf("expected BaseURL origin %q to be allowed, got Access-Control-Allow-Origin=%q", baseOrigin, got)
	}
}

func TestCORS_DevConfig_AllowsLocalhostOrigin(t *testing.T) {
	env := th.SetupTestApp(t, th.WithAppEnv("development"))

	if got := preflight(t, env, "http://localhost:5173", "/api/v1/projects"); got != "http://localhost:5173" {
		t.Fatalf("expected localhost origin to be allowed in development, got Access-Control-Allow-Origin=%q", got)
	}
}

func TestCORS_DevConfig_AllowsBaseURL(t *testing.T) {
	env := th.SetupTestApp(t, th.WithAppEnv("development"))

	baseOrigin := env.Config.BaseURL.Scheme + "://" + env.Config.BaseURL.Host
	if got := preflight(t, env, baseOrigin, "/api/v1/projects"); got != baseOrigin {
		t.Fatalf("expected BaseURL origin %q to be allowed, got Access-Control-Allow-Origin=%q", baseOrigin, got)
	}
}
