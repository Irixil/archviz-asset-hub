package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/repository"
	apptelemetry "damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
)

const (
	minAssetEventsPageSize       = 50
	maxAssetEventsPageSize       = 200
	minWorkspaceActivityPageSize = 20
	maxWorkspaceActivityPageSize = 100
)

// AuditEventActorDTO is the actor embedded in each audit event.
type AuditEventActorDTO struct {
	Type string  `json:"type"`
	ID   *string `json:"id"`
	Name *string `json:"name"`
}

// AuditEventDTO is a single audit event returned by the service.
type AuditEventDTO struct {
	ID            string             `json:"id"`
	EventType     string             `json:"event_type"`
	Actor         AuditEventActorDTO `json:"actor"`
	Payload       json.RawMessage    `json:"payload"`
	CreatedAt     string             `json:"created_at"`
	HumanReadable string             `json:"human_readable"`
}

// ActivityEventDTO extends AuditEventDTO with entity context for the workspace feed.
type ActivityEventDTO struct {
	AuditEventDTO

	EntityType string `json:"entity_type"` // "asset" | "project"
	EntityID   string `json:"entity_id"`
}

// AuditEventListDTO is a paginated list of audit events.
type AuditEventListDTO struct {
	Events     []AuditEventDTO `json:"events"`
	NextCursor *string         `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
}

// ActivityListDTO is a paginated workspace-wide activity feed.
type ActivityListDTO struct {
	Events     []ActivityEventDTO `json:"events"`
	NextCursor *string            `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

// ListAssetEventsParams is the input for AuditLogService.ListAssetEvents.
type ListAssetEventsParams struct {
	AssetID     string
	WorkspaceID string
	Limit       int64
	Cursor      string
	Types       []string
}

// ListProjectEventsParams is the input for AuditLogService.ListProjectEvents.
type ListProjectEventsParams struct {
	ProjectID   string
	WorkspaceID string
	Limit       int64
	Cursor      string
	Types       []string
}

// ListWorkspaceActivityParams is the input for AuditLogService.ListWorkspaceActivity.
type ListWorkspaceActivityParams struct {
	WorkspaceID string
	Limit       int64
	Cursor      string
	UserID      string
	Types       []string
}

// ExportActivityParams is the input for AuditLogService.ExportActivity.
type ExportActivityParams struct {
	WorkspaceID string
	Since       string // YYYY-MM-DD, optional
	Until       string // YYYY-MM-DD, optional
}

type auditLogService struct {
	repo repository.AuditLogRepository
}

// NewAuditLogService returns an AuditLogService.
func NewAuditLogService(repo repository.AuditLogRepository) AuditLogService {
	return &auditLogService{repo: repo}
}

func (s *auditLogService) ListAssetEvents(ctx context.Context, p ListAssetEventsParams) (*AuditEventListDTO, error) {
	limit := clampLimit(p.Limit, minAssetEventsPageSize, maxAssetEventsPageSize)
	typesFilter := makeTypesFilter(p.Types)

	rows, err := s.repo.ListAssetEvents(ctx, repository.ListAuditEventsParams{
		EntityID:    p.AssetID,
		WorkspaceID: p.WorkspaceID,
		Cursor:      p.Cursor,
		EventType:   singleTypeString(typesFilter),
		Limit:       limit + 1,
	})
	if err != nil {
		return nil, err
	}

	return listAuditEvents(rows, limit, typesFilter), nil
}

func (s *auditLogService) ListProjectEvents(
	ctx context.Context,
	p ListProjectEventsParams,
) (*AuditEventListDTO, error) {
	limit := clampLimit(p.Limit, minAssetEventsPageSize, maxAssetEventsPageSize)
	typesFilter := makeTypesFilter(p.Types)

	rows, err := s.repo.ListProjectEvents(ctx, repository.ListAuditEventsParams{
		EntityID:    p.ProjectID,
		WorkspaceID: p.WorkspaceID,
		Cursor:      p.Cursor,
		EventType:   singleTypeString(typesFilter),
		Limit:       limit + 1,
	})
	if err != nil {
		return nil, err
	}

	return listAuditEvents(rows, limit, typesFilter), nil
}

func listAuditEvents(rows []repository.AuditEvent, limit int64, typesFilter map[string]bool) *AuditEventListDTO {
	events := make([]AuditEventDTO, 0, len(rows))
	for _, r := range rows {
		if len(typesFilter) > 0 && !typesFilter[r.EventType] {
			continue
		}
		events = append(
			events,
			buildAuditEventDTO(r.ID, r.EventType, r.CreatedAt, r.Payload, r.UserID, r.UserName, r.ActorType),
		)
	}
	return paginateAuditEvents(events, limit)
}

func (s *auditLogService) ListWorkspaceActivity(
	ctx context.Context,
	p ListWorkspaceActivityParams,
) (out *ActivityListDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.audit.list_workspace_activity",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.Int64("damask.audit.limit", p.Limit),
		attribute.Bool("damask.audit.has_cursor", p.Cursor != ""),
		attribute.Bool("damask.audit.has_user_filter", p.UserID != ""),
		attribute.Int("damask.audit.type_filter_count", len(p.Types)),
	)
	defer func() {
		if out != nil {
			span.SetAttributes(attribute.Int("damask.audit.result_count", len(out.Events)))
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "audit activity list failed", "workspace_id", p.WorkspaceID, "error", err)
		}
	}()

	limit := clampLimit(p.Limit, minWorkspaceActivityPageSize, maxWorkspaceActivityPageSize)
	typesFilter := makeTypesFilter(p.Types)

	assetRows, err := s.repo.ListWorkspaceAssetEvents(ctx, repository.ListAuditEventsParams{
		WorkspaceID: p.WorkspaceID,
		Cursor:      p.Cursor,
		UserID:      p.UserID,
		EventType:   singleTypeString(typesFilter),
		Limit:       limit + 1,
	})
	if err != nil {
		return nil, err
	}

	projectRows, err := s.repo.ListWorkspaceProjectEvents(ctx, repository.ListAuditEventsParams{
		WorkspaceID: p.WorkspaceID,
		Cursor:      p.Cursor,
		UserID:      p.UserID,
		EventType:   singleTypeString(typesFilter),
		Limit:       limit + 1,
	})
	if err != nil {
		return nil, err
	}

	merged := make([]ActivityEventDTO, 0, len(assetRows)+len(projectRows))
	for _, r := range assetRows {
		if len(typesFilter) > 0 && !typesFilter[r.EventType] {
			continue
		}
		merged = append(merged, ActivityEventDTO{
			AuditEventDTO: buildAuditEventDTO(
				r.ID,
				r.EventType,
				r.CreatedAt,
				r.Payload,
				r.UserID,
				r.UserName,
				r.ActorType,
			),
			EntityType: string(AutomationScopeAsset),
			EntityID:   r.EntityID,
		})
	}
	for _, r := range projectRows {
		if len(typesFilter) > 0 && !typesFilter[r.EventType] {
			continue
		}
		merged = append(merged, ActivityEventDTO{
			AuditEventDTO: buildAuditEventDTO(
				r.ID,
				r.EventType,
				r.CreatedAt,
				r.Payload,
				r.UserID,
				r.UserName,
				r.ActorType,
			),
			EntityType: "project",
			EntityID:   r.EntityID,
		})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].CreatedAt > merged[j].CreatedAt
	})

	hasMore := int64(len(merged)) > limit
	if hasMore {
		merged = merged[:limit]
	}
	var nextCursor *string
	if hasMore && len(merged) > 0 {
		nc := merged[len(merged)-1].CreatedAt
		nextCursor = &nc
	}
	if merged == nil {
		merged = []ActivityEventDTO{}
	}
	out = &ActivityListDTO{Events: merged, NextCursor: nextCursor, HasMore: hasMore}
	return out, nil
}

func (s *auditLogService) ExportActivity(ctx context.Context, p ExportActivityParams) (csv string, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.audit.export_activity",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.Bool("damask.audit.has_since", p.Since != ""),
		attribute.Bool("damask.audit.has_until", p.Until != ""),
	)
	defer func() {
		span.SetAttributes(attribute.Int("damask.audit.export_bytes", len(csv)))
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "audit activity export failed", "workspace_id", p.WorkspaceID, "error", err)
		}
	}()

	cursor := ""
	if p.Until != "" {
		cursor = p.Until + " 23:59:59"
	}

	const maxRows = 10000
	assetRows, err := s.repo.ListWorkspaceAssetEvents(ctx, repository.ListAuditEventsParams{
		WorkspaceID: p.WorkspaceID,
		Cursor:      cursor,
		Limit:       maxRows,
	})
	if err != nil {
		return "", err
	}
	projectRows, err := s.repo.ListWorkspaceProjectEvents(ctx, repository.ListAuditEventsParams{
		WorkspaceID: p.WorkspaceID,
		Cursor:      cursor,
		Limit:       maxRows,
	})
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("event_id,event_type,entity_type,entity_id,actor_type,actor_name,payload_summary,created_at\n")

	writeRow := func(id, eventType, entityType, entityID, actorType string, userName *string, payload, createdAt string) {
		if p.Since != "" && createdAt < p.Since {
			return
		}
		name := ""
		if userName != nil {
			name = *userName
		}
		summary := audit.RenderHumanReadable(eventType, payload)
		sb.WriteString(csvEscape(id) + "," +
			csvEscape(eventType) + "," +
			csvEscape(entityType) + "," +
			csvEscape(entityID) + "," +
			csvEscape(actorType) + "," +
			csvEscape(name) + "," +
			csvEscape(summary) + "," +
			csvEscape(createdAt) + "\n")
	}

	for _, r := range assetRows {
		writeRow(r.ID, r.EventType, "asset", r.EntityID, r.ActorType, r.UserName, r.Payload, r.CreatedAt)
	}
	for _, r := range projectRows {
		writeRow(r.ID, r.EventType, "project", r.EntityID, r.ActorType, r.UserName, r.Payload, r.CreatedAt)
	}
	csv = sb.String()
	return csv, nil
}

// -- helpers --

func buildAuditEventDTO(
	id, eventType, createdAt, payload string,
	userID, userName *string,
	actorType string,
) AuditEventDTO {
	return AuditEventDTO{
		ID:            id,
		EventType:     eventType,
		Actor:         AuditEventActorDTO{Type: actorType, ID: userID, Name: userName},
		Payload:       json.RawMessage(payload),
		CreatedAt:     createdAt,
		HumanReadable: audit.RenderHumanReadable(eventType, payload),
	}
}

func paginateAuditEvents(events []AuditEventDTO, limit int64) *AuditEventListDTO {
	hasMore := int64(len(events)) > limit
	if hasMore {
		events = events[:limit]
	}
	var nextCursor *string
	if hasMore && len(events) > 0 {
		nc := events[len(events)-1].CreatedAt
		nextCursor = &nc
	}
	if events == nil {
		events = []AuditEventDTO{}
	}
	return &AuditEventListDTO{Events: events, NextCursor: nextCursor, HasMore: hasMore}
}

func clampLimit(v, defaultVal, maxValue int64) int64 {
	if v <= 0 {
		return defaultVal
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func makeTypesFilter(types []string) map[string]bool {
	if len(types) == 0 {
		return nil
	}
	m := make(map[string]bool, len(types))
	for _, t := range types {
		t = strings.TrimSpace(t)
		if t != "" {
			m[t] = true
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func singleTypeString(types map[string]bool) string {
	if len(types) == 1 {
		for t := range types {
			return t
		}
	}
	return ""
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, `",`+"\n") {
		s = strings.ReplaceAll(s, `"`, `""`)
		return `"` + s + `"`
	}
	return s
}

// ParseLimit parses a string limit query param, returning defaultVal on invalid input.
func ParseLimit(s string, defaultVal, maxValue int64) int64 {
	if s == "" {
		return defaultVal
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return defaultVal
	}
	if n > maxValue {
		return maxValue
	}
	return n
}

// ParseTypesFilter parses a comma-separated types query param into a slice.
func ParseTypesFilter(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ValidateExportDateRange returns an error string if since/until are invalid, empty otherwise.
func ValidateExportDateRange(since, until string) error {
	if since != "" {
		if _, err := time.Parse("2006-01-02", since); err != nil {
			return fmt.Errorf("invalid since date; use YYYY-MM-DD: %w", apperr.ErrInvalidInput)
		}
	}
	if until != "" {
		if _, err := time.Parse("2006-01-02", until); err != nil {
			return fmt.Errorf("invalid until date; use YYYY-MM-DD: %w", apperr.ErrInvalidInput)
		}
	}
	return nil
}
