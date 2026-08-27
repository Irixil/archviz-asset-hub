//go:build !demo

package api

import (
	"damask/server/internal/config"

	"github.com/gofiber/fiber/v3"
)

func (s *Server) registerDemoRoutes(_ *fiber.App, _ *config.Config) {}

func demoBlockMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error { return c.Next() }
}
