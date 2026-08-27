package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/apperr"
	"damask/server/internal/auth"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

func TestListWorkflowsOK(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workflows.ListFn = func(
		_ context.Context,
		workspaceID string,
		_ service.ListWorkflowsParams,
	) ([]service.WorkflowDTO, error) {
		return []service.WorkflowDTO{{ID: "wf_1", WorkspaceID: workspaceID, Name: "Auto Resize"}}, nil
	}
	req := testutil.BearerRequest(http.MethodGet, "/api/v1/workflows", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestListWorkflowsEmptyWorkspaceReturnsPersistedRowsOnly(t *testing.T) {
	env := testutil.NewTestEnv(t)
	createCalls := 0
	env.Workflows.ListFn = func(
		_ context.Context,
		workspaceID string,
		_ service.ListWorkflowsParams,
	) ([]service.WorkflowDTO, error) {
		if workspaceID != "ws_1" {
			t.Fatalf("workspaceID = %q, want ws_1", workspaceID)
		}
		return []service.WorkflowDTO{}, nil
	}
	env.Workflows.CreateFn = func(_ context.Context, _, _ string, _ service.CreateWorkflowParams) (*service.WorkflowDTO, error) {
		createCalls++
		return nil, nil //nolint:nilnil // test
	}

	req := testutil.BearerRequest(http.MethodGet, "/api/v1/workflows", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got []map[string]any
	testutil.DecodeJSON(t, resp, &got)
	if len(got) != 0 {
		t.Fatalf("expected no persisted workflows, got %d", len(got))
	}
	if createCalls != 0 {
		t.Fatalf("Create() called %d times during list, want 0", createCalls)
	}
}

func TestListWorkflowsFilterByTriggerType(t *testing.T) {
	env := testutil.NewTestEnv(t)
	var got service.ListWorkflowsParams
	env.Workflows.ListFn = func(
		_ context.Context,
		workspaceID string,
		params service.ListWorkflowsParams,
	) ([]service.WorkflowDTO, error) {
		if workspaceID != "ws_1" {
			t.Fatalf("workspaceID = %q, want ws_1", workspaceID)
		}
		got = params
		return []service.WorkflowDTO{{ID: "wf_1", WorkspaceID: workspaceID, Name: "Manual", Enabled: true}}, nil
	}
	req := testutil.BearerRequest(http.MethodGet,
		"/api/v1/workflows?trigger_type=trigger.manual&enabled_only=true", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
	if got.TriggerType == nil || *got.TriggerType != "trigger.manual" || !got.EnabledOnly {
		t.Fatalf("params = %#v", got)
	}
	var body []map[string]any
	testutil.DecodeJSON(t, resp, &body)
	if len(body) != 1 || body[0]["id"] != "wf_1" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestBulkManualRunOKReturns202(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workflows.TriggerManualBulkFn = func(_ context.Context, workspaceID, workflowID string, assetIDs []string) ([]string, error) {
		if workspaceID != "ws_1" || workflowID != "wf_1" || len(assetIDs) != 3 {
			t.Fatalf("unexpected call: workspace=%q workflow=%q assets=%v", workspaceID, workflowID, assetIDs)
		}
		return []string{"r1", "r2", "r3"}, nil
	}
	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/workflows/wf_1/runs/bulk",
		strings.NewReader(`{"asset_ids":["a1","a2","a3"]}`),
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusAccepted)
	var body map[string]any
	testutil.DecodeJSON(t, resp, &body)
	if body["count"].(float64) != 3 {
		t.Fatalf("body = %#v", body)
	}
}

func TestBulkManualRunUnauthenticatedReturns401(t *testing.T) {
	env := testutil.NewTestEnv(t)
	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/workflows/wf_1/runs/bulk",
		strings.NewReader(`{"asset_ids":["a1"]}`),
		"",
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestBulkManualRunViewerReturns403(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return &service.MemberDTO{UserID: userID, Role: string(auth.Viewer)}, nil
	}
	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/workflows/wf_1/runs/bulk",
		strings.NewReader(`{"asset_ids":["a1"]}`),
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestBulkManualRunDisabledWorkflowReturns409(t *testing.T) {
	assertBulkManualRunError(t, apperr.ErrConflict, http.StatusConflict)
}

func TestBulkManualRunEmptyAssetIDsReturns422(t *testing.T) {
	assertBulkManualRunError(t, apperr.ErrInvalidInput, http.StatusUnprocessableEntity)
}

func TestBulkManualRunWorkflowNotFoundReturns404(t *testing.T) {
	assertBulkManualRunError(t, apperr.ErrNotFound, http.StatusNotFound)
}

func assertBulkManualRunError(t *testing.T, svcErr error, wantStatus int) {
	t.Helper()
	env := testutil.NewTestEnv(t)
	env.Workflows.TriggerManualBulkFn = func(_ context.Context, _, _ string, _ []string) ([]string, error) {
		return nil, svcErr
	}
	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/workflows/wf_1/runs/bulk",
		strings.NewReader(`{"asset_ids":[]}`),
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, wantStatus)
	if svcErr == nil {
		t.Fatal("test helper requires an error")
	}
}

func TestManualWorkflowRun_NoBody_CallsWithEmptyAssetID(t *testing.T) {
	env := testutil.NewTestEnv(t)
	var capturedAssetID string
	env.Workflows.TriggerManualFn = func(_ context.Context, _, _ string, assetID string) (string, error) {
		capturedAssetID = assetID
		return "run_1", nil
	}
	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/workflows/wf_1/runs",
		nil,
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusAccepted)
	if capturedAssetID != "" {
		t.Fatalf("assetID: got %q, want empty", capturedAssetID)
	}
}

func TestManualWorkflowRun_WithAssetID_ForwardsAssetID(t *testing.T) {
	env := testutil.NewTestEnv(t)
	var capturedAssetID string
	env.Workflows.TriggerManualFn = func(_ context.Context, _, _ string, assetID string) (string, error) {
		capturedAssetID = assetID
		return "run_1", nil
	}
	body := strings.NewReader(`{"asset_id":"ast_1"}`)
	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/workflows/wf_1/runs",
		body,
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusAccepted)
	if capturedAssetID != "ast_1" {
		t.Fatalf("assetID: got %q, want ast_1", capturedAssetID)
	}
}

func TestGetWorkflowTemplatesOK(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workflows.TemplatesFn = func() []service.WorkflowTemplateDTO {
		return []service.WorkflowTemplateDTO{{
			ID:          "blank-manual",
			Name:        "Start Blank",
			Description: "Manual trigger.",
			TriggerType: "trigger.manual",
			Graph:       `{"nodes":[],"edges":[]}`,
		}}
	}
	req := testutil.BearerRequest(http.MethodGet, "/api/v1/workflows/templates", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestListTemplatesAllFieldsPresent(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workflows.TemplatesFn = func() []service.WorkflowTemplateDTO {
		return []service.WorkflowTemplateDTO{{
			ID:          "blank-manual",
			Name:        "Start Blank",
			Description: "Manual trigger.",
			TriggerType: "trigger.manual",
			Graph:       `{"nodes":[{"type":"trigger.manual"}],"edges":[]}`,
			Icon:        "plus",
			Category:    "",
		}}
	}
	req := testutil.BearerRequest(http.MethodGet, "/api/v1/workflows/templates", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
	var out []api.WorkflowTemplateResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&out); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 template, got %d", len(out))
	}
	tpl := out[0]
	if tpl.Name == "" || tpl.Description == "" || tpl.Icon == "" {
		t.Fatalf("expected non-empty Name/Description/Icon, got %+v", tpl)
	}
}

func TestListTemplatesIncludesBlankManual(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workflows.TemplatesFn = func() []service.WorkflowTemplateDTO {
		return []service.WorkflowTemplateDTO{
			{
				ID:          "blank-manual",
				Name:        "Start Blank",
				Description: "Manual trigger.",
				TriggerType: "trigger.manual",
				Graph:       `{"nodes":[{"id":"trigger","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
				Icon:        "plus",
			},
		}
	}
	req := testutil.BearerRequest(http.MethodGet, "/api/v1/workflows/templates", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
	var out []api.WorkflowTemplateResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&out); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	var blank *api.WorkflowTemplateResponse
	for i := range out {
		if out[i].ID == "blank-manual" {
			blank = &out[i]
		}
	}
	if blank == nil {
		t.Fatal("expected blank-manual template in response")
	}
	var graph struct {
		Nodes []struct {
			Type string `json:"type"`
		} `json:"nodes"`
	}
	if unmarshalErr := json.Unmarshal([]byte(blank.Graph), &graph); unmarshalErr != nil {
		t.Fatal(unmarshalErr)
	}
	if len(graph.Nodes) != 1 || graph.Nodes[0].Type != "trigger.manual" {
		t.Fatalf("expected single trigger.manual node, got %+v", graph.Nodes)
	}
}

func TestGetWorkflowTemplatesDoesNotMaterializeWorkflowRows(t *testing.T) {
	env := testutil.NewTestEnv(t)
	createCalls := 0
	env.Workflows.TemplatesFn = func() []service.WorkflowTemplateDTO {
		return []service.WorkflowTemplateDTO{{
			ID:          "blank-manual",
			Name:        "Start Blank",
			Description: "Manual trigger.",
			TriggerType: "trigger.manual",
			Graph:       `{"nodes":[],"edges":[]}`,
		}}
	}
	env.Workflows.CreateFn = func(_ context.Context, _, _ string, _ service.CreateWorkflowParams) (*service.WorkflowDTO, error) {
		createCalls++
		return nil, nil //nolint:nilnil // test
	}

	req := testutil.BearerRequest(http.MethodGet, "/api/v1/workflows/templates", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got []map[string]any
	testutil.DecodeJSON(t, resp, &got)
	if len(got) != 1 {
		t.Fatalf("expected 1 template, got %d", len(got))
	}
	if got[0]["id"] != "blank-manual" {
		t.Fatalf("template id = %v, want blank-manual", got[0]["id"])
	}
	if createCalls != 0 {
		t.Fatalf("Create() called %d times during templates fetch, want 0", createCalls)
	}
}
