//go:build integration

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/apperr"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
	"damask/server/internal/testutil/fixtures"
)

// fixedShare builds a ShareDTO with sensible defaults for a given id and target.
func fixedShare(id, wsID, targetType, targetID string) *service.ShareDTO {
	return &service.ShareDTO{
		ID:            id,
		WorkspaceID:   wsID,
		TargetType:    targetType,
		TargetID:      targetID,
		AllowDownload: true,
		CreatedAt:     time.Now(),
	}
}

// shareEnv wires a testutil env with a project mock returning prj_1 and a share store
// that tracks one share by ID.
func shareEnv(t *testing.T) (*testutil.TestEnv, *service.ShareDTO) {
	t.Helper()
	env := testutil.NewTestEnv(t)
	env.Projects.GetFn = func(_ context.Context, _, id string) (*service.ProjectDTO, error) {
		return fixtures.Project(func(p *service.ProjectDTO) { p.ID = id }), nil
	}
	env.Assets.GetFn = func(_ context.Context, _, id string) (*service.AssetDTO, error) {
		return fixtures.Asset(func(a *service.AssetDTO) { a.ID = id }), nil
	}
	sh := fixedShare("shr_1", "ws_1", "project", "prj_1")
	env.Shares.CreateFn = func(_ context.Context, wsID string, p service.CreateShareParams) (*service.ShareDTO, error) {
		s := fixedShare("shr_1", wsID, p.TargetType, p.TargetID)
		s.Label = p.Label
		s.AllowDownload = p.AllowDownload
		if p.Password != nil {
			hash := "hashed"
			s.PasswordHash = &hash
		}
		if p.ExpiresInDays != nil {
			future := time.Now().Add(time.Duration(*p.ExpiresInDays) * 24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
			s.ExpiresAt = &future
		}
		s.AllowComments = p.AllowComments
		*sh = *s
		return sh, nil
	}
	env.Shares.GetFn = func(_ context.Context, wsID, id string) (*service.ShareDTO, error) {
		if wsID != sh.WorkspaceID || id != sh.ID {
			return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		return sh, nil
	}
	env.Shares.ListFn = func(_ context.Context, wsID string) ([]*service.ShareDTO, error) {
		if wsID != sh.WorkspaceID {
			return []*service.ShareDTO{}, nil
		}
		return []*service.ShareDTO{sh}, nil
	}
	env.Shares.UpdateFn = func(_ context.Context, wsID, id string, p service.UpdateShareParams) (*service.ShareDTO, error) {
		if wsID != sh.WorkspaceID || id != sh.ID {
			return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		if p.Label != nil {
			sh.Label = *p.Label
		}
		if p.Password != nil {
			h := "hashed"
			sh.PasswordHash = &h
		}
		if p.ClearPassword {
			sh.PasswordHash = nil
		}
		if p.AllowComments != nil {
			sh.AllowComments = *p.AllowComments
		}
		return sh, nil
	}
	env.Shares.RevokeFn = func(_ context.Context, wsID, id string) error {
		if wsID != sh.WorkspaceID || id != sh.ID {
			return fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		sh.RevokedAt = &now
		return nil
	}
	return env, sh
}

// --- POST /shares ---

func TestCreateShare_ProjectTarget(t *testing.T) {
	env, _ := shareEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")
	allowDownload := true

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/shares",
		testutil.JSONBody(api.CreateShareRequest{
			Label:         "Nike Q3 delivery",
			TargetType:    "project",
			TargetID:      "prj_1",
			AllowDownload: &allowDownload,
		}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusCreated)

	var sh api.ShareResponse
	testutil.DecodeJSON(t, resp, &sh)
	if sh.TargetType != "project" {
		t.Errorf("target_type = %q, want project", sh.TargetType)
	}
	if sh.Label != "Nike Q3 delivery" {
		t.Errorf("label = %q, want Nike Q3 delivery", sh.Label)
	}
	if !sh.AllowDownload {
		t.Error("expected allow_download = true")
	}
	if sh.HasPassword {
		t.Error("expected has_password = false")
	}
	if sh.PublicURL == "" || !strings.Contains(sh.PublicURL, "/s/"+sh.ID) {
		t.Errorf("public_url = %q, expected to contain /s/%s", sh.PublicURL, sh.ID)
	}
}

func TestCreateShare_AssetTarget(t *testing.T) {
	env, _ := shareEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/shares",
		testutil.JSONBody(api.CreateShareRequest{TargetType: "asset", TargetID: "ast_1"}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusCreated)

	var sh api.ShareResponse
	testutil.DecodeJSON(t, resp, &sh)
	if sh.TargetType != "asset" {
		t.Errorf("target_type = %q, want asset", sh.TargetType)
	}
}

func TestCreateShare_WithPassword(t *testing.T) {
	env, _ := shareEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")
	password := "hunter2"

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/shares",
		testutil.JSONBody(api.CreateShareRequest{TargetType: "project", TargetID: "prj_1", Password: &password}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusCreated)

	var sh api.ShareResponse
	testutil.DecodeJSON(t, resp, &sh)
	if !sh.HasPassword {
		t.Error("expected has_password = true")
	}
}

func TestCreateShare_WithExpiry(t *testing.T) {
	env, _ := shareEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")
	expiresInDays := 14

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/shares",
		testutil.JSONBody(api.CreateShareRequest{TargetType: "project", TargetID: "prj_1", ExpiresInDays: &expiresInDays}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusCreated)

	var sh api.ShareResponse
	testutil.DecodeJSON(t, resp, &sh)
	if sh.ExpiresAt == nil {
		t.Error("expected expires_at to be set")
	}
	if sh.IsExpired {
		t.Error("expected is_expired = false for future expiry")
	}
}

func TestCreateShare_TargetNotFound(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Projects.GetFn = func(_ context.Context, _, _ string) (*service.ProjectDTO, error) {
		return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/shares",
		testutil.JSONBody(api.CreateShareRequest{TargetType: "project", TargetID: "nonexistent"}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestCreateShare_TargetFromOtherWorkspace(t *testing.T) {
	env := testutil.NewTestEnv(t)
	// Project exists in ws_1, but requester is ws_2 — service returns not found
	env.Projects.GetFn = func(_ context.Context, wsID, _ string) (*service.ProjectDTO, error) {
		if wsID != "ws_1" {
			return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		return fixtures.Project(), nil
	}
	cookie := env.MintCookie(t, "usr_2", "ws_2")

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/shares",
		testutil.JSONBody(api.CreateShareRequest{TargetType: "project", TargetID: "prj_1"}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestCreateShare_Unauthenticated(t *testing.T) {
	env := testutil.NewTestEnv(t)
	req := testutil.AuthRequest(http.MethodPost, "/api/v1/shares",
		testutil.JSONBody(api.CreateShareRequest{TargetType: "project", TargetID: "x"}), nil)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

// --- GET /shares ---

func TestListShares_Empty(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Shares.ListFn = func(_ context.Context, _ string) ([]*service.ShareDTO, error) {
		return []*service.ShareDTO{}, nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodGet, "/api/v1/shares", nil, cookie))
	testutil.AssertStatus(t, resp, http.StatusOK)

	var items []api.ShareResponse
	testutil.DecodeJSON(t, resp, &items)
	if len(items) != 0 {
		t.Errorf("expected 0 shares, got %d", len(items))
	}
}

func TestListShares_WorkspaceIsolation(t *testing.T) {
	env, _ := shareEnv(t)
	// Authenticate as ws_other — shareEnv's ListFn returns empty for other workspaces
	cookie := env.MintCookie(t, "usr_other", "ws_other")

	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodGet, "/api/v1/shares", nil, cookie))
	var items []api.ShareResponse
	testutil.DecodeJSON(t, resp, &items)
	if len(items) != 0 {
		t.Errorf("expected 0 shares for other workspace, got %d", len(items))
	}
}

// --- GET /shares/:id ---

func TestGetShare_Success(t *testing.T) {
	env, sh := shareEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodGet, "/api/v1/shares/"+sh.ID, nil, cookie))
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got api.ShareResponse
	testutil.DecodeJSON(t, resp, &got)
	if got.ID != sh.ID {
		t.Errorf("id = %q, want %s", got.ID, sh.ID)
	}
}

func TestGetShare_OtherWorkspace(t *testing.T) {
	env, _ := shareEnv(t)
	// Authenticate as ws_other — shareEnv's GetFn returns 404 for wrong workspace
	cookie := env.MintCookie(t, "usr_other", "ws_other")

	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodGet, "/api/v1/shares/shr_1", nil, cookie))
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

// --- PUT /shares/:id ---

func TestUpdateShare_Label(t *testing.T) {
	env, _ := shareEnv(t)
	// Create the share first so it exists in the mock store
	cookie := env.MintCookie(t, "usr_1", "ws_1")
	env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/shares", //nolint:errcheck
		testutil.JSONBody(api.CreateShareRequest{TargetType: "project", TargetID: "prj_1"}), cookie))

	label := "New Label"
	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodPut, "/api/v1/shares/shr_1",
		testutil.JSONBody(api.UpdateShareRequest{Label: &label}), cookie))
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got api.ShareResponse
	testutil.DecodeJSON(t, resp, &got)
	if got.Label != "New Label" {
		t.Errorf("label = %q, want New Label", got.Label)
	}
}

func TestUpdateShare_SetAndClearPassword(t *testing.T) {
	env, _ := shareEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")
	env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/shares", //nolint:errcheck
		testutil.JSONBody(api.CreateShareRequest{TargetType: "project", TargetID: "prj_1"}), cookie))

	password := "s3cr3t"
	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodPut, "/api/v1/shares/shr_1",
		testutil.JSONBody(api.UpdateShareRequest{Password: &password}), cookie))
	testutil.AssertStatus(t, resp, http.StatusOK)
	var got api.ShareResponse
	testutil.DecodeJSON(t, resp, &got)
	if !got.HasPassword {
		t.Error("expected has_password = true after setting password")
	}

	clearPassword := true
	resp2, _ := env.App.Test(testutil.AuthRequest(http.MethodPut, "/api/v1/shares/shr_1",
		testutil.JSONBody(api.UpdateShareRequest{ClearPassword: &clearPassword}), cookie))
	testutil.AssertStatus(t, resp2, http.StatusOK)
	var got2 api.ShareResponse
	testutil.DecodeJSON(t, resp2, &got2)
	if got2.HasPassword {
		t.Error("expected has_password = false after clearing password")
	}
}

func TestUpdateShare_AllowComments(t *testing.T) {
	env, sh := shareEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")
	env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/shares", //nolint:errcheck
		testutil.JSONBody(api.CreateShareRequest{TargetType: "project", TargetID: "prj_1"}), cookie))

	if sh.AllowComments {
		t.Fatal("expected allow_comments = false by default")
	}

	allowComments := true
	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodPut, "/api/v1/shares/shr_1",
		testutil.JSONBody(api.UpdateShareRequest{AllowComments: &allowComments}), cookie))
	var got api.ShareResponse
	testutil.DecodeJSON(t, resp, &got)
	if !got.AllowComments {
		t.Error("expected allow_comments = true after update")
	}
}

func TestUpdateShare_NotFound(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Shares.UpdateFn = func(_ context.Context, _, _ string, _ service.UpdateShareParams) (*service.ShareDTO, error) {
		return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	x := "x"
	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodPut, "/api/v1/shares/nonexistent",
		testutil.JSONBody(api.UpdateShareRequest{Label: &x}), cookie))
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

// --- DELETE /shares/:id ---

func TestRevokeShare_Success(t *testing.T) {
	env, sh := shareEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")
	// Populate the share in the mock store
	env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/shares", //nolint:errcheck
		testutil.JSONBody(api.CreateShareRequest{TargetType: "project", TargetID: "prj_1"}), cookie))

	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID, nil, cookie))
	testutil.AssertStatus(t, resp, http.StatusNoContent)

	// Share should still be retrievable with revoked_at set
	resp2, _ := env.App.Test(testutil.AuthRequest(http.MethodGet, "/api/v1/shares/"+sh.ID, nil, cookie))
	testutil.AssertStatus(t, resp2, http.StatusOK)
	var got api.ShareResponse
	testutil.DecodeJSON(t, resp2, &got)
	if got.RevokedAt == nil {
		t.Error("expected revoked_at to be set after DELETE")
	}
}

func TestRevokeShare_OtherWorkspace(t *testing.T) {
	env, _ := shareEnv(t)
	cookie := env.MintCookie(t, "usr_other", "ws_other")

	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodDelete, "/api/v1/shares/shr_1", nil, cookie))
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestListShares_IncludesIsExpired(t *testing.T) {
	env := testutil.NewTestEnv(t)
	pastExpiry := "2020-01-01T00:00:00Z"
	env.Shares.ListFn = func(_ context.Context, _ string) ([]*service.ShareDTO, error) {
		return []*service.ShareDTO{
			fixtures.Share(func(s *service.ShareDTO) { s.ExpiresAt = &pastExpiry }),
		}, nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, _ := env.App.Test(testutil.AuthRequest(http.MethodGet, "/api/v1/shares", nil, cookie))
	var items []api.ShareResponse
	testutil.DecodeJSON(t, resp, &items)

	if len(items) != 1 {
		t.Fatalf("expected 1 share, got %d", len(items))
	}
	if !items[0].IsExpired {
		t.Error("expected is_expired = true for past expires_at")
	}
}
