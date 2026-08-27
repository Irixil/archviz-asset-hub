package api

import (
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

type ProjectResponse struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	Name           string    `json:"name"`
	Description    *string   `json:"description,omitempty"`
	Color          *string   `json:"color,omitempty"`
	CoverAssetID   *string   `json:"cover_asset_id,omitempty"`
	CoverVersionID *string   `json:"cover_version_id,omitempty"`
	AssetCount     int64     `json:"asset_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func projectDTOToResponse(d *service.ProjectDTO) ProjectResponse {
	return ProjectResponse{
		ID:             d.ID,
		WorkspaceID:    d.WorkspaceID,
		Name:           d.Name,
		Description:    d.Description,
		Color:          d.Color,
		CoverAssetID:   d.CoverAssetID,
		CoverVersionID: d.CoverVersionID,
		AssetCount:     d.AssetCount,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

// handleCreateProject creates a new project in the active workspace.
//
// @Summary Create a project
// @Description Creates a new project container. Projects group assets and folders together. An optional color (hex) and description can be provided at creation time.
// @Tags Projects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateProjectRequest true "Project details"
// @Success 201 {object} ProjectResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/projects [post].
func (s *Server) handleCreateProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &CreateProjectRequest{})
	if !ok {
		return nil
	}

	dto, err := s.projects.Create(c.Context(), claims.WorkspaceID, service.CreateProjectParams{
		Name:        body.Name,
		Description: body.Description,
		Color:       body.Color,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(projectDTOToResponse(dto))
}

// handleListProjects returns all projects in the active workspace.
//
// @Summary List projects
// @Description Returns all projects in the workspace, each with an <code>asset_count</code> field reflecting the number of assets currently assigned to it.
// @Tags Projects
// @Produce json
// @Security BearerAuth
// @Success 200 {array} ProjectResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/projects [get].
func (s *Server) handleListProjects(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	dtos, err := s.projects.List(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	items := make([]ProjectResponse, len(dtos))
	for i, d := range dtos {
		items[i] = projectDTOToResponse(d)
	}
	return c.JSON(items)
}

// handleGetProject returns a single project by ID.
//
// @Summary Get a project
// @Description Returns the project with the given ID. Returns 404 if the project does not exist or belongs to a different workspace.
// @Tags Projects
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 200 {object} ProjectResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Router /api/v1/projects/{id} [get].
func (s *Server) handleGetProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	dto, err := s.projects.Get(c.Context(), claims.WorkspaceID, c.Params("id"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(projectDTOToResponse(dto))
}

// handleUpdateProject updates a project's metadata.
//
// @Summary Update a project
// @Description Updates the project's name, description, color, or cover asset. All fields are optional — only the fields present in the request body are updated; omitted fields retain their current values.
// @Tags Projects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Param body body UpdateProjectRequest true "Fields to update"
// @Success 200 {object} ProjectResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/projects/{id} [put].
func (s *Server) handleUpdateProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &UpdateProjectRequest{})
	if !ok {
		return nil
	}

	dto, err := s.projects.Update(c.Context(), claims.WorkspaceID, id, service.UpdateProjectParams{
		Name:         body.Name,
		Description:  body.Description,
		Color:        body.Color,
		CoverAssetID: body.CoverAssetID,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(projectDTOToResponse(dto))
}

// handleDeleteProject deletes a project.
//
// @Summary Delete a project
// @Description Permanently deletes the project. Assets that belonged to the project are <em>not</em> deleted — their <code>project_id</code> is set to null so they remain accessible in the unfiltered asset library.
// @Tags Projects
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Router /api/v1/projects/{id} [delete].
func (s *Server) handleDeleteProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if err := s.projects.Delete(c.Context(), claims.WorkspaceID, id); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// fiber:context-methods migrated
