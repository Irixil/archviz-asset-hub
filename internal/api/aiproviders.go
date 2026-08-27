package api

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"damask/server/internal/ai"
	"damask/server/internal/apperr"
	"damask/server/internal/auth"

	"github.com/gofiber/fiber/v3"
)

// AIProviderModelResponse is a model entry returned to clients.
type AIProviderModelResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ProviderID    string   `json:"provider_id"`
	PricePerImage float64  `json:"price_per_image"`
	Capabilities  []string `json:"capabilities"`
}

// AIProviderStatusResponse is a single provider entry in the list response.
type AIProviderStatusResponse struct {
	ID           string                    `json:"id"`
	Configured   bool                      `json:"configured"`
	KeySource    string                    `json:"key_source"`
	Capabilities []string                  `json:"capabilities"`
	Models       []AIProviderModelResponse `json:"models"`
	// ModelsError is set when the provider is configured but fetching its
	// model list failed, so Models is empty despite Configured being true.
	ModelsError string `json:"models_error,omitempty"`
}

type AIProviderKeyStatusResponse struct {
	KeySet bool   `json:"key_set"`
	Source string `json:"source"`
}

// AIProvidersListResponse is returned by GET /api/v1/aiproviders.
type AIProvidersListResponse struct {
	Providers []AIProviderStatusResponse `json:"providers"`
}

// UpdateAIProviderKeyRequest is the body for PUT /api/v1/workspace/settings/aiproviders/:provider.
type UpdateAIProviderKeyRequest struct {
	Key string `json:"key"`
}

func (r *UpdateAIProviderKeyRequest) Valid(_ context.Context) map[string]string {
	if strings.TrimSpace(r.Key) == "" {
		return map[string]string{"key": "required"}
	}
	return nil
}

// handleListAIProviders handles GET /api/v1/aiproviders.
//
// @Summary List AI providers
// @Description Returns all supported AI providers and their configuration status for the current workspace, including available models and capabilities.
// @Tags AIProviders
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AIProvidersListResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/aiproviders [get].
func (s *Server) handleListAIProviders(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	providers, err := s.workspace.ListAIProviders(c.Context(), claims.WorkspaceID, ai.CapBgRemove|ai.CapImageToImage)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	resp := AIProvidersListResponse{Providers: make([]AIProviderStatusResponse, len(providers))}
	for i, p := range providers {
		models := make([]AIProviderModelResponse, len(p.Models))
		for j, m := range p.Models {
			models[j] = AIProviderModelResponse{
				ID:            m.ID,
				Name:          m.Name,
				ProviderID:    m.ProviderID,
				PricePerImage: m.PricePerImage,
				Capabilities:  m.Capabilities.Names(),
			}
		}
		resp.Providers[i] = AIProviderStatusResponse{
			ID:           p.ID,
			Configured:   p.Configured,
			KeySource:    p.KeySource,
			Capabilities: p.Capabilities,
			Models:       models,
			ModelsError:  p.ModelsError,
		}
	}
	return c.JSON(resp)
}

// handleGetAIProviderKeyStatus handles GET /api/v1/workspace/settings/aiproviders/:provider.
//
// @Summary Get AI provider key status
// @Description Returns whether an API key is configured for the given provider and its source (workspace or environment).
// @Tags AIProviders
// @Produce json
// @Security BearerAuth
// @Param provider path string true "Provider ID (e.g. openrouter)"
// @Success 200 {object} AIProviderKeyStatusResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Provider not found"
// @Router /api/v1/workspace/settings/aiproviders/{provider} [get].
func (s *Server) handleGetAIProviderKeyStatus(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	providerName := c.Params("provider")
	status, err := s.workspace.GetAIProviderKeyStatus(c.Context(), claims.WorkspaceID, providerName)
	if err != nil {
		slog.ErrorContext(c.Context(), "get ai provider key status", "provider", providerName, "error", err)
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(AIProviderKeyStatusResponse{
		KeySet: status.KeySet,
		Source: string(status.Source),
	})
}

// handleSetAIProviderKey handles PUT /api/v1/workspace/settings/aiproviders/:provider.
//
// @Summary Set AI provider API key
// @Description Stores an API key for the given provider at workspace scope, overriding any environment-level key.
// @Tags AIProviders
// @Accept json
// @Security BearerAuth
// @Param provider path string true "Provider ID"
// @Param body body UpdateAIProviderKeyRequest true "API key"
// @Success 204
// @Failure 400 {object} ErrorResponse "Validation error"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Provider not found"
// @Router /api/v1/workspace/settings/aiproviders/{provider} [put].
func (s *Server) handleSetAIProviderKey(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	providerName := c.Params("provider")
	body, ok := decodeAndValidate(c, &UpdateAIProviderKeyRequest{})
	if !ok {
		return nil
	}
	if err := s.workspace.SetAIProviderKey(c.Context(), claims.WorkspaceID, providerName, body.Key); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleClearAIProviderKey handles DELETE /api/v1/workspace/settings/aiproviders/:provider.
//
// @Summary Clear AI provider API key
// @Description Removes the workspace-scoped API key for the given provider. Falls back to environment key if one exists.
// @Tags AIProviders
// @Security BearerAuth
// @Param provider path string true "Provider ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Provider not found"
// @Router /api/v1/workspace/settings/aiproviders/{provider} [delete].
func (s *Server) handleClearAIProviderKey(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	providerName := c.Params("provider")
	if err := s.workspace.ClearAIProviderKey(c.Context(), claims.WorkspaceID, providerName); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleTestAIProviderKey handles POST /api/v1/workspace/settings/aiproviders/:provider/test.
//
// @Summary Test AI provider API key
// @Description Validates the configured API key for the given provider by making a live request. Returns 422 if the key is missing or rejected.
// @Tags AIProviders
// @Security BearerAuth
// @Param provider path string true "Provider ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Provider not found"
// @Failure 422 {object} ErrorResponse "Key invalid or unreachable"
// @Router /api/v1/workspace/settings/aiproviders/{provider}/test [post].
func (s *Server) handleTestAIProviderKey(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	providerName := c.Params("provider")
	if err := s.workspace.TestAIProviderKey(c.Context(), claims.WorkspaceID, providerName); err != nil {
		if errors.Is(err, apperr.ErrInvalidInput) {
			return errRes(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
