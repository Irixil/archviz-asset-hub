//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/apperr"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
	"damask/server/internal/testutil/fixtures"
)

func TestCollections_Create(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Collections.CreateFn = func(_ context.Context, _ string, p service.CreateCollectionParams) (*service.CollectionDTO, error) {
		return fixtures.Collection(func(c *service.CollectionDTO) { c.Name = p.Name }), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/collections",
		testutil.JSONBody(map[string]any{"name": "My Collection"}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusCreated)

	var col api.CollectionResponse
	testutil.DecodeJSON(t, resp, &col)
	if col.Name != "My Collection" {
		t.Errorf("name = %q, want My Collection", col.Name)
	}
}

func TestCollections_CreateWithAssets(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Assets.CountByIDsFn = func(_ context.Context, _ string, ids []string) (int64, error) {
		return int64(len(ids)), nil
	}
	env.Collections.CreateFn = func(_ context.Context, _ string, p service.CreateCollectionParams) (*service.CollectionDTO, error) {
		return fixtures.Collection(func(c *service.CollectionDTO) {
			c.Name = p.Name
			c.AssetCount = int64(len(p.AssetIDs))
		}), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/collections",
		testutil.JSONBody(map[string]any{"name": "Stack Save", "asset_ids": []string{"ast_1"}}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusCreated)

	var col api.CollectionResponse
	testutil.DecodeJSON(t, resp, &col)
	if col.AssetCount != 1 {
		t.Errorf("asset_count = %d, want 1", col.AssetCount)
	}
}

func TestCollections_List(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Collections.ListFn = func(_ context.Context, _ string) ([]*service.CollectionDTO, error) {
		return []*service.CollectionDTO{
			fixtures.Collection(func(c *service.CollectionDTO) { c.ID = "col_1"; c.Name = "Alpha" }),
			fixtures.Collection(func(c *service.CollectionDTO) { c.ID = "col_2"; c.Name = "Beta" }),
		}, nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodGet, "/api/v1/collections", nil, cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var cols []api.CollectionResponse
	testutil.DecodeJSON(t, resp, &cols)
	if len(cols) != 2 {
		t.Errorf("expected 2 collections, got %d", len(cols))
	}
}

func TestCollections_Get(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Collections.GetFn = func(_ context.Context, _, id string) (*service.CollectionDTO, error) {
		return fixtures.Collection(func(c *service.CollectionDTO) { c.ID = id }), nil
	}
	env.Collections.ListAssetsFn = func(_ context.Context, _, _ string) ([]*service.AssetDTO, error) {
		return []*service.AssetDTO{fixtures.Asset()}, nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodGet, "/api/v1/collections/col_1", nil, cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var detail struct {
		api.CollectionResponse
		Assets []api.AssetResponse `json:"assets"`
	}
	testutil.DecodeJSON(t, resp, &detail)
	if len(detail.Assets) != 1 {
		t.Errorf("expected 1 asset, got %d", len(detail.Assets))
	}
}

func TestCollections_Update(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Collections.UpdateFn = func(_ context.Context, _, id string, p service.UpdateCollectionParams) (*service.CollectionDTO, error) {
		return fixtures.Collection(func(c *service.CollectionDTO) {
			c.ID = id
			if p.Name != nil {
				c.Name = *p.Name
			}
		}), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPut, "/api/v1/collections/col_1",
		testutil.JSONBody(map[string]any{"name": "After", "description": "desc"}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var col api.CollectionResponse
	testutil.DecodeJSON(t, resp, &col)
	if col.Name != "After" {
		t.Errorf("name = %q, want After", col.Name)
	}
}

func TestCollections_Delete(t *testing.T) {
	env := testutil.NewTestEnv(t)
	notFound := false
	env.Collections.DeleteFn = func(_ context.Context, _, _ string) error {
		notFound = true
		return nil
	}
	env.Collections.GetFn = func(_ context.Context, _, _ string) (*service.CollectionDTO, error) {
		if notFound {
			return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		return fixtures.Collection(), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodDelete, "/api/v1/collections/col_1", nil, cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusNoContent)

	getResp, _ := env.App.Test(testutil.AuthRequest(http.MethodGet, "/api/v1/collections/col_1", nil, cookie))
	testutil.AssertStatus(t, getResp, http.StatusNotFound)
}

func TestCollections_AddRemoveAsset(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Collections.AddAssetFn = func(_ context.Context, _, _, _ string) error { return nil }
	env.Collections.RemoveAssetFn = func(_ context.Context, _, _, _ string) error { return nil }
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	addResp, err := env.App.Test(testutil.AuthRequest(http.MethodPost,
		"/api/v1/collections/col_1/assets/ast_1", nil, cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, addResp, http.StatusNoContent)

	rmResp, err := env.App.Test(testutil.AuthRequest(http.MethodDelete,
		"/api/v1/collections/col_1/assets/ast_1", nil, cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, rmResp, http.StatusNoContent)
}

func TestCollections_CreateWithForeignAsset(t *testing.T) {
	env := testutil.NewTestEnv(t)
	// CountByIDs returns 0 — asset not in workspace
	env.Assets.CountByIDsFn = func(_ context.Context, _ string, _ []string) (int64, error) {
		return 0, nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/collections",
		testutil.JSONBody(map[string]any{"name": "Bad", "asset_ids": []string{"ast_foreign"}}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestCollections_CreateMixedOwnership(t *testing.T) {
	env := testutil.NewTestEnv(t)
	// Only 1 of 2 assets found in workspace
	env.Assets.CountByIDsFn = func(_ context.Context, _ string, _ []string) (int64, error) {
		return 1, nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/collections",
		testutil.JSONBody(map[string]any{"name": "Mixed", "asset_ids": []string{"ast_1", "ast_foreign"}}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestCollections_WorkspaceIsolation(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Collections.GetFn = func(_ context.Context, wsID, _ string) (*service.CollectionDTO, error) {
		if wsID != "ws_owner" {
			return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		return fixtures.Collection(), nil
	}
	// Authenticate as a user from a different workspace
	cookie := env.MintCookie(t, "usr_other", "ws_other")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodGet, "/api/v1/collections/col_1", nil, cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestCollections_Unauthenticated(t *testing.T) {
	env := testutil.NewTestEnv(t)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/collections", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

// Compile-time check that CollectionResponse is exported (used in test above).
var _ = json.Marshal
