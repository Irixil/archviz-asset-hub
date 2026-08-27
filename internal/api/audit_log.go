package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

// EventActor is the actor embedded in each event response.
type EventActor struct {
	Type string  `json:"type"`
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

// EventResponse is a single event in the API response. Exported for test access.
type EventResponse struct {
	ID            string          `json:"id"`
	EventType     string          `json:"event_type"`
	Actor         EventActor      `json:"actor"`
	Payload       json.RawMessage `json:"payload"        swaggertype:"object"`
	CreatedAt     string          `json:"created_at"`
	HumanReadable string          `json:"human_readable"`
}

// EventListResponse wraps paginated events. Exported for test access.
type EventListResponse struct {
	Events     []EventResponse `json:"events"`
	NextCursor *string         `json:"next_cursor,omitempty"`
	HasMore    bool            `json:"has_more"`
}

// ActivityEventResponse is a unified event for the workspace activity feed.
type ActivityEventResponse struct {
	EventResponse

	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
}

// ActivityFeedResponse is the paginated workspace activity feed.
type ActivityFeedResponse struct {
	Events     []ActivityEventResponse `json:"events"`
	NextCursor *string                 `json:"next_cursor,omitempty"`
	HasMore    bool                    `json:"has_more"`
}

func auditDTOToEventResponse(d service.AuditEventDTO) EventResponse {
	return EventResponse{
		ID:        d.ID,
		EventType: d.EventType,
		Actor: EventActor{
			Type: d.Actor.Type,
			ID:   d.Actor.ID,
			Name: d.Actor.Name,
		},
		Payload:       d.Payload,
		CreatedAt:     d.CreatedAt,
		HumanReadable: d.HumanReadable,
	}
}

// @Summary List asset events
// @Description Returns paginated audit events for the given asset.
// @Tags Audit
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param limit query integer false "Page size (1–200, default 50)"
// @Param cursor query string false "Pagination cursor"
// @Param types query string false "Comma-separated event type filter"
// @Success 200 {object} EventListResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/events [get].
func (s *Server) handleListAssetEvents(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	limit, cursor, types := parseEventQueryParams(c)

	result, err := s.auditLog.ListAssetEvents(c.Context(), service.ListAssetEventsParams{
		AssetID:     assetID,
		WorkspaceID: claims.WorkspaceID,
		Limit:       limit,
		Cursor:      cursor,
		Types:       types,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(auditListDTOToResponse(result))
}

// @Summary List project events
// @Description Returns paginated audit events for the given project.
// @Tags Audit
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Param limit query integer false "Page size (1–200, default 50)"
// @Param cursor query string false "Pagination cursor"
// @Param types query string false "Comma-separated event type filter"
// @Success 200 {object} EventListResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Router /api/v1/projects/{id}/events [get].
func (s *Server) handleListProjectEvents(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	projectID := c.Params("id")

	if _, err := s.projects.Get(c.Context(), claims.WorkspaceID, projectID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	limit, cursor, types := parseEventQueryParams(c)

	result, err := s.auditLog.ListProjectEvents(c.Context(), service.ListProjectEventsParams{
		ProjectID:   projectID,
		WorkspaceID: claims.WorkspaceID,
		Limit:       limit,
		Cursor:      cursor,
		Types:       types,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(auditListDTOToResponse(result))
}

// @Summary List workspace activity
// @Description Returns a unified, cursor-paginated activity feed combining asset and project events.
// @Tags Audit
// @Produce json
// @Security BearerAuth
// @Param limit query integer false "Page size (1–100, default 20)"
// @Param cursor query string false "Pagination cursor"
// @Param user_id query string false "Filter by actor user ID"
// @Param types query string false "Comma-separated event type filter"
// @Success 200 {object} ActivityFeedResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/activity [get].
func (s *Server) handleListWorkspaceActivity(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	limit, cursor, types := parseEventQueryParams(c)
	userID := c.Query("user_id")

	result, err := s.auditLog.ListWorkspaceActivity(c.Context(), service.ListWorkspaceActivityParams{
		WorkspaceID: claims.WorkspaceID,
		Limit:       limit,
		Cursor:      cursor,
		UserID:      userID,
		Types:       types,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	events := make([]ActivityEventResponse, len(result.Events))
	for i, d := range result.Events {
		events[i] = ActivityEventResponse{
			EventResponse: auditDTOToEventResponse(d.AuditEventDTO),
			EntityType:    d.EntityType,
			EntityID:      d.EntityID,
		}
	}

	return c.JSON(ActivityFeedResponse{
		Events:     events,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	})
}

// @Summary Export workspace activity
// @Description Exports up to 10,000 workspace audit events as a CSV download.
// @Tags Audit
// @Produce text/csv
// @Security BearerAuth
// @Param since query string false "Start date filter (YYYY-MM-DD)"
// @Param until query string false "End date filter (YYYY-MM-DD)"
// @Param format query string false "Export format (only csv supported)"
// @Success 200 {file} binary
// @Failure 400 {object} ErrorResponse "Invalid date format or unsupported format"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/activity/export [get].
func (s *Server) handleExportActivity(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	since := c.Query("since")
	until := c.Query("until")
	format := c.Query("format", "csv")
	if format != "csv" {
		return errRes(c, fiber.StatusBadRequest, "unsupported format; use format=csv")
	}
	if since != "" {
		if _, err := time.Parse("2006-01-02", since); err != nil {
			return errRes(c, fiber.StatusBadRequest, "invalid since date; use YYYY-MM-DD")
		}
	}
	if until != "" {
		if _, err := time.Parse("2006-01-02", until); err != nil {
			return errRes(c, fiber.StatusBadRequest, "invalid until date; use YYYY-MM-DD")
		}
	}

	csv, err := s.auditLog.ExportActivity(c.Context(), service.ExportActivityParams{
		WorkspaceID: claims.WorkspaceID,
		Since:       since,
		Until:       until,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", `attachment; filename="activity-export.csv"`)
	return c.SendString(csv)
}

// parseEventQueryParams extracts limit, cursor, and types filter from query params.
func parseEventQueryParams(c fiber.Ctx) (limit int64, cursor string, types []string) {
	limit = 50
	if l := c.Query("limit"); l != "" {
		var n int64
		if _, err := fmt.Sscan(l, &n); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	cursor = c.Query("cursor")
	raw := strings.TrimSpace(c.Query("types"))
	if raw != "" {
		for t := range strings.SplitSeq(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				types = append(types, t)
			}
		}
	}
	return limit, cursor, types
}

func auditListDTOToResponse(d *service.AuditEventListDTO) EventListResponse {
	events := make([]EventResponse, len(d.Events))
	for i, e := range d.Events {
		events[i] = auditDTOToEventResponse(e)
	}
	return EventListResponse{
		Events:     events,
		NextCursor: d.NextCursor,
		HasMore:    d.HasMore,
	}
}
