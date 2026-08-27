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

// minimalPNG is a 1x1 transparent PNG for test payloads.
var minimalPNG = func() []byte {
	b, _ := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
	)
	return b
}()

func imageRouterSuccessServer(t *testing.T) *httptest.Server {
	t.Helper()
	b64 := base64.StdEncoding.EncodeToString(minimalPNG)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": b64}},
		})
	}))
}

func TestImageRouterProvider_BgRemove_Success(t *testing.T) {
	srv := imageRouterSuccessServer(t)
	defer srv.Close()

	p := ai.NewImageRouterProviderForTest("test-key", "env", true, srv.URL+"/v1")
	got, err := p.BgRemove(context.Background(), minimalPNG, "bria/remove-background")
	if err != nil {
		t.Fatalf("BgRemove: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestImageRouterProvider_Transform_Success(t *testing.T) {
	srv := imageRouterSuccessServer(t)
	defer srv.Close()

	p := ai.NewImageRouterProviderForTest("test-key", "env", true, srv.URL+"/v1")
	got, err := p.Transform(context.Background(), minimalPNG, "make it pop", "black-forest-labs/FLUX-2-klein-4b")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestImageRouterProvider_ListModels_CachesOnSuccess(t *testing.T) {
	ai.ResetModelCacheForTest()
	t.Cleanup(ai.ResetModelCacheForTest)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "test/model", "price": map[string]any{"min": 0.01, "average": 0.01}},
		})
	}))
	defer srv.Close()

	p := ai.NewImageRouterProviderForTest("test-key", "env", false, srv.URL+"/v1")
	models1, _ := p.ListModels(context.Background())
	models2, _ := p.ListModels(context.Background())

	if calls != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", calls)
	}
	if len(models1) != len(models2) {
		t.Fatal("expected same models on second call")
	}
}

func TestImageRouterProvider_Unconfigured_ReturnsUnconfigured(t *testing.T) {
	p := ai.NewImageRouterProvider("", "none", true)
	if p.(interface{ IsConfigured() bool }).IsConfigured() {
		t.Fatal("expected unconfigured provider")
	}
}

func TestImageRouterProvider_ValidateKey_ReturnsErrInvalidKeyOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := ai.NewImageRouterProviderForTest("bad-key", "env", false, srv.URL+"/v1")
	err := p.ValidateKey(context.Background())
	if !errors.Is(err, ai.ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestImageRouterProvider_ValidateKey_NilOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]any{})
	}))
	defer srv.Close()

	p := ai.NewImageRouterProviderForTest("valid-key", "env", false, srv.URL+"/v1")
	if err := p.ValidateKey(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
