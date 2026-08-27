package api

import (
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/service"
	"damask/server/internal/transform"

	"github.com/gofiber/fiber/v3"
)

// AutoTagSuggestionResponse is the public representation of a pending AI tag suggestion.
type AutoTagSuggestionResponse struct {
	ID        string    `json:"id"`
	AssetID   string    `json:"asset_id"`
	TagName   string    `json:"tag_name"`
	CreatedAt time.Time `json:"created_at"`
}

func autoTagSuggestionToResponse(d service.AutoTagSuggestionDTO) AutoTagSuggestionResponse {
	return AutoTagSuggestionResponse{ID: d.ID, AssetID: d.AssetID, TagName: d.TagName, CreatedAt: d.CreatedAt}
}

const fieldMessageKey = "message"

func autoTagErrorResponse(c fiber.Ctx, code, message string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
		apiErrorKey:     code,
		fieldMessageKey: message,
	})
}

// handleTriggerAutoTag handles POST /api/v1/assets/:id/auto-tag.
//
// @Summary Manually trigger AI tag suggestions for an asset
// @Description Enqueues an auto_tag job for the asset, bypassing the workspace's auto_tag_enabled setting.
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 202 {object} map[string]string
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ErrorResponse "Ineligible MIME type or no AI provider configured"
// @Router /api/v1/assets/{id}/auto-tag [post].
func (s *Server) handleTriggerAutoTag(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if !transform.IsAutoTaggable(asset.MimeType) {
		return autoTagErrorResponse(c, "ineligible_mime_type", "This asset type cannot be auto-tagged.")
	}
	if !s.autoTag.IsProviderAvailable(c.Context(), claims.WorkspaceID, asset.MimeType) {
		return autoTagErrorResponse(c, "no_provider_configured",
			"No AI provider is configured for this asset type. Add an API key in Settings → Integrations.")
	}
	if err = s.autoTag.Enqueue(c.Context(), claims.WorkspaceID, assetID, true); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"job": "queued"})
}

// handleListAutoTagSuggestions handles GET /api/v1/assets/:id/auto-tag/suggestions.
//
// @Summary List pending AI tag suggestions for an asset
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} map[string][]AutoTagSuggestionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/auto-tag/suggestions [get].
func (s *Server) handleListAutoTagSuggestions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	sugs, err := s.autoTag.ListSuggestions(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	out := make([]AutoTagSuggestionResponse, len(sugs))
	for i, sg := range sugs {
		out[i] = autoTagSuggestionToResponse(sg)
	}
	return c.JSON(fiber.Map{"suggestions": out})
}

// handleAcceptAutoTagSuggestion handles POST /api/v1/assets/:id/auto-tag/suggestions/:sid/accept.
//
// @Summary Accept a pending AI tag suggestion
// @Description Applies the suggested tag to the asset and deletes the suggestion.
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param sid path string true "Suggestion ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Suggestion not found"
// @Router /api/v1/assets/{id}/auto-tag/suggestions/{sid}/accept [post].
func (s *Server) handleAcceptAutoTagSuggestion(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	suggestionID := c.Params("sid")

	tag, err := s.autoTag.AcceptSuggestion(c.Context(), claims.WorkspaceID, assetID, suggestionID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(fiber.Map{"name": tag.Name})
}

// handleAcceptAllAutoTagSuggestions handles POST /api/v1/assets/:id/auto-tag/suggestions/accept-all.
//
// @Summary Accept every pending AI tag suggestion for an asset
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} map[string]int
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/assets/{id}/auto-tag/suggestions/accept-all [post].
func (s *Server) handleAcceptAllAutoTagSuggestions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	accepted, err := s.autoTag.AcceptAll(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(fiber.Map{"accepted": accepted})
}

// handleDismissAutoTagSuggestion handles DELETE /api/v1/assets/:id/auto-tag/suggestions/:sid.
//
// @Summary Dismiss a pending AI tag suggestion
// @Tags Assets
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param sid path string true "Suggestion ID"
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Suggestion not found"
// @Router /api/v1/assets/{id}/auto-tag/suggestions/{sid} [delete].
func (s *Server) handleDismissAutoTagSuggestion(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	suggestionID := c.Params("sid")

	if err := s.autoTag.DismissSuggestion(c.Context(), claims.WorkspaceID, assetID, suggestionID); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleDismissAllAutoTagSuggestions handles DELETE /api/v1/assets/:id/auto-tag/suggestions.
//
// @Summary Dismiss every pending AI tag suggestion for an asset
// @Tags Assets
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/assets/{id}/auto-tag/suggestions [delete].
func (s *Server) handleDismissAllAutoTagSuggestions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if err := s.autoTag.DismissAll(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
