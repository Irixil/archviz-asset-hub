package api

import (
	"strconv"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

const maxExportRunsPageSize = 100

func (s *Server) handleCreateExportConfig(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var body service.CreateExportConfigParams
	if err := c.Bind().JSON(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	dto, err := s.exports.Create(c.Context(), claims.WorkspaceID, claims.UserID, body)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(dto)
}

func (s *Server) handleListExportConfigs(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	configs, err := s.exports.List(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(configs)
}

func (s *Server) handleGetExportConfig(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")
	dto, err := s.exports.Get(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(dto)
}

func (s *Server) handleUpdateExportConfig(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	var body service.UpdateExportConfigParams
	if err := c.Bind().JSON(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	dto, err := s.exports.Update(c.Context(), claims.WorkspaceID, id, body)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(dto)
}

func (s *Server) handleDeleteExportConfig(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")
	if err := s.exports.Delete(c.Context(), claims.WorkspaceID, id); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleValidateExportDestination(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var body service.CreateExportConfigParams
	if err := c.Bind().JSON(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := s.exports.ValidateDestinationConfig(
		c.Context(),
		claims.WorkspaceID,
		body.DestType,
		body.DestConfig,
	); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"ok": false, apiErrorKey: err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) handleTriggerExport(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")
	run, err := s.exports.TriggerManual(c.Context(), claims.WorkspaceID, claims.UserID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.Status(fiber.StatusAccepted).JSON(run)
}

func (s *Server) handleGetExportRun(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	runID := c.Params("runID")
	run, err := s.exports.GetRun(c.Context(), claims.WorkspaceID, runID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(run)
}

func (s *Server) handleListExportRuns(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	limit := 20
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxExportRunsPageSize {
		limit = maxExportRunsPageSize
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	runs, err := s.exports.ListRuns(c.Context(), claims.WorkspaceID, id, limit, offset)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(runs)
}
