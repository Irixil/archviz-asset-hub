package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"damask/server/internal/ai"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"go.opentelemetry.io/otel/attribute"
)

type aiVariantParams struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	Provider string `json:"provider"`
}

// imageBgRemoveBuild is the variantBuildFn for background removal.
func (s *JobServer) imageBgRemoveBuild(
	_ context.Context, _, _, workspaceID string, params json.RawMessage,
) (variantTransformer, error) {
	return s.imageBgRemoveTransformer(workspaceID, params)
}

// imageBgRemoveTransformer returns a variantTransformer for background removal.
func (s *JobServer) imageBgRemoveTransformer(workspaceID string, params json.RawMessage) (variantTransformer, error) {
	var p aiVariantParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parse bg remove params: %w", err)
	}
	return func(ctx context.Context, storedSourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "image_bg_remove"),
		)
		result, err := s.runAIProviderJob(ctx, workspaceID, storedSourceKey, p.Provider, ai.CapBgRemove,
			func(provider ai.Provider, imageData []byte) ([]byte, error) {
				return provider.BgRemove(ctx, imageData, p.Model)
			},
		)
		if errors.Is(err, ai.ErrModelNotSupported) {
			err = errors.New(
				"the selected model is not available for background removal. please choose another model and try again",
			)
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("remove background: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return result, "image/png", nil
	}, nil
}

// imageWithPromptBuild is the variantBuildFn for AI image-with-prompt.
func (s *JobServer) imageWithPromptBuild(
	_ context.Context, _, _, workspaceID string, params json.RawMessage,
) (variantTransformer, error) {
	return s.imageWithPromptTransformer(workspaceID, params)
}

// imageWithPromptTransformer returns a variantTransformer for AI image-with-prompt.
func (s *JobServer) imageWithPromptTransformer(workspaceID string, params json.RawMessage) (variantTransformer, error) {
	var p aiVariantParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parse image prompt params: %w", err)
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "image_with_prompt"),
		)
		_, prompt := transform.StripLeadingDescription(p.Prompt)
		result, err := s.runAIProviderJob(ctx, workspaceID, sourceKey, p.Provider, ai.CapImageToImage,
			func(provider ai.Provider, imageData []byte) ([]byte, error) {
				return provider.Transform(ctx, imageData, prompt, p.Model)
			},
		)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("image transform with prompt: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return result, "image/png", nil
	}, nil
}

func (s *JobServer) runAIProviderJob(
	ctx context.Context,
	workspaceID string,
	storedSourceKey string,
	aiProvider string,
	capability ai.Capability,
	callFn func(ai.Provider, []byte) ([]byte, error),
) ([]byte, error) {
	ctx, span := telemetry.StartBackgroundSpan(ctx, "ai.job",
		attribute.String("damask.workspace_id", workspaceID),
	)

	provider, err := s.resolveProvider(ctx, workspaceID, aiProvider, capability)
	if err != nil {
		telemetry.EndSpan(span, err)
		return nil, err
	}

	rc, err := s.storage.Get(ctx, storedSourceKey)
	if err != nil {
		telemetry.EndSpan(span, err)
		return nil, fmt.Errorf("get source: %w", err)
	}
	defer rc.Close()

	imageData, err := io.ReadAll(rc)
	if err != nil {
		telemetry.EndSpan(span, err)
		return nil, fmt.Errorf("read source: %w", err)
	}

	result, err := callFn(provider, imageData)
	telemetry.EndSpan(span, err)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// resolveProvider returns the first available provider that supports the required capability.
// When aiProvider is set, only that provider is tried. When empty, all known providers are
// tried in order and the first with a configured key is used.
func (s *JobServer) resolveProvider(
	ctx context.Context,
	workspaceID, aiProvider string,
	capability ai.Capability,
) (ai.Provider, error) {
	candidates := []string{aiProvider}
	if aiProvider == "" {
		candidates = []string{string(ai.ProviderImageRouter), string(ai.ProviderOpenRouter)}
	}

	for _, providerID := range candidates {
		apiKey, apiKeySource, err := s.aiAPIKeyResolver(ctx, workspaceID, providerID)
		if err != nil || apiKey == "" {
			continue
		}
		provider, err := s.aiProviderFactory(ai.ProviderID(providerID), apiKey, apiKeySource)
		if err != nil {
			continue
		}
		if provider.Capabilities()&capability == 0 {
			continue
		}
		return provider, nil
	}

	if aiProvider != "" {
		return nil, fmt.Errorf(
			"%w: provider %q not configured or lacks required capability",
			ai.ErrNotConfigured,
			aiProvider,
		)
	}
	return nil, fmt.Errorf("%w: no configured provider supports this operation", ai.ErrNotConfigured)
}
