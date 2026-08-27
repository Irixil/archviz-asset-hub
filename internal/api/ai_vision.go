package api

import (
	"context"
	"strings"

	"damask/server/internal/ai"
	"damask/server/internal/auth"
	"damask/server/internal/service"
	"damask/server/internal/telemetry"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel/attribute"
)

// supportedLang describes a language offered in the AI image description
// form. English is used when instructing the model to respond in a given
// language; Native is the label shown to the user in the language picker.
// This is the single source of truth for supported languages — the frontend
// fetches this list from handleListVisionModels instead of hardcoding it.
type supportedLang struct {
	Code    string `json:"code"`
	English string `json:"english"`
	Native  string `json:"native"`
}

const langNameEnglish = "English"

//nolint:gochecknoglobals // intentional package-level list; read-only after init
var supportedLangs = []supportedLang{
	{"en", langNameEnglish, langNameEnglish},
	{"es", "Spanish", "Español"},
	{"fr", "French", "Français"},
	{"de", "German", "Deutsch"},
	{"it", "Italian", "Italiano"},
	{"pt", "Portuguese", "Português"},
	{"nl", "Dutch", "Nederlands"},
	{"pl", "Polish", "Polski"},
	{"ru", "Russian", "Русский"},
	{"zh", "Chinese", "中文"},
	{"ja", "Japanese", "日本語"},
	{"ko", "Korean", "한국어"},
	{"ar", "Arabic", "العربية"},
	{"ca", "Catalan", "Català"},
}

func iso639Name(code string) string {
	for _, l := range supportedLangs {
		if l.Code == code {
			return l.English
		}
	}
	return code
}

func isCuratedVisionModel(model string) bool {
	for _, m := range ai.CuratedVisionModels {
		if m.ID == model {
			return true
		}
	}
	return false
}

func (s *Server) prepareAIImageDescriptionParams(
	ctx context.Context,
	c fiber.Ctx,
	workspaceID, assetID string,
	asset *service.AssetDTO,
	lang *string,
	params map[string]any,
	createParams *service.CreateTextTrackParams,
) error {
	// Guard: OpenRouter must be configured.
	orStatus, err := s.workspace.GetAIProviderKeyStatus(ctx, workspaceID, string(ai.ProviderOpenRouter))
	if err != nil || !orStatus.KeySet {
		return &apiValidationError{fiber.StatusUnprocessableEntity, "openrouter_not_configured"}
	}

	// Guard: asset must be an image.
	if !strings.HasPrefix(asset.MimeType, "image/") {
		return &apiValidationError{fiber.StatusUnprocessableEntity, "unsupported_mime"}
	}

	// Guard: asset must have a current version.
	if asset.CurrentVersionID == nil {
		return &apiValidationError{fiber.StatusUnprocessableEntity, "asset has no current version"}
	}

	versionCtx, versionSpan := telemetry.StartSpan(ctx, "api.text_tracks.create.load_current_version_ai",
		attribute.String("damask.asset_id", assetID),
	)
	currentVersion, versionErr := s.versions.GetCurrentByAsset(versionCtx, assetID)
	telemetry.EndSpan(versionSpan, versionErr)
	if versionErr != nil {
		return ErrorStatusResponse(c, versionErr)
	}

	// Resolve model with default, restricted to the curated allowlist.
	model, _ := params["model"].(string)
	if strings.TrimSpace(model) == "" {
		model = ai.DefaultVisionModel
	} else if !isCuratedVisionModel(model) {
		return &apiValidationError{fiber.StatusBadRequest, "unsupported_model"}
	}

	// Resolve prompt with default.
	prompt, _ := params["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		prompt = ai.DefaultImageDescriptionPrompt
	} else if len(prompt) > ai.MaxImageDescriptionPromptLen {
		return &apiValidationError{fiber.StatusBadRequest, "prompt_too_long"}
	}

	// Append language instruction when non-English.
	resolvedLang := "en"
	if lang != nil && strings.TrimSpace(*lang) != "" {
		resolvedLang = strings.TrimSpace(*lang)
	}
	if resolvedLang != "en" {
		prompt = prompt + "\n\nRespond in " + iso639Name(resolvedLang) + "."
	}

	resolvedLangCopy := resolvedLang
	createParams.Lang = &resolvedLangCopy
	params["storage_key"] = currentVersion.StorageKey
	params["mime_type"] = currentVersion.MimeType
	params["model"] = model
	params["prompt"] = prompt
	return nil
}

// handleListVisionModels handles GET /api/v1/ai/vision-models.
func (s *Server) handleListVisionModels(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	status, _ := s.workspace.GetAIProviderKeyStatus(c.Context(), claims.WorkspaceID, string(ai.ProviderOpenRouter))
	configured := status != nil && status.KeySet
	return c.JSON(fiber.Map{
		"configured":    configured,
		"default_model": ai.DefaultVisionModel,
		"models":        ai.CuratedVisionModels,
		"languages":     supportedLangs,
	})
}
