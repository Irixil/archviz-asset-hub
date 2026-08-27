package api

import (
	"context"

	"github.com/gofiber/fiber/v3"
)

type Validator interface {
	Valid(ctx context.Context) map[string]string
}

// decodeAndValidate binds the body and runs validation in one call.
// Returns true if the handler should continue, false if it already responded.
func decodeAndValidate[T Validator](c fiber.Ctx, body T) (T, bool) {
	if err := c.Bind().Body(&body); err != nil {
		_ = errRes(c, fiber.StatusBadRequest, "invalid request body")
		return body, false
	}
	if problems := body.Valid(c.Context()); len(problems) > 0 {
		_ = c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			apiErrorKey: "validation failed",
			"fields":    problems,
		})
		return body, false
	}
	return body, true
}
