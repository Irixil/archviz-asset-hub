package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"damask/server/internal/ai"
	"damask/server/internal/api"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

func textTrackTestAsset() *service.AssetDTO {
	versionID := "ver_1"
	return &service.AssetDTO{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		OriginalFilename: "scan.jpg",
		StorageKey:       "ws_1/ast_1/original.jpg",
		MimeType:         "image/jpeg",
		Size:             12,
		CurrentVersionID: &versionID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

func TestListTextTracksReturnsTracks(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	asset := textTrackTestAsset()
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return asset, nil
	}
	env.TextTracks.ListFn = func(_ context.Context, _, _ string) ([]service.TextTrackDTO, error) {
		return []service.TextTrackDTO{{
			ID:        "tt_1",
			AssetID:   asset.ID,
			Source:    "manual",
			Content:   "hello world",
			Status:    "ready",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}}, nil
	}

	req := testutil.BearerRequest(http.MethodGet, "/api/v1/assets/ast_1/text-tracks", nil, token)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body api.ListTextTracksResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if len(body.TextTracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(body.TextTracks))
	}
}

func TestCreateManualTextTrackReturnsOK(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	asset := textTrackTestAsset()
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return asset, nil
	}
	env.TextTracks.CreateFn = func(_ context.Context, p service.CreateTextTrackParams) (service.TextTrackDTO, error) {
		if p.Source != "manual" {
			t.Fatalf("unexpected source %q", p.Source)
		}
		return service.TextTrackDTO{
			ID:        "tt_1",
			AssetID:   asset.ID,
			Source:    "manual",
			Content:   p.InitialContent,
			Status:    "ready",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/assets/ast_1/text-tracks",
		testutil.JSONBody(api.CreateTextTrackRequest{
			Source: "manual",
			Params: map[string]any{"content": "hello world"},
		}),
		token,
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestCreateOCRUnsupportedMIMEReturns422(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	asset := textTrackTestAsset()
	asset.MimeType = "application/pdf"
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return asset, nil
	}

	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/assets/ast_1/text-tracks",
		testutil.JSONBody(api.CreateTextTrackRequest{
			Source: "ocr",
			Params: map[string]any{"output_format": "txt"},
		}),
		token,
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestCreateAIImageDescriptionRejectsUncuratedModel(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	asset := textTrackTestAsset()
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return asset, nil
	}
	env.Workspace.GetAIProviderKeyStatusFn = func(_ context.Context, _, _ string) (*ai.KeyStatus, error) {
		return &ai.KeyStatus{KeySet: true}, nil
	}
	env.Versions.GetCurrentByAssetFn = func(_ context.Context, _ string) (*service.VersionDTO, error) {
		return &service.VersionDTO{ID: "ver_1", StorageKey: asset.StorageKey, MimeType: asset.MimeType}, nil
	}

	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/assets/ast_1/text-tracks",
		testutil.JSONBody(api.CreateTextTrackRequest{
			Source: "ai_image_description",
			Params: map[string]any{"model": "not-a-curated-model"},
		}),
		token,
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusBadRequest)
}

func TestCreateAIImageDescriptionRejectsOverlongPrompt(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "usr_1", "ws_1")
	asset := textTrackTestAsset()
	env.Assets.GetFn = func(_ context.Context, _, _ string) (*service.AssetDTO, error) {
		return asset, nil
	}
	env.Workspace.GetAIProviderKeyStatusFn = func(_ context.Context, _, _ string) (*ai.KeyStatus, error) {
		return &ai.KeyStatus{KeySet: true}, nil
	}
	env.Versions.GetCurrentByAssetFn = func(_ context.Context, _ string) (*service.VersionDTO, error) {
		return &service.VersionDTO{ID: "ver_1", StorageKey: asset.StorageKey, MimeType: asset.MimeType}, nil
	}

	req := testutil.BearerRequest(
		http.MethodPost,
		"/api/v1/assets/ast_1/text-tracks",
		testutil.JSONBody(api.CreateTextTrackRequest{
			Source: "ai_image_description",
			Params: map[string]any{"prompt": strings.Repeat("a", ai.MaxImageDescriptionPromptLen+1)},
		}),
		token,
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusBadRequest)
}
