//go:build integration

package api_test

import (
	"context"
	"net/http"
	"testing"

	"encoding/json"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
	"damask/server/internal/testutil/fixtures"
)

func TestCreateProject_Success(t *testing.T) {
	env := testutil.NewTestEnv(t)
	color := "#3b82f6"
	desc := "Summer campaign"
	env.Projects.CreateFn = func(_ context.Context, wsID string, p service.CreateProjectParams) (*service.ProjectDTO, error) {
		return fixtures.Project(func(pr *service.ProjectDTO) {
			pr.WorkspaceID = wsID
			pr.Name = p.Name
			pr.Color = p.Color
			pr.Description = p.Description
		}), nil
	}

	cookie := env.MintCookie(t, "usr_1", "ws_1")
	req := testutil.AuthRequest(http.MethodPost, "/api/v1/projects",
		testutil.JSONBody(api.CreateProjectRequest{
			Name:        "Campaign 2024",
			Color:       &color,
			Description: &desc,
		}),
		cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusCreated)

	var p api.ProjectResponse
	testutil.DecodeJSON(t, resp, &p)
	if p.Name != "Campaign 2024" {
		t.Errorf("name = %q, want Campaign 2024", p.Name)
	}
	if p.Color == nil || *p.Color != "#3b82f6" {
		t.Errorf("color = %v, want #3b82f6", p.Color)
	}
	if p.Description == nil || *p.Description != "Summer campaign" {
		t.Errorf("description = %v, want Summer campaign", p.Description)
	}
}

func TestCreateProject_ViewerRejected(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetMemberFn = func(_ context.Context, _, uID string) (*service.MemberDTO, error) {
		return &service.MemberDTO{UserID: uID, Role: string(auth.Viewer)}, nil
	}
	token := env.MintToken(t, "usr_viewer", "ws_1")

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/projects",
		testutil.JSONBody(api.CreateProjectRequest{Name: "My Project"}), token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestListProjects_Empty(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Projects.ListFn = func(_ context.Context, _ string) ([]*service.ProjectDTO, error) {
		return []*service.ProjectDTO{}, nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodGet, "/api/v1/projects", nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var items []api.ProjectResponse
	testutil.DecodeJSON(t, resp, &items)
	if len(items) != 0 {
		t.Errorf("expected 0 projects, got %d", len(items))
	}
}

func TestListProjects_WithCount(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Projects.ListFn = func(_ context.Context, _ string) ([]*service.ProjectDTO, error) {
		return []*service.ProjectDTO{
			fixtures.Project(func(p *service.ProjectDTO) { p.AssetCount = 1 }),
		}, nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodGet, "/api/v1/projects", nil, cookie)
	resp, _ := env.App.Test(req)
	var items []api.ProjectResponse
	_ = json.NewDecoder(resp.Body).Decode(&items)

	if len(items) != 1 {
		t.Fatalf("expected 1 project, got %d", len(items))
	}
	if items[0].AssetCount != 1 {
		t.Errorf("asset_count = %d, want 1", items[0].AssetCount)
	}
}

func TestGetProject_Success(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Projects.GetFn = func(_ context.Context, _, id string) (*service.ProjectDTO, error) {
		return fixtures.Project(func(p *service.ProjectDTO) { p.ID = id }), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodGet, "/api/v1/projects/prj_1", nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got api.ProjectResponse
	testutil.DecodeJSON(t, resp, &got)
	if got.ID != "prj_1" {
		t.Errorf("id = %q, want prj_1", got.ID)
	}
}

func TestUpdateProject(t *testing.T) {
	env := testutil.NewTestEnv(t)
	newName := "New Name"
	newColor := "#112233"
	env.Projects.UpdateFn = func(_ context.Context, _, _ string, p service.UpdateProjectParams) (*service.ProjectDTO, error) {
		return fixtures.Project(func(pr *service.ProjectDTO) {
			pr.Name = *p.Name
			pr.Color = p.Color
		}), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPut, "/api/v1/projects/prj_1",
		testutil.JSONBody(api.UpdateProjectRequest{Name: &newName, Color: &newColor}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got api.ProjectResponse
	testutil.DecodeJSON(t, resp, &got)
	if got.Name != "New Name" {
		t.Errorf("name = %q, want New Name", got.Name)
	}
	if got.Color == nil || *got.Color != "#112233" {
		t.Errorf("color = %v, want #112233", got.Color)
	}
}

func TestUpdateProject_PartialName(t *testing.T) {
	env := testutil.NewTestEnv(t)
	origColor := "#aabbcc"
	origDesc := "original desc"
	newName := "Updated Name"
	env.Projects.UpdateFn = func(_ context.Context, _, _ string, p service.UpdateProjectParams) (*service.ProjectDTO, error) {
		return fixtures.Project(func(pr *service.ProjectDTO) {
			pr.Name = *p.Name
			pr.Color = &origColor
			pr.Description = &origDesc
		}), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPut, "/api/v1/projects/prj_1",
		testutil.JSONBody(api.UpdateProjectRequest{Name: &newName}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got api.ProjectResponse
	testutil.DecodeJSON(t, resp, &got)
	if got.Name != "Updated Name" {
		t.Errorf("name = %q, want Updated Name", got.Name)
	}
	if got.Color == nil || *got.Color != origColor {
		t.Errorf("color = %v, want %s (preserved)", got.Color, origColor)
	}
	if got.Description == nil || *got.Description != origDesc {
		t.Errorf("description = %v, want %q (preserved)", got.Description, origDesc)
	}
}

func TestUpdateProject_PartialColor(t *testing.T) {
	env := testutil.NewTestEnv(t)
	newColor := "#112233"
	env.Projects.UpdateFn = func(_ context.Context, _, _ string, p service.UpdateProjectParams) (*service.ProjectDTO, error) {
		origName := "My Project"
		return fixtures.Project(func(pr *service.ProjectDTO) {
			pr.Name = origName
			pr.Color = p.Color
		}), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPut, "/api/v1/projects/prj_1",
		testutil.JSONBody(api.UpdateProjectRequest{Color: &newColor}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got api.ProjectResponse
	testutil.DecodeJSON(t, resp, &got)
	if got.Color == nil || *got.Color != "#112233" {
		t.Errorf("color = %v, want #112233", got.Color)
	}
	if got.Name != "My Project" {
		t.Errorf("name = %q, want My Project (preserved)", got.Name)
	}
}

func TestUpdateProject_PartialDescription_PreservesCoverAsset(t *testing.T) {
	env := testutil.NewTestEnv(t)
	assetID := "ast_cover_1"
	newDesc := "updated desc"
	env.Projects.UpdateFn = func(_ context.Context, _, _ string, p service.UpdateProjectParams) (*service.ProjectDTO, error) {
		return fixtures.Project(func(pr *service.ProjectDTO) {
			pr.Description = p.Description
			pr.CoverAssetID = &assetID
		}), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPut, "/api/v1/projects/prj_1",
		testutil.JSONBody(api.UpdateProjectRequest{Description: &newDesc}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got api.ProjectResponse
	testutil.DecodeJSON(t, resp, &got)
	if got.Description == nil || *got.Description != "updated desc" {
		t.Errorf("description = %v, want updated desc", got.Description)
	}
	if got.CoverAssetID == nil || *got.CoverAssetID != assetID {
		t.Errorf("cover_asset_id = %v, want %s (preserved)", got.CoverAssetID, assetID)
	}
}

func TestDeleteProject_Success(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Projects.DeleteFn = func(_ context.Context, _, _ string) error { return nil }
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodDelete, "/api/v1/projects/prj_1", nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusNoContent)
}
