//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	th "damask/server/internal/testhelpers"
)

func urlQueryEscapeForTest(s string) string { return url.QueryEscape(s) }

// ---- Create variant ----

func TestCreateVariant_CustomFFmpeg_HappyPath_Returns202(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	paramsData := json.RawMessage(`{"command":"ffmpeg -i {input} -c copy {output}"}`)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "custom_ffmpeg", Params: paramsData}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var result api.CreateVariantResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.JobID == "" {
		t.Error("expected job_id in response")
	}

	var payload string
	if err := env.Database.QueryRow(`SELECT payload FROM jobs WHERE id = ?`, result.JobID).Scan(&payload); err != nil {
		t.Fatalf("load job payload: %v", err)
	}
	var jobPayload map[string]any
	if err := json.Unmarshal([]byte(payload), &jobPayload); err != nil {
		t.Fatalf("decode job payload: %v", err)
	}
	params, ok := jobPayload["params"].(map[string]any)
	if !ok {
		t.Fatalf("expected params object in payload, got %#v", jobPayload["params"])
	}
	if params["command"] != "ffmpeg -i {input} -c copy {output}" {
		t.Fatalf("unexpected command in payload: %#v", params["command"])
	}
}

func TestCreateVariant_CustomFFmpeg_MissingCommand_Returns422(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "custom_ffmpeg", Params: json.RawMessage(`{"command":""}`)}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_CustomFFmpeg_MissingInputToken_Returns422(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	params := json.RawMessage(`{"command":"ffmpeg -i src.mp4 -c copy {output}"}`)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "custom_ffmpeg", Params: params}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_CustomFFmpeg_MissingOutputToken_Returns422(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	params := json.RawMessage(`{"command":"ffmpeg -i {input} -c copy out.mp4"}`)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "custom_ffmpeg", Params: params}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_CustomFFmpeg_BlacklistedFlag_Returns422(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	params := json.RawMessage(`{"command":"ffmpeg -i {input} -c copy {output}; rm -rf /"}`)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "custom_ffmpeg", Params: params}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_CustomFFmpeg_ViewerForbidden_Returns403(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, ownerCookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, ownerCookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	viewerToken := th.MintEditorToken(t, env, a.WorkspaceID, auth.Viewer)
	params := json.RawMessage(`{"command":"ffmpeg -i {input} -c copy {output}"}`)
	createReq := th.BearerRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "custom_ffmpeg", Params: params}), viewerToken)
	createResp, _ := env.App.Test(createReq)
	if createResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer create variant, got %d", createResp.StatusCode)
	}
}

func TestCreateVariant_CustomFFmpeg_WorkspaceIsolation_Returns404(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, _ := createTestAsset(t, env)

	other := th.Register(t, env, "Other User", "ffmpeg-other@test.com", "password123")
	params := json.RawMessage(`{"command":"ffmpeg -i {input} -c copy {output}"}`)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "custom_ffmpeg", Params: params}), other.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-workspace asset, got %d", resp.StatusCode)
	}
}

// ---- Validate command endpoint ----

func TestValidateCommand_Valid_Returns200(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Validate User", "validate@test.com", "password123")

	req := th.AuthRequest(http.MethodGet,
		"/api/v1/variants/validate-command?q="+urlQueryEscapeForTest("ffmpeg -i {input} -c copy {output}"),
		nil, res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result api.ValidateCommandResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if !result.Valid {
		t.Fatalf("expected valid=true, got %+v", result)
	}
}

func TestValidateCommand_MissingOutput_Returns200WithValidFalse(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Validate User 2", "validate2@test.com", "password123")

	req := th.AuthRequest(http.MethodGet,
		"/api/v1/variants/validate-command?q="+urlQueryEscapeForTest("ffmpeg -i {input} -c copy out.mp4"),
		nil, res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result api.ValidateCommandResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Valid || result.Error != "missing_output_token" {
		t.Fatalf("unexpected response: %+v", result)
	}
}

func TestValidateCommand_Blacklisted_Returns200WithValidFalse(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Validate User 3", "validate3@test.com", "password123")

	req := th.AuthRequest(http.MethodGet,
		"/api/v1/variants/validate-command?q="+urlQueryEscapeForTest("ffmpeg -i {input} -c copy {output}; rm -rf /"),
		nil, res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result api.ValidateCommandResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Valid || result.Error != "command_blacklisted" || result.Detail == "" {
		t.Fatalf("unexpected response: %+v", result)
	}
}

func TestValidateCommand_TooLong_Returns200WithCommandTooLong(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Validate User 4", "validate4@test.com", "password123")

	cmd := "ffmpeg -i {input} -c copy {output} " + strings.Repeat("a", 2000)
	req := th.AuthRequest(http.MethodGet,
		"/api/v1/variants/validate-command?q="+urlQueryEscapeForTest(cmd),
		nil, res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result api.ValidateCommandResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Valid || result.Error != "command_too_long" {
		t.Fatalf("unexpected response: %+v", result)
	}
}

func TestValidateCommand_MalformedRefToken_Returns200WithValidFalse(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Validate User 5", "validate5@test.com", "password123")

	req := th.AuthRequest(http.MethodGet,
		"/api/v1/variants/validate-command?q="+urlQueryEscapeForTest("ffmpeg -i {input} -i {asset:} {output}"),
		nil, res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result api.ValidateCommandResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Valid || result.Error != "bad_ref_token" {
		t.Fatalf("unexpected response: %+v", result)
	}
}

func TestValidateCommand_WellFormedRefToken_Returns200WithValidTrue(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Validate User 6", "validate6@test.com", "password123")

	req := th.AuthRequest(http.MethodGet,
		"/api/v1/variants/validate-command?q="+
			urlQueryEscapeForTest("ffmpeg -i {input} -i {asset:01ARZ3NDEKTSV4RRFFQ69G5FAV} {output}"),
		nil, res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result api.ValidateCommandResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if !result.Valid {
		t.Fatalf("expected valid=true, got %+v", result)
	}
}

func TestValidateCommand_Unauthenticated_Returns401(t *testing.T) {
	env := th.SetupTestApp(t)

	req := th.AuthRequest(http.MethodGet,
		"/api/v1/variants/validate-command?q="+urlQueryEscapeForTest("ffmpeg -i {input} -c copy {output}"),
		nil, nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
