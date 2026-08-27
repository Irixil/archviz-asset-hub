//go:build integration

package api_test

import (
	"damask/server/internal/api"
	"damask/server/internal/auth"
	th "damask/server/internal/testhelpers"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gofiber/fiber/v3"
)

func TestCreateWorkspace_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace",
		th.JSONBody(api.CreateWorkspaceRequest{Name: "My New Workspace"}), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body api.AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Workspace == nil {
		t.Fatal("expected workspace in response")
	}
	if body.Workspace.Name != "My New Workspace" {
		t.Errorf("workspace name = %q, want %q", body.Workspace.Name, "My New Workspace")
	}
	if body.Workspace.ID == result.WorkspaceID {
		t.Error("new workspace should have a different ID than the original")
	}
}

func TestCreateWorkspace_MissingName(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace",
		th.JSONBody(api.CreateWorkspaceRequest{Name: ""}), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateWorkspace_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace",
		th.JSONBody(api.CreateWorkspaceRequest{Name: "My Workspace"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWorkspaceMe_Authenticated(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/workspace/me", nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.WorkspaceMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.User.Email != "alice@example.com" {
		t.Errorf("user email = %q, want %q", body.User.Email, "alice@example.com")
	}
	if body.Role != auth.Owner {
		t.Errorf("role = %q, want %q", body.Role, auth.Owner)
	}
}

func TestWorkspaceMe_TotalAssetCount(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// No assets yet — count should be 0.
	req := th.AuthRequest(http.MethodGet, "/api/v1/workspace/me", nil, result.Cookie)
	resp, _ := env.App.Test(req)
	var body api.WorkspaceMeResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.TotalAssetCount != 0 {
		t.Errorf("expected 0 assets before upload, got %d", body.TotalAssetCount)
	}

	// Upload two assets (no project assigned).
	for range 2 {
		uploadReq := th.BuildUploadRequest(t, "file.jpg", th.MakeJPEG(10, 10), result.Cookie)
		uploadResp, err := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
		if err != nil || uploadResp.StatusCode != http.StatusCreated {
			t.Fatalf("upload failed: status=%d err=%v", uploadResp.StatusCode, err)
		}
	}

	req = th.AuthRequest(http.MethodGet, "/api/v1/workspace/me", nil, result.Cookie)
	resp, _ = env.App.Test(req)
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.TotalAssetCount != 2 {
		t.Errorf("expected 2 after upload, got %d", body.TotalAssetCount)
	}
}

func TestWorkspaceMe_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace/me", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateInvite_AsOwner(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JSONBody(api.CreateInviteRequest{Email: "bob@example.com", Role: auth.Editor}), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body api.InviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.InviteToken == "" {
		t.Error("expected non-empty invite_token")
	}
	if body.Email != "bob@example.com" {
		t.Errorf("invite email = %q, want %q", body.Email, "bob@example.com")
	}
}

func TestCreateInvite_AsEditor_Forbidden(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")
	editorToken := th.MintEditorToken(t, env, owner.WorkspaceID, auth.Editor)

	req := th.BearerRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JSONBody(api.CreateInviteRequest{Email: "carol@example.com", Role: auth.Viewer}), editorToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateInvite_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JSONBody(api.CreateInviteRequest{Email: "bob@example.com", Role: auth.Editor}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAcceptInvite_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create an invite as owner
	invReq := th.AuthRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JSONBody(api.CreateInviteRequest{Email: "bob@example.com", Role: auth.Editor}), owner.Cookie)
	invResp, err := env.App.Test(invReq)
	if err != nil {
		t.Fatalf("create invite request: %v", err)
	}
	if invResp.StatusCode != http.StatusCreated {
		t.Fatalf("create invite: expected 201, got %d", invResp.StatusCode)
	}

	var invite api.InviteResponse
	if err := json.NewDecoder(invResp.Body).Decode(&invite); err != nil {
		t.Fatalf("decode invite: %v", err)
	}

	// Accept the invite
	req := httptest.NewRequest(http.MethodPost, "/auth/invite/accept",
		th.JSONBody(api.AcceptInviteRequest{Token: invite.InviteToken, Name: "Bob", Password: "password123"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("accept invite request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	cookie := th.FindCookie(resp, "auth_token")
	if cookie == nil {
		t.Fatal("expected auth_token cookie after accepting invite")
	}
}

func TestAcceptInvite_InvalidToken(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/invite/accept",
		th.JSONBody(api.AcceptInviteRequest{Token: "does-not-exist", Name: "Bob", Password: "password123"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListWorkspaces_SingleWorkspace(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/workspaces", nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body []api.WorkspaceWithRoleResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(body))
	}
	if body[0].Role != string(auth.Owner) {
		t.Errorf("role = %q, want %q", body[0].Role, auth.Owner)
	}
	if body[0].ID != result.WorkspaceID {
		t.Errorf("workspace id = %q, want %q", body[0].ID, result.WorkspaceID)
	}
}

func TestListWorkspaces_MultipleWorkspaces(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create a second workspace
	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace",
		th.JSONBody(api.CreateWorkspaceRequest{Name: "Second Workspace"}), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("create workspace request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create workspace: expected 201, got %d", resp.StatusCode)
	}

	req = th.AuthRequest(http.MethodGet, "/api/v1/workspaces", nil, result.Cookie)
	resp, err = env.App.Test(req)
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body []api.WorkspaceWithRoleResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(body))
	}
}

func TestListWorkspaces_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSwitchWorkspace_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create a second workspace
	createReq := th.AuthRequest(http.MethodPost, "/api/v1/workspace",
		th.JSONBody(api.CreateWorkspaceRequest{Name: "Second Workspace"}), result.Cookie)
	createResp, err := env.App.Test(createReq)
	if err != nil {
		t.Fatalf("create workspace request: %v", err)
	}
	var created api.AuthResponse
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	secondID := created.Workspace.ID

	// Switch to second workspace
	switchReq := th.AuthRequest(http.MethodPost, "/api/v1/workspace/switch",
		th.JSONBody(api.SwitchWorkspaceRequest{WorkspaceID: secondID}), result.Cookie)
	switchResp, err := env.App.Test(switchReq)
	if err != nil {
		t.Fatalf("switch request: %v", err)
	}
	if switchResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", switchResp.StatusCode)
	}

	var switchBody api.SwitchWorkspaceResponse
	if err := json.NewDecoder(switchResp.Body).Decode(&switchBody); err != nil {
		t.Fatalf("decode switch: %v", err)
	}
	if switchBody.Workspace.ID != secondID {
		t.Errorf("workspace id = %q, want %q", switchBody.Workspace.ID, secondID)
	}
	if switchBody.Role != auth.Owner {
		t.Errorf("role = %q, want owner", switchBody.Role)
	}

	// Verify new cookie reflects the new workspace
	newCookie := th.FindCookie(switchResp, "auth_token")
	if newCookie == nil {
		t.Fatal("expected new auth_token cookie")
	}
	meReq := th.AuthRequest(http.MethodGet, "/api/v1/workspace/me", nil, newCookie)
	meResp, err := env.App.Test(meReq)
	if err != nil {
		t.Fatalf("me request: %v", err)
	}
	var meBody api.WorkspaceMeResponse
	if err := json.NewDecoder(meResp.Body).Decode(&meBody); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if meBody.Workspace.ID != secondID {
		t.Errorf("me workspace id = %q, want %q", meBody.Workspace.ID, secondID)
	}
}

func TestSwitchWorkspace_NotMember(t *testing.T) {
	env := th.SetupTestApp(t)
	th.Register(t, env, "Alice", "alice@example.com", "password123")
	bob := th.Register(t, env, "Bob", "bob@example.com", "password123")

	// Alice's workspace ID is stored in result; Bob tries to access it
	alice, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/workspace/me", nil, bob.Cookie))
	var bobMe api.WorkspaceMeResponse
	_ = json.NewDecoder(alice.Body).Decode(&bobMe)

	// Re-register alice to get her workspace ID
	aliceResult := th.Register(t, env, "Alice2", "alice2@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace/switch",
		th.JSONBody(api.SwitchWorkspaceRequest{WorkspaceID: aliceResult.WorkspaceID}), bob.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestSwitchWorkspace_InvalidWorkspaceID(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace/switch",
		th.JSONBody(api.SwitchWorkspaceRequest{WorkspaceID: "does-not-exist"}), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestSwitchWorkspace_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace/switch",
		th.JSONBody(api.SwitchWorkspaceRequest{WorkspaceID: "anything"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAcceptInvite_ExpiredInvite(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create an invite
	invReq := th.AuthRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JSONBody(api.CreateInviteRequest{Email: "bob@example.com", Role: auth.Editor}), owner.Cookie)
	invResp, err := env.App.Test(invReq)
	if err != nil {
		t.Fatalf("create invite request: %v", err)
	}
	var invite api.InviteResponse
	if err := json.NewDecoder(invResp.Body).Decode(&invite); err != nil {
		t.Fatalf("decode invite: %v", err)
	}

	// Expire the invite directly in the DB
	_, err = env.Database.Exec(
		`UPDATE workspace_invites SET expires_at = datetime('now', '-1 day') WHERE token = ?`,
		invite.InviteToken,
	)
	if err != nil {
		t.Fatalf("expire invite: %v", err)
	}

	// Attempt to accept the expired invite
	req := httptest.NewRequest(http.MethodPost, "/auth/invite/accept",
		th.JSONBody(api.AcceptInviteRequest{Token: invite.InviteToken, Name: "Bob", Password: "password123"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateWorkspaceSettings_ExifKeep(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	body := map[string]any{
		"version_retention_count": 0,
		"exif_keep":               true,
		"exif_keep_gps":           false,
	}
	req := th.AuthRequest(http.MethodPut, "/api/v1/workspace/settings", th.JSONBody(body), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var ws struct {
		ExifKeep    bool `json:"exif_keep"`
		ExifKeepGps bool `json:"exif_keep_gps"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ws); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !ws.ExifKeep {
		t.Errorf("exif_keep = %v, want true", ws.ExifKeep)
	}
	if ws.ExifKeepGps {
		t.Errorf("exif_keep_gps = %v, want false", ws.ExifKeepGps)
	}
}

func TestUpdateWorkspaceSettings_ExifKeep_NonOwner(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	editorToken := th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	body := map[string]any{"version_retention_count": 0, "exif_keep": true}
	req := th.BearerRequest(http.MethodPut, "/api/v1/workspace/settings", th.JSONBody(body), editorToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestTriggerWorkspaceJob_UnknownType(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace/jobs/not_a_real_job/trigger", nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestTriggerWorkspaceJob_ExtractExif_NoAssets(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace/jobs/extract_exif/trigger", nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, b)
	}

	var body struct {
		Enqueued int `json:"enqueued"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Enqueued != 0 {
		t.Errorf("enqueued = %d, want 0", body.Enqueued)
	}
}

func TestTriggerWorkspaceJob_ExtractExif_WithAssets(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Upload two image assets
	th.UploadAsset(t, env, result.Cookie)
	th.UploadAsset(t, env, result.Cookie)

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace/jobs/extract_exif/trigger", nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, b)
	}

	var body struct {
		Enqueued int `json:"enqueued"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Enqueued != 2 {
		t.Errorf("enqueued = %d, want 2", body.Enqueued)
	}
}

func TestTriggerWorkspaceJob_ExtractExif_NonOwner(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	editorToken := th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	req := th.BearerRequest(http.MethodPost, "/api/v1/workspace/jobs/extract_exif/trigger", nil, editorToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// ── Members ──────────────────────────────────────────────────────────────────

func TestListMembers_Owner(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	req := th.AuthRequest(http.MethodGet, "/api/v1/workspace/members", nil, result.Cookie)
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
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
}

func TestListMembers_NonOwner(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	editorToken := th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	req := th.BearerRequest(http.MethodGet, "/api/v1/workspace/members", nil, editorToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRemoveMember_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	// Find the editor's user ID
	var editorUserID string
	row := env.Database.QueryRow(`SELECT user_id FROM workspace_members WHERE workspace_id = ? AND role = 'editor'`, result.WorkspaceID)
	if err := row.Scan(&editorUserID); err != nil {
		t.Fatalf("find editor: %v", err)
	}

	req := th.AuthRequest(http.MethodDelete, "/api/v1/workspace/members/"+editorUserID, nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestRemoveMember_Self(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodDelete, "/api/v1/workspace/members/"+result.UserID, nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRemoveMember_NonOwner(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	editorToken := th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	req := th.BearerRequest(http.MethodDelete, "/api/v1/workspace/members/"+result.UserID, nil, editorToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestUpdateMemberRole_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	var editorUserID string
	row := env.Database.QueryRow(`SELECT user_id FROM workspace_members WHERE workspace_id = ? AND role = 'editor'`, result.WorkspaceID)
	if err := row.Scan(&editorUserID); err != nil {
		t.Fatalf("find editor: %v", err)
	}

	req := th.AuthRequest(http.MethodPut, "/api/v1/workspace/members/"+editorUserID,
		th.JSONBody(api.UpdateMemberRoleRequest{Role: auth.Viewer}), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestUpdateMemberRole_InvalidRole(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	var editorUserID string
	row := env.Database.QueryRow(`SELECT user_id FROM workspace_members WHERE workspace_id = ? AND role = 'editor'`, result.WorkspaceID)
	if err := row.Scan(&editorUserID); err != nil {
		t.Fatalf("find editor: %v", err)
	}

	req := th.AuthRequest(http.MethodPut, "/api/v1/workspace/members/"+editorUserID,
		th.JSONBody(api.UpdateMemberRoleRequest{Role: "superadmin"}), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestUpdateMemberRole_DemoteLastOwner(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPut, "/api/v1/workspace/members/"+result.UserID,
		th.JSONBody(api.UpdateMemberRoleRequest{Role: auth.Editor}), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// ── Invites (list + delete) ───────────────────────────────────────────────────

func TestListInvites_Owner(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create an invite first
	invReq := th.AuthRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JSONBody(api.CreateInviteRequest{Email: "bob@example.com", Role: auth.Editor}), result.Cookie)
	invResp, err := env.App.Test(invReq)
	if err != nil || invResp.StatusCode != http.StatusCreated {
		t.Fatalf("create invite failed: %v / %d", err, invResp.StatusCode)
	}

	req := th.AuthRequest(http.MethodGet, "/api/v1/workspace/invites", nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var invites []api.InviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&invites); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(invites) != 1 {
		t.Fatalf("expected 1 invite, got %d", len(invites))
	}
	if invites[0].Email != "bob@example.com" {
		t.Errorf("invite email = %q, want bob@example.com", invites[0].Email)
	}
}

func TestDeleteInvite_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	invReq := th.AuthRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JSONBody(api.CreateInviteRequest{Email: "bob@example.com", Role: auth.Editor}), result.Cookie)
	invResp, err := env.App.Test(invReq)
	if err != nil || invResp.StatusCode != http.StatusCreated {
		t.Fatalf("create invite: %v / %d", err, invResp.StatusCode)
	}
	var invite api.InviteResponse
	if err := json.NewDecoder(invResp.Body).Decode(&invite); err != nil {
		t.Fatalf("decode invite: %v", err)
	}

	req := th.AuthRequest(http.MethodDelete, "/api/v1/workspace/invites/"+invite.ID, nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Confirm it's gone
	listReq := th.AuthRequest(http.MethodGet, "/api/v1/workspace/invites", nil, result.Cookie)
	listResp, _ := env.App.Test(listReq)
	var remaining []api.InviteResponse
	_ = json.NewDecoder(listResp.Body).Decode(&remaining)
	if len(remaining) != 0 {
		t.Fatalf("expected 0 invites after delete, got %d", len(remaining))
	}
}

func TestListInvites_NonOwner(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	editorToken := th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	req := th.BearerRequest(http.MethodGet, "/api/v1/workspace/invites", nil, editorToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// TestRoleCache_RemovalTakesEffectImmediately proves the role cache
// (ROADMAP 68 DB-3) is invalidated on removal rather than just expiring after
// its TTL: a removed member must lose access on their very next request, not
// up to 15s later.
func TestRoleCache_RemovalTakesEffectImmediately(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	editorToken := th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	var editorUserID string
	row := env.Database.QueryRow(
		`SELECT user_id FROM workspace_members WHERE workspace_id = ? AND role = 'editor'`, result.WorkspaceID)
	if err := row.Scan(&editorUserID); err != nil {
		t.Fatalf("find editor: %v", err)
	}

	createProject := func() int {
		req := th.BearerRequest(http.MethodPost, "/api/v1/projects",
			th.JSONBody(api.CreateProjectRequest{Name: "p"}), editorToken)
		resp, err := env.App.Test(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp.StatusCode
	}

	// Warm the role cache with the editor's role.
	if status := createProject(); status != http.StatusCreated {
		t.Fatalf("expected 201 before removal, got %d", status)
	}

	removeReq := th.AuthRequest(http.MethodDelete, "/api/v1/workspace/members/"+editorUserID, nil, result.Cookie)
	removeResp, err := env.App.Test(removeReq)
	if err != nil {
		t.Fatalf("remove request: %v", err)
	}
	if removeResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 removing member, got %d", removeResp.StatusCode)
	}

	// No sleep: if invalidation didn't fire, the cached "editor" role would
	// still be valid for up to 15s (roleCacheTTL) and this would wrongly succeed.
	if status := createProject(); status != http.StatusForbidden {
		t.Fatalf("expected 403 immediately after removal, got %d", status)
	}
}

// TestRoleCache_UpdateTakesEffectImmediately mirrors the removal case for a
// role demotion: editor → viewer must be enforced on the next request.
func TestRoleCache_UpdateTakesEffectImmediately(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")
	editorToken := th.MintEditorToken(t, env, result.WorkspaceID, auth.Editor)

	var editorUserID string
	row := env.Database.QueryRow(
		`SELECT user_id FROM workspace_members WHERE workspace_id = ? AND role = 'editor'`, result.WorkspaceID)
	if err := row.Scan(&editorUserID); err != nil {
		t.Fatalf("find editor: %v", err)
	}

	createProject := func() int {
		req := th.BearerRequest(http.MethodPost, "/api/v1/projects",
			th.JSONBody(api.CreateProjectRequest{Name: "p"}), editorToken)
		resp, err := env.App.Test(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp.StatusCode
	}

	if status := createProject(); status != http.StatusCreated {
		t.Fatalf("expected 201 before demotion, got %d", status)
	}

	demoteReq := th.AuthRequest(http.MethodPut, "/api/v1/workspace/members/"+editorUserID,
		th.JSONBody(api.UpdateMemberRoleRequest{Role: auth.Viewer}), result.Cookie)
	demoteResp, err := env.App.Test(demoteReq)
	if err != nil {
		t.Fatalf("demote request: %v", err)
	}
	if demoteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 demoting member, got %d", demoteResp.StatusCode)
	}

	if status := createProject(); status != http.StatusForbidden {
		t.Fatalf("expected 403 immediately after demotion to viewer, got %d", status)
	}
}

// TestRoleCache_CrossWorkspaceIsolation proves the cache key is scoped by
// (workspaceID, userID) together: invalidating a user's cached role in one
// workspace must not disturb their independently-cached role in another
// workspace, even though it's the same user ID.
func TestRoleCache_CrossWorkspaceIsolation(t *testing.T) {
	env := th.SetupTestApp(t)
	alice := th.Register(t, env, "Alice", "alice@example.com", "password123")
	bob := th.Register(t, env, "Bob", "bob@example.com", "password123")

	// sharedUserID joins Alice's workspace as editor and Bob's workspace as owner.
	sharedUserID := "shared-user-id"
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if _, err := env.Database.Exec(
		`INSERT INTO users (id, email, password_hash, name, created_at) VALUES (?, ?, ?, ?, ?)`,
		sharedUserID, "shared@example.com", string(hash), "Shared User", now,
	); err != nil {
		t.Fatalf("insert shared user: %v", err)
	}
	if _, err := env.Database.Exec(
		`INSERT INTO workspace_members (workspace_id, user_id, role, created_at) VALUES (?, ?, 'editor', ?)`,
		alice.WorkspaceID, sharedUserID, now,
	); err != nil {
		t.Fatalf("insert membership in alice's workspace: %v", err)
	}
	if _, err := env.Database.Exec(
		`INSERT INTO workspace_members (workspace_id, user_id, role, created_at) VALUES (?, ?, 'owner', ?)`,
		bob.WorkspaceID, sharedUserID, now,
	); err != nil {
		t.Fatalf("insert membership in bob's workspace: %v", err)
	}

	tokenInAlice, err := env.Maker.CreateToken(sharedUserID, alice.WorkspaceID, time.Hour)
	if err != nil {
		t.Fatalf("mint token for alice's workspace: %v", err)
	}
	tokenInBob, err := env.Maker.CreateToken(sharedUserID, bob.WorkspaceID, time.Hour)
	if err != nil {
		t.Fatalf("mint token for bob's workspace: %v", err)
	}

	// Warm both cache entries: (alice.WorkspaceID, sharedUserID)->editor and
	// (bob.WorkspaceID, sharedUserID)->owner.
	createProjectReq := th.BearerRequest(http.MethodPost, "/api/v1/projects",
		th.JSONBody(api.CreateProjectRequest{Name: "p"}), tokenInAlice)
	warmAliceResp, err := env.App.Test(createProjectReq)
	if err != nil || warmAliceResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 warming editor cache in alice's workspace, got status=%d err=%v",
			statusOf(warmAliceResp), err)
	}
	settingsReq := th.BearerRequest(http.MethodPut, "/api/v1/workspace/settings",
		th.JSONBody(api.UpdateWorkspaceSettingsRequest{VersionRetentionCount: 5}), tokenInBob)
	warmBobResp, err := env.App.Test(settingsReq)
	if err != nil || warmBobResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 warming owner cache in bob's workspace, got status=%d err=%v",
			statusOf(warmBobResp), err)
	}

	// Remove sharedUserID from Alice's workspace only — invalidates
	// (alice.WorkspaceID, sharedUserID), must not touch (bob.WorkspaceID, sharedUserID).
	removeReq := th.AuthRequest(http.MethodDelete, "/api/v1/workspace/members/"+sharedUserID, nil, alice.Cookie)
	removeResp, err := env.App.Test(removeReq)
	if err != nil {
		t.Fatalf("remove request: %v", err)
	}
	if removeResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 removing shared user from alice's workspace, got %d", removeResp.StatusCode)
	}

	// Alice's workspace: access must now be denied.
	afterRemovalReq := th.BearerRequest(http.MethodPost, "/api/v1/projects",
		th.JSONBody(api.CreateProjectRequest{Name: "p2"}), tokenInAlice)
	afterRemovalResp, err := env.App.Test(afterRemovalReq)
	if err != nil {
		t.Fatalf("post-removal request: %v", err)
	}
	if afterRemovalResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 in alice's workspace after removal, got %d", afterRemovalResp.StatusCode)
	}

	// Bob's workspace: the owner role cached earlier must be entirely
	// unaffected by the invalidation scoped to alice's workspace.
	stillOwnerReq := th.BearerRequest(http.MethodPut, "/api/v1/workspace/settings",
		th.JSONBody(api.UpdateWorkspaceSettingsRequest{VersionRetentionCount: 7}), tokenInBob)
	stillOwnerResp, err := env.App.Test(stillOwnerReq)
	if err != nil {
		t.Fatalf("bob workspace request: %v", err)
	}
	if stillOwnerResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 in bob's workspace (unaffected by alice's cache invalidation), got %d",
			stillOwnerResp.StatusCode)
	}
}

func statusOf(resp *http.Response) int {
	if resp == nil {
		return -1
	}
	return resp.StatusCode
}
