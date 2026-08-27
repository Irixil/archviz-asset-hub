//go:build integration

package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"damask/server/internal/api"
	th "damask/server/internal/testhelpers"
	"damask/server/internal/visualsimilarity"

	"github.com/gofiber/fiber/v3"
)

func uploadNamedJPEG(t *testing.T, env *th.TestEnv, cookie *http.Cookie, filename string, form map[string]string) api.AssetResponse {
	t.Helper()
	req := th.BuildUploadRequest(t, filename, th.MakeJPEG(32, 32), cookie, form)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload %s: %v", filename, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload %s: expected 201, got %d: %s", filename, resp.StatusCode, body)
	}
	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode upload: %v", err)
	}
	return asset
}

func currentVersionID(t *testing.T, env *th.TestEnv, assetID string) string {
	t.Helper()
	var id string
	if err := env.Database.QueryRowContext(t.Context(), `SELECT current_version_id FROM assets WHERE id = ?`, assetID).Scan(&id); err != nil {
		t.Fatalf("current version for %s: %v", assetID, err)
	}
	return id
}

func seedSimilarityHash(t *testing.T, env *th.TestEnv, workspaceID, versionID string, central uint64, hashSet []uint64) {
	t.Helper()
	encoded, err := visualsimilarity.MarshalHashSet(hashSet)
	if err != nil {
		t.Fatalf("marshal hash set: %v", err)
	}
	_, err = env.Database.ExecContext(
		t.Context(),
		`INSERT INTO asset_visual_similarity_hashes (asset_version_id, workspace_id, central_hash, hash_set)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(asset_version_id) DO UPDATE SET central_hash = excluded.central_hash, hash_set = excluded.hash_set`,
		versionID,
		workspaceID,
		int64(central),
		encoded,
	)
	if err != nil {
		t.Fatalf("seed similarity hash: %v", err)
	}
}

func listAssets(t *testing.T, env *th.TestEnv, cookie *http.Cookie, rawQuery string) (int, api.AssetListResponse) {
	t.Helper()
	path := "/api/v1/assets"
	if rawQuery != "" {
		path += "?" + rawQuery
	}
	req := th.AuthRequest(http.MethodGet, path, nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	defer resp.Body.Close()
	var body api.AssetListResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func seedSimilarSet(t *testing.T, env *th.TestEnv, workspaceID string, anchor api.AssetResponse, matches ...api.AssetResponse) {
	t.Helper()
	anchorVersionID := currentVersionID(t, env, anchor.ID)
	seedSimilarityHash(t, env, workspaceID, anchorVersionID, 900, []uint64{101, 102, 103})
	for i, match := range matches {
		seedSimilarityHash(t, env, workspaceID, currentVersionID(t, env, match.ID), uint64(101+i), []uint64{uint64(101 + i)})
	}
}

func TestListAssets_SimilarToPinsAnchorAndReturnsMatches(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	anchor := uploadNamedJPEG(t, env, owner.Cookie, "hero.jpg", nil)
	match := uploadNamedJPEG(t, env, owner.Cookie, "match.jpg", nil)
	other := uploadNamedJPEG(t, env, owner.Cookie, "other.jpg", nil)
	seedSimilarSet(t, env, owner.WorkspaceID, anchor, match)

	code, body := listAssets(t, env, owner.Cookie, "similar_to="+url.QueryEscape(anchor.ID))
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body.Similarity == nil || body.Similarity.AnchorAssetID != anchor.ID || body.Similarity.ResultCount != 1 {
		t.Fatalf("unexpected similarity meta: %#v", body.Similarity)
	}
	if len(body.Assets) != 2 || body.Assets[0].ID != anchor.ID || body.Assets[1].ID != match.ID {
		t.Fatalf("expected anchor then match, got %#v", body.Assets)
	}
	for _, asset := range body.Assets {
		if asset.ID == other.ID {
			t.Fatalf("unrelated asset %s should not be returned", other.ID)
		}
	}
}

func TestListAssets_SimilarToIntersectsFolderFilter(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	project := createProject(t, env, owner.Cookie, "Shoot", "#6366f1")
	folder := createFolder(t, env, owner.Cookie, project.ID, "Set A", nil)
	anchor := uploadNamedJPEG(t, env, owner.Cookie, "hero.jpg", nil)
	inFolder := uploadNamedJPEG(t, env, owner.Cookie, "in-folder.jpg", map[string]string{"folder_id": folder.ID})
	outside := uploadNamedJPEG(t, env, owner.Cookie, "outside.jpg", nil)
	seedSimilarSet(t, env, owner.WorkspaceID, anchor, inFolder, outside)

	q := fmt.Sprintf("similar_to=%s&folder_id=%s", url.QueryEscape(anchor.ID), url.QueryEscape(folder.ID))
	code, body := listAssets(t, env, owner.Cookie, q)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body.Similarity == nil || body.Similarity.ResultCount != 1 {
		t.Fatalf("unexpected similarity meta: %#v", body.Similarity)
	}
	if len(body.Assets) != 2 || body.Assets[0].ID != anchor.ID || body.Assets[1].ID != inFolder.ID {
		t.Fatalf("expected anchor then in-folder match, got %#v", body.Assets)
	}
}

func TestListAssets_SimilarToIntersectsTagFilter(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	anchor := uploadNamedJPEG(t, env, owner.Cookie, "hero.jpg", nil)
	tagged := uploadNamedJPEG(t, env, owner.Cookie, "tagged.jpg", nil)
	untagged := uploadNamedJPEG(t, env, owner.Cookie, "untagged.jpg", nil)
	addTag(t, env, owner.Cookie, tagged.ID, "campaign")
	seedSimilarSet(t, env, owner.WorkspaceID, anchor, tagged, untagged)

	code, body := listAssets(t, env, owner.Cookie, "similar_to="+url.QueryEscape(anchor.ID)+"&tags=campaign")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(body.Assets) != 2 || body.Assets[0].ID != anchor.ID || body.Assets[1].ID != tagged.ID {
		t.Fatalf("expected anchor then tagged match, got %#v", body.Assets)
	}
}

func TestListAssets_SimilarToRejectsNonImageAndUnknownAnchor(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	req := th.BuildUploadRequest(t, "readme.txt", []byte("hello"), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload text: %v", err)
	}
	defer resp.Body.Close()
	var textAsset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&textAsset); err != nil {
		t.Fatalf("decode text asset: %v", err)
	}

	req = th.AuthRequest(http.MethodGet, "/api/v1/assets?similar_to="+url.QueryEscape(textAsset.ID), nil, owner.Cookie)
	resp, err = env.App.Test(req)
	if err != nil {
		t.Fatalf("non-image request: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}

	req = th.AuthRequest(http.MethodGet, "/api/v1/assets?similar_to=does-not-exist", nil, owner.Cookie)
	resp, err = env.App.Test(req)
	if err != nil {
		t.Fatalf("unknown request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListAssets_SimilarToNotIndexed(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	anchor := uploadNamedJPEG(t, env, owner.Cookie, "hero.jpg", nil)

	code, body := listAssets(t, env, owner.Cookie, "similar_to="+url.QueryEscape(anchor.ID))
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if !body.SimilarToNoMatches {
		t.Fatal("expected similar_to_no_matches")
	}
	if body.Similarity == nil || body.Similarity.ResultCount != 0 {
		t.Fatalf("unexpected similarity meta: %#v", body.Similarity)
	}
	if len(body.Assets) != 1 || body.Assets[0].ID != anchor.ID {
		t.Fatalf("expected pinned anchor only, got %#v", body.Assets)
	}
}

func TestListAssets_SimilarToDeduplicatesAnchor(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	anchor := uploadNamedJPEG(t, env, owner.Cookie, "hero.jpg", nil)
	anchorVersionID := currentVersionID(t, env, anchor.ID)
	seedSimilarityHash(t, env, owner.WorkspaceID, anchorVersionID, 101, []uint64{101})

	code, body := listAssets(t, env, owner.Cookie, "similar_to="+url.QueryEscape(anchor.ID))
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	count := 0
	for _, asset := range body.Assets {
		if strings.EqualFold(asset.ID, anchor.ID) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected anchor once, got %d occurrences in %#v", count, body.Assets)
	}
}
