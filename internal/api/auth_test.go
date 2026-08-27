//go:build integration

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/apperr"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

// fixedUser returns a minimal UserDTO for auth handler tests.
func fixedUser(id, email, name string) *service.UserDTO {
	return &service.UserDTO{ID: id, Email: email, Name: name, CreatedAt: time.Now()}
}

// fixedWorkspace returns a minimal WorkspaceDTO for auth handler tests.
func fixedWorkspace(id string) *service.WorkspaceDTO {
	return &service.WorkspaceDTO{ID: id, Name: "Test Workspace"}
}

func TestRegister_Success(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Users.RegisterFn = func(_ context.Context, p service.RegisterUserParams) (*service.RegisterUserResult, error) {
		return &service.RegisterUserResult{User: fixedUser("usr_1", p.Email, p.Name), WorkspaceID: "ws_1"}, nil
	}
	env.Workspace.GetFn = func(_ context.Context, _ string) (*service.WorkspaceDTO, error) {
		return fixedWorkspace("ws_1"), nil
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/register",
		testutil.JSONBody(api.RegisterRequest{
			Name:     "Alice",
			Email:    "alice@example.com",
			Password: "password123",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	testutil.AssertStatus(t, resp, http.StatusCreated)

	var body api.AuthResponse
	testutil.DecodeJSON(t, resp, &body)
	if body.User.Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", body.User.Email)
	}
	if body.Token == "" {
		t.Error("expected non-empty token")
	}

	cookie := testutil.FindCookie(resp, "auth_token")
	if cookie == nil {
		t.Fatal("auth_token cookie not set")
	}
	if !cookie.HttpOnly {
		t.Error("cookie should be HttpOnly")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Users.RegisterFn = func(_ context.Context, _ service.RegisterUserParams) (*service.RegisterUserResult, error) {
		return nil, fmt.Errorf("email already in use: %w", apperr.ErrConflict)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/register",
		testutil.JSONBody(api.RegisterRequest{
			Name:     "Alice2",
			Email:    "alice@example.com",
			Password: "password456",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	testutil.AssertStatus(t, resp, http.StatusConflict)
}

func TestRegister_InvalidBody(t *testing.T) {
	env := testutil.NewTestEnv(t)

	cases := []struct {
		name string
		req  api.RegisterRequest
	}{
		{"missing name", api.RegisterRequest{Name: "", Email: "a@b.com", Password: "password123"}},
		{"missing email", api.RegisterRequest{Name: "Alice", Email: "", Password: "password123"}},
		{"missing password", api.RegisterRequest{Name: "Alice", Email: "a@b.com", Password: ""}},
		{"short password", api.RegisterRequest{Name: "Alice", Email: "a@b.com", Password: "short"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/auth/register", testutil.JSONBody(tc.req))
			req.Header.Set("Content-Type", "application/json")
			resp, err := env.App.Test(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
		})
	}
}

func TestLogin_Success(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Users.LoginFn = func(_ context.Context, _ service.LoginUserParams) (*service.LoginUserResult, error) {
		return &service.LoginUserResult{User: fixedUser("usr_1", "alice@example.com", "Alice"), WorkspaceID: "ws_1"}, nil
	}
	env.Workspace.GetFn = func(_ context.Context, _ string) (*service.WorkspaceDTO, error) {
		return fixedWorkspace("ws_1"), nil
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		testutil.JSONBody(api.LoginRequest{
			Email:    "alice@example.com",
			Password: "password123",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	testutil.AssertStatus(t, resp, http.StatusOK)

	cookie := testutil.FindCookie(resp, "auth_token")
	if cookie == nil {
		t.Fatal("auth_token cookie not set on login")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Users.LoginFn = func(_ context.Context, _ service.LoginUserParams) (*service.LoginUserResult, error) {
		return nil, fmt.Errorf("invalid credentials: %w", apperr.ErrForbidden)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		testutil.JSONBody(api.LoginRequest{
			Email:    "alice@example.com",
			Password: "wrongpassword",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestLogin_UnknownEmail(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Users.LoginFn = func(_ context.Context, _ service.LoginUserParams) (*service.LoginUserResult, error) {
		return nil, fmt.Errorf("invalid credentials: %w", apperr.ErrForbidden)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		testutil.JSONBody(api.LoginRequest{
			Email:    "nobody@example.com",
			Password: "password123",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestLogout(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPost, "/auth/logout", nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	testutil.AssertStatus(t, resp, http.StatusOK)

	authCookie := testutil.FindCookie(resp, "auth_token")
	if authCookie == nil {
		t.Fatal("expected auth_token cookie in response")
	}
	if authCookie.Value != "" {
		t.Errorf("expected empty cookie value, got %q", authCookie.Value)
	}
}

func TestRefresh_Success(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPost, "/auth/refresh", nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	testutil.AssertStatus(t, resp, http.StatusOK)

	newCookie := testutil.FindCookie(resp, "auth_token")
	if newCookie == nil {
		t.Fatal("expected new auth_token cookie after refresh")
	}
}

func TestRefresh_Unauthenticated(t *testing.T) {
	env := testutil.NewTestEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}
