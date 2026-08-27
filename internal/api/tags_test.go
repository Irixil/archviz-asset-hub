//go:build integration

package api_test

import (
	"damask/server/internal/api"
	"damask/server/internal/auth"
	th "damask/server/internal/testhelpers"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// addTag is a test helper that adds a tag to an asset.
func addTag(t *testing.T, env *th.TestEnv, cookie *http.Cookie, assetID, name string) int {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q}`, name)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/tags", strings.NewReader(body), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("add tag: %v", err)
	}
	return resp.StatusCode
}

func TestAddTag_AutoCreates(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	code := addTag(t, env, owner.Cookie, assetID, "summer")
	if code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", code)
	}

	// Tag should appear in /api/v1/tags
	req := th.AuthRequest(http.MethodGet, "/api/v1/tags", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var tags []api.TagResponse
	_ = json.NewDecoder(resp.Body).Decode(&tags)
	if len(tags) != 1 || tags[0].Name != "summer" {
		t.Errorf("expected tag 'summer', got %+v", tags)
	}
}

func TestAddTag_Idempotent(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, assetID, "beach")
	addTag(t, env, owner.Cookie, assetID, "beach") // duplicate — should not error

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/tags", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var names []string
	_ = json.NewDecoder(resp.Body).Decode(&names)
	if len(names) != 1 {
		t.Errorf("expected 1 tag after duplicate add, got %d", len(names))
	}
}

func TestRemoveTag_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, assetID, "travel")

	req := th.AuthRequest(http.MethodDelete, "/api/v1/assets/"+assetID+"/tags/travel", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Confirm removed
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/tags", nil, owner.Cookie)
	resp2, _ := env.App.Test(req2)
	var names []string
	_ = json.NewDecoder(resp2.Body).Decode(&names)
	if len(names) != 0 {
		t.Errorf("expected 0 tags after removal, got %d", len(names))
	}
}

func TestListTags_WithCounts(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, a1, "nature")
	addTag(t, env, owner.Cookie, a2, "nature")
	addTag(t, env, owner.Cookie, a1, "sunset")

	req := th.AuthRequest(http.MethodGet, "/api/v1/tags", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var tags []api.TagResponse
	_ = json.NewDecoder(resp.Body).Decode(&tags)

	counts := map[string]int64{}
	for _, tag := range tags {
		counts[tag.Name] = tag.AssetCount
	}
	if counts["nature"] != 2 {
		t.Errorf("nature count = %d, want 2", counts["nature"])
	}
	if counts["sunset"] != 1 {
		t.Errorf("sunset count = %d, want 1", counts["sunset"])
	}
}

func TestListTags_DefaultExcludesSystemTags(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, assetID, "_watermark")

	req := th.AuthRequest(http.MethodGet, "/api/v1/tags", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var tags []api.TagResponse
	_ = json.NewDecoder(resp.Body).Decode(&tags)
	if len(tags) != 0 {
		t.Fatalf("expected system tags to be excluded by default, got %+v", tags)
	}
}

func TestListTags_SystemTrue_IncludesSystemTags(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, assetID, "_watermark")

	req := th.AuthRequest(http.MethodGet, "/api/v1/tags?system=true", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var tags []api.TagResponse
	_ = json.NewDecoder(resp.Body).Decode(&tags)
	if len(tags) != 1 || tags[0].Name != "_watermark" {
		t.Fatalf("expected _watermark in response, got %+v", tags)
	}
	if !tags[0].IsSystem {
		t.Fatalf("expected is_system=true, got %+v", tags[0])
	}
}

func TestAddTag_SystemTag_EnsuresTagRow(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	code := addTag(t, env, owner.Cookie, assetID, "_watermark")
	if code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", code)
	}

	var groupName string
	if err := env.Database.QueryRow(`SELECT group_name FROM tags WHERE workspace_id = ? AND name = ?`, owner.WorkspaceID, "_watermark").Scan(&groupName); err != nil {
		t.Fatalf("load tag row: %v", err)
	}
	if groupName != "system" {
		t.Fatalf("expected group_name=system, got %q", groupName)
	}
}

func TestGetAssetTags(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, assetID, "alpha")
	addTag(t, env, owner.Cookie, assetID, "beta")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/tags", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var names []string
	_ = json.NewDecoder(resp.Body).Decode(&names)

	if len(names) != 2 {
		t.Errorf("expected 2 tags, got %d", len(names))
	}
}

func TestGetAsset_IncludesTags(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, assetID, "coast")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	if len(a.Tags) != 1 || a.Tags[0] != "coast" {
		t.Errorf("expected tags=[coast], got %v", a.Tags)
	}
}

func TestListAssets_FilterBySingleTag(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, a1, "forest")
	addTag(t, env, owner.Cookie, a2, "city")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?tags=forest", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var result api.AssetListResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Assets) != 1 {
		t.Fatalf("expected 1 asset for tag=forest, got %d", len(result.Assets))
	}
	if result.Assets[0].ID != a1 {
		t.Errorf("wrong asset returned: got %s, want %s", result.Assets[0].ID, a1)
	}
}

func TestListAssets_FilterByMultiTagAND(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	// a1 has both tags
	addTag(t, env, owner.Cookie, a1, "red")
	addTag(t, env, owner.Cookie, a1, "blue")
	// a2 only has red
	addTag(t, env, owner.Cookie, a2, "red")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?tags=red,blue", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var result api.AssetListResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Assets) != 1 {
		t.Fatalf("expected 1 asset with both tags, got %d", len(result.Assets))
	}
	if result.Assets[0].ID != a1 {
		t.Errorf("wrong asset: got %s, want %s", result.Assets[0].ID, a1)
	}
}

func TestBulkTag(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	body := fmt.Sprintf(`{"asset_ids":[%q,%q],"tag_name":"promo"}`, a1, a2)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/tag", strings.NewReader(body), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify both have the tag
	for _, id := range []string{a1, a2} {
		req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+id+"/tags", nil, owner.Cookie)
		resp2, _ := env.App.Test(req2)
		var names []string
		_ = json.NewDecoder(resp2.Body).Decode(&names)
		if len(names) != 1 || names[0] != "promo" {
			t.Errorf("asset %s: expected tag promo, got %v", id, names)
		}
	}
}

func TestBulkProject(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	p := createProject(t, env, owner.Cookie, "Batch", "#abcdef")

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	body := fmt.Sprintf(`{"asset_ids":[%q,%q],"project_id":%q}`, a1, a2, p.ID)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/project", strings.NewReader(body), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify assets are assigned
	for _, id := range []string{a1, a2} {
		req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+id, nil, owner.Cookie)
		resp2, _ := env.App.Test(req2)
		var a api.AssetResponse
		_ = json.NewDecoder(resp2.Body).Decode(&a)
		if a.ProjectID == nil || *a.ProjectID != p.ID {
			t.Errorf("asset %s: expected project_id=%s, got %v", id, p.ID, a.ProjectID)
		}
	}
}

// ── handleCreateTag ──────────────────────────────────────────────────────────

func TestCreateTag_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	body := `{"name":"landscape","color":"#22c55e","group_name":"nature"}`
	req := th.AuthRequest(http.MethodPost, "/api/v1/tags", strings.NewReader(body), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var tag api.TagResponse
	_ = json.NewDecoder(resp.Body).Decode(&tag)
	if tag.Name != "landscape" {
		t.Errorf("name = %q, want landscape", tag.Name)
	}
	if tag.Color == nil || *tag.Color != "#22c55e" {
		t.Errorf("color = %v, want #22c55e", tag.Color)
	}
	if tag.GroupName == nil || *tag.GroupName != "nature" {
		t.Errorf("group_name = %v, want nature", tag.GroupName)
	}
	if tag.AssetCount != 0 {
		t.Errorf("asset_count = %d, want 0", tag.AssetCount)
	}
}

func TestCreateTag_Conflict(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	body := `{"name":"duplicate"}`
	req := th.AuthRequest(http.MethodPost, "/api/v1/tags", strings.NewReader(body), owner.Cookie)
	_, _ = env.App.Test(req)

	req2 := th.AuthRequest(http.MethodPost, "/api/v1/tags", strings.NewReader(`{"name":"duplicate"}`), owner.Cookie)
	resp, _ := env.App.Test(req2)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestCreateTag_InvalidColor(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	body := `{"name":"oops","color":"notacolor"}`
	req := th.AuthRequest(http.MethodPost, "/api/v1/tags", strings.NewReader(body), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateTag_MissingName(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodPost, "/api/v1/tags", strings.NewReader(`{}`), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateTag_RequiresEditor(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Viewer token cannot create tags
	viewerToken, _ := env.Maker.CreateToken("viewer-user", owner.WorkspaceID, 0)
	_, _ = env.Database.Exec(
		`INSERT INTO users (id, name, email, password_hash) VALUES ('viewer-user','V','v@test.com','x')`,
	)
	_, _ = env.Database.Exec(
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES (?, 'viewer-user', 'viewer')`,
		owner.WorkspaceID,
	)
	_ = viewerToken

	editorToken := th.MintEditorToken(t, env, owner.WorkspaceID, auth.Viewer)
	req := th.BearerRequest(http.MethodPost, "/api/v1/tags", strings.NewReader(`{"name":"x"}`), editorToken)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer, got %d", resp.StatusCode)
	}
}

func TestCreateTag_Unauthenticated(t *testing.T) {
	env, _ := th.SetupWithOwner(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/tags", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ── handlePatchTag ────────────────────────────────────────────────────────────

func TestPatchTag_Rename(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, assetID, "old-name")

	body := `{"name":"new-name"}`
	req := th.AuthRequest(http.MethodPatch, "/api/v1/tags/old-name", strings.NewReader(body), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var tag api.TagResponse
	_ = json.NewDecoder(resp.Body).Decode(&tag)
	if tag.Name != "new-name" {
		t.Errorf("name = %q, want new-name", tag.Name)
	}
}

func TestPatchTag_UpdateColor(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, assetID, "mytag")

	body := `{"color":"#ff0000"}`
	req := th.AuthRequest(http.MethodPatch, "/api/v1/tags/mytag", strings.NewReader(body), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var tag api.TagResponse
	_ = json.NewDecoder(resp.Body).Decode(&tag)
	if tag.Color == nil || *tag.Color != "#ff0000" {
		t.Errorf("color = %v, want #ff0000", tag.Color)
	}
}

func TestPatchTag_NotFound(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodPatch, "/api/v1/tags/nonexistent", strings.NewReader(`{"name":"x"}`), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestPatchTag_RenameConflict(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	a := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, a, "tagA")
	addTag(t, env, owner.Cookie, a, "tagB")

	req := th.AuthRequest(http.MethodPatch, "/api/v1/tags/tagA", strings.NewReader(`{"name":"tagB"}`), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestPatchTag_InvalidColor(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	a := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, a, "tag1")

	req := th.AuthRequest(http.MethodPatch, "/api/v1/tags/tag1", strings.NewReader(`{"color":"bad"}`), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestPatchTag_RenameSystemTag_Returns422(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, assetID, "_watermark")

	req := th.AuthRequest(http.MethodPatch, "/api/v1/tags/_watermark", strings.NewReader(`{"name":"other"}`), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

// ── handleBulkDeleteTags ─────────────────────────────────────────────────────

func TestBulkDeleteTags_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	a := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, a, "alpha")
	addTag(t, env, owner.Cookie, a, "beta")

	body := `{"names":["alpha","beta"]}`
	req := th.AuthRequest(http.MethodDelete, "/api/v1/tags", strings.NewReader(body), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]int
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["deleted"] != 2 {
		t.Errorf("deleted = %d, want 2", result["deleted"])
	}

	// Tags should be gone
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/tags", nil, owner.Cookie)
	resp2, _ := env.App.Test(req2)
	var tags []api.TagResponse
	_ = json.NewDecoder(resp2.Body).Decode(&tags)
	if len(tags) != 0 {
		t.Errorf("expected 0 tags after bulk delete, got %d", len(tags))
	}
}

func TestBulkDeleteTags_SkipsUnknown(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	a := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, a, "real")

	body := `{"names":["real","doesnotexist"]}`
	req := th.AuthRequest(http.MethodDelete, "/api/v1/tags", strings.NewReader(body), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]int
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["deleted"] != 1 {
		t.Errorf("deleted = %d, want 1", result["deleted"])
	}
}

func TestBulkDeleteTags_EmptyNames(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodDelete, "/api/v1/tags", strings.NewReader(`{"names":[]}`), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestBulkDeleteTags_RemovesAssetAssociations(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	a := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, a, "removeme")

	req := th.AuthRequest(http.MethodDelete, "/api/v1/tags", strings.NewReader(`{"names":["removeme"]}`), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]int
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["removed_from_assets"] != 1 {
		t.Errorf("removed_from_assets = %d, want 1", result["removed_from_assets"])
	}
}

func TestBulkDeleteTags_SystemTagInList_Returns422(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	addTag(t, env, owner.Cookie, assetID, "_watermark")

	req := th.AuthRequest(http.MethodDelete, "/api/v1/tags", strings.NewReader(`{"names":["_watermark"]}`), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

// ── handleMergeTags ───────────────────────────────────────────────────────────

func TestMergeTags_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, a1, "src1")
	addTag(t, env, owner.Cookie, a2, "src2")

	body := `{"sources":["src1","src2"],"target":"merged"}`
	req := th.AuthRequest(http.MethodPost, "/api/v1/tags/merge", strings.NewReader(body), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		MergedAssets int64           `json:"merged_assets"`
		Target       api.TagResponse `json:"target"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Target.Name != "merged" {
		t.Errorf("target name = %q, want merged", result.Target.Name)
	}
	if result.Target.AssetCount != 2 {
		t.Errorf("target asset_count = %d, want 2", result.Target.AssetCount)
	}

	// Source tags should be deleted
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/tags", nil, owner.Cookie)
	resp2, _ := env.App.Test(req2)
	var tags []api.TagResponse
	_ = json.NewDecoder(resp2.Body).Decode(&tags)
	for _, tag := range tags {
		if tag.Name == "src1" || tag.Name == "src2" {
			t.Errorf("source tag %q should have been deleted after merge", tag.Name)
		}
	}
}

func TestMergeTags_TargetExists(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, a1, "old")
	addTag(t, env, owner.Cookie, a2, "existing")

	body := `{"sources":["old"],"target":"existing"}`
	req := th.AuthRequest(http.MethodPost, "/api/v1/tags/merge", strings.NewReader(body), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Target api.TagResponse `json:"target"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Target.AssetCount != 2 {
		t.Errorf("target asset_count = %d, want 2", result.Target.AssetCount)
	}
}

func TestMergeTags_MissingTarget(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodPost, "/api/v1/tags/merge", strings.NewReader(`{"sources":["x"],"target":""}`), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestMergeTags_MissingSources(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodPost, "/api/v1/tags/merge", strings.NewReader(`{"sources":[],"target":"x"}`), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

// ── handleTagDuplicateSuggestions ─────────────────────────────────────────────

func TestTagDuplicateSuggestions_FindsSimilar(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	a := uploadTestAsset(t, env, owner)

	// "colour" and "color" are close
	addTag(t, env, owner.Cookie, a, "colour")
	addTag(t, env, owner.Cookie, a, "color")

	req := th.AuthRequest(http.MethodGet, "/api/v1/tags/suggestions/duplicates", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var pairs []struct {
		A     string  `json:"a"`
		B     string  `json:"b"`
		Score float64 `json:"score"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&pairs)
	if len(pairs) == 0 {
		t.Error("expected at least one duplicate suggestion for 'colour'/'color'")
	}
}

func TestTagDuplicateSuggestions_EmptyWhenNoTags(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/tags/suggestions/duplicates", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var pairs []interface{}
	_ = json.NewDecoder(resp.Body).Decode(&pairs)
	if len(pairs) != 0 {
		t.Errorf("expected empty array, got %d items", len(pairs))
	}
}

func TestTagDuplicateSuggestions_Unauthenticated(t *testing.T) {
	env, _ := th.SetupWithOwner(t)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/tags/suggestions/duplicates", nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestBulkDelete(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	body := fmt.Sprintf(`{"asset_ids":[%q,%q]}`, a1, a2)
	req := th.AuthRequest(http.MethodDelete, "/api/v1/assets/bulk", strings.NewReader(body), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Both should be gone
	for _, id := range []string{a1, a2} {
		req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+id, nil, owner.Cookie)
		resp2, _ := env.App.Test(req2)
		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("asset %s: expected 404 after bulk delete, got %d", id, resp2.StatusCode)
		}
	}
}
