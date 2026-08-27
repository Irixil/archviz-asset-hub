package api

import (
	"strings"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

type FolderResponse struct {
	ID          string           `json:"id"`
	WorkspaceID string           `json:"workspace_id"`
	ProjectID   string           `json:"project_id"`
	ParentID    *string          `json:"parent_id,omitempty"`
	Name        string           `json:"name"`
	Slug        *string          `json:"slug,omitempty"`
	Position    int64            `json:"position"`
	AssetCount  int64            `json:"asset_count"`
	Children    []FolderResponse `json:"children"`
	CreatedAt   string           `json:"created_at"`
}

func folderDTOToResponse(d *service.FolderDTO) FolderResponse {
	return FolderResponse{
		ID:          d.ID,
		WorkspaceID: d.WorkspaceID,
		ProjectID:   d.ProjectID,
		ParentID:    d.ParentID,
		Name:        d.Name,
		Slug:        d.Slug,
		Position:    d.Position,
		AssetCount:  0,
		Children:    []FolderResponse{},
		CreatedAt:   d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// handleCreateFolder creates a folder inside a project.
//
// @Summary Create a folder
// @Description Creates a new folder in the given project. Folders are at most 2 levels deep — a root folder can have child folders, but child folders cannot have their own children.<br> Use <code>parent_id</code> to create a sub-folder inside an existing root folder. An optional <code>position</code> integer controls ordering in the UI.
// @Tags Folders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Param body body CreateFolderRequest true "Folder details"
// @Success 201 {object} FolderResponse
// @Failure 400 {object} ErrorResponse "Parent folder belongs to a different project"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project or parent folder not found"
// @Failure 409 {object} ErrorResponse "A folder with that name already exists here"
// @Failure 422 {object} ErrorResponse "Max folder depth exceeded"
// @Router /api/v1/projects/{id}/folders [post].
func (s *Server) handleCreateFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	projectID := c.Params("id")

	// Verify project belongs to workspace via the projects service.
	if _, err := s.projects.Get(c.Context(), claims.WorkspaceID, projectID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	body, ok := decodeAndValidate(c, &CreateFolderRequest{})
	if !ok {
		return nil
	}

	dto, err := s.folders.Create(c.Context(), claims.WorkspaceID, projectID, service.CreateFolderParams{
		Name:     body.Name,
		ParentID: body.ParentID,
		Position: body.Position,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(folderDTOToResponse(dto))
}

// handleGetFolders returns the full folder tree for a project.
//
// @Summary Get folder tree
// @Description Returns a recursive tree of all folders in the project (up to 2 levels deep). Each folder includes its children in a <code>children</code> array and an <code>asset_count</code> showing how many assets are directly inside it (not counting descendants).
// @Tags Folders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 200 {array} FolderResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Router /api/v1/projects/{id}/folders [get].
func (s *Server) handleGetFolders(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	projectID := c.Params("id")

	if _, err := s.projects.Get(c.Context(), claims.WorkspaceID, projectID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	tree, err := s.folders.ListTree(c.Context(), claims.WorkspaceID, projectID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load folders")
	}

	roots := folderTreeToResponse(tree)
	return c.JSON(roots)
}

func folderTreeToResponse(nodes []*service.FolderTreeDTO) []FolderResponse {
	out := make([]FolderResponse, len(nodes))
	for i, n := range nodes {
		out[i] = FolderResponse{
			ID:          n.ID,
			WorkspaceID: n.WorkspaceID,
			ProjectID:   n.ProjectID,
			ParentID:    n.ParentID,
			Name:        n.Name,
			Slug:        n.Slug,
			Position:    n.Position,
			AssetCount:  n.AssetCount,
			Children:    folderTreeToResponse(n.Children),
			CreatedAt:   n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return out
}

// handleUpdateFolder renames or repositions a folder.
//
// @Summary Update a folder
// @Description Updates the folder's name and/or position. All fields are optional. Renaming automatically regenerates the folder's URL slug.
// @Tags Folders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Folder ID"
// @Param body body UpdateFolderRequest true "Fields to update"
// @Success 200 {object} FolderResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Folder not found"
// @Failure 409 {object} ErrorResponse "A folder with that name already exists here"
// @Router /api/v1/folders/{id} [put].
func (s *Server) handleUpdateFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &UpdateFolderRequest{})
	if !ok {
		return nil
	}

	dto, err := s.folders.Update(c.Context(), claims.WorkspaceID, id, service.UpdateFolderParams{
		Name:     body.Name,
		Position: body.Position,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errRes(c, fiber.StatusConflict, "a folder with that name already exists here")
		}
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(folderDTOToResponse(dto))
}

// handleDeleteFolder deletes a folder and its children.
//
// @Summary Delete a folder
// @Description Permanently deletes the folder and any child folders. Assets inside the deleted folder(s) are <em>not</em> deleted — their <code>folder_id</code> is set to null so they remain in the project.
// @Tags Folders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Folder ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Folder not found"
// @Router /api/v1/folders/{id} [delete].
func (s *Server) handleDeleteFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if err := s.folders.Delete(c.Context(), claims.WorkspaceID, id); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// fiber:context-methods migrated
