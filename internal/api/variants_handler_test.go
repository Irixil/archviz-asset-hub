package api_test

import (
	"context"
	"net/http"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/auth"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

func TestListVariantsCoveringWorkflowPresent(t *testing.T) {
	env := testutil.NewTestEnv(t)
	projectID := "prj_1"
	folderID := "fld_1"
	env.Assets.GetFn = func(_ context.Context, workspaceID, assetID string) (*service.AssetDTO, error) {
		return &service.AssetDTO{ID: assetID, WorkspaceID: workspaceID, ProjectID: &projectID, FolderID: &folderID}, nil
	}
	env.Variants.ListFn = func(_ context.Context, p service.ListVariantsParams) (*service.ListVariantsResult, error) {
		if p.AssetProjectID != projectID || p.AssetFolderID != folderID {
			t.Fatalf("unexpected list params: %#v", p)
		}
		return &service.ListVariantsResult{
			CoveringWorkflow: &service.CoveringWorkflowDTO{ID: "wf_1", Name: "Auto resize", Scope: "project"},
		}, nil
	}
	req := testutil.BearerRequest(
		http.MethodGet,
		"/api/v1/assets/ast_1/variants",
		nil,
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
	var body map[string]any
	testutil.DecodeJSON(t, resp, &body)
	cw := body["covering_workflow"].(map[string]any)
	if cw["id"] != "wf_1" || cw["workflow_url"] == "" {
		t.Fatalf("unexpected covering workflow response: %#v", cw)
	}
}

func TestAutomateVariantsOK(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workflows.CreateFromVariantsFn = func(_ context.Context, workspaceID string, p service.CreateVariantAutomationParams) (*service.WorkflowDTO, error) {
		if workspaceID != "ws_1" || p.AssetID != "ast_1" || p.Scope != service.AutomationScopeProject ||
			p.CreatedBy != "usr_1" {
			t.Fatalf("unexpected create params: workspace=%s params=%#v", workspaceID, p)
		}
		return &service.WorkflowDTO{ID: "wf_new"}, nil
	}
	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/assets/ast_1/variants/automate",
		testutil.JSONBody(map[string]string{"scope": "project"}),
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusCreated)
	var body map[string]any
	testutil.DecodeJSON(t, resp, &body)
	if body["workflow_id"] != "wf_new" || body["workflow_url"] == "" {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestAutomateVariantsConflictAndViewer(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workflows.CreateFromVariantsFn = func(context.Context, string, service.CreateVariantAutomationParams) (*service.WorkflowDTO, error) {
		return nil, apperr.ErrConflict
	}
	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/assets/ast_1/variants/automate",
		testutil.JSONBody(map[string]string{"scope": "project"}),
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusConflict)

	env = testutil.NewTestEnv(t)
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return &service.MemberDTO{UserID: userID, Role: string(auth.Viewer)}, nil
	}
	req = testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/assets/ast_1/variants/automate",
		testutil.JSONBody(map[string]string{"scope": "project"}),
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err = env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}
