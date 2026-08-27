package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"damask/server/internal/ai"
	"damask/server/internal/api"
	"damask/server/internal/apperr"
	"damask/server/internal/auth"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

func TestListAIProviders_ReturnsAllProviders(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.ListAIProvidersFn = func(_ context.Context, _ string, _ ai.Capability) ([]service.AIProviderStatusDTO, error) {
		return []service.AIProviderStatusDTO{
			{
				ID:           "openrouter",
				Configured:   false,
				KeySource:    "none",
				Capabilities: []string{"bg_remove", "image_to_image"},
				Models:       []service.AIProviderModelDTO{},
			},
			{
				ID:           "imagerouter",
				Configured:   false,
				KeySource:    "none",
				Capabilities: []string{"bg_remove", "image_to_image"},
				Models:       []service.AIProviderModelDTO{},
			},
		}, nil
	}

	req := testutil.BearerRequest(http.MethodGet, "/api/v1/aiproviders", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.AIProvidersListResponse
	if decErr := json.NewDecoder(resp.Body).Decode(&body); decErr != nil {
		t.Fatalf("decode: %v", decErr)
	}
	if len(body.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(body.Providers))
	}
}

func TestListAIProviders_RequiresAuth(t *testing.T) {
	env := testutil.NewTestEnv(t)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/aiproviders", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetOpenRouterKeyStatus_ReturnsSource(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.GetAIProviderKeyStatusFn = func(_ context.Context, _ string, _ string) (*ai.KeyStatus, error) {
		return &ai.KeyStatus{KeySet: true, Source: ai.SourceEnv}, nil
	}

	req := testutil.BearerRequest(
		http.MethodGet,
		"/api/v1/workspace/settings/aiproviders/openrouter",
		nil,
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["source"] != "env" {
		t.Errorf("expected source=env, got %v", body["source"])
	}
}

func TestSetOpenRouterKey_OwnerOnly(t *testing.T) {
	env := testutil.NewTestEnv(t)
	// Override GetMember to return editor role.
	env.Workspace.GetMemberFn = func(_ context.Context, _, userID string) (*service.MemberDTO, error) {
		return &service.MemberDTO{UserID: userID, Role: string(auth.Editor)}, nil
	}

	req := testutil.BearerRequest(http.MethodPut, "/api/v1/workspace/settings/aiproviders/openrouter",
		testutil.JSONBody(map[string]string{"key": "sk-or-test"}),
		env.MintToken(t, "usr_2", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestSetOpenRouterKey_EmptyKeyRejected(t *testing.T) {
	env := testutil.NewTestEnv(t)

	req := testutil.BearerRequest(http.MethodPut, "/api/v1/workspace/settings/aiproviders/openrouter",
		testutil.JSONBody(map[string]string{"key": ""}),
		env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestClearOpenRouterKey_Idempotent(t *testing.T) {
	env := testutil.NewTestEnv(t)

	for i := range 2 {
		req := testutil.BearerRequest(http.MethodDelete, "/api/v1/workspace/settings/aiproviders/openrouter", nil,
			env.MintToken(t, "usr_1", "ws_1"))
		resp, err := env.App.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("call %d: expected 204, got %d", i+1, resp.StatusCode)
		}
	}
}

func TestTestOpenRouterKey_NoKeyConfigured(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.TestAIProviderKeyFn = func(_ context.Context, _ string, _ string) error {
		return apperr.ErrInvalidInput
	}

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/workspace/settings/aiproviders/openrouter/test", nil,
		env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestTestOpenRouterKey_InvalidKey(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.TestAIProviderKeyFn = func(_ context.Context, _ string, _ string) error {
		return apperr.ErrInvalidInput
	}

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/workspace/settings/aiproviders/openrouter/test", nil,
		env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}
