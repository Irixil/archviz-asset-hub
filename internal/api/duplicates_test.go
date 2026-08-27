//go:build integration

package api_test

import (
	"damask/server/internal/api"
	th "damask/server/internal/testhelpers"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// doUpload runs an upload request and fails the test on transport errors,
// mirroring the fiber.TestConfig timeout used throughout assets_test.go.
func doUpload(t *testing.T, env *th.TestEnv, req *http.Request) *http.Response {
	t.Helper()
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// setDuplicateDetectionMode sets the workspace's duplicate_detection_mode via
// the existing PUT /workspace/settings endpoint (owner-only).
func setDuplicateDetectionMode(t *testing.T, env *th.TestEnv, cookie *http.Cookie, mode string) {
	t.Helper()
	req := th.AuthRequest(http.MethodPut, "/api/v1/workspace/settings",
		th.JSONBody(api.UpdateWorkspaceSettingsRequest{DuplicateDetectionMode: &mode}), cookie)
	resp := doUpload(t, env, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("set duplicate_detection_mode: expected 200, got %d: %s", resp.StatusCode, body)
	}
}

func TestUploadAsset_DuplicateWarn_ReturnsDuplicateOf(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	content := th.MakeJPEG(64, 64)

	first := doUpload(t, env, th.BuildUploadRequest(t, "first.jpg", content, owner.Cookie))
	var firstAsset api.AssetResponse
	if err := json.NewDecoder(first.Body).Decode(&firstAsset); err != nil {
		t.Fatalf("decode first response: %v", err)
	}

	second := doUpload(t, env, th.BuildUploadRequest(t, "second.jpg", content, owner.Cookie))
	if second.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(second.Body)
		t.Fatalf("expected 201, got %d: %s", second.StatusCode, body)
	}
	var secondAsset api.AssetResponse
	if err := json.NewDecoder(second.Body).Decode(&secondAsset); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if secondAsset.DuplicateOf == nil {
		t.Fatal("expected duplicate_of to be present on the second upload")
	}
	if secondAsset.DuplicateOf.AssetID != firstAsset.ID {
		t.Errorf("duplicate_of.asset_id: got %q, want %q", secondAsset.DuplicateOf.AssetID, firstAsset.ID)
	}
}

func TestUploadAsset_DuplicateBlock_Returns409(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	setDuplicateDetectionMode(t, env, owner.Cookie, "block")
	content := th.MakeJPEG(64, 64)

	first := doUpload(t, env, th.BuildUploadRequest(t, "first.jpg", content, owner.Cookie))
	if first.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(first.Body)
		t.Fatalf("expected first upload to succeed, got %d: %s", first.StatusCode, body)
	}
	var firstAsset api.AssetResponse
	if err := json.NewDecoder(first.Body).Decode(&firstAsset); err != nil {
		t.Fatalf("decode first response: %v", err)
	}

	second := doUpload(t, env, th.BuildUploadRequest(t, "second.jpg", content, owner.Cookie))
	if second.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(second.Body)
		t.Fatalf("expected 409, got %d: %s", second.StatusCode, body)
	}
	var body map[string]any
	if err := json.NewDecoder(second.Body).Decode(&body); err != nil {
		t.Fatalf("decode 409 body: %v", err)
	}
	if body["error"] != "duplicate_content" {
		t.Errorf("error: got %v, want %q", body["error"], "duplicate_content")
	}
	dup, _ := body["duplicate_of"].(map[string]any)
	if dup == nil || dup["asset_id"] != firstAsset.ID {
		t.Errorf("duplicate_of.asset_id: got %v, want %q", dup, firstAsset.ID)
	}

	// The blocked upload must not be left in the library.
	listReq := th.AuthRequest(http.MethodGet, "/api/v1/assets", nil, owner.Cookie)
	listResp := doUpload(t, env, listReq)
	var list api.AssetListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(list.Assets) != 1 {
		t.Fatalf("expected exactly 1 asset after a blocked duplicate, got %d", len(list.Assets))
	}
}

func TestUploadAsset_NoDuplicate_OmitsField(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	content := th.MakeJPEG(64, 64)

	resp := doUpload(t, env, th.BuildUploadRequest(t, "unique.jpg", content, owner.Cookie))
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.Contains(string(raw), "duplicate_of") {
		t.Errorf("expected no duplicate_of key in the response, got: %s", raw)
	}
}

func TestUploadAsset_DuplicateOff_NeverChecks(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	setDuplicateDetectionMode(t, env, owner.Cookie, "off")
	content := th.MakeJPEG(64, 64)

	first := doUpload(t, env, th.BuildUploadRequest(t, "first.jpg", content, owner.Cookie))
	if first.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(first.Body)
		t.Fatalf("expected first upload to succeed, got %d: %s", first.StatusCode, body)
	}

	second := doUpload(t, env, th.BuildUploadRequest(t, "second.jpg", content, owner.Cookie))
	if second.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(second.Body)
		t.Fatalf("expected second upload to succeed in off mode, got %d: %s", second.StatusCode, body)
	}
	raw, err := io.ReadAll(second.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.Contains(string(raw), "duplicate_of") {
		t.Errorf("expected no duplicate check in off mode, got: %s", raw)
	}
}
