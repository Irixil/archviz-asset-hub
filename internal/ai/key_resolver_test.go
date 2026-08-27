package ai_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/ai"
	"damask/server/internal/ingress"
)

type stubWorkspaceKeyRepo struct {
	key string
	err error
}

func (r stubWorkspaceKeyRepo) GetAIProviderKey(_ context.Context, _ string, _ string) (string, error) {
	return r.key, r.err
}

func TestResolveKeyWorkspaceOverride(t *testing.T) {
	encKey, err := ingress.EncryptConfig("secret", []byte("workspace-key"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	key, source, err := ai.ResolveKey(
		context.Background(),
		"ws_1",
		string(ai.ProviderImageRouter),
		stubWorkspaceKeyRepo{key: encKey},
		"secret",
		"env-key",
	)
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if key != "workspace-key" {
		t.Fatalf("key = %q, want workspace-key", key)
	}
	if source != ai.SourceWorkspace {
		t.Fatalf("source = %q, want %q", source, ai.SourceWorkspace)
	}
}

func TestResolveKeyFallsBackToEnv(t *testing.T) {
	key, source, err := ai.ResolveKey(
		context.Background(),
		"ws_1",
		string(ai.ProviderImageRouter),
		stubWorkspaceKeyRepo{},
		"secret",
		"env-key",
	)
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if key != "env-key" {
		t.Fatalf("key = %q, want env-key", key)
	}
	if source != ai.SourceEnv {
		t.Fatalf("source = %q, want %q", source, ai.SourceEnv)
	}
}

func TestResolveKeyNoneConfigured(t *testing.T) {
	key, source, err := ai.ResolveKey(
		context.Background(),
		"ws_1",
		string(ai.ProviderImageRouter),
		stubWorkspaceKeyRepo{},
		"secret",
		"",
	)
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if key != "" {
		t.Fatalf("key = %q, want empty", key)
	}
	if source != ai.SourceNone {
		t.Fatalf("source = %q, want %q", source, ai.SourceNone)
	}
}

func TestResolveKeyCorruptCiphertext(t *testing.T) {
	_, _, err := ai.ResolveKey(
		context.Background(),
		"ws_1",
		string(ai.ProviderImageRouter),
		stubWorkspaceKeyRepo{key: "not-base64"},
		"secret",
		"env-key",
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetKeyStatus(t *testing.T) {
	status, err := ai.GetKeyStatus(
		context.Background(),
		"ws_1",
		string(ai.ProviderImageRouter),
		stubWorkspaceKeyRepo{},
		"secret",
		"env-key",
	)
	if err != nil {
		t.Fatalf("GetKeyStatus: %v", err)
	}
	if !status.KeySet || status.Source != ai.SourceEnv {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestResolveKeyPropagatesRepoError(t *testing.T) {
	want := errors.New("boom")
	_, _, err := ai.ResolveKey(
		context.Background(),
		"ws_1",
		string(ai.ProviderImageRouter),
		stubWorkspaceKeyRepo{err: want},
		"secret",
		"",
	)
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}
