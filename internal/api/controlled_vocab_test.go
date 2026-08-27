//go:build integration

package api_test

import (
	"context"
	"net/http"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

// --- helpers -----------------------------------------------------------------

func lockedWorkspaceDTO(id string) *service.WorkspaceDTO {
	return &service.WorkspaceDTO{ID: id, Name: "Locked WS", LockedTaxonomy: true}
}

func unlockedWorkspaceDTO(id string) *service.WorkspaceDTO {
	return &service.WorkspaceDTO{ID: id, Name: "Open WS", LockedTaxonomy: false}
}

func memberDTO(userID, role string) *service.MemberDTO {
	return &service.MemberDTO{UserID: userID, Role: role}
}

// --- CV-1.2: PUT /workspace/settings (locked_taxonomy) ----------------------

func TestLockedTaxonomy_OwnerCanEnable(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.UpdateFn = func(_ context.Context, wsID string, p service.UpdateWorkspaceParams) (*service.WorkspaceDTO, error) {
		if p.LockedTaxonomy == nil || !*p.LockedTaxonomy {
			t.Error("expected LockedTaxonomy=true")
		}
		return &service.WorkspaceDTO{ID: wsID, LockedTaxonomy: true}, nil
	}

	tok := env.MintToken(t, "usr_1", "ws_1")
	req := testutil.BearerRequest(http.MethodPut, "/api/v1/workspace/settings",
		testutil.JSONBody(map[string]any{"locked_taxonomy": true}), tok)
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestLockedTaxonomy_EditorCannotEnable(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return memberDTO(userID, "editor"), nil
	}

	tok := env.MintToken(t, "usr_editor", "ws_1")
	req := testutil.BearerRequest(http.MethodPut, "/api/v1/workspace/settings",
		testutil.JSONBody(map[string]any{"locked_taxonomy": true}), tok)
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestLockedTaxonomy_ViewerCannotEnable(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return memberDTO(userID, "viewer"), nil
	}

	tok := env.MintToken(t, "usr_viewer", "ws_1")
	req := testutil.BearerRequest(http.MethodPut, "/api/v1/workspace/settings",
		testutil.JSONBody(map[string]any{"locked_taxonomy": true}), tok)
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

// --- CV-1.3: POST /assets/:id/tags locked-mode checks -----------------------

func TestAddTag_LockedWorkspace_ExistingTag_OK(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetFn = func(_ context.Context, id string) (*service.WorkspaceDTO, error) {
		return lockedWorkspaceDTO(id), nil
	}
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return memberDTO(userID, "editor"), nil
	}
	env.Tags.GetByNameFn = func(_ context.Context, _, _ string) (*service.TagDTO, error) {
		return &service.TagDTO{Name: "existing"}, nil
	}
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return &service.AssetDTO{ID: "ast_1"}, nil
	}
	env.Tags.AddToAssetFn = func(_ context.Context, _, _, name string) (*service.TagDTO, error) {
		return &service.TagDTO{Name: name}, nil
	}

	tok := env.MintToken(t, "usr_editor", "ws_1")
	req := testutil.BearerRequest(http.MethodPost, "/api/v1/assets/ast_1/tags",
		testutil.JSONBody(map[string]string{"name": "existing"}), tok)
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusCreated)
}

func TestAddTag_LockedWorkspace_NewTag_NonOwner_Rejected(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetFn = func(_ context.Context, id string) (*service.WorkspaceDTO, error) {
		return lockedWorkspaceDTO(id), nil
	}
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return memberDTO(userID, "editor"), nil
	}
	env.Tags.GetByNameFn = func(_ context.Context, _, _ string) (*service.TagDTO, error) {
		return nil, apperr.ErrNotFound
	}
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return &service.AssetDTO{ID: "ast_1"}, nil
	}

	tok := env.MintToken(t, "usr_editor", "ws_1")
	req := testutil.BearerRequest(http.MethodPost, "/api/v1/assets/ast_1/tags",
		testutil.JSONBody(map[string]string{"name": "brandnew"}), tok)
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestAddTag_LockedWorkspace_NewTag_Owner_OK(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetFn = func(_ context.Context, id string) (*service.WorkspaceDTO, error) {
		return lockedWorkspaceDTO(id), nil
	}
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return memberDTO(userID, "owner"), nil
	}
	env.Tags.GetByNameFn = func(_ context.Context, _, _ string) (*service.TagDTO, error) {
		return nil, apperr.ErrNotFound
	}
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return &service.AssetDTO{ID: "ast_1"}, nil
	}
	env.Tags.AddToAssetFn = func(_ context.Context, _, _, name string) (*service.TagDTO, error) {
		return &service.TagDTO{Name: name}, nil
	}

	tok := env.MintToken(t, "usr_owner", "ws_1")
	req := testutil.BearerRequest(http.MethodPost, "/api/v1/assets/ast_1/tags",
		testutil.JSONBody(map[string]string{"name": "brandnew"}), tok)
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusCreated)
}

// --- CV-1.4: POST /tags locked-mode -----------------------------------------

func TestCreateTag_LockedWorkspace_NonOwner_Forbidden(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetFn = func(_ context.Context, id string) (*service.WorkspaceDTO, error) {
		return lockedWorkspaceDTO(id), nil
	}
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return memberDTO(userID, "editor"), nil
	}

	tok := env.MintToken(t, "usr_editor", "ws_1")
	req := testutil.BearerRequest(http.MethodPost, "/api/v1/tags",
		testutil.JSONBody(map[string]string{"name": "newtag"}), tok)
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

// --- CV-1.5: GET /tags hide_empty -------------------------------------------

func TestGetTags_LockedWorkspace_HidesEmptyForNonOwner(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetFn = func(_ context.Context, id string) (*service.WorkspaceDTO, error) {
		return lockedWorkspaceDTO(id), nil
	}
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return memberDTO(userID, "editor"), nil
	}
	env.Tags.ListFn = func(_ context.Context, _ string, _ bool) ([]*service.TagDTO, error) {
		return []*service.TagDTO{
			{Name: "used", AssetCount: 3},
			{Name: "unused", AssetCount: 0},
		}, nil
	}

	tok := env.MintToken(t, "usr_editor", "ws_1")
	req := testutil.BearerRequest(http.MethodGet, "/api/v1/tags", nil, tok)

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var tags []map[string]any
	testutil.DecodeJSON(t, resp, &tags)
	if len(tags) != 1 {
		t.Errorf("expected 1 tag (unused hidden), got %d", len(tags))
	}
	if tags[0]["name"] != "used" {
		t.Errorf("expected tag=used, got %v", tags[0]["name"])
	}
}

func TestGetTags_LockedWorkspace_ShowsEmptyForOwner(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetFn = func(_ context.Context, id string) (*service.WorkspaceDTO, error) {
		return lockedWorkspaceDTO(id), nil
	}
	// GetMember default returns owner
	env.Tags.ListFn = func(_ context.Context, _ string, _ bool) ([]*service.TagDTO, error) {
		return []*service.TagDTO{
			{Name: "used", AssetCount: 3},
			{Name: "unused", AssetCount: 0},
		}, nil
	}

	tok := env.MintToken(t, "usr_owner", "ws_1")
	req := testutil.BearerRequest(http.MethodGet, "/api/v1/tags", nil, tok)

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var tags []map[string]any
	testutil.DecodeJSON(t, resp, &tags)
	if len(tags) != 2 {
		t.Errorf("expected 2 tags (owner sees all), got %d", len(tags))
	}
}

func TestGetTags_UnlockedWorkspace_ShowsAll(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetFn = func(_ context.Context, id string) (*service.WorkspaceDTO, error) {
		return unlockedWorkspaceDTO(id), nil
	}
	env.Tags.ListFn = func(_ context.Context, _ string, _ bool) ([]*service.TagDTO, error) {
		return []*service.TagDTO{
			{Name: "used", AssetCount: 3},
			{Name: "unused", AssetCount: 0},
		}, nil
	}

	tok := env.MintToken(t, "usr_editor", "ws_1")
	req := testutil.BearerRequest(http.MethodGet, "/api/v1/tags", nil, tok)

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var tags []map[string]any
	testutil.DecodeJSON(t, resp, &tags)
	if len(tags) != 2 {
		t.Errorf("expected 2 tags (unlocked shows all), got %d", len(tags))
	}
}
