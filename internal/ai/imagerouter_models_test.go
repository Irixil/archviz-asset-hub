package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchImageRouterModelsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path + "?" + r.URL.RawQuery; got != "/v2/models?inputType=image&outputType=image" {
			t.Fatalf("unexpected URL: %s", got)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "provider/b-model", "price": map[string]any{"average": 0.2}},
			{"id": "provider/a-model", "price": map[string]any{"average": 0.1}},
		})
	}))
	defer srv.Close()

	models, err := fetchImageRouterModels(context.Background(), "test-key", srv.URL+"/v1")
	if err != nil {
		t.Fatalf("fetchImageRouterModels: %v", err)
	}
	if len(models) != 2 || models[0].ID != "provider/a-model" {
		t.Fatalf("unexpected models: %#v", models)
	}
	if models[0].Name != "A Model" || models[0].Provider != "provider" {
		t.Fatalf("unexpected normalized model: %#v", models[0])
	}
	if models[0].Capabilities != CapImageToImage {
		t.Fatalf("expected CapImageToImage, got %d", models[0].Capabilities)
	}
}

func TestFetchImageRouterModelsCapabilities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "bria/remove-background", "price": map[string]any{"average": 0.001}},
			{"id": "provider/flux-dev", "price": map[string]any{"average": 0.01}},
		})
	}))
	defer srv.Close()

	models, err := fetchImageRouterModels(context.Background(), "test-key", srv.URL+"/v1")
	if err != nil {
		t.Fatalf("fetchImageRouterModels: %v", err)
	}
	for _, m := range models {
		if m.ID == "bria/remove-background" && m.Capabilities != CapBgRemove {
			t.Fatalf("remove-background model should have CapBgRemove, got %d", m.Capabilities)
		}
		if m.ID == "provider/flux-dev" && m.Capabilities != CapImageToImage {
			t.Fatalf("flux-dev model should have CapImageToImage, got %d", m.Capabilities)
		}
	}
}

func TestFetchImageRouterModelsErrorOnBadGateway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := fetchImageRouterModels(context.Background(), "test-key", srv.URL+"/v1")
	if err == nil {
		t.Fatal("expected error on non-2xx response, got nil")
	}
}

func TestFetchImageRouterModelsErrorOnTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode([]any{})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := fetchImageRouterModels(ctx, "test-key", srv.URL+"/v1")
	if err == nil {
		t.Fatal("expected error on timeout, got nil")
	}
}
