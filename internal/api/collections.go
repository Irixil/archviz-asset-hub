package api

import (
	"context"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

// allAssetsInWorkspace returns true iff every ID in assetIDs belongs to workspaceID.
func (s *Server) allAssetsInWorkspace(ctx context.Context, workspaceID string, assetIDs []string) (bool, error) {
	if len(assetIDs) == 0 {
		return true, nil
	}
	count, err := s.assets.CountByIDs(ctx, workspaceID, assetIDs)
	if err != nil {
		return false, err
	}
	return count == int64(len(assetIDs)), nil
}

type CollectionResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	AssetCount  int64     `json:"asset_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func collectionDTOToResponse(d *service.CollectionDTO) CollectionResponse {
	return CollectionResponse{
		ID:          d.ID,
		WorkspaceID: d.WorkspaceID,
		Name:        d.Name,
		Description: d.Description,
		CreatedBy:   d.CreatedBy,
		AssetCount:  d.AssetCount,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

// @Summary Create a collection
// @Description Creates a named collection in the workspace. Supply <code>asset_ids</code> to pre-populate it (e.g. when saving a working stack). All asset IDs must belong to the authenticated workspace.
// @Tags Collections
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createCollectionRequest true "Collection details"
// @Success 201 {object} CollectionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "One or more assets not in workspace"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/collections [post].
func (s *Server) handleCreateCollection(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &createCollectionRequest{})
	if !ok {
		return nil
	}

	if len(body.AssetIDs) > 0 {
		allFound, err := s.allAssetsInWorkspace(c.Context(), claims.WorkspaceID, body.AssetIDs)
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not verify assets")
		}
		if !allFound {
			return errRes(c, fiber.StatusForbidden, "one or more assets not found in workspace")
		}
	}

	col, err := s.collections.Create(c.Context(), claims.WorkspaceID, service.CreateCollectionParams{
		Name:        body.Name,
		Description: body.Description,
		CreatedBy:   claims.UserID,
		AssetIDs:    body.AssetIDs,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(collectionDTOToResponse(col))
}

// @Summary List collections
// @Description Returns all collections in the workspace with their asset counts.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Success 200 {array} CollectionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/collections [get].
func (s *Server) handleListCollections(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	cols, err := s.collections.List(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	out := make([]CollectionResponse, len(cols))
	for i, col := range cols {
		out[i] = collectionDTOToResponse(col)
	}
	return c.JSON(out)
}

// @Summary Get a collection
// @Description Returns collection metadata and its full asset list, ordered by position then insertion date.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "Collection ID"
// @Success 200 {object} CollectionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Collection not found"
// @Router /api/v1/collections/{id} [get].
func (s *Server) handleGetCollection(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	col, err := s.collections.Get(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	assets, err := s.collections.ListAssets(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	type collectionDetailResponse struct {
		CollectionResponse

		Assets []AssetResponse `json:"assets"`
	}

	assetResponses := make([]AssetResponse, len(assets))
	for i, a := range assets {
		assetResponses[i] = assetToResponse(dtoToDBAsset(a))
	}

	return c.JSON(collectionDetailResponse{
		CollectionResponse: collectionDTOToResponse(col),
		Assets:             assetResponses,
	})
}

// @Summary Update a collection
// @Description Updates the collection's name and description. Requires editor role.
// @Tags Collections
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Collection ID"
// @Param body body updateCollectionRequest true "Fields to update"
// @Success 200 {object} CollectionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Insufficient role"
// @Failure 404 {object} ErrorResponse "Collection not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/collections/{id} [put].
func (s *Server) handleUpdateCollection(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &updateCollectionRequest{})
	if !ok {
		return nil
	}

	col, err := s.collections.Update(c.Context(), claims.WorkspaceID, id, service.UpdateCollectionParams{
		Name:        &body.Name,
		Description: &body.Description,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(collectionDTOToResponse(col))
}

// @Summary Delete a collection
// @Description Deletes the collection and removes all its asset memberships. The assets themselves are not deleted. Requires owner role.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "Collection ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Insufficient role"
// @Failure 404 {object} ErrorResponse "Collection not found"
// @Router /api/v1/collections/{id} [delete].
func (s *Server) handleDeleteCollection(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if err := s.collections.Delete(c.Context(), claims.WorkspaceID, id); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Add asset to collection
// @Description Adds a single asset to the collection. Idempotent — adding an asset that is already in the collection succeeds silently. Requires editor role.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "Collection ID"
// @Param aid path string true "Asset ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Insufficient role"
// @Failure 404 {object} ErrorResponse "Collection or asset not found"
// @Router /api/v1/collections/{id}/assets/{aid} [post].
func (s *Server) handleAddCollectionAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")
	assetID := c.Params("aid")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	if err := s.collections.AddAsset(c.Context(), claims.WorkspaceID, id, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Remove asset from collection
// @Description Removes a single asset from the collection. The asset itself is not deleted. Requires editor role.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "Collection ID"
// @Param aid path string true "Asset ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Insufficient role"
// @Failure 404 {object} ErrorResponse "Collection not found"
// @Router /api/v1/collections/{id}/assets/{aid} [delete].
func (s *Server) handleRemoveCollectionAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")
	assetID := c.Params("aid")

	if err := s.collections.RemoveAsset(c.Context(), claims.WorkspaceID, id, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary List collections containing an asset
// @Description Returns all collections in the workspace that contain the given asset.
// @Tags Collections
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {array} CollectionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/collections [get].
func (s *Server) handleListAssetCollections(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	cols, err := s.collections.ListForAsset(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	out := make([]CollectionResponse, len(cols))
	for i, col := range cols {
		out[i] = collectionDTOToResponse(col)
	}
	return c.JSON(out)
}
