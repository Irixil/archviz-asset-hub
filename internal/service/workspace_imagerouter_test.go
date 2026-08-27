package service

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/ai"
	"damask/server/internal/apperr"
	"damask/server/internal/ingress"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
)

func newWorkspaceServiceForImageRouterTests(
	t *testing.T,
	envKey string,
) (*workspaceService, *memory.RealWorkspaceRepo) {
	t.Helper()
	repo := memory.NewRealWorkspaceRepo()
	repo.Seed(repository.Workspace{ID: "ws_1", Name: "Acme"})
	resolver := ai.KeyResolver(
		func(ctx context.Context, workspaceID, providerName string) (string, ai.KeySource, error) {
			return ai.ResolveKey(ctx, workspaceID, providerName, repo, "test-secret", envKey)
		},
	)
	return &workspaceService{
		workspaces:       repo,
		users:            memory.NewUserRepo(),
		appSecret:        "test-secret",
		aiAPIKeyResolver: resolver,
	}, repo
}

func TestWorkspaceServiceSetImageRouterKeyEncryptsAtRest(t *testing.T) {
	t.Parallel()
	svc, repo := newWorkspaceServiceForImageRouterTests(t, "")

	if err := svc.SetAIProviderKey(
		context.Background(),
		"ws_1",
		string(ai.ProviderImageRouter),
		"plain-key",
	); err != nil {
		t.Fatalf("SetAIProviderKey: %v", err)
	}

	encKey, err := repo.GetAIProviderKey(context.Background(), "ws_1", string(ai.ProviderImageRouter))
	if err != nil {
		t.Fatalf("GetAIProviderKey: %v", err)
	}
	if encKey == "" || encKey == "plain-key" {
		t.Fatalf("expected encrypted key, got %q", encKey)
	}

	plain, err := ingress.DecryptConfig("test-secret", encKey)
	if err != nil {
		t.Fatalf("DecryptConfig: %v", err)
	}
	if string(plain) != "plain-key" {
		t.Fatalf("decrypted key = %q, want plain-key", string(plain))
	}
}

func TestWorkspaceServiceGetImageRouterKeyStatusUsesEnvFallback(t *testing.T) {
	t.Parallel()
	svc, _ := newWorkspaceServiceForImageRouterTests(t, "env-key")

	status, err := svc.GetAIProviderKeyStatus(context.Background(), "ws_1", string(ai.ProviderImageRouter))
	if err != nil {
		t.Fatalf("GetAIProviderKeyStatus: %v", err)
	}
	if !status.KeySet || status.Source != ai.SourceEnv {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestWorkspaceServiceClearImageRouterKey(t *testing.T) {
	t.Parallel()
	svc, repo := newWorkspaceServiceForImageRouterTests(t, "")
	if err := svc.SetAIProviderKey(
		context.Background(),
		"ws_1",
		string(ai.ProviderImageRouter),
		"plain-key",
	); err != nil {
		t.Fatalf("SetAIProviderKey: %v", err)
	}

	if err := svc.ClearAIProviderKey(context.Background(), "ws_1", string(ai.ProviderImageRouter)); err != nil {
		t.Fatalf("ClearAIProviderKey: %v", err)
	}

	key, err := repo.GetAIProviderKey(context.Background(), "ws_1", string(ai.ProviderImageRouter))
	if err != nil {
		t.Fatalf("GetAIProviderKey: %v", err)
	}
	if key != "" {
		t.Fatalf("expected cleared key, got %q", key)
	}
}

func TestWorkspaceServiceSetImageRouterKeyRejectsEmpty(t *testing.T) {
	t.Parallel()
	svc, _ := newWorkspaceServiceForImageRouterTests(t, "")

	err := svc.SetAIProviderKey(context.Background(), "ws_1", string(ai.ProviderImageRouter), "   ")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestWorkspaceServiceTestImageRouterKeySucceedsWithEnvKey(t *testing.T) {
	t.Parallel()
	svc, _ := newWorkspaceServiceForImageRouterTests(t, "env-key")

	if err := svc.TestAIProviderKey(context.Background(), "ws_1", string(ai.ProviderImageRouter)); err != nil {
		t.Fatalf("TestAIProviderKey: %v", err)
	}
}

func TestWorkspaceServiceTestImageRouterKeyNoConfiguredKey(t *testing.T) {
	t.Parallel()
	svc, _ := newWorkspaceServiceForImageRouterTests(t, "")

	err := svc.TestAIProviderKey(context.Background(), "ws_1", string(ai.ProviderImageRouter))
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
