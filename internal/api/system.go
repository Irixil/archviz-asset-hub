package api

import (
	"damask/server/internal/auth"
	"damask/server/internal/config"

	"github.com/gofiber/fiber/v3"
)

// handleHealthz returns a basic availability response.
//
// @Summary Show the status of server.
// @Description get the status of server.
// @Tags Config
// @Accept */*
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /healthz [get].
func handleHealthz(c fiber.Ctx) error {
	return c.JSON(fiber.Map{apiStatusKey: "ok"})
}

// handleSSEEvents streams workspace-scoped Server-Sent Events to the caller.
//
// @Summary Subscribe to real-time events
// @Description Opens a persistent Server-Sent Events (SSE) stream scoped to the authenticated user's workspace. The connection stays open until the client disconnects. A keep-alive comment is sent every 10 seconds to prevent proxy timeouts.<br><br> Each frame carries an <code>id:</code> line and a <code>data:</code> line containing a JSON envelope of the shape <code>{"id": string, "type": string, "payload": object}</code>. On reconnect the browser's native <code>EventSource</code> API automatically sends the last received id as a <code>Last-Event-ID</code> header; the server replays any events published while disconnected, or sends a <code>resync</code> event if the gap has fallen out of the replay buffer (256 events per workspace) and the client should refetch state instead. Current event types include <strong>thumbnail_ready</strong>, <strong>variant_ready</strong>, <strong>variant_failed</strong>, <strong>stack_merge_done</strong>, <strong>variant_draft.ready</strong>, <strong>variant_draft.error</strong>, <strong>workflow_run_failed</strong>, <strong>workflow_run_step_updated</strong>, and <strong>resync</strong>.
// @Tags Events
// @Produce text/event-stream
// @Security BearerAuth
// @Success 200 {string} string "SSE stream"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/events [get].
func (s *Server) handleSSEEvents(c fiber.Ctx) error {
	return s.hub.EventHandler(c)
}

// ConfigResponse is the public server configuration returned to the frontend.
type ConfigResponse struct {
	Demo     bool   `json:"demo"`
	MailHost string `json:"mailHost,omitempty"`
	ExifKeep *bool  `json:"exif_keep,omitempty"`
}

// handleConfig returns public server configuration for the frontend.
//
// @Summary Get server config
// @Description Returns public feature flags and configuration values that the frontend needs before authentication. Currently exposes the <code>demo</code> boolean which, when true, puts the server into read-only demonstration mode.
// @Tags Config
// @Produce json
// @Success 200 {object} ConfigResponse
// @Router /config [get]
// GET /config.
func (s *Server) handleConfig(c fiber.Ctx) error {
	out := ConfigResponse{
		Demo: s.cfg.Demo.DemoMode,
	}
	if claims := auth.GetClaims(c); claims != nil {
		out.MailHost = s.cfg.MailServerHost
		if ws, err := s.workspace.Get(c.Context(), claims.WorkspaceID); err == nil {
			out.ExifKeep = &ws.ExifKeep
		}
	}
	return c.JSON(out)
}

// handleAuthConfig returns which login methods are enabled. Public, reads only config.
func (s *Server) handleAuthConfig(c fiber.Ctx) error {
	oidcRT := config.GetOIDCRuntime()
	googleRT := config.GetGoogleRuntime()
	return c.JSON(fiber.Map{
		"password_auth":  true,
		"signup_enabled": s.cfg.EnableSignup,
		"oidc_enabled":   oidcRT != nil,
		"oidc_label":     s.cfg.OIDC.Label,
		"google_enabled": googleRT != nil && s.cfg.Google.Auth,
		"canva_enabled":  s.cfg.Canva.ClientID != "" && s.cfg.Canva.Auth,
	})
}
