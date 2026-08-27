// Package ai defines the unified interface for AI image providers.
package ai

import (
	"context"
	"errors"
	"log/slog"
	"time"

	cache "github.com/go-pkgz/expirable-cache/v3"
	"golang.org/x/sync/errgroup"
)

const (
	modelCacheTTL     = 5 * time.Minute
	modelCacheMaxKeys = 512
)

//nolint:gochecknoglobals // shared TTL cache for provider model lists, safe concurrent access
var modelCache = cache.NewCache[string, []Model]().
	WithMaxKeys(modelCacheMaxKeys).
	WithTTL(modelCacheTTL)

// ProviderID identifies an AI provider.
type ProviderID string

// KeySource indicates where a provider's API key was configured (e.g. workspace, env, none).
type KeySource string

type KeyStatus struct {
	KeySet bool      `json:"key_set"`
	Source KeySource `json:"source"`
}

var (
	ErrNotCapable        = errors.New("ai: this api provider cannot do that")
	ErrInvalidModel      = errors.New("ai: model not found or invalid")
	ErrNotConfigured     = errors.New("ai: api key not configured")
	ErrUnknownProvider   = errors.New("ai: unknown provider")
	ErrAPIError          = errors.New("ai: api returned non-2xx")
	ErrInvalidKey        = errors.New("ai: invalid api key")
	ErrModelNotSupported = errors.New("ai: model does not support this operation")
)

const (
	ProviderImageRouter ProviderID = "imagerouter"
	ProviderOpenRouter  ProviderID = "openrouter"

	SourceWorkspace KeySource = "workspace"
	SourceNone      KeySource = "none"
	SourceEnv       KeySource = "env"
)

// Capability is a bitmask of features a provider may support.
type Capability uint32

var capNames = map[Capability]string{
	CapBgRemove:           "background removal",
	CapImageToImage:       "image to image",
	CapTextToImage:        "text to image",
	CapImageToText:        "image to text",
	CapImageDescription:   "image description",
	CapVisionTag:          "vision tag suggestion",
	CapTextTag:            "text tag suggestion",
	CapAudioTranscription: "audio transcription",
}

func (c Capability) Names() []string {
	var names []string
	for i, name := range capNames {
		if c&i != 0 {
			names = append(names, name)
		}
	}
	return names
}

const (
	CapBgRemove           Capability = 1 << iota // image → image, background removal
	CapImageToImage                              // image + prompt → image
	CapTextToImage                               // prompt → image (reserved)
	CapImageToText                               // image → text (reserved)
	CapImageDescription                          // image → text description via vision model
	CapVisionTag                                 // image → suggested tags via vision model
	CapTextTag                                   // extracted text → suggested tags via chat model
	CapAudioTranscription                        // audio → transcript via speech-to-text model
)

// Provider is a configured, ready-to-use AI provider.
// Only methods matching the provider's declared Capabilities are safe to call.
type Provider interface {
	ID() ProviderID
	Capabilities() Capability
	IsConfigured() bool
	KeySource() KeySource
	// BgRemove removes the background from imageData. Returns PNG bytes.
	BgRemove(ctx context.Context, imageData []byte, model string) ([]byte, error)
	// Transform applies a prompt-guided transform. Returns PNG bytes.
	Transform(ctx context.Context, imageData []byte, prompt, model string) ([]byte, error)
	// ListModels returns models available for this provider.
	ListModels(ctx context.Context) ([]Model, error)
	// ValidateKey checks that the configured API key is accepted by the provider.
	// Returns ErrInvalidKey if rejected, or another error for transport failures.
	ValidateKey(ctx context.Context) error
	// DescribeImage sends imageData to the provider's vision endpoint using the
	// given model and prompt, and returns the model's text response. Supported
	// for providers that declare CapVisionTag or CapImageDescription.
	DescribeImage(ctx context.Context, model, prompt string, imageData []byte, mimeType string) (string, error)
	// TranscribeAudio sends audioData to the provider's speech-to-text endpoint
	// and returns the transcript. Supported for providers that declare
	// CapAudioTranscription. format is a short audio format hint (e.g. "mp3", "wav").
	TranscribeAudio(ctx context.Context, model string, audioData []byte, format string) (string, error)
	// TagText sends a text-only prompt to the provider's chat endpoint and returns
	// the model's text response. Supported for providers that declare CapTextTag.
	TagText(ctx context.Context, model, prompt string) (string, error)
}

type ProviderWithModels struct {
	Provider

	Models []Model
	// ModelsErr is set when fetching Models failed. Configured stays true and
	// Models stays empty, but callers can use this to distinguish "fetch
	// failed" from "provider genuinely has no models."
	ModelsErr error
}

// Model is a provider model returned to clients.
type Model struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	ProviderID    ProviderID `json:"provider_id"`
	PricePerImage float64    `json:"price_per_image"`
	Capabilities  Capability `json:"capability"`
}

// ProviderFactory constructs a Provider from resolved credentials.
// The default implementation is NewProvider.
type ProviderFactory func(providerID ProviderID, apiKey string, keySource KeySource) (Provider, error)

// ResetModelCacheForTest clears the package-level model cache. Call in tests to prevent cross-test pollution.
func ResetModelCacheForTest() {
	modelCache.Purge()
}

func NewProvider(providerID ProviderID, apiKey string, keySource KeySource) (Provider, error) {
	switch providerID {
	case ProviderImageRouter:
		return NewImageRouterProvider(apiKey, keySource, true), nil
	case ProviderOpenRouter:
		return NewOpenRouterProvider(
			apiKey,
			keySource,
			"google/gemini-2.5-flash",
			"google/gemini-2.5-flash",
		), nil
	default:
		return nil, ErrUnknownProvider
	}
}

func AllProviders(
	ctx context.Context,
	workspaceID string,
	apiKeyResolver KeyResolver,
	capabilities Capability,
) ([]ProviderWithModels, error) {
	pkIDs := []ProviderID{ProviderImageRouter, ProviderOpenRouter}
	pks := make([]ProviderWithModels, len(pkIDs))

	g, gctx := errgroup.WithContext(ctx)
	for i, pk := range pkIDs {
		apiKey, source, keyErr := apiKeyResolver(ctx, workspaceID, string(pk))
		if keyErr != nil {
			return nil, keyErr
		}
		p, pErr := NewProvider(pk, apiKey, source)
		if pErr != nil {
			return nil, pErr
		}
		pm := ProviderWithModels{
			Provider: p,
			Models:   []Model{},
		}
		if !p.IsConfigured() {
			pks[i] = pm
			continue
		}

		g.Go(func() error {
			models, lErr := p.ListModels(gctx)
			if lErr != nil {
				slog.WarnContext(gctx, "ai: list models failed", "provider", string(pk), "error", lErr)
				pm.ModelsErr = lErr
			}
			for _, m := range models {
				if (m.Capabilities & capabilities) == 0 {
					continue
				}
				pm.Models = append(pm.Models, m)
			}
			pks[i] = pm
			return nil
		})
	}
	_ = g.Wait() // per-provider fetch failures are captured in ModelsErr, not returned

	return pks, nil
}
