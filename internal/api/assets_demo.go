//go:build demo

package api

import (
	"fmt"
	"time"

	"damask/server/internal/auth"

	"github.com/gofiber/fiber/v3"
)

const DemoMaxAssets = 50
const DemoMaxStorageBytes int64 = 100 * 1024 * 1024 // 100 MB

// checkDemoUploadCap enforces the demo asset and storage limits.
// Returns (true, nil) when the cap is hit and the 429 response has already been written.
// Returns (false, nil) when the upload should proceed.
func (s *Server) checkDemoUploadCap(c fiber.Ctx, claims *auth.Claims) (bool, error) {
	if !claims.IsDemo || s.demo == nil {
		return false, nil
	}
	assetCount, storageUsed, err := s.demo.GetUsage(c.Context(), claims.WorkspaceID)
	if err != nil {
		return false, nil // non-fatal: let the upload proceed
	}
	if assetCount >= DemoMaxAssets {
		msg := fmt.Sprintf("Demo upload limit reached (%d assets).", DemoMaxAssets)
		if next := s.demo.NextResetAt(); !next.IsZero() {
			remaining := time.Until(next).Round(time.Minute)
			h := int(remaining.Hours())
			m := int(remaining.Minutes()) % 60
			msg = fmt.Sprintf("Demo upload limit reached. Resets in %dh %dm.", h, m)
		}
		return true, errRes(c, fiber.StatusTooManyRequests, msg)
	}
	if storageUsed >= DemoMaxStorageBytes {
		return true, errRes(c, fiber.StatusTooManyRequests, "Demo storage limit reached (100 MB).")
	}
	return false, nil
}
