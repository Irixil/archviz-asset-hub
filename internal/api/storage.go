package api

import (
	"damask/server/internal/auth"

	"github.com/gofiber/fiber/v3"
)

// GET /api/v1/workspace/storage.
func (s *Server) handleGetWorkspaceStorage(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	usage, err := s.storageSvc.GetUsage(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(usage)
}

// GET /api/v1/workspace/storage/projects/:project_id/folders.
func (s *Server) handleGetProjectFolderStorage(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	projectID := c.Params("project_id")
	folders, err := s.storageSvc.GetFolderUsage(c.Context(), claims.WorkspaceID, projectID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(fiber.Map{"project_id": projectID, "folders": folders})
}
