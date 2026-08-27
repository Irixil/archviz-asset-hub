//go:build integration

package api_test

import (
	"bytes"
	"damask/server/internal/api"
	"damask/server/internal/auth"
	th "damask/server/internal/testhelpers"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// createFolder creates a folder in the given project via the HTTP API and returns its parsed response.
// Used by tests that still rely on testhelpers (th). Remove once assets_test.go and
// router_isolation_test.go are migrated to testutil.
func createFolder(t *testing.T, env *th.TestEnv, cookie *http.Cookie, projectID, name string, parentID *string) api.FolderResponse {
	t.Helper()
	req := th.AuthRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/folders",
		th.JSONBody(api.CreateFolderRequest{Name: name, ParentID: parentID}), cookie)
	res, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var out api.FolderResponse
	_ = json.Unmarshal(b, &out)
	return out
}

func TestUploadAsset_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	jpegData := th.MakeJPEG(200, 150)
	req := th.BuildUploadRequest(t, "photo.jpg", jpegData, owner.Cookie)

	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if asset.ID == "" {
		t.Error("expected non-empty asset ID")
	}
	if asset.OriginalFilename != "photo.jpg" {
		t.Errorf("filename: got %q, want %q", asset.OriginalFilename, "photo.jpg")
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		t.Errorf("mime_type: got %q, expected image prefix", asset.MimeType)
	}
	if asset.Size <= 0 {
		t.Error("expected positive size")
	}
	if asset.Width == nil || *asset.Width != 200 {
		t.Errorf("width: got %v, want 200", asset.Width)
	}
	if asset.Height == nil || *asset.Height != 150 {
		t.Errorf("height: got %v, want 150", asset.Height)
	}
}

func TestUploadAsset_InFolder(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Create a project
	projRes, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JSONBody(api.CreateProjectRequest{Name: "My Project"}), owner.Cookie))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	defer projRes.Body.Close()
	if projRes.StatusCode != http.StatusCreated {
		t.Fatalf("create project: got %d", projRes.StatusCode)
	}
	var proj api.ProjectResponse
	if err := json.NewDecoder(projRes.Body).Decode(&proj); err != nil {
		t.Fatalf("decode project: %v", err)
	}

	// Create a folder inside the project
	folder := createFolder(t, env, owner.Cookie, proj.ID, "Photos", nil)

	// Upload an asset into the folder
	req := th.BuildUploadRequest(t, "photo.jpg", th.MakeJPEG(10, 10), owner.Cookie,
		map[string]string{"folder_id": folder.ID})
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode asset: %v", err)
	}
	if asset.FolderID == nil || *asset.FolderID != folder.ID {
		t.Errorf("folder_id = %v, want %s", asset.FolderID, folder.ID)
	}
}

func TestUploadAsset_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := th.BuildUploadRequest(t, "file.jpg", th.MakeJPEG(10, 10), nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestUploadAsset_ViewerForbidden(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	viewerToken := th.MintEditorToken(t, env, owner.WorkspaceID, auth.Viewer)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, _ := w.CreateFormFile("file", "file.jpg")
	fw.Write(th.MakeJPEG(10, 10)) //nolint:errcheck
	err := w.Close()
	if err != nil {
		t.Fatalf("close form: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/assets", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+viewerToken)

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestListAssets_Empty(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result api.AssetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Assets) != 0 {
		t.Errorf("expected empty list, got %d assets", len(result.Assets))
	}
	if result.NextCursor != nil {
		t.Error("expected nil next_cursor for empty list")
	}
}

func TestListAssets_Pagination(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload 3 assets with small delays to ensure distinct created_at
	for i := range 3 {
		jpegData := th.MakeJPEG(10, 10)
		req := th.BuildUploadRequest(t, "img"+string(rune('a'+i))+".jpg", jpegData, owner.Cookie)
		resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %d: status %d err %v", i, resp.StatusCode, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Fetch with limit=2
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?limit=2", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}

	var page1 api.AssetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&page1); err != nil {
		t.Fatalf("decode page1: %v", err)
	}
	if len(page1.Assets) != 2 {
		t.Fatalf("expected 2 assets on page1, got %d", len(page1.Assets))
	}
	if page1.NextCursor == nil {
		t.Fatal("expected next_cursor on page1")
	}

	// Fetch page 2
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets?limit=2&cursor="+*page1.NextCursor, nil, owner.Cookie)
	resp2, err := env.App.Test(req2)
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}

	var page2 api.AssetListResponse
	if err := json.NewDecoder(resp2.Body).Decode(&page2); err != nil {
		t.Fatalf("decode page2: %v", err)
	}
	if len(page2.Assets) != 1 {
		t.Fatalf("expected 1 asset on page2, got %d", len(page2.Assets))
	}
	if page2.NextCursor != nil {
		t.Error("expected nil next_cursor on last page")
	}
}

func TestGetAsset(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	uploadReq := th.BuildUploadRequest(t, "test.jpg", th.MakeJPEG(50, 50), owner.Cookie)
	uploadResp, _ := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	var uploaded api.AssetResponse
	_ = json.NewDecoder(uploadResp.Body).Decode(&uploaded)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+uploaded.ID, nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var got api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.ID != uploaded.ID {
		t.Errorf("id mismatch: got %q want %q", got.ID, uploaded.ID)
	}
}

func TestGetAsset_NotFound(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/nonexistent", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetAssetFile(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	jpegData := th.MakeJPEG(20, 20)
	uploadReq := th.BuildUploadRequest(t, "file.jpg", jpegData, owner.Cookie)
	uploadResp, _ := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	var uploaded api.AssetResponse
	_ = json.NewDecoder(uploadResp.Body).Decode(&uploaded)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+uploaded.ID+"/file", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected non-empty file content")
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		t.Errorf("expected image content-type, got %q", ct)
	}
}

func TestGetAssetFile_ServesCurrentVersion(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload original asset (100×100).
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Upload a second version with different dimensions (200×200).
	v2Data := th.MakeJPEG(200, 200)
	vReq := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", v2Data, "", owner.Cookie)
	vResp, err := env.App.Test(vReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload v2: %v", err)
	}
	if vResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(vResp.Body)
		t.Fatalf("expected 201 for v2 upload, got %d: %s", vResp.StatusCode, b)
	}

	// GET /api/v1/assets/:id/file must return v2 bytes (larger than v1).
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/file", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("get asset file: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	fileBytes, _ := io.ReadAll(resp.Body)
	if len(fileBytes) == 0 {
		t.Fatal("expected non-empty file content")
	}
	// v2 (200×200) must be larger than v1 (100×100).
	v1Bytes := th.MakeJPEG(100, 100)
	if len(fileBytes) <= len(v1Bytes) {
		t.Errorf("expected v2 file (%d bytes) to be larger than v1 (%d bytes)", len(fileBytes), len(v1Bytes))
	}
}

func TestDeleteAsset(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	uploadReq := th.BuildUploadRequest(t, "del.jpg", th.MakeJPEG(10, 10), owner.Cookie)
	uploadResp, _ := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	var uploaded api.AssetResponse
	_ = json.NewDecoder(uploadResp.Body).Decode(&uploaded)

	delReq := th.AuthRequest(http.MethodDelete, "/api/v1/assets/"+uploaded.ID, nil, owner.Cookie)
	resp, err := env.App.Test(delReq)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify gone from DB
	getReq := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+uploaded.ID, nil, owner.Cookie)
	getResp, _ := env.App.Test(getReq)
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getResp.StatusCode)
	}
}

func TestListAssets_Sort(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload assets with distinct sizes: 10x10, 50x50, 100x100
	sizes := []int{10, 50, 100}
	for _, s := range sizes {
		req := th.BuildUploadRequest(t, "img.jpg", th.MakeJPEG(s, s), owner.Cookie)
		resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %dx%d: status %d err %v", s, s, resp.StatusCode, err)
		}
		var a api.AssetResponse
		_ = json.NewDecoder(resp.Body).Decode(&a)
		time.Sleep(10 * time.Millisecond)
	}

	cases := []struct {
		sort  string
		check func(t *testing.T, assets []api.AssetResponse)
	}{
		{
			sort: "id_desc",
			check: func(t *testing.T, assets []api.AssetResponse) {
				if assets[0].ID < assets[1].ID || assets[1].ID < assets[2].ID {
					t.Errorf("id_desc: IDs not descending: %v", []string{assets[0].ID, assets[1].ID, assets[2].ID})
				}
			},
		},
		{
			sort: "id_asc",
			check: func(t *testing.T, assets []api.AssetResponse) {
				if assets[0].ID > assets[1].ID || assets[1].ID > assets[2].ID {
					t.Errorf("id_asc: IDs not ascending: %v", []string{assets[0].ID, assets[1].ID, assets[2].ID})
				}
			},
		},
		{
			sort: "size_asc",
			check: func(t *testing.T, assets []api.AssetResponse) {
				if assets[0].Size > assets[1].Size || assets[1].Size > assets[2].Size {
					t.Errorf("size_asc: sizes not ascending: %v", []int64{assets[0].Size, assets[1].Size, assets[2].Size})
				}
			},
		},
		{
			sort: "size_desc",
			check: func(t *testing.T, assets []api.AssetResponse) {
				if assets[0].Size < assets[1].Size || assets[1].Size < assets[2].Size {
					t.Errorf("size_desc: sizes not descending: %v", []int64{assets[0].Size, assets[1].Size, assets[2].Size})
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.sort, func(t *testing.T) {
			req := th.AuthRequest(http.MethodGet, "/api/v1/assets?sort="+tc.sort, nil, owner.Cookie)
			resp, err := env.App.Test(req)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
			var result api.AssetListResponse
			_ = json.NewDecoder(resp.Body).Decode(&result)
			if len(result.Assets) != 3 {
				t.Fatalf("expected 3 assets, got %d", len(result.Assets))
			}
			tc.check(t, result.Assets)
		})
	}
}

func TestSearchAssets(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload two assets with distinct names
	for _, name := range []string{"sunset_beach.jpg", "mountain_peak.jpg"} {
		req := th.BuildUploadRequest(t, name, th.MakeJPEG(10, 10), owner.Cookie)
		resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %s: %v %d", name, err, resp.StatusCode)
		}
	}

	// Search for "sunset"
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?q=sunset", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result api.AssetListResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Assets) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(result.Assets))
	}
	if result.Assets[0].OriginalFilename != "sunset_beach.jpg" {
		t.Errorf("expected sunset_beach.jpg, got %q", result.Assets[0].OriginalFilename)
	}
}

func TestSearchAssets_Pagination(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload 3 assets whose names all match "photo"
	for i := range 3 {
		name := "photo_" + string(rune('a'+i)) + ".jpg"
		req := th.BuildUploadRequest(t, name, th.MakeJPEG(10, 10), owner.Cookie)
		resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %s: %v %d", name, err, resp.StatusCode)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Page 1: limit=2, expect next_cursor
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?q=photo&limit=2", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("search page1: %v", err)
	}
	var page1 api.AssetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&page1); err != nil {
		t.Fatalf("decode page1: %v", err)
	}
	if len(page1.Assets) != 2 {
		t.Fatalf("expected 2 assets on page1, got %d", len(page1.Assets))
	}
	if page1.NextCursor == nil {
		t.Fatal("expected next_cursor on page1")
	}

	// Page 2: use cursor, expect 1 remaining asset and no next_cursor
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets?q=photo&limit=2&cursor="+*page1.NextCursor, nil, owner.Cookie)
	resp2, err := env.App.Test(req2)
	if err != nil {
		t.Fatalf("search page2: %v", err)
	}
	var page2 api.AssetListResponse
	if err := json.NewDecoder(resp2.Body).Decode(&page2); err != nil {
		t.Fatalf("decode page2: %v", err)
	}
	if len(page2.Assets) != 1 {
		t.Fatalf("expected 1 asset on page2, got %d", len(page2.Assets))
	}
	if page2.NextCursor != nil {
		t.Error("expected nil next_cursor on last page")
	}

	// Ensure no asset ID appears in both pages
	seen := make(map[string]bool)
	for _, a := range page1.Assets {
		seen[a.ID] = true
	}
	for _, a := range page2.Assets {
		if seen[a.ID] {
			t.Errorf("duplicate asset %s returned on both pages", a.ID)
		}
	}
}

// insertAssetWithSize inserts a minimal asset row directly via SQL with a known size value.
func insertAssetWithSize(t *testing.T, env *th.TestEnv, workspaceID string, size int64) string {
	t.Helper()
	id := fmt.Sprintf("asset-%d", size)
	_, err := env.Database.Exec(`
		INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, id, workspaceID, fmt.Sprintf("file-%d.bin", size), fmt.Sprintf("key-%d", size), "application/octet-stream", size)
	if err != nil {
		t.Fatalf("insert asset with size %d: %v", size, err)
	}
	return id
}

func TestListAssets_PaginationSortBySizeDesc(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Insert 25 assets with distinct sizes 1..25 bytes
	var inserted []string
	for i := int64(1); i <= 25; i++ {
		inserted = append(inserted, insertAssetWithSize(t, env, owner.WorkspaceID, i))
	}

	getPage := func(cursor string) api.AssetListResponse {
		url := "/api/v1/assets?sort=size_desc&limit=10"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		resp, err := env.App.Test(th.AuthRequest(http.MethodGet, url, nil, owner.Cookie), fiber.TestConfig{Timeout: 5000})
		if err != nil {
			t.Fatalf("list assets: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		var result api.AssetListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return result
	}

	page1 := getPage("")
	if len(page1.Assets) != 10 {
		t.Fatalf("page1: expected 10, got %d", len(page1.Assets))
	}
	if page1.NextCursor == nil {
		t.Fatal("page1: expected next_cursor")
	}

	page2 := getPage(*page1.NextCursor)
	if len(page2.Assets) != 10 {
		t.Fatalf("page2: expected 10, got %d", len(page2.Assets))
	}
	if page2.NextCursor == nil {
		t.Fatal("page2: expected next_cursor")
	}

	page3 := getPage(*page2.NextCursor)
	if len(page3.Assets) != 5 {
		t.Fatalf("page3: expected 5, got %d", len(page3.Assets))
	}
	if page3.NextCursor != nil {
		t.Error("page3: expected nil next_cursor")
	}

	// Collect all IDs and check for duplicates
	seen := make(map[string]bool)
	allAssets := append(append(page1.Assets, page2.Assets...), page3.Assets...)
	if len(allAssets) != 25 {
		t.Fatalf("expected 25 total assets, got %d", len(allAssets))
	}
	for _, a := range allAssets {
		if seen[a.ID] {
			t.Errorf("duplicate asset ID: %s", a.ID)
		}
		seen[a.ID] = true
	}

	// Verify non-increasing size order across all pages
	for i := 1; i < len(allAssets); i++ {
		if allAssets[i].Size > allAssets[i-1].Size {
			t.Errorf("size not non-increasing at position %d: %d > %d", i, allAssets[i].Size, allAssets[i-1].Size)
		}
	}

	// Verify all 25 inserted assets are present
	for _, id := range inserted {
		if !seen[id] {
			t.Errorf("inserted asset %s missing from results", id)
		}
	}
}

func TestListAssets_PaginationSortBySizeAsc(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	var inserted []string
	for i := int64(1); i <= 25; i++ {
		inserted = append(inserted, insertAssetWithSize(t, env, owner.WorkspaceID, i))
	}

	getPage := func(cursor string) api.AssetListResponse {
		url := "/api/v1/assets?sort=size_asc&limit=10"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		resp, err := env.App.Test(th.AuthRequest(http.MethodGet, url, nil, owner.Cookie), fiber.TestConfig{Timeout: 5000})
		if err != nil {
			t.Fatalf("list assets: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		var result api.AssetListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return result
	}

	page1 := getPage("")
	if len(page1.Assets) != 10 {
		t.Fatalf("page1: expected 10, got %d", len(page1.Assets))
	}
	if page1.NextCursor == nil {
		t.Fatal("page1: expected next_cursor")
	}

	page2 := getPage(*page1.NextCursor)
	if len(page2.Assets) != 10 {
		t.Fatalf("page2: expected 10, got %d", len(page2.Assets))
	}
	if page2.NextCursor == nil {
		t.Fatal("page2: expected next_cursor")
	}

	page3 := getPage(*page2.NextCursor)
	if len(page3.Assets) != 5 {
		t.Fatalf("page3: expected 5, got %d", len(page3.Assets))
	}
	if page3.NextCursor != nil {
		t.Error("page3: expected nil next_cursor")
	}

	seen := make(map[string]bool)
	allAssets := append(append(page1.Assets, page2.Assets...), page3.Assets...)
	if len(allAssets) != 25 {
		t.Fatalf("expected 25 total assets, got %d", len(allAssets))
	}
	for _, a := range allAssets {
		if seen[a.ID] {
			t.Errorf("duplicate asset ID: %s", a.ID)
		}
		seen[a.ID] = true
	}

	// Verify non-decreasing size order across all pages
	for i := 1; i < len(allAssets); i++ {
		if allAssets[i].Size < allAssets[i-1].Size {
			t.Errorf("size not non-decreasing at position %d: %d < %d", i, allAssets[i].Size, allAssets[i-1].Size)
		}
	}

	for _, id := range inserted {
		if !seen[id] {
			t.Errorf("inserted asset %s missing from results", id)
		}
	}
}

func TestGetComments_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Create a share with comments enabled and post two comments via the share endpoint
	sh := createShare(t, env, owner.Cookie, api.CreateShareRequest{
		TargetType:    "asset",
		TargetID:      asset.ID,
		AllowComments: true,
	})
	token := accessShare(t, env, sh.ID, "")

	for _, name := range []string{"Alice", "Bob"} {
		body := fmt.Sprintf(`{"asset_id":%q,"author_name":%q,"body":"hello"}`, asset.ID, name)
		req := shareRequest(http.MethodPost, "/shared/"+sh.ID+"/comments", body, token)
		resp, _ := env.App.Test(req)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create comment for %s: got %d", name, resp.StatusCode)
		}
	}

	// Fetch comments via the authenticated asset endpoint
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/comments", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var comments []api.CommentResponse
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].AuthorName != "Alice" || comments[1].AuthorName != "Bob" {
		t.Errorf("unexpected author names: %q, %q", comments[0].AuthorName, comments[1].AuthorName)
	}
}

func TestGetComments_NotFound(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/nonexistent/comments", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetComments_Unauthenticated(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/assets/"+asset.ID+"/comments", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetAssetThumb_NotReady(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	// No thumbnail has been generated yet — thumbnail_key is NULL
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/thumb", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 (thumbnail not ready), got %d", resp.StatusCode)
	}
}

func TestGetAssetThumb_Ready(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Store a fake thumbnail in local storage and point the DB row at it
	thumbKey := "thumbs/" + asset.ID + ".jpg"
	thumbData := th.MakeJPEG(50, 50)
	if err := env.Storage.Put(t.Context(), thumbKey, bytes.NewReader(thumbData)); err != nil {
		t.Fatalf("put thumbnail: %v", err)
	}
	if _, err := env.Database.Exec(`UPDATE assets SET thumbnail_key = ? WHERE id = ?`, thumbKey, asset.ID); err != nil {
		t.Fatalf("set thumbnail_key: %v", err)
	}

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/thumb", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
}

func TestGetAssetThumb_NotFound(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/nonexistent/thumb", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetAssetThumb_Unauthenticated(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/assets/"+asset.ID+"/thumb", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListAssetsInFolder_ByFolderID(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	proj := th.CreateProject(t, env, owner.Cookie, "Proj", "")
	folder := createFolder(t, env, owner.Cookie, proj.ID, "Docs", nil)

	// Upload one asset into the folder, one without
	req1 := th.BuildUploadRequest(t, "in-folder.jpg", th.MakeJPEG(10, 10), owner.Cookie,
		map[string]string{"folder_id": folder.ID, "project_id": proj.ID})
	resp1, err := env.App.Test(req1, fiber.TestConfig{Timeout: 5000})
	if err != nil || resp1.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp1.Body)
		t.Fatalf("upload in-folder: %d %s %v", resp1.StatusCode, body, err)
	}
	req2 := th.BuildUploadRequest(t, "no-folder.jpg", th.MakeJPEG(10, 10), owner.Cookie)
	resp2, err := env.App.Test(req2, fiber.TestConfig{Timeout: 5000})
	if err != nil || resp2.StatusCode != http.StatusCreated {
		t.Fatalf("upload no-folder: %v %d", err, resp2.StatusCode)
	}

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?folder_id="+folder.ID, nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var result api.AssetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Assets) != 1 {
		t.Fatalf("expected 1 asset in folder, got %d", len(result.Assets))
	}
	if result.Assets[0].OriginalFilename != "in-folder.jpg" {
		t.Errorf("unexpected asset: %s", result.Assets[0].OriginalFilename)
	}
}

func TestListAssetsInFolder_Root(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	proj := th.CreateProject(t, env, owner.Cookie, "Proj", "")
	folder := createFolder(t, env, owner.Cookie, proj.ID, "Sub", nil)

	// Asset in root of project (no folder)
	req1 := th.BuildUploadRequest(t, "root.jpg", th.MakeJPEG(10, 10), owner.Cookie,
		map[string]string{"project_id": proj.ID})
	resp1, err := env.App.Test(req1, fiber.TestConfig{Timeout: 5000})
	if err != nil || resp1.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp1.Body)
		t.Fatalf("upload root: %d %s %v", resp1.StatusCode, body, err)
	}
	// Asset inside a subfolder
	req2 := th.BuildUploadRequest(t, "sub.jpg", th.MakeJPEG(10, 10), owner.Cookie,
		map[string]string{"project_id": proj.ID, "folder_id": folder.ID})
	resp2, err := env.App.Test(req2, fiber.TestConfig{Timeout: 5000})
	if err != nil || resp2.StatusCode != http.StatusCreated {
		t.Fatalf("upload sub: %v %d", err, resp2.StatusCode)
	}

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?folder_id=root&project_id="+proj.ID, nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("list root: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var result api.AssetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Assets) != 1 {
		t.Fatalf("expected 1 root asset, got %d", len(result.Assets))
	}
	if result.Assets[0].OriginalFilename != "root.jpg" {
		t.Errorf("unexpected asset: %s", result.Assets[0].OriginalFilename)
	}
}

func TestListAssetsInFolder_Root_MissingProjectID(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?folder_id=root", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetComments_Empty(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/comments", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var comments []api.CommentResponse
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected empty slice, got %d comments", len(comments))
	}
}

func TestListAssets_SortByTakenAt(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	a1 := th.UploadAsset(t, env, owner.Cookie)
	a2 := th.UploadAsset(t, env, owner.Cookie)
	a3 := th.UploadAsset(t, env, owner.Cookie) // no EXIF date — should sort last

	// Enable exif_keep and create the field definition
	_, err := env.Database.Exec(`UPDATE workspaces SET exif_keep = 1 WHERE id = ?`, owner.WorkspaceID)
	if err != nil {
		t.Fatalf("enable exif: %v", err)
	}

	// Drain to create field definitions via tombstone
	th.DrainJobs(t, env)

	// Manually set taken_at values for a1 and a2
	var fieldID string
	if err := env.Database.QueryRow(
		`SELECT id FROM field_definitions WHERE key = '_exif_taken_at' AND workspace_id = ?`,
		owner.WorkspaceID,
	).Scan(&fieldID); err != nil {
		t.Fatalf("get field id: %v", err)
	}

	userID := owner.UserID
	// a1 gets 2023, a2 gets 2024 — expect a2 first (ASC: 2023 before 2024, so a1 first for ASC)
	for _, row := range []struct {
		assetID string
		date    string
	}{
		{a1.ID, "2023-06-15"},
		{a2.ID, "2024-01-20"},
	} {
		_, err := env.Database.Exec(
			`INSERT OR REPLACE INTO asset_field_values (id, asset_id, field_id, value_date, created_by)
			 VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?)`,
			row.assetID, fieldID, row.date, userID,
		)
		if err != nil {
			t.Fatalf("insert field value for %s: %v", row.assetID, err)
		}
	}

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?sort=taken_at", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var body struct {
		Assets []api.AssetResponse `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Assets) != 3 {
		t.Fatalf("expected 3 assets, got %d", len(body.Assets))
	}
	// ASC: a1 (2023) first, then a2 (2024), then a3 (no date) last
	if body.Assets[0].ID != a1.ID {
		t.Errorf("first asset = %s, want %s (2023)", body.Assets[0].ID, a1.ID)
	}
	if body.Assets[1].ID != a2.ID {
		t.Errorf("second asset = %s, want %s (2024)", body.Assets[1].ID, a2.ID)
	}
	if body.Assets[2].ID != a3.ID {
		t.Errorf("third asset (no date) = %s, want %s", body.Assets[2].ID, a3.ID)
	}
}

func TestDeleteAsset_ConflictProjectCover(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload asset
	uploadReq := th.BuildUploadRequest(t, "cover.jpg", th.MakeJPEG(10, 10), owner.Cookie)
	uploadResp, err := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	if err != nil || uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: status %d err %v", uploadResp.StatusCode, err)
	}
	var asset api.AssetResponse
	_ = json.NewDecoder(uploadResp.Body).Decode(&asset)

	// Create project
	projResp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JSONBody(api.CreateProjectRequest{Name: "Proj"}), owner.Cookie))
	if err != nil || projResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project: status %d err %v", projResp.StatusCode, err)
	}
	var proj api.ProjectResponse
	_ = json.NewDecoder(projResp.Body).Decode(&proj)

	// Set asset as project cover
	updateResp, err := env.App.Test(th.AuthRequest(http.MethodPut, "/api/v1/projects/"+proj.ID,
		th.JSONBody(api.UpdateProjectRequest{CoverAssetID: &asset.ID}), owner.Cookie))
	if err != nil || updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update project cover: status %d err %v", updateResp.StatusCode, err)
	}

	// Delete asset — must be blocked
	delResp, err := env.App.Test(th.AuthRequest(http.MethodDelete, "/api/v1/assets/"+asset.ID, nil, owner.Cookie))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if delResp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", delResp.StatusCode)
	}
}

func TestDeleteAsset_ConflictWorkspaceIcon(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload asset
	uploadReq := th.BuildUploadRequest(t, "icon.jpg", th.MakeJPEG(10, 10), owner.Cookie)
	uploadResp, err := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	if err != nil || uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: status %d err %v", uploadResp.StatusCode, err)
	}
	var asset api.AssetResponse
	_ = json.NewDecoder(uploadResp.Body).Decode(&asset)

	// Set asset as workspace icon via direct SQL
	_, err = env.Database.Exec("UPDATE workspaces SET icon_asset_id = ? WHERE id = ?", asset.ID, owner.WorkspaceID)
	if err != nil {
		t.Fatalf("set icon: %v", err)
	}

	// Delete asset — must be blocked
	delResp, err := env.App.Test(th.AuthRequest(http.MethodDelete, "/api/v1/assets/"+asset.ID, nil, owner.Cookie))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if delResp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", delResp.StatusCode)
	}
}

func TestDeleteAsset_OkAfterCoverCleared(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload asset
	uploadReq := th.BuildUploadRequest(t, "cover.jpg", th.MakeJPEG(10, 10), owner.Cookie)
	uploadResp, err := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	if err != nil || uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: status %d err %v", uploadResp.StatusCode, err)
	}
	var asset api.AssetResponse
	_ = json.NewDecoder(uploadResp.Body).Decode(&asset)

	// Create project and set cover
	projResp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JSONBody(api.CreateProjectRequest{Name: "Proj"}), owner.Cookie))
	if err != nil || projResp.StatusCode != http.StatusCreated {
		t.Fatalf("create project: %v", err)
	}
	var proj api.ProjectResponse
	_ = json.NewDecoder(projResp.Body).Decode(&proj)

	_, err = env.App.Test(th.AuthRequest(http.MethodPut, "/api/v1/projects/"+proj.ID,
		th.JSONBody(api.UpdateProjectRequest{CoverAssetID: &asset.ID}), owner.Cookie))
	if err != nil {
		t.Fatalf("set cover: %v", err)
	}

	// Clear cover via direct SQL (no API endpoint to null it)
	_, err = env.Database.Exec("UPDATE projects SET cover_asset_id = NULL WHERE id = ?", proj.ID)
	if err != nil {
		t.Fatalf("clear cover: %v", err)
	}

	// Delete asset — must succeed now
	delResp, err := env.App.Test(th.AuthRequest(http.MethodDelete, "/api/v1/assets/"+asset.ID, nil, owner.Cookie))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", delResp.StatusCode)
	}
}

func TestGetAsset_VariantCount(t *testing.T) {
	env, _ := th.SetupWithOwner(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("get asset: %v %d", err, resp.StatusCode)
	}
	var asset api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&asset)
	if asset.VariantCount != 0 {
		t.Errorf("expected variant_count=0, got %d", asset.VariantCount)
	}

	// Insert a variant directly and re-fetch.
	var ws struct{ WorkspaceID string }
	_ = env.Database.QueryRow("SELECT workspace_id FROM assets WHERE id = ?", assetID).Scan(&ws.WorkspaceID)
	insertVariantDirectly(t, env, assetID, ws.WorkspaceID)

	resp2, err := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie))
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("get asset after variant: %v %d", err, resp2.StatusCode)
	}
	var asset2 api.AssetResponse
	_ = json.NewDecoder(resp2.Body).Decode(&asset2)
	if asset2.VariantCount != 1 {
		t.Errorf("expected variant_count=1, got %d", asset2.VariantCount)
	}
}

func TestListAssets_VariantCount(t *testing.T) {
	env, _ := th.SetupWithOwner(t)
	assetID, cookie := createTestAsset(t, env)

	var workspaceID string
	_ = env.Database.QueryRow("SELECT workspace_id FROM assets WHERE id = ?", assetID).Scan(&workspaceID)
	insertVariantDirectly(t, env, assetID, workspaceID)

	resp, err := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/assets", nil, cookie))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("list assets: %v %d", err, resp.StatusCode)
	}
	var list api.AssetListResponse
	_ = json.NewDecoder(resp.Body).Decode(&list)
	if len(list.Assets) == 0 {
		t.Fatal("expected at least one asset")
	}
	found := false
	for _, a := range list.Assets {
		if a.ID == assetID {
			found = true
			if a.VariantCount != 1 {
				t.Errorf("expected variant_count=1, got %d", a.VariantCount)
			}
		}
	}
	if !found {
		t.Error("asset not found in list")
	}
}
