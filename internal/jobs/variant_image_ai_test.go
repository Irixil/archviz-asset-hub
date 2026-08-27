package jobs_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	th "damask/server/internal/testhelpers"
)

func encodeTinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestImageBgRemoveJobCreatesVariantAndThumbnail(t *testing.T) {
	outputPNG := encodeTinyPNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(outputPNG)}},
		})
	}))
	defer srv.Close()

	env := th.SetupTestApp(t,
		th.WithImageRouterAPIKey("test-key"),
		th.WithImageRouterBaseURL(srv.URL+"/v1"),
	)
	res := th.Register(t, env, "Worker User", "worker@test.com", "password123")
	assetID := env.UploadTestAsset(t, res.Cookie)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(map[string]any{"type": "image_bg_remove", "params": map[string]any{}}), res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(`SELECT COUNT(*) FROM variants WHERE type = 'image_bg_remove'`).
		Scan(&count); e != nil {
		t.Fatalf("count variants: %v", e)
	}
	if count != 1 {
		t.Fatalf("expected 1 variant, got %d", count)
	}

	var thumbKey string
	if e := env.Database.QueryRow(`SELECT COALESCE(thumbnail_key, '') FROM variants WHERE type = 'image_bg_remove' LIMIT 1`).
		Scan(&thumbKey); e != nil {
		t.Fatalf("load thumbnail_key: %v", e)
	}
	if thumbKey == "" {
		t.Fatal("expected thumbnail_key to be set after drain")
	}
}

func TestImageWithPromptJobFailureDoesNotCreateVariant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid model", http.StatusUnprocessableEntity)
	}))
	defer srv.Close()

	env := th.SetupTestApp(t,
		th.WithImageRouterAPIKey("test-key"),
		th.WithImageRouterBaseURL(srv.URL+"/v1"),
	)
	res := th.Register(t, env, "Worker User 2", "worker2@test.com", "password123")
	assetID := env.UploadTestAsset(t, res.Cookie)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(map[string]any{
			"type": "image_with_prompt",
			"params": map[string]any{
				"prompt": "add mist",
				"model":  "bad/model",
			},
		}), res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var body struct {
		JobID string `json:"job_id"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(`SELECT COUNT(*) FROM variants WHERE type = 'image_with_prompt'`).
		Scan(&count); e != nil {
		t.Fatalf("count variants: %v", e)
	}
	if count != 0 {
		t.Fatalf("expected no variants to be created on failure, got %d", count)
	}
}

func TestImageWithPromptJobStripsLeadingDescriptionLine(t *testing.T) {
	outputPNG := encodeTinyPNG(t)
	var sentPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		sentPrompt = r.FormValue("prompt")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(outputPNG)}},
		})
	}))
	defer srv.Close()

	env := th.SetupTestApp(t,
		th.WithImageRouterAPIKey("test-key"),
		th.WithImageRouterBaseURL(srv.URL+"/v1"),
	)
	res := th.Register(t, env, "Worker User 4", "worker4@test.com", "password123")
	assetID := env.UploadTestAsset(t, res.Cookie)

	const fullPrompt = "# vintage filter\nadd a warm vintage film grain"
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(map[string]any{
			"type": "image_with_prompt",
			"params": map[string]any{
				"prompt": fullPrompt,
				"model":  "black-forest-labs/FLUX.1-fill-dev",
			},
		}), res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	env.JobServer.DrainForTest(context.Background())

	if sentPrompt != "add a warm vintage film grain" {
		t.Fatalf("expected description line stripped from prompt sent to provider, got %q", sentPrompt)
	}

	var storedParams string
	if e := env.Database.QueryRow(`SELECT transform_params FROM variants WHERE type = 'image_with_prompt' LIMIT 1`).
		Scan(&storedParams); e != nil {
		t.Fatalf("load transform_params: %v", e)
	}
	var decoded struct {
		Prompt string `json:"prompt"`
	}
	if e := json.Unmarshal([]byte(storedParams), &decoded); e != nil {
		t.Fatalf("decode stored params: %v", e)
	}
	if decoded.Prompt != fullPrompt {
		t.Fatalf("expected stored prompt to retain the description line, got %q", decoded.Prompt)
	}
}

func TestImageWithPromptJobRetriesPaidModelWhenConfigured(t *testing.T) {
	outputPNG := encodeTinyPNG(t)
	var models []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		models = append(models, r.FormValue("model"))
		if len(models) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write(
				[]byte(
					`{"error":{"message":"Daily limit of 3 free requests reached. Remove \":free\" from the model name to continue with the paid model."}}`,
				),
			)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(outputPNG)}},
		})
	}))
	defer srv.Close()

	env := th.SetupTestApp(
		t,
		th.WithImageRouterAPIKey("test-key"),
		th.WithImageRouterRetryPaidOnFreeLimit(true),
		th.WithImageRouterBaseURL(srv.URL+"/v1"),
	)
	res := th.Register(t, env, "Worker User 3", "worker3@test.com", "password123")
	assetID := env.UploadTestAsset(t, res.Cookie)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(map[string]any{
			"type": "image_with_prompt",
			"params": map[string]any{
				"prompt": "add mist",
				"model":  "black-forest-labs/FLUX.1-fill-dev:free",
			},
		}), res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	env.JobServer.DrainForTest(context.Background())

	if len(models) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(models))
	}
	if models[0] != "black-forest-labs/FLUX.1-fill-dev:free" {
		t.Fatalf("unexpected first model: %q", models[0])
	}
	if models[1] != "black-forest-labs/FLUX.1-fill-dev" {
		t.Fatalf("unexpected retry model: %q", models[1])
	}

	var count int
	if e := env.Database.QueryRow(`SELECT COUNT(*) FROM variants WHERE type = 'image_with_prompt'`).
		Scan(&count); e != nil {
		t.Fatalf("count variants: %v", e)
	}
	if count != 1 {
		t.Fatalf("expected 1 variant to be created after retry, got %d", count)
	}
}
