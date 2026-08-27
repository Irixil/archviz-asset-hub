//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

// -- BulkFieldsPreview -------------------------------------------------------

func TestBulkFieldPreview_ReturnsOverwriteCounts(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "user_1", "ws_1")

	env.AssetFields.BulkPreviewFn = func(_ context.Context, workspaceID string, assetIDs, fieldIDs []string) ([]service.BulkPreviewEntry, error) {
		return []service.BulkPreviewEntry{
			{FieldID: "fld_1", FieldName: "Client", FieldType: "text", AssetsWithValue: 3, DistinctValues: []string{"Nike", "Adidas", "Puma"}},
		}, nil
	}

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/fields/preview",
		testutil.JSONStr(`{"asset_ids":["a1","a2","a3","a4","a5"],"field_ids":["fld_1"]}`),
		cookie)

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.BulkFieldsPreviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Fields) != 1 {
		t.Fatalf("expected 1 field entry, got %d", len(body.Fields))
	}
	if body.Fields[0].AssetsWithValue != 3 {
		t.Errorf("expected assets_with_value=3, got %d", body.Fields[0].AssetsWithValue)
	}
}

func TestBulkFieldPreview_ExcludesEmptyValues(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "user_1", "ws_1")

	env.AssetFields.BulkPreviewFn = func(_ context.Context, _ string, _, _ []string) ([]service.BulkPreviewEntry, error) {
		return []service.BulkPreviewEntry{
			{FieldID: "fld_1", FieldName: "Budget", FieldType: "number", AssetsWithValue: 0, DistinctValues: []string{}},
		}, nil
	}

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/fields/preview",
		testutil.JSONStr(`{"asset_ids":["a1","a2"]}`), cookie)

	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.BulkFieldsPreviewResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Fields[0].AssetsWithValue != 0 {
		t.Errorf("expected 0 assets_with_value, got %d", body.Fields[0].AssetsWithValue)
	}
}

func TestBulkFieldPreview_DistinctValuesCappedAt5(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "user_1", "ws_1")

	env.AssetFields.BulkPreviewFn = func(_ context.Context, _ string, _, _ []string) ([]service.BulkPreviewEntry, error) {
		return []service.BulkPreviewEntry{
			{
				FieldID: "fld_1", FieldName: "Tag", FieldType: "text",
				AssetsWithValue: 7,
				DistinctValues:  []string{"a", "b", "c", "d", "e", "+2 more"},
			},
		}, nil
	}

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/fields/preview",
		testutil.JSONStr(`{"asset_ids":["a1"]}`), cookie)

	resp, _ := env.App.Test(req)
	var body api.BulkFieldsPreviewResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)

	distinct := body.Fields[0].DistinctValues
	if len(distinct) != 6 {
		t.Fatalf("expected 6 entries (5 + overflow), got %d", len(distinct))
	}
	if distinct[5] != "+2 more" {
		t.Errorf("expected '+2 more', got %q", distinct[5])
	}
}

func TestBulkFieldPreview_RejectsViewer(t *testing.T) {
	env := testutil.NewTestEnv(t)
	// No workspace service override → getRoleFn returns Viewer, which should block Editor-gated route.
	// We test rejection by omitting the auth cookie entirely.
	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/fields/preview",
		testutil.JSONStr(`{"asset_ids":["a1"]}`), nil)

	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestBulkFieldPreview_RequiresAssetIDs(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "user_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/fields/preview",
		testutil.JSONStr(`{"asset_ids":[]}`), cookie)

	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

// -- BulkPatchAssetFields cleared count --------------------------------------

func TestBulkFieldClear_NullDeletesValues(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "user_1", "ws_1")

	env.AssetFields.BulkSetValuesFn = func(_ context.Context, _, _ string, _ []string, _ []service.SetFieldValueInput) (service.BulkSetValuesResult, error) {
		return service.BulkSetValuesResult{Updated: 1, Cleared: 1}, nil
	}

	req := testutil.AuthRequest(http.MethodPatch, "/api/v1/assets/bulk/fields",
		testutil.JSONStr(`{"asset_ids":["a1"],"values":[{"field_id":"fld_1","value":null}]}`),
		cookie)

	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.BulkPatchAssetFieldsResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Cleared != 1 {
		t.Errorf("expected cleared=1, got %d", body.Cleared)
	}
}

// -- BulkTag remove mode -----------------------------------------------------

func TestBulkTagRemove_RemovesFromAllAssets(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "user_1", "ws_1")

	called := false
	env.Assets.BulkRemoveTagFn = func(_ context.Context, workspaceID, tagName string, assetIDs []string) error {
		called = true
		if tagName != "urgent" {
			t.Errorf("expected tag 'urgent', got %q", tagName)
		}
		if len(assetIDs) != 2 {
			t.Errorf("expected 2 asset IDs, got %d", len(assetIDs))
		}
		return nil
	}

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/tag",
		testutil.JSONStr(`{"asset_ids":["a1","a2"],"tag_name":"urgent","mode":"remove"}`),
		cookie)

	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if !called {
		t.Error("BulkRemoveTag was not called")
	}
}

func TestBulkTagAdd_DefaultsToAddMode(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "user_1", "ws_1")

	addCalled := false
	env.Assets.BulkTagFn = func(_ context.Context, _, tagName string, assetIDs []string) error {
		addCalled = true
		if tagName != "urgent" {
			t.Errorf("expected tag 'urgent', got %q", tagName)
		}
		if len(assetIDs) != 1 {
			t.Errorf("expected 1 asset ID, got %d", len(assetIDs))
		}
		return nil
	}

	// Omit mode entirely — should default to add.
	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/tag",
		testutil.JSONStr(`{"asset_ids":["a1"],"tag_name":"urgent"}`),
		cookie)

	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if !addCalled {
		t.Error("BulkSetTag was not called when mode was omitted")
	}
}

// TestAssetList_IncludesTags verifies that the asset list endpoint returns tags
// populated via BatchTagsForAssets — the root cause of the bulk-tag display bug.
func TestAssetList_IncludesTags(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "user_1", "ws_1")

	env.Assets.ListFn = func(_ context.Context, _ service.ListAssetsParams) ([]*service.AssetDTO, error) {
		return []*service.AssetDTO{
			{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "photo.jpg", MimeType: "image/jpeg"},
			{ID: "a2", WorkspaceID: "ws_1", OriginalFilename: "video.mp4", MimeType: "video/mp4"},
		}, nil
	}
	env.Tags.BatchTagsForAssetsFn = func(_ context.Context, assetIDs []string) (map[string][]string, error) {
		return map[string][]string{
			"a1": {"urgent", "client-x"},
			// a2 intentionally has no tags
		}, nil
	}

	req := testutil.AuthRequest(http.MethodGet, "/api/v1/assets", nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Assets []struct {
			ID   string   `json:"id"`
			Tags []string `json:"tags"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(body.Assets))
	}

	a1 := body.Assets[0]
	if a1.ID != "a1" {
		t.Fatalf("expected a1 first, got %q", a1.ID)
	}
	if len(a1.Tags) != 2 {
		t.Errorf("expected 2 tags on a1, got %v", a1.Tags)
	}

	a2 := body.Assets[1]
	if len(a2.Tags) != 0 {
		t.Errorf("expected empty tags on a2, got %v", a2.Tags)
	}
}

func TestBulkTagRemove_InvalidModeRejected(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "user_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/tag",
		testutil.JSONStr(`{"asset_ids":["a1"],"tag_name":"urgent","mode":"upsert"}`),
		cookie)

	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid mode, got %d", resp.StatusCode)
	}
}
