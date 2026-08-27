//go:build integration

package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"damask/server/internal/api"
	th "damask/server/internal/testhelpers"
)

func TestGetSimilarAssets_NotImage(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload a non-image asset (text/plain).
	txtData := []byte("hello world")
	req := th.BuildUploadRequest(t, "readme.txt", txtData, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload: expected 201, got %d: %s", resp.StatusCode, body)
	}
	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode asset: %v", err)
	}

	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/similar", nil, owner.Cookie)
	resp2, err := env.App.Test(req2)
	if err != nil {
		t.Fatalf("similar request: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp2.StatusCode)
	}
}

func TestGetSimilarAssets_UnknownAsset(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/does-not-exist/similar", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetSimilarAssets_ImageNoSimilar(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	jpegData := th.MakeJPEG(64, 64)
	req := th.BuildUploadRequest(t, "photo.jpg", jpegData, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload: expected 201, got %d: %s", resp.StatusCode, body)
	}
	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode asset: %v", err)
	}

	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/similar", nil, owner.Cookie)
	resp2, err := env.App.Test(req2)
	if err != nil {
		t.Fatalf("similar request: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200, got %d: %s", resp2.StatusCode, body)
	}

	var body struct {
		Results []api.VisualSimilarResult `json:"results"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Results) != 0 {
		t.Errorf("expected 0 results (no hash computed yet without draining jobs), got %d", len(body.Results))
	}
}

func TestGetSimilarAssets_BackfillEndpoint_NonOwnerForbidden(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	editorToken := th.MintEditorToken(t, env, owner.WorkspaceID, "editor")

	req := th.BearerRequest(http.MethodPost, "/api/v1/workspace/jobs/visual-similarity-backfill/trigger", nil, editorToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}
