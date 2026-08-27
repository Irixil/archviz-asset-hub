package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/apperr"
	"damask/server/internal/auth"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

func autoTagTestAsset() *service.AssetDTO {
	return &service.AssetDTO{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		OriginalFilename: "photo.jpg",
		StorageKey:       "ws_1/ast_1/original.jpg",
		MimeType:         "image/jpeg",
		Size:             12,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

func asViewer(env *testutil.TestEnv) {
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return &service.MemberDTO{UserID: userID, Role: string(auth.Viewer)}, nil
	}
}

func TestHandleTriggerAutoTag_EnqueuesJob(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return autoTagTestAsset(), nil
	}
	env.AutoTag.IsProviderAvailableFn = func(_ context.Context, _, _ string) bool { return true }
	var enqueued bool
	env.AutoTag.EnqueueFn = func(_ context.Context, _, _ string, manual bool) error {
		enqueued = manual
		return nil
	}

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/assets/ast_1/auto-tag", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusAccepted)
	if !enqueued {
		t.Fatal("expected Enqueue to be called with manual=true")
	}
}

func TestHandleTriggerAutoTag_IneligibleMime_Returns422(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	asset := autoTagTestAsset()
	asset.MimeType = "text/plain"
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return asset, nil
	}

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/assets/ast_1/auto-tag", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestHandleTriggerAutoTag_NoProviderConfigured_Returns422(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return autoTagTestAsset(), nil
	}
	env.AutoTag.IsProviderAvailableFn = func(_ context.Context, _, _ string) bool { return false }

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/assets/ast_1/auto-tag", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestHandleTriggerAutoTag_NotFound_Returns404(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return nil, apperr.ErrNotFound
	}

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/assets/ast_1/auto-tag", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestHandleTriggerAutoTag_ViewerForbidden(t *testing.T) {
	env := testutil.NewTestEnv(t)
	asViewer(env)
	token := env.MintToken(t, "usr_1", "ws_1")

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/assets/ast_1/auto-tag", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestHandleListAutoTagSuggestions_ReturnsSuggestions(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return autoTagTestAsset(), nil
	}
	env.AutoTag.ListSuggestionsFn = func(_ context.Context, _, _ string) ([]service.AutoTagSuggestionDTO, error) {
		return []service.AutoTagSuggestionDTO{
			{ID: "sug_1", AssetID: "ast_1", TagName: "hero", CreatedAt: time.Now()},
		}, nil
	}

	req := testutil.BearerRequest(http.MethodGet, "/api/v1/assets/ast_1/auto-tag/suggestions", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body struct {
		Suggestions []api.AutoTagSuggestionResponse `json:"suggestions"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode: %v", decodeErr)
	}
	if len(body.Suggestions) != 1 || body.Suggestions[0].TagName != "hero" {
		t.Fatalf("expected 1 suggestion 'hero', got %+v", body.Suggestions)
	}
}

func TestHandleListAutoTagSuggestions_EmptyWhenNone(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return autoTagTestAsset(), nil
	}

	req := testutil.BearerRequest(http.MethodGet, "/api/v1/assets/ast_1/auto-tag/suggestions", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body struct {
		Suggestions []api.AutoTagSuggestionResponse `json:"suggestions"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode: %v", decodeErr)
	}
	if len(body.Suggestions) != 0 {
		t.Fatalf("expected no suggestions, got %+v", body.Suggestions)
	}
}

func TestHandleAcceptAutoTagSuggestion_AppliesTag(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	env.AutoTag.AcceptSuggestionFn = func(_ context.Context, _, _, suggestionID string) (*service.TagDTO, error) {
		if suggestionID != "sug_1" {
			t.Fatalf("unexpected suggestion id %q", suggestionID)
		}
		return &service.TagDTO{Name: "hero"}, nil
	}

	req := testutil.BearerRequest(
		http.MethodPost, "/api/v1/assets/ast_1/auto-tag/suggestions/sug_1/accept", nil, token,
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestHandleAcceptAutoTagSuggestion_NotFound_Returns404(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	env.AutoTag.AcceptSuggestionFn = func(_ context.Context, _, _, _ string) (*service.TagDTO, error) {
		return nil, apperr.ErrNotFound
	}

	req := testutil.BearerRequest(
		http.MethodPost, "/api/v1/assets/ast_1/auto-tag/suggestions/sug_missing/accept", nil, token,
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestHandleAcceptAllAutoTagSuggestions_AppliesAllTags(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	env.AutoTag.AcceptAllFn = func(_ context.Context, _, _ string) (int, error) { return 3, nil }

	req := testutil.BearerRequest(
		http.MethodPost, "/api/v1/assets/ast_1/auto-tag/suggestions/accept-all", nil, token,
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body struct {
		Accepted int `json:"accepted"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode: %v", decodeErr)
	}
	if body.Accepted != 3 {
		t.Fatalf("expected accepted=3, got %d", body.Accepted)
	}
}

func TestHandleDismissAutoTagSuggestion_DeletesRow(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	var dismissed string
	env.AutoTag.DismissSuggestionFn = func(_ context.Context, _, _, suggestionID string) error {
		dismissed = suggestionID
		return nil
	}

	req := testutil.BearerRequest(http.MethodDelete, "/api/v1/assets/ast_1/auto-tag/suggestions/sug_1", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusNoContent)
	if dismissed != "sug_1" {
		t.Fatalf("expected DismissSuggestion called with sug_1, got %q", dismissed)
	}
}

func TestHandleDismissAllAutoTagSuggestions_ClearsAll(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	var dismissedAsset string
	env.AutoTag.DismissAllFn = func(_ context.Context, _, assetID string) error {
		dismissedAsset = assetID
		return nil
	}

	req := testutil.BearerRequest(http.MethodDelete, "/api/v1/assets/ast_1/auto-tag/suggestions", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusNoContent)
	if dismissedAsset != "ast_1" {
		t.Fatalf("expected DismissAll called with ast_1, got %q", dismissedAsset)
	}
}

func TestHandleAcceptAllAutoTagSuggestions_ViewerForbidden(t *testing.T) {
	env := testutil.NewTestEnv(t)
	asViewer(env)
	token := env.MintToken(t, "usr_1", "ws_1")

	req := testutil.BearerRequest(
		http.MethodPost, "/api/v1/assets/ast_1/auto-tag/suggestions/accept-all", nil, token,
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}
