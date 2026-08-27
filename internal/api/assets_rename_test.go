//go:build integration

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/apperr"
	"damask/server/internal/auth"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
	"damask/server/internal/testutil/fixtures"
)

func TestRenameAsset_Success(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Assets.RenameFn = func(_ context.Context, _, _, _ string) (*service.AssetDTO, error) {
		return fixtures.Asset(func(a *service.AssetDTO) { a.OriginalFilename = "renamed.jpg" }), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPut, "/api/v1/assets/ast_1/rename",
		testutil.JSONBody(api.RenameAssetRequest{Name: "renamed"}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusOK)

	var asset api.AssetResponse
	testutil.DecodeJSON(t, resp, &asset)
	if asset.OriginalFilename != "renamed.jpg" {
		t.Errorf("filename = %q, want renamed.jpg", asset.OriginalFilename)
	}
}

func TestRenameAsset_Unauthenticated(t *testing.T) {
	env := testutil.NewTestEnv(t)

	req := testutil.AuthRequest(http.MethodPut, "/api/v1/assets/ast_1/rename",
		testutil.JSONBody(api.RenameAssetRequest{Name: "new"}), nil)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestRenameAsset_ViewerForbidden(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetMemberFn = func(_ context.Context, _, uID string) (*service.MemberDTO, error) {
		return &service.MemberDTO{UserID: uID, Role: string(auth.Viewer)}, nil
	}
	token := env.MintToken(t, "usr_viewer", "ws_1")

	req := testutil.BearerRequest(http.MethodPut, "/api/v1/assets/ast_1/rename",
		testutil.JSONBody(api.RenameAssetRequest{Name: "new"}), token)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestRenameAsset_EmptyName(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPut, "/api/v1/assets/ast_1/rename",
		testutil.JSONBody(api.RenameAssetRequest{Name: "   "}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestRenameAsset_NotFound(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Assets.RenameFn = func(_ context.Context, _, _, _ string) (*service.AssetDTO, error) {
		return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPut, "/api/v1/assets/nonexistent/rename",
		testutil.JSONBody(api.RenameAssetRequest{Name: "new"}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}
