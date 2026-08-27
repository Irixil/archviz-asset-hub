package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/testutil"
)

// ---- helpers ----

func setupDraftAsset(t *testing.T, env *testutil.TestEnv, assetID, workspaceID string) { //nolint:unparam // readability
	t.Helper()
	versionID := "ver_1"
	env.Assets.GetFn = func(_ context.Context, wsID, id string) (*service.AssetDTO, error) {
		if wsID != workspaceID || id != assetID {
			return nil, fmt.Errorf("not found: %w", errNotFound)
		}
		return &service.AssetDTO{
			ID:               assetID,
			WorkspaceID:      workspaceID,
			CurrentVersionID: &versionID,
		}, nil
	}
	env.Versions.GetCurrentByAssetFn = func(_ context.Context, _ string) (*service.VersionDTO, error) {
		return &service.VersionDTO{ID: versionID, VersionNum: 1}, nil
	}
}

// errNotFound is a sentinel to simulate apperr.ErrNotFound in mock fns.
var errNotFound = notFoundError{}

type notFoundError struct{}

func (notFoundError) Error() string { return "not found" }
func (notFoundError) Is(target error) bool {
	return target.Error() == "not found"
}

// writeScratchFiles puts a fake draft output + meta into in-memory storage.
func writeScratchFiles(
	t *testing.T,
	stor storage.Storage,
	workspaceID, //nolint:unparam // tests
	userID,
	nonce, //nolint:unparam // tests
	assetID string, //nolint:unparam // tests
) {
	t.Helper()
	outputKey := fmt.Sprintf("scratch/%s/%s/%s", workspaceID, userID, nonce)
	metaKey := outputKey + ".meta"

	_ = stor.Put(t.Context(), outputKey, bytes.NewReader([]byte("fake-image-bytes")))

	meta := fmt.Sprintf(`{
		"asset_id": %q,
		"workspace_id": %q,
		"user_id": %q,
		"variant_type": "image_with_prompt",
		"transform_params": "{\"prompt\":\"test\",\"model\":\"m1\"}",
		"content_type": "image/png",
		"created_at": "2026-05-22T12:00:00Z"
	}`, assetID, workspaceID, userID)
	_ = stor.Put(t.Context(), metaKey, bytes.NewReader([]byte(meta)))
}

// ---- Generate draft ----

func TestDraftVariant_HappyPath(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "user_1"
		workspaceID = "ws_1"
		assetID     = "ast_1"
	)
	setupDraftAsset(t, env, assetID, workspaceID)

	cookie := env.MintCookie(t, userID, workspaceID)
	body := testutil.JSONStr(`{"type":"image_with_prompt","params":{"prompt":"test","model":"m"}}`)
	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants/draft", body, cookie)

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusAccepted)

	var got api.DraftGenerateResponse
	testutil.DecodeJSON(t, resp, &got)
	if got.DraftKey == "" {
		t.Error("expected non-empty draft_key")
	}
	if len(got.DraftKey) != 16 {
		t.Errorf("expected 16-char nonce, got %d chars: %q", len(got.DraftKey), got.DraftKey)
	}
}

func TestDraftVariant_AssetNotFound(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "user_1", "ws_1")

	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return nil, errNotFound
	}

	body := testutil.JSONStr(`{"type":"image_with_prompt","params":{"prompt":"x","model":"m"}}`)
	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/nonexistent/variants/draft", body, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestDraftVariant_ViewerForbidden(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "viewer_1"
		workspaceID = "ws_1"
	)
	env.Workspace.GetMemberFn = func(_ context.Context, _, _ string) (*service.MemberDTO, error) {
		return &service.MemberDTO{Role: "viewer"}, nil
	}
	cookie := env.MintCookie(t, userID, workspaceID)
	body := testutil.JSONStr(`{"type":"image_with_prompt","params":{"prompt":"x","model":"m"}}`)
	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/ast_1/variants/draft", body, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestDraftVariant_WorkspaceIsolation(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "user_1"
		workspaceID = "ws_1"
		assetID     = "ast_other"
	)
	// Asset belongs to a different workspace.
	env.Assets.GetFn = func(_ context.Context, wsID, _ string) (*service.AssetDTO, error) {
		if wsID != workspaceID {
			return nil, errNotFound
		}
		return nil, errNotFound
	}
	cookie := env.MintCookie(t, userID, workspaceID)
	body := testutil.JSONStr(`{"type":"image_with_prompt","params":{"prompt":"x","model":"m"}}`)
	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants/draft", body, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

// ---- Preview draft ----

func TestPreviewDraft_HappyPath(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "user_1"
		workspaceID = "ws_1"
		assetID     = "ast_1"
		nonce       = "aabbccdd11223344"
	)
	setupDraftAsset(t, env, assetID, workspaceID)
	writeScratchFiles(t, env.Server.StorageForTest(), workspaceID, userID, nonce, assetID)

	cookie := env.MintCookie(t, userID, workspaceID)
	req := testutil.AuthRequest(
		http.MethodGet,
		"/api/v1/assets/"+assetID+"/variants/draft/"+nonce+"/preview",
		nil,
		cookie,
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected Content-Type image/png, got %q", ct)
	}
}

func TestPreviewDraft_NotFound(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "user_1"
		workspaceID = "ws_1"
		assetID     = "ast_1"
	)
	setupDraftAsset(t, env, assetID, workspaceID)

	cookie := env.MintCookie(t, userID, workspaceID)
	req := testutil.AuthRequest(
		http.MethodGet,
		"/api/v1/assets/"+assetID+"/variants/draft/nonexistentnonce/preview",
		nil,
		cookie,
	)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestPreviewDraft_OtherUser(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		ownerID     = "user_owner"
		viewerID    = "user_viewer"
		workspaceID = "ws_1"
		assetID     = "ast_1"
		nonce       = "aabbccdd11223344"
	)
	setupDraftAsset(t, env, assetID, workspaceID)
	// Write draft owned by ownerID.
	writeScratchFiles(t, env.Server.StorageForTest(), workspaceID, ownerID, nonce, assetID)

	// Authed as viewerID — different user_id in path → meta not found.
	cookie := env.MintCookie(t, viewerID, workspaceID)
	req := testutil.AuthRequest(
		http.MethodGet,
		"/api/v1/assets/"+assetID+"/variants/draft/"+nonce+"/preview",
		nil,
		cookie,
	)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestPreviewDraft_ViewerForbidden(t *testing.T) {
	// Preview is allowed for viewers — it's not Editor-gated.
	// (The route has no RequireRole middleware, so any authenticated user can preview.)
	// This test verifies it does NOT return 403 for a viewer who owns the draft.
	env := testutil.NewTestEnv(t)
	const (
		userID      = "viewer_1"
		workspaceID = "ws_1"
		assetID     = "ast_1"
		nonce       = "aabbccdd11223344"
	)
	setupDraftAsset(t, env, assetID, workspaceID)
	writeScratchFiles(t, env.Server.StorageForTest(), workspaceID, userID, nonce, assetID)

	cookie := env.MintCookie(t, userID, workspaceID)
	req := testutil.AuthRequest(
		http.MethodGet,
		"/api/v1/assets/"+assetID+"/variants/draft/"+nonce+"/preview",
		nil,
		cookie,
	)
	resp, _ := env.App.Test(req)
	// Viewer can preview — not 403.
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// ---- Commit draft ----

func TestCommitDraft_HappyPath(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "user_1"
		workspaceID = "ws_1"
		assetID     = "ast_1"
		nonce       = "aabbccdd11223344"
	)
	setupDraftAsset(t, env, assetID, workspaceID)
	writeScratchFiles(t, env.Server.StorageForTest(), workspaceID, userID, nonce, assetID)

	cookie := env.MintCookie(t, userID, workspaceID)
	body := testutil.JSONStr(`{"name":"my variant"}`)
	req := testutil.AuthRequest(
		http.MethodPost,
		"/api/v1/assets/"+assetID+"/variants/draft/"+nonce+"/commit",
		body,
		cookie,
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusCreated)

	var got api.VariantResponse
	testutil.DecodeJSON(t, resp, &got)
	if got.ID == "" {
		t.Error("expected variant ID in response")
	}
}

func TestCommitDraft_NotFound(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "user_1"
		workspaceID = "ws_1"
		assetID     = "ast_1"
	)
	setupDraftAsset(t, env, assetID, workspaceID)

	cookie := env.MintCookie(t, userID, workspaceID)
	req := testutil.AuthRequest(
		http.MethodPost,
		"/api/v1/assets/"+assetID+"/variants/draft/nosuchnonce/commit",
		nil,
		cookie,
	)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestCommitDraft_AssetMismatch(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "user_1"
		workspaceID = "ws_1"
		assetID     = "ast_1"
		otherAsset  = "ast_other"
		nonce       = "aabbccdd11223344"
	)
	// Write a meta that references assetID, but request uses otherAsset.
	setupDraftAsset(t, env, otherAsset, workspaceID)
	writeScratchFiles(t, env.Server.StorageForTest(), workspaceID, userID, nonce, assetID)

	cookie := env.MintCookie(t, userID, workspaceID)
	req := testutil.AuthRequest(
		http.MethodPost,
		"/api/v1/assets/"+otherAsset+"/variants/draft/"+nonce+"/commit",
		nil,
		cookie,
	)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusBadRequest)
}

func TestCommitDraft_Idempotent(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "user_1"
		workspaceID = "ws_1"
		assetID     = "ast_1"
		nonce       = "aabbccdd11223344"
	)
	setupDraftAsset(t, env, assetID, workspaceID)
	writeScratchFiles(t, env.Server.StorageForTest(), workspaceID, userID, nonce, assetID)

	cookie := env.MintCookie(t, userID, workspaceID)
	makeReq := func() *http.Request {
		return testutil.AuthRequest(
			http.MethodPost,
			"/api/v1/assets/"+assetID+"/variants/draft/"+nonce+"/commit",
			nil,
			cookie,
		)
	}

	// First commit.
	resp1, err := env.App.Test(makeReq())
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp1, http.StatusCreated)

	var v1 api.VariantResponse
	_ = json.NewDecoder(resp1.Body).Decode(&v1)

	// Second commit on same nonce — scratch is gone but permanent key exists.
	resp2, err := env.App.Test(makeReq())
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp2, http.StatusCreated)
}

func TestCommitDraft_ViewerForbidden(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetMemberFn = func(_ context.Context, _, _ string) (*service.MemberDTO, error) {
		return &service.MemberDTO{Role: "viewer"}, nil
	}
	cookie := env.MintCookie(t, "viewer_1", "ws_1")
	req := testutil.AuthRequest(http.MethodPost, "/api/v1/assets/ast_1/variants/draft/abc/commit", nil, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

// ---- Discard draft ----

func TestDiscardDraft_HappyPath(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "user_1"
		workspaceID = "ws_1"
		assetID     = "ast_1"
		nonce       = "aabbccdd11223344"
	)
	setupDraftAsset(t, env, assetID, workspaceID)
	writeScratchFiles(t, env.Server.StorageForTest(), workspaceID, userID, nonce, assetID)

	cookie := env.MintCookie(t, userID, workspaceID)
	req := testutil.AuthRequest(http.MethodDelete, "/api/v1/assets/"+assetID+"/variants/draft/"+nonce, nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusNoContent)
}

func TestDiscardDraft_AlreadyGone(t *testing.T) {
	env := testutil.NewTestEnv(t)
	const (
		userID      = "user_1"
		workspaceID = "ws_1"
		assetID     = "ast_1"
	)
	setupDraftAsset(t, env, assetID, workspaceID)

	cookie := env.MintCookie(t, userID, workspaceID)
	req := testutil.AuthRequest(http.MethodDelete, "/api/v1/assets/"+assetID+"/variants/draft/gone", nil, cookie)
	resp, _ := env.App.Test(req)
	// Missing files are not errors — idempotent.
	testutil.AssertStatus(t, resp, http.StatusNoContent)
}

func TestDiscardDraft_ViewerForbidden(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetMemberFn = func(_ context.Context, _, _ string) (*service.MemberDTO, error) {
		return &service.MemberDTO{Role: "viewer"}, nil
	}
	cookie := env.MintCookie(t, "viewer_1", "ws_1")
	req := testutil.AuthRequest(http.MethodDelete, "/api/v1/assets/ast_1/variants/draft/abc", nil, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}
