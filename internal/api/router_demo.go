//go:build demo

package api

import (
	"damask/server/internal/auth"
	"damask/server/internal/config"

	"github.com/gofiber/fiber/v3"
)

func (s *Server) registerDemoRoutes(app *fiber.App, cfg *config.Config) {
	if !cfg.Demo.DemoMode {
		return
	}
	app.Post("/demo/session", s.handleDemoSession)
	app.Get("/demo/status", s.handleDemoStatus)
}

func demoBlockMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		claims := auth.GetClaims(c)
		if claims != nil && claims.IsDemo {
			return c.Status(fiber.StatusForbidden).JSON(demoRestrictedResponse)
		}
		return c.Next()
	}
}
