package api

import (
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

// EmbedTokenResponse is the JSON shape for a public asset embed token.
type EmbedTokenResponse struct {
	ID        string `json:"id"`
	AssetID   string `json:"asset_id"`
	PublicURL string `json:"public_url"`
	ThumbURL  string `json:"thumb_url"`
	CreatedAt string `json:"created_at"`
	Revoked   bool   `json:"revoked"`
}

func embedTokenDTOToResponse(d *service.EmbedTokenDTO) EmbedTokenResponse {
	return EmbedTokenResponse{
		ID:        d.ID,
		AssetID:   d.AssetID,
		PublicURL: d.PublicURL,
		ThumbURL:  d.ThumbURL,
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
		Revoked:   d.Revoked,
	}
}

// handleCreateEmbedToken creates or returns the active public embed token for an asset.
//
// @Summary Create or return the active public embed token for an asset
// @Description Idempotent. If an active token already exists it is returned unchanged. Creates a new 16-char base62 token otherwise. Requires editor role or above.
// @Tags Embed Tokens
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} EmbedTokenResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Not an editor"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/embed-token [post].
func (s *Server) handleCreateEmbedToken(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	dto, err := s.embedTokens.GetOrCreate(c.Context(), claims.WorkspaceID, assetID, claims.UserID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(embedTokenDTOToResponse(dto))
}

// handleGetEmbedToken returns the active public embed token for an asset, if any.
//
// @Summary Get the active public embed token for an asset
// @Description Returns 404 if no active token exists — this is a valid non-error state used by the frontend to decide which UI to show.
// @Tags Embed Tokens
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} EmbedTokenResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "No active token for this asset"
// @Router /api/v1/assets/{id}/embed-token [get].
//
// Deliberately skips s.assets.Get — workspace_id+asset_id scoping inside
// GetActiveByAssetID already provides workspace isolation.
func (s *Server) handleGetEmbedToken(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	dto, err := s.embedTokens.GetActive(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(embedTokenDTOToResponse(dto))
}

// handleDeleteEmbedToken revokes the active public embed token for an asset.
//
// @Summary Revoke the active public embed token for an asset
// @Description Sets revoked_at on the token so the public /e/:token endpoints return 410 Gone. Requires editor role or above.
// @Tags Embed Tokens
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Not an editor"
// @Failure 404 {object} ErrorResponse "No active token for this asset"
// @Router /api/v1/assets/{id}/embed-token [delete].
func (s *Server) handleDeleteEmbedToken(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if err := s.embedTokens.Revoke(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}
