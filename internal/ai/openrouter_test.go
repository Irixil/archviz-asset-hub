package ai_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"damask/server/internal/ai"
)

func openRouterSuccessServer(t *testing.T) *httptest.Server {
	t.Helper()
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(minimalPNG)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "",
						"images": []map[string]any{
							{"type": "image_url", "image_url": map[string]any{"url": dataURL}},
						},
					},
				},
			},
		})
	}))
}

func newOR(t *testing.T, srv *httptest.Server, apiKey, bgModel, i2iModel string) ai.Provider {
	t.Helper()
	return ai.NewOpenRouterProviderForTest(apiKey, "env", bgModel, i2iModel, srv.URL+"/api/v1")
}

func TestOpenRouterProvider_BgRemove_Success(t *testing.T) {
	srv := openRouterSuccessServer(t)
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "openai/dall-e-2", "openai/dall-e-2")
	got, err := p.BgRemove(context.Background(), minimalPNG, "openai/dall-e-2")
	if err != nil {
		t.Fatalf("BgRemove: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestOpenRouterProvider_BgRemove_APIError_Returns422(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "invalid model", "code": 422},
		})
	}))
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "openai/dall-e-2", "openai/dall-e-2")
	_, err := p.BgRemove(context.Background(), minimalPNG, "bad/model")
	if err == nil {
		t.Fatal("expected error from API 422")
	}
}

func TestOpenRouterProvider_Transform_Success(t *testing.T) {
	srv := openRouterSuccessServer(t)
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "openai/dall-e-2", "openai/dall-e-2")
	got, err := p.Transform(context.Background(), minimalPNG, "make it look vintage", "openai/dall-e-2")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestOpenRouterProvider_Transform_EmptyPromptErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "openai/dall-e-2", "openai/dall-e-2")
	_, err := p.Transform(context.Background(), minimalPNG, "", "openai/dall-e-2")
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestOpenRouterProvider_ValidateKey_ReturnsErrInvalidKeyOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := newOR(t, srv, "bad-key", "", "")
	err := p.ValidateKey(context.Background())
	if !errors.Is(err, ai.ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestOpenRouterProvider_ValidateKey_ReturnsErrInvalidKeyOn403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	p := newOR(t, srv, "bad-key", "", "")
	err := p.ValidateKey(context.Background())
	if !errors.Is(err, ai.ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestOpenRouterProvider_ValidateKey_NilOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := newOR(t, srv, "valid-key", "", "")
	if err := p.ValidateKey(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestOpenRouterProvider_ValidateKey_APIErrorOn500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newOR(t, srv, "some-key", "", "")
	err := p.ValidateKey(context.Background())
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if errors.Is(err, ai.ErrInvalidKey) {
		t.Fatal("expected ErrAPIError, not ErrInvalidKey")
	}
}

func TestOpenRouterProvider_ListModels_ErrorOnAPIError(t *testing.T) {
	ai.ResetModelCacheForTest()
	t.Cleanup(ai.ResetModelCacheForTest)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "openai/dall-e-2", "openai/dall-e-2")
	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error on API failure")
	}
}

func TestOpenRouterProvider_ListModels_CachesOnSuccess(t *testing.T) {
	ai.ResetModelCacheForTest()
	t.Cleanup(ai.ResetModelCacheForTest)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "openai/gpt-4o", "name": "GPT-4o",
					"architecture": map[string]any{
						"modality":          "image->image",
						"input_modalities":  []string{"image"},
						"output_modalities": []string{"image"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "", "")
	models1, _ := p.ListModels(context.Background())
	models2, _ := p.ListModels(context.Background())

	if calls != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", calls)
	}
	if len(models1) != len(models2) {
		t.Fatal("expected same models on second call")
	}
}

func TestOpenRouterProvider_TranscribeAudio_Success(t *testing.T) {
	const audioBytes = "fake-audio-bytes"
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"text": "hello world"})
	}))
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "", "")
	got, err := p.TranscribeAudio(context.Background(), "openai/whisper-1", []byte(audioBytes), "wav")
	if err != nil {
		t.Fatalf("TranscribeAudio: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("expected transcript %q, got %q", "hello world", got)
	}
	if gotPath != "/api/v1/audio/transcriptions" {
		t.Fatalf("expected transcriptions endpoint, got %q", gotPath)
	}
	inputAudio, ok := gotBody["input_audio"].(map[string]any)
	if !ok {
		t.Fatalf("expected input_audio object in request body, got %v", gotBody)
	}
	if inputAudio["format"] != "wav" {
		t.Fatalf("expected format=wav, got %v", inputAudio["format"])
	}
	wantData := base64.StdEncoding.EncodeToString([]byte(audioBytes))
	if inputAudio["data"] != wantData {
		t.Fatalf("expected base64 audio data %q, got %v", wantData, inputAudio["data"])
	}
}

func TestOpenRouterProvider_TranscribeAudio_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad audio"}}`))
	}))
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "", "")
	_, err := p.TranscribeAudio(context.Background(), "openai/whisper-1", []byte("x"), "wav")
	if !errors.Is(err, ai.ErrTranscribeAudioAPIError) {
		t.Fatalf("expected ErrTranscribeAudioAPIError, got %v", err)
	}
}

func TestOpenRouterProvider_TagText_Success(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": `["interview","music"]`}},
			},
		})
	}))
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "", "")
	got, err := p.TagText(context.Background(), "google/gemini-2.5-flash", "tag this transcript")
	if err != nil {
		t.Fatalf("TagText: %v", err)
	}
	if got != `["interview","music"]` {
		t.Fatalf("unexpected content: %q", got)
	}
	messages, ok := gotBody["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("expected 1 message, got %v", gotBody["messages"])
	}
	msg := messages[0].(map[string]any)
	if msg["content"] != "tag this transcript" {
		t.Fatalf("expected plain string content, got %v", msg["content"])
	}
}

func TestOpenRouterProvider_TagText_NoChoicesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{}})
	}))
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "", "")
	_, err := p.TagText(context.Background(), "google/gemini-2.5-flash", "tag this")
	if !errors.Is(err, ai.ErrTagTextNoContent) {
		t.Fatalf("expected ErrTagTextNoContent, got %v", err)
	}
}

func TestOpenRouterProvider_ListModels_ErrorNotCached(t *testing.T) {
	ai.ResetModelCacheForTest()
	t.Cleanup(ai.ResetModelCacheForTest)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newOR(t, srv, "sk-or-test", "", "")
	p.ListModels(context.Background())
	p.ListModels(context.Background())

	if calls != 2 {
		t.Fatalf("expected 2 HTTP calls (error not cached), got %d", calls)
	}
}
