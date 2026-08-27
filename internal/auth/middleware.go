package auth

import (
	"context"
	"strings"

	"damask/server/internal/telemetry"

	"github.com/gofiber/fiber/v3"
)

type Role string

var Owner = Role("owner")
var Editor = Role("editor")
var Viewer = Role("viewer")

const claimsKey = "claims"

// RequireAuth validates the token from Authorization header or auth_token cookie.
// On success it stores the Claims in c.Locals and enriches c.Context() with the actor.
func RequireAuth(maker *Maker) fiber.Handler {
	return func(c fiber.Ctx) error {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing token")
		}

		claims, err := maker.VerifyToken(tokenStr)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid token")
		}

		c.Locals(claimsKey, claims)
		c.SetContext(WithActor(c.Context(), Actor{
			Type:        "user",
			UserID:      &claims.UserID,
			WorkspaceID: &claims.WorkspaceID,
		}))
		telemetry.EnrichSpan(c, claims.WorkspaceID, claims.UserID)
		return c.Next()
	}
}

// GetClaims retrieves the validated Claims from the request context.
// Must be called after RequireAuth middleware.
func GetClaims(c fiber.Ctx) *Claims {
	claims, _ := c.Locals(claimsKey).(*Claims)
	return claims
}

// roleRank maps role names to an integer for comparison.
var roleRank = map[Role]int{
	Viewer: 1,
	Editor: 2, //nolint:mnd // rank value
	Owner:  3, //nolint:mnd // rank value
}

// RequireRole returns a middleware that enforces a minimum role level.
// It expects a getRoleFn to look up the current user's role in the workspace.
func RequireRole(
	getRoleFn func(ctx context.Context, workspaceID, userID string) (Role, error),
	minRole Role,
) fiber.Handler {
	return func(c fiber.Ctx) error {
		claims := GetClaims(c)
		if claims == nil {
			return fiber.NewError(fiber.StatusUnauthorized, "missing token")
		}

		role, err := getRoleFn(c.Context(), claims.WorkspaceID, claims.UserID)
		if err != nil {
			return fiber.NewError(fiber.StatusForbidden, "access denied")
		}

		if roleRank[role] < roleRank[minRole] {
			return fiber.NewError(fiber.StatusForbidden, "insufficient permissions")
		}

		return c.Next()
	}
}

// OptionalAuth attempts token validation but never rejects the request.
// If a valid token is present, Claims are stored in c.Locals and c.Context() is enriched with the actor.
func OptionalAuth(maker *Maker) fiber.Handler {
	return func(c fiber.Ctx) error {
		tokenStr := extractToken(c)
		if tokenStr != "" {
			if claims, err := maker.VerifyToken(tokenStr); err == nil {
				c.Locals(claimsKey, claims)
				c.SetContext(WithActor(c.Context(), Actor{
					Type:        "user",
					UserID:      &claims.UserID,
					WorkspaceID: &claims.WorkspaceID,
				}))
				telemetry.EnrichSpan(c, claims.WorkspaceID, claims.UserID)
			}
		}
		return c.Next()
	}
}

func extractToken(c fiber.Ctx) string {
	// Try Authorization: Bearer <token>
	if h := c.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// Fall back to httpOnly cookie
	return c.Cookies("auth_token")
}

const shareClaimsKey = "share_claims"

// RequireShareSession validates the share session token from the Authorization
// header or share_token cookie. On success it stores ShareClaims in c.Locals.
// It also re-validates that the share_id in the token matches the :id path param.
func RequireShareSession(maker *Maker) fiber.Handler {
	return func(c fiber.Ctx) error {
		tokenStr := extractShareToken(c)
		if tokenStr == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing share token")
		}

		claims, err := maker.VerifyShareToken(tokenStr)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid share token")
		}

		// Path param :id must match the token's share_id
		if shareID := c.Params("id"); shareID != "" && shareID != claims.ShareID {
			return fiber.NewError(fiber.StatusForbidden, "share token does not match this share")
		}

		c.Locals(shareClaimsKey, claims)
		return c.Next()
	}
}

// GetShareClaims retrieves the validated ShareClaims from the request context.
// Must be called after RequireShareSession middleware.
func GetShareClaims(c fiber.Ctx) *ShareClaims {
	claims, _ := c.Locals(shareClaimsKey).(*ShareClaims)
	return claims
}

func extractShareToken(c fiber.Ctx) string {
	if token := c.Get("X-Share-Token"); token != "" {
		return token
	}
	ckt := c.Cookies("share_token")
	if ckt != "" {
		return ckt
	}
	shareID := c.Params("id")
	return c.Cookies("share_token_" + shareID)
}

// fiber:context-methods migrated
