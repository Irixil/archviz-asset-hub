package api

import (
	"context"
	"errors"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

// ShareResponse is the JSON shape for a share object returned to clients.
type ShareResponse struct {
	ID            string  `json:"id"`
	WorkspaceID   string  `json:"workspace_id"`
	CreatedBy     string  `json:"created_by"`
	Label         string  `json:"label"`
	TargetType    string  `json:"target_type"`
	TargetID      string  `json:"target_id"`
	HasPassword   bool    `json:"has_password"`
	ExpiresAt     *string `json:"expires_at,omitempty"`
	AllowComments bool    `json:"allow_comments"`
	AllowDownload bool    `json:"allow_download"`
	ViewCount     int64   `json:"view_count"`
	CreatedAt     string  `json:"created_at"`
	RevokedAt     *string `json:"revoked_at,omitempty"`
	IsExpired     bool    `json:"is_expired"`
	PublicURL     string  `json:"public_url"`
}

func shareDTOToResponse(d *service.ShareDTO, baseURL string) ShareResponse {
	isExpired := false
	if d.ExpiresAt != nil {
		t, err := time.Parse("2006-01-02T15:04:05Z", *d.ExpiresAt)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", *d.ExpiresAt)
		}
		if err == nil && time.Now().After(t) {
			isExpired = true
		}
	}
	return ShareResponse{
		ID:            d.ID,
		WorkspaceID:   d.WorkspaceID,
		CreatedBy:     d.CreatedBy,
		Label:         d.Label,
		TargetType:    d.TargetType,
		TargetID:      d.TargetID,
		HasPassword:   d.PasswordHash != nil,
		ExpiresAt:     d.ExpiresAt,
		AllowComments: d.AllowComments,
		AllowDownload: d.AllowDownload,
		ViewCount:     d.ViewCount,
		CreatedAt:     d.CreatedAt.Format(time.RFC3339),
		RevokedAt:     d.RevokedAt,
		IsExpired:     isExpired,
		PublicURL:     baseURL + "/s/" + d.ID,
	}
}

// shareTargetError is a sentinel for target validation failures.
type shareTargetError struct {
	status int
	msg    string
}

func (e *shareTargetError) Error() string { return e.msg }

// targetNotFound maps a target lookup failure to a 404, unless the request
// context itself was cancelled, in which case the ctx error is returned as-is.
func targetNotFound(ctx context.Context) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return &shareTargetError{status: fiber.StatusNotFound, msg: apiTargetNotFound}
}

// validateShareTarget checks that the given target_id exists in the workspace.
func (s *Server) validateShareTarget(ctx context.Context, workspaceID, targetType, targetID string) error {
	switch targetType {
	case apiTargetProject:
		if _, err := s.projects.Get(ctx, workspaceID, targetID); err != nil {
			return targetNotFound(ctx)
		}
	case apiTargetAsset:
		if _, err := s.assets.Get(ctx, workspaceID, targetID); err != nil {
			return targetNotFound(ctx)
		}
	case "collection":
		if _, err := s.collections.Get(ctx, workspaceID, targetID); err != nil {
			return targetNotFound(ctx)
		}
	}
	return nil
}

// @Summary Create a share
// @Description Creates a public share link for the given target (asset or project).
// @Tags Shares
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateShareRequest true "Share settings"
// @Success 201 {object} ShareResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Target asset or project not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/shares [post].
func (s *Server) handleCreateShare(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &CreateShareRequest{})
	if !ok {
		return nil
	}

	if err := s.validateShareTarget(c.Context(), claims.WorkspaceID, body.TargetType, body.TargetID); err != nil {
		te := &shareTargetError{}
		if errors.As(err, &te) {
			return errRes(c, te.status, te.msg)
		}
		return errRes(c, fiber.StatusInternalServerError, "could not validate target")
	}

	allowDownload := true
	if body.AllowDownload != nil {
		allowDownload = *body.AllowDownload
	}

	sh, err := s.shares.Create(c.Context(), claims.WorkspaceID, service.CreateShareParams{
		CreatedBy:     claims.UserID,
		Label:         body.Label,
		TargetType:    body.TargetType,
		TargetID:      body.TargetID,
		Password:      body.Password,
		ExpiresInDays: body.ExpiresInDays,
		AllowComments: body.AllowComments,
		AllowDownload: allowDownload,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(shareDTOToResponse(sh, s.cfg.BaseURL.String()))
}

// @Summary List shares
// @Description Returns all shares created in the workspace.
// @Tags Shares
// @Produce json
// @Security BearerAuth
// @Success 200 {array} ShareResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/shares [get].
func (s *Server) handleListShares(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	shares, err := s.shares.List(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	items := make([]ShareResponse, len(shares))
	for i, sh := range shares {
		items[i] = shareDTOToResponse(sh, s.cfg.BaseURL.String())
	}
	return c.JSON(items)
}

// @Summary Get a share
// @Description Returns the share record including its current status.
// @Tags Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Success 200 {object} ShareResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Router /api/v1/shares/{id} [get].
func (s *Server) handleGetShare(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	sh, err := s.shares.Get(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(shareDTOToResponse(sh, s.cfg.BaseURL.String()))
}

// @Summary Update a share
// @Description Updates the share's label, password, expiry, and permission flags.
// @Tags Shares
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param body body UpdateShareRequest true "Fields to update"
// @Success 200 {object} ShareResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/shares/{id} [put].
func (s *Server) handleUpdateShare(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &UpdateShareRequest{})
	if !ok {
		return nil
	}

	var clearPassword bool
	if body.ClearPassword != nil {
		clearPassword = *body.ClearPassword
	}
	var clearExpiry bool
	if body.ClearExpiry != nil {
		clearExpiry = *body.ClearExpiry
	}

	sh, err := s.shares.Update(c.Context(), claims.WorkspaceID, id, service.UpdateShareParams{
		Label:         body.Label,
		Password:      body.Password,
		ClearPassword: clearPassword,
		ExpiresAt:     body.ExpiresAt,
		ClearExpiry:   clearExpiry,
		AllowComments: body.AllowComments,
		AllowDownload: body.AllowDownload,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(shareDTOToResponse(sh, s.cfg.BaseURL.String()))
}

// @Summary Revoke a share
// @Description Sets revoked_at on the share so that all future public access attempts return 404.
// @Tags Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Router /api/v1/shares/{id} [delete].
func (s *Server) handleRevokeShare(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if err := s.shares.Revoke(c.Context(), claims.WorkspaceID, id); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}
