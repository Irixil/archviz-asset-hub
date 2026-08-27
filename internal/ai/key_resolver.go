package ai

import (
	"context"
	"fmt"
	"strings"

	"damask/server/internal/apperr"
	"damask/server/internal/config"
	"damask/server/internal/ingress"
)

type KeyResolver func(ctx context.Context, workspaceID, providerName string) (string, KeySource, error)

type workspaceKeyReader interface {
	GetAIProviderKey(ctx context.Context, workspaceID, providerName string) (string, error)
}

func NewKeyResolver(repo workspaceKeyReader, cfg config.Config) KeyResolver {
	return func(ctx context.Context, workspaceID, providerName string) (string, KeySource, error) {
		switch ProviderID(providerName) {
		case ProviderImageRouter:
			return ResolveKey(ctx, workspaceID, providerName, repo, cfg.AppSecret, cfg.ImageRouter.APIKey)
		case ProviderOpenRouter:
			return ResolveKey(ctx, workspaceID, providerName, repo, cfg.AppSecret, cfg.OpenRouter.APIKey)
		default:
			return "", SourceNone, fmt.Errorf("%w: %w", ErrUnknownProvider, apperr.ErrNotFound)
		}
	}
}

func ResolveKey(
	ctx context.Context,
	workspaceID string,
	providerName string,
	repo workspaceKeyReader,
	appSecret string,
	envKey string,
) (string, KeySource, error) {
	encKey, err := repo.GetAIProviderKey(ctx, workspaceID, providerName)
	if err != nil {
		return "", SourceNone, err
	}
	if strings.TrimSpace(encKey) != "" {
		plain, decErr := ingress.DecryptConfig(appSecret, encKey)
		if decErr != nil {
			return "", SourceNone, decErr
		}
		return string(plain), SourceWorkspace, nil
	}

	envKey = strings.TrimSpace(envKey)
	if envKey != "" {
		return envKey, SourceEnv, nil
	}
	return "", SourceNone, nil
}

func GetKeyStatus(
	ctx context.Context,
	workspaceID string,
	providerName string,
	repo workspaceKeyReader,
	appSecret string,
	envKey string,
) (KeyStatus, error) {
	_, source, err := ResolveKey(ctx, workspaceID, providerName, repo, appSecret, envKey)
	if err != nil {
		return KeyStatus{}, err
	}
	return KeyStatus{
		KeySet: source != SourceNone,
		Source: source,
	}, nil
}
