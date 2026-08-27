//go:build integration

package api_test

import (
	"damask/server/internal/api"
	th "damask/server/internal/testhelpers"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// setupTwoWorkspaces th.Registers two independent owners, each in their own workspace.
func setupTwoWorkspaces(t *testing.T) (env *th.TestEnv, ws1 th.AuthResult, ws2 th.AuthResult) {
	t.Helper()
	env = th.SetupTestApp(t)
	ws1 = th.Register(t, env, "Alice", "alice@example.com", "password123")
	ws2 = th.Register(t, env, "Bob", "bob@example.com", "password456")
	return
}

// --- Projects ---

func TestIsolation_ListProjects(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")

	req := th.AuthRequest(http.MethodGet, "/api/v1/projects", nil, ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var projects []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("ws2 should see 0 projects, got %d", len(projects))
	}
}

func TestIsolation_GetProject(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")

	req := th.AuthRequest(http.MethodGet, "/api/v1/projects/"+p.ID, nil, ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_UpdateProject(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")

	hackedName := "Hacked"
	req := th.AuthRequest(http.MethodPut, "/api/v1/projects/"+p.ID,
		th.JSONBody(api.UpdateProjectRequest{Name: &hackedName}), ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_DeleteProject(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")

	req := th.AuthRequest(http.MethodDelete, "/api/v1/projects/"+p.ID, nil, ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Folders ---

func TestIsolation_CreateFolderInOtherProject(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")

	req := th.AuthRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/folders", p.ID),
		th.JSONBody(api.CreateFolderRequest{Name: "Intruder"}), ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_GetFolders(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")
	createFolder(t, env, ws1.Cookie, p.ID, "Folder A", nil)

	req := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/folders", p.ID), nil, ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_UpdateFolder(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")
	folder := createFolder(t, env, ws1.Cookie, p.ID, "Folder A", nil)
	folderID := folder.ID

	hackedFolderName := "Hacked"
	req := th.AuthRequest(http.MethodPut, "/api/v1/folders/"+folderID,
		th.JSONBody(api.UpdateFolderRequest{Name: &hackedFolderName}), ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_DeleteFolder(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")
	folder := createFolder(t, env, ws1.Cookie, p.ID, "Folder A", nil)
	folderID := folder.ID

	req := th.AuthRequest(http.MethodDelete, "/api/v1/folders/"+folderID, nil, ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Assets ---

func TestIsolation_ListAssets(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	uploadTestAsset(t, env, ws1)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets", nil, ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 0 {
		t.Errorf("ws2 should see 0 assets, got %d", len(body.Data))
	}
}

func TestIsolation_GetAsset(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, ws2.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_GetAssetFile(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/file", nil, ws2.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_UpdateAssetFolder(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := th.AuthRequest(http.MethodPatch, "/api/v1/assets/"+assetID,
		th.JSONBody(api.UpdateAssetFolderRequest{FolderID: nil}), ws2.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_DeleteAsset(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := th.AuthRequest(http.MethodDelete, "/api/v1/assets/"+assetID, nil, ws2.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_RenameAsset(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := th.AuthRequest(http.MethodPut, "/api/v1/assets/"+assetID+"/rename",
		th.JSONBody(api.RenameAssetRequest{Name: "hacked"}), ws2.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Tags ---

func TestIsolation_ListTags(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	// Add a tag to ws1's asset so there is something to be isolated
	assetID := uploadTestAsset(t, env, ws1)
	tagReq := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/tags",
		th.JSONBody(api.AddTagRequest{Name: "secret-tag"}), ws1.Cookie)
	env.App.Test(tagReq, fiber.TestConfig{Timeout: 5000}) //nolint:errcheck

	req := th.AuthRequest(http.MethodGet, "/api/v1/tags", nil, ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var tags []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("ws2 should see 0 tags, got %d", len(tags))
	}
}

func TestIsolation_GetAssetTags(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/tags", nil, ws2.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_AddTagToAsset(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/tags",
		th.JSONBody(api.AddTagRequest{Name: "intruder"}), ws2.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Workspace ---

func TestIsolation_WorkspaceMe(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/workspace/me", nil, ws2.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Workspace struct {
			ID string `json:"id"`
		} `json:"workspace"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Workspace.ID != ws2.WorkspaceID {
		t.Errorf("ws2 got workspace %q, want %q", body.Workspace.ID, ws2.WorkspaceID)
	}
	if body.Workspace.ID == ws1.WorkspaceID {
		t.Error("ws2 must not see ws1's workspace")
	}
}

func TestIsolation_Members_CannotAccessOtherWorkspace(t *testing.T) {
	env := th.SetupTestApp(t)
	ws1 := th.Register(t, env, "Alice", "alice@example.com", "password123")
	ws2 := th.Register(t, env, "Bob", "bob@example.com", "password456")

	// ws1 owner cannot list/modify ws2 members — they have no JWT for ws2
	// The owner token for ws1 should return their own workspace members (ws1 only)
	req := th.AuthRequest(http.MethodGet, "/api/v1/workspace/members", nil, ws1.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var members []api.MemberResponse
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, m := range members {
		if m.UserID == ws2.UserID {
			t.Error("ws1 owner must not see ws2 member in their member list")
		}
	}
}

func TestIsolation_EmbedTokens(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	createReq := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/embed-token", nil, ws2.Cookie)
	createResp, err := env.App.Test(createReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if createResp.StatusCode != http.StatusNotFound {
		t.Errorf("create: expected 404, got %d", createResp.StatusCode)
	}

	getReq := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/embed-token", nil, ws2.Cookie)
	getResp, err := env.App.Test(getReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("get: expected 404, got %d", getResp.StatusCode)
	}

	delReq := th.AuthRequest(http.MethodDelete, "/api/v1/assets/"+assetID+"/embed-token", nil, ws2.Cookie)
	delResp, err := env.App.Test(delReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if delResp.StatusCode != http.StatusNotFound {
		t.Errorf("delete: expected 404, got %d", delResp.StatusCode)
	}
}
