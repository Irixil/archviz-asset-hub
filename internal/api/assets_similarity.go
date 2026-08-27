package api

import (
	"database/sql"
	"errors"
	"strings"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/telemetry"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel/attribute"
)

// VisualSimilarResult is one entry in the similar-assets response.
type VisualSimilarResult struct {
	AssetID          string  `json:"asset_id"`
	AssetVersionID   string  `json:"asset_version_id"`
	OriginalFilename string  `json:"original_filename"`
	ThumbnailURL     *string `json:"thumbnail_url,omitempty"`
	MimeType         string  `json:"mime_type"`
	Width            *int64  `json:"width,omitempty"`
	Height           *int64  `json:"height,omitempty"`
}

// handleGetSimilarAssets returns visually similar images for the current version of an asset.
//
// @Summary Find similar images
// @Description Returns asset versions that are visually similar to the current version of the given image asset.
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} map[string][]VisualSimilarResult
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ErrorResponse "not_an_image"
// @Router /api/v1/assets/{id}/similar [get].
func (s *Server) handleGetSimilarAssets(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	asset, err := s.readQueries.GetAssetByID(c.Context(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not fetch asset")
	}

	if !strings.HasPrefix(asset.MimeType, "image/") {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{apiErrorKey: "not_an_image"})
	}

	if asset.CurrentVersionID == nil {
		return c.JSON(fiber.Map{apiResultsKey: []VisualSimilarResult{}})
	}

	if s.visualSimilaritySvc == nil {
		return c.JSON(fiber.Map{apiResultsKey: []VisualSimilarResult{}})
	}

	_, span := telemetry.StartSpan(c.Context(), "api.assets.similar",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
		attribute.String("damask.asset_id", id),
	)
	similar, err := s.visualSimilaritySvc.FindSimilarEnriched(c.Context(), claims.WorkspaceID, *asset.CurrentVersionID)
	telemetry.EndSpan(span, err)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not find similar assets")
	}

	results := make([]VisualSimilarResult, 0, len(similar))
	for _, sa := range similar {
		var thumbURL *string
		if sa.ThumbnailKey != nil {
			u := versionThumbURL(sa.AssetID, sa.AssetVersionID)
			thumbURL = &u
		}
		results = append(results, VisualSimilarResult{
			AssetID:          sa.AssetID,
			AssetVersionID:   sa.AssetVersionID,
			OriginalFilename: sa.OriginalFilename,
			ThumbnailURL:     thumbURL,
			MimeType:         sa.MimeType,
			Width:            sa.Width,
			Height:           sa.Height,
		})
	}

	return c.JSON(fiber.Map{"results": results})
}
