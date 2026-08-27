package api

import (
	"errors"
	"log/slog"

	"damask/server/internal/apperr"
	"damask/server/internal/auth"

	"github.com/gofiber/fiber/v3"
)

// ErrorResponse is the standard error envelope returned on non-2xx responses.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ValidationErrorResponse is returned with HTTP 422 when request validation fails.
type ValidationErrorResponse struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields"`
}

func errRes(c fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(fiber.Map{apiErrorKey: msg})
}

func isInvalidInput(err error) bool {
	return errors.Is(err, apperr.ErrInvalidInput)
}

// ErrorStatusResponse maps a service-layer error to the appropriate HTTP response.
// ErrNotFound -> 404, ErrForbidden -> 403, ErrConflict -> 409,
// ErrInvalidInput -> 422, anything else -> 500.
// Mapped 4xx errors are logged at Warn, unmapped errors at Error.
func ErrorStatusResponse(c fiber.Ctx, err error) error {
	workspaceID := ""
	if claims := auth.GetClaims(c); claims != nil {
		workspaceID = claims.WorkspaceID
	}

	var status int
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		status = fiber.StatusNotFound
	case errors.Is(err, apperr.ErrForbidden):
		status = fiber.StatusForbidden
	case errors.Is(err, apperr.ErrConflict):
		status = fiber.StatusConflict
	case errors.Is(err, apperr.ErrInvalidInput):
		status = fiber.StatusUnprocessableEntity
	default:
		slog.ErrorContext(c.Context(), "unhandled service error",
			"path", c.Path(), "method", c.Method(), "workspace_id", workspaceID, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{apiErrorKey: "internal error"})
	}

	slog.WarnContext(c.Context(), "service error",
		"status", status, "path", c.Path(), "method", c.Method(), "workspace_id", workspaceID, "error", err)
	return c.Status(status).JSON(fiber.Map{apiErrorKey: err.Error()})
}
