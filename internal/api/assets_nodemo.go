//go:build !demo

package api

import (
	"damask/server/internal/auth"

	"github.com/gofiber/fiber/v3"
)

const DemoMaxAssets = 0
const DemoMaxStorageBytes int64 = 0

func (s *Server) checkDemoUploadCap(_ fiber.Ctx, _ *auth.Claims) (bool, error) { return false, nil }
