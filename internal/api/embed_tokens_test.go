//go:build integration

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/apperr"
	"damask/server/internal/auth"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

const embedTestAssetID = "ast_1"
const embedTestWorkspaceID = "ws_1"

// embedTokenEnv wires a testutil env with an asset "ast_1" in "ws_1" and an
// embed token store that tracks at most one active token, matching the
// partial-unique-index invariant enforced by the real repository.
func embedTokenEnv(t *testing.T) *testutil.TestEnv {
	t.Helper()
	env := testutil.NewTestEnv(t)

	env.Assets.GetFn = func(_ context.Context, wsID, id string) (*service.AssetDTO, error) {
		if wsID != embedTestWorkspaceID || id != embedTestAssetID {
			return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		return &service.AssetDTO{ID: id, WorkspaceID: wsID}, nil
	}

	var active *service.EmbedTokenDTO
	env.EmbedTokens.GetOrCreateFn = func(
		_ context.Context,
		wsID, assetID, _ string,
	) (*service.EmbedTokenDTO, error) {
		if active == nil {
			active = &service.EmbedTokenDTO{
				ID:        "tok_1",
				AssetID:   assetID,
				PublicURL: "https://app.example.com/e/tok_1",
				ThumbURL:  "https://app.example.com/e/tok_1/thumb",
				CreatedAt: time.Now(),
			}
		}
		return active, nil
	}
	env.EmbedTokens.GetActiveFn = func(_ context.Context, _, _ string) (*service.EmbedTokenDTO, error) {
		if active == nil {
			return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		return active, nil
	}
	env.EmbedTokens.RevokeFn = func(_ context.Context, _, _ string) error {
		if active == nil {
			return fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		active = nil
		return nil
	}
	return env
}

// --- POST /api/v1/assets/:id/embed-token ---

func TestCreateEmbedToken_CreatesToken(t *testing.T) {
	env := embedTokenEnv(t)
	cookie := env.MintCookie(t, "usr_1", embedTestWorkspaceID)

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body api.EmbedTokenResponse
	testutil.DecodeJSON(t, resp, &body)
	if body.PublicURL == "" {
		t.Error("expected non-empty public_url")
	}
	if body.AssetID != embedTestAssetID {
		t.Errorf("asset_id = %q, want %q", body.AssetID, embedTestAssetID)
	}
}

func TestCreateEmbedToken_IsIdempotent(t *testing.T) {
	env := embedTokenEnv(t)
	cookie := env.MintCookie(t, "usr_1", embedTestWorkspaceID)

	req1 := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	resp1, _ := env.App.Test(req1)
	var first api.EmbedTokenResponse
	testutil.DecodeJSON(t, resp1, &first)

	req2 := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	resp2, _ := env.App.Test(req2)
	var second api.EmbedTokenResponse
	testutil.DecodeJSON(t, resp2, &second)

	if first.ID != second.ID {
		t.Errorf("expected idempotent token id, got %q then %q", first.ID, second.ID)
	}
}

func TestCreateEmbedToken_RequiresAuth(t *testing.T) {
	env := embedTokenEnv(t)

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, nil)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestCreateEmbedToken_RequiresEditorRole(t *testing.T) {
	env := embedTokenEnv(t)
	env.Workspace.GetMemberFn = func(_ context.Context, _, uID string) (*service.MemberDTO, error) {
		return &service.MemberDTO{UserID: uID, Role: string(auth.Viewer)}, nil
	}
	token := env.MintToken(t, "usr_viewer", embedTestWorkspaceID)

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, token)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestCreateEmbedToken_Returns404ForUnknownAsset(t *testing.T) {
	env := embedTokenEnv(t)
	cookie := env.MintCookie(t, "usr_1", embedTestWorkspaceID)

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/ast_unknown/embed-token", nil, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestCreateEmbedToken_RejectsAssetFromOtherWorkspace(t *testing.T) {
	env := embedTokenEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_other")

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

// --- GET /api/v1/assets/:id/embed-token ---

func TestGetEmbedToken_Returns200WhenActive(t *testing.T) {
	env := embedTokenEnv(t)
	cookie := env.MintCookie(t, "usr_1", embedTestWorkspaceID)

	createReq := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	env.App.Test(createReq) //nolint:errcheck // setup call

	getReq := testutil.AuthRequest(http.MethodGet, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	resp, _ := env.App.Test(getReq)
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestGetEmbedToken_Returns404WhenNoneActive(t *testing.T) {
	env := embedTokenEnv(t)
	cookie := env.MintCookie(t, "usr_1", embedTestWorkspaceID)

	req := testutil.AuthRequest(http.MethodGet, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

// --- DELETE /api/v1/assets/:id/embed-token ---

func TestDeleteEmbedToken_Revokes(t *testing.T) {
	env := embedTokenEnv(t)
	cookie := env.MintCookie(t, "usr_1", embedTestWorkspaceID)

	createReq := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	env.App.Test(createReq) //nolint:errcheck // setup call

	delReq := testutil.AuthRequest(http.MethodDelete, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	resp, _ := env.App.Test(delReq)
	testutil.AssertStatus(t, resp, http.StatusNoContent)

	getReq := testutil.AuthRequest(http.MethodGet, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	getResp, _ := env.App.Test(getReq)
	testutil.AssertStatus(t, getResp, http.StatusNotFound)
}

func TestDeleteEmbedToken_Returns404WhenNoneActive(t *testing.T) {
	env := embedTokenEnv(t)
	cookie := env.MintCookie(t, "usr_1", embedTestWorkspaceID)

	req := testutil.AuthRequest(http.MethodDelete, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestDeleteEmbedToken_RequiresEditorRole(t *testing.T) {
	env := embedTokenEnv(t)
	env.Workspace.GetMemberFn = func(_ context.Context, _, uID string) (*service.MemberDTO, error) {
		return &service.MemberDTO{UserID: uID, Role: string(auth.Viewer)}, nil
	}
	token := env.MintToken(t, "usr_viewer", embedTestWorkspaceID)

	req := testutil.BearerRequest(http.MethodDelete, "/api/v1/assets/"+embedTestAssetID+"/embed-token", nil, token)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}
