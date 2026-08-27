package api

import (
	"encoding/json"
	"strings"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

// -- Request / response types

// RawToNullableString converts a *[json.RawMessage] field to *string.
// Returns (nil, false) when the field was absent (pointer is nil).
// Returns (nil, true) when the field was explicitly JSON null → clear the value.
// Returns (&s, true) when the field was a JSON string → set to s.
func RawToNullableString(r *json.RawMessage) (value *string, present bool) {
	if r == nil {
		return nil, false
	}
	if string(*r) == "null" {
		return nil, true
	}
	var s string
	if err := json.Unmarshal(*r, &s); err != nil {
		return nil, true
	}
	return &s, true
}

type IngressSourceResponse struct {
	ID              string         `json:"id"`
	WorkspaceID     string         `json:"workspace_id"`
	CreatedBy       string         `json:"created_by"`
	Type            string         `json:"type"`
	Label           string         `json:"label"`
	PublicToken     string         `json:"public_token"`
	Config          map[string]any `json:"config"`
	DestFolderID    *string        `json:"dest_folder_id,omitempty"`
	DestProjectID   *string        `json:"dest_project_id,omitempty"`
	Enabled         bool           `json:"enabled"`
	PollIntervalMin int64          `json:"poll_interval_min"`
	LastPolledAt    *time.Time     `json:"last_polled_at,omitempty"`
	LastError       *string        `json:"last_error,omitempty"`
	ErrorCount      int64          `json:"error_count"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type IngressLogResponse struct {
	ID         string    `json:"id"`
	SourceID   string    `json:"source_id"`
	RemoteID   string    `json:"remote_id"`
	Filename   string    `json:"filename"`
	AssetID    *string   `json:"asset_id,omitempty"`
	Status     string    `json:"status"`
	Error      *string   `json:"error,omitempty"`
	ImportedAt time.Time `json:"imported_at"`
}

type IngressRuleResponse struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id"`
	Position int64  `json:"position"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Action   string `json:"action"`
}

var sensitiveKeys = []string{"password", "secret", "key", "token"}

func RedactConfig(raw map[string]any) map[string]any {
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		kl := strings.ToLower(k)
		redact := false
		for _, s := range sensitiveKeys {
			if strings.Contains(kl, s) {
				redact = true
				break
			}
		}
		if redact {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	return out
}

func sourceDTOToResponse(d *service.IngressSourceDTO) IngressSourceResponse {
	return IngressSourceResponse{
		ID:              d.ID,
		WorkspaceID:     d.WorkspaceID,
		CreatedBy:       d.CreatedBy,
		Type:            d.Type,
		Label:           d.Label,
		PublicToken:     d.PublicToken,
		Config:          d.Config,
		DestFolderID:    d.DestFolderID,
		DestProjectID:   d.DestProjectID,
		Enabled:         d.Enabled,
		PollIntervalMin: d.PollIntervalMin,
		LastPolledAt:    d.LastPolledAt,
		LastError:       d.LastError,
		ErrorCount:      d.ErrorCount,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

func ruleDTOToResponse(d *service.IngressRuleDTO) IngressRuleResponse {
	return IngressRuleResponse{
		ID:       d.ID,
		SourceID: d.SourceID,
		Position: d.Position,
		Field:    d.Field,
		Operator: d.Operator,
		Value:    d.Value,
		Action:   d.Action,
	}
}

func logEntryDTOToResponse(d *service.IngressLogEntryDTO) IngressLogResponse {
	return IngressLogResponse{
		ID:         d.ID,
		SourceID:   d.SourceID,
		RemoteID:   d.RemoteID,
		Filename:   d.Filename,
		AssetID:    d.AssetID,
		Status:     d.Status,
		Error:      d.Error,
		ImportedAt: d.ImportedAt,
	}
}

// -- Rules CRUD

// @Summary List ingress rules
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 200 {array} IngressRuleResponse
// @Router /api/v1/ingress/sources/{id}/rules [get].
func (s *Server) handleListIngressRules(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rules, err := s.ingress.ListRules(c.Context(), claims.WorkspaceID, c.Params("id"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	result := make([]IngressRuleResponse, len(rules))
	for i, r := range rules {
		result[i] = ruleDTOToResponse(r)
	}
	return c.JSON(result)
}

// @Summary Create an ingress rule
// @Tags Ingress
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Param body body IngressRuleReq true "Rule definition"
// @Success 201 {object} IngressRuleResponse
// @Router /api/v1/ingress/sources/{id}/rules [post].
func (s *Server) handleCreateIngressRule(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &IngressRuleReq{})
	if !ok {
		return nil
	}

	r, err := s.ingress.CreateRule(c.Context(), claims.WorkspaceID, c.Params("id"), service.CreateIngressRuleParams{
		Position: req.Position,
		Field:    req.Field,
		Operator: req.Operator,
		Value:    req.Value,
		Action:   req.Action,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(ruleDTOToResponse(r))
}

// @Summary Update an ingress rule
// @Tags Ingress
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Param rid path string true "Rule ID"
// @Param body body IngressRuleReq true "Rule definition"
// @Success 200 {object} IngressRuleResponse
// @Router /api/v1/ingress/sources/{id}/rules/{rid} [put].
func (s *Server) handleUpdateIngressRule(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var req IngressRuleReq
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}

	r, err := s.ingress.UpdateRule(
		c.Context(),
		claims.WorkspaceID,
		c.Params("id"),
		c.Params("rid"),
		service.UpdateIngressRuleParams{
			Position: req.Position,
			Field:    req.Field,
			Operator: req.Operator,
			Value:    req.Value,
			Action:   req.Action,
		},
	)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(ruleDTOToResponse(r))
}

// @Summary Delete an ingress rule
// @Tags Ingress
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Param rid path string true "Rule ID"
// @Success 204
// @Router /api/v1/ingress/sources/{id}/rules/{rid} [delete].
func (s *Server) handleDeleteIngressRule(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	if err := s.ingress.DeleteRule(c.Context(), claims.WorkspaceID, c.Params("id"), c.Params("rid")); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Reorder ingress rules
// @Tags Ingress
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Param body body []ReorderRuleEntry true "Reorder entries"
// @Success 200 {array} IngressRuleResponse
// @Router /api/v1/ingress/sources/{id}/rules/reorder [put].
func (s *Server) handleReorderIngressRules(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var entries []ReorderRuleEntry
	if err := c.Bind().JSON(&entries); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}

	svcEntries := make([]service.ReorderRuleEntry, len(entries))
	for i, e := range entries {
		svcEntries[i] = service.ReorderRuleEntry{ID: e.ID, Position: e.Position}
	}

	rules, err := s.ingress.ReorderRules(c.Context(), claims.WorkspaceID, c.Params("id"), svcEntries)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	result := make([]IngressRuleResponse, len(rules))
	for i, r := range rules {
		result[i] = ruleDTOToResponse(r)
	}
	return c.JSON(result)
}

// -- Source CRUD

// @Summary Create an ingress source
// @Tags Ingress
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateIngressSourceReq true "Source configuration"
// @Success 201 {object} IngressSourceResponse
// @Router /api/v1/ingress/sources [post].
func (s *Server) handleCreateIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &CreateIngressSourceReq{})
	if !ok {
		return nil
	}

	rules := make([]service.CreateIngressRuleParams, len(req.Rules))
	for i, r := range req.Rules {
		rules[i] = service.CreateIngressRuleParams{
			Position: r.Position,
			Field:    r.Field,
			Operator: r.Operator,
			Value:    r.Value,
			Action:   r.Action,
		}
	}

	src, err := s.ingress.CreateSource(
		c.Context(),
		claims.WorkspaceID,
		claims.UserID,
		service.CreateIngressSourceParams{
			Type:            req.Type,
			Label:           req.Label,
			Config:          req.Config,
			DestFolderID:    req.DestFolderID,
			DestProjectID:   req.DestProjectID,
			Enabled:         req.Enabled,
			PollIntervalMin: req.PollIntervalMin,
			Rules:           rules,
		},
	)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(sourceDTOToResponse(src))
}

// @Summary List ingress sources
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Success 200 {array} IngressSourceResponse
// @Router /api/v1/ingress/sources [get].
func (s *Server) handleListIngressSources(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	sources, err := s.ingress.ListSources(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	result := make([]IngressSourceResponse, len(sources))
	for i, src := range sources {
		result[i] = sourceDTOToResponse(src)
	}
	return c.JSON(result)
}

// @Summary Get an ingress source
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 200 {object} IngressSourceResponse
// @Router /api/v1/ingress/sources/{id} [get].
func (s *Server) handleGetIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	src, err := s.ingress.GetSource(c.Context(), claims.WorkspaceID, c.Params("id"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(sourceDTOToResponse(src))
}

// @Summary Update an ingress source
// @Tags Ingress
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Param body body UpdateIngressSourceReq true "Fields to update"
// @Success 200 {object} IngressSourceResponse
// @Router /api/v1/ingress/sources/{id} [put].
func (s *Server) handleUpdateIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &UpdateIngressSourceReq{})
	if !ok {
		return nil
	}

	var destFolderID *string
	var destFolderSet bool
	if val, present := RawToNullableString(req.DestFolderID); present {
		destFolderID = val
		destFolderSet = true
	}
	var destProjectID *string
	var destProjectSet bool
	if val, present := RawToNullableString(req.DestProjectID); present {
		destProjectID = val
		destProjectSet = true
	}

	src, err := s.ingress.UpdateSource(
		c.Context(),
		claims.WorkspaceID,
		c.Params("id"),
		service.UpdateIngressSourceParams{
			Label:           req.Label,
			Config:          req.Config,
			DestFolderID:    destFolderID,
			DestFolderSet:   destFolderSet,
			DestProjectID:   destProjectID,
			DestProjectSet:  destProjectSet,
			Enabled:         req.Enabled,
			PollIntervalMin: req.PollIntervalMin,
		},
	)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(sourceDTOToResponse(src))
}

// @Summary Delete an ingress source
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 204
// @Router /api/v1/ingress/sources/{id} [delete].
func (s *Server) handleDeleteIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	if err := s.ingress.DeleteSource(c.Context(), claims.WorkspaceID, c.Params("id")); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Test an ingress source
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 200 {object} object{ok=bool}
// @Router /api/v1/ingress/sources/{id}/test [post].
func (s *Server) handleTestIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	if err := s.ingress.TestSource(c.Context(), claims.WorkspaceID, c.Params("id")); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(fiber.Map{"ok": true})
}

// @Summary Trigger a poll
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 202 {object} object{job_id=string}
// @Router /api/v1/ingress/sources/{id}/poll [post].
func (s *Server) handlePollIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	jobID, err := s.ingress.TriggerPoll(c.Context(), claims.WorkspaceID, c.Params("id"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{apiJobIDKey: jobID})
}

// -- Log API

// @Summary List source log entries
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 200 {array} IngressLogResponse
// @Router /api/v1/ingress/sources/{id}/log [get].
func (s *Server) handleListIngressSourceLog(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	entries, err := s.ingress.ListSourceLog(c.Context(), claims.WorkspaceID, c.Params("id"), maxPageSize, 0)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	result := make([]IngressLogResponse, len(entries))
	for i, e := range entries {
		result[i] = logEntryDTOToResponse(e)
	}
	return c.JSON(result)
}

// @Summary List workspace ingress log
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param status query string false "Filter by status"
// @Success 200 {array} IngressLogResponse
// @Router /api/v1/ingress/log [get].
func (s *Server) handleListIngressLog(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	entries, err := s.ingress.ListLog(c.Context(), claims.WorkspaceID, c.Query(apiStatusKey), maxPageSize, 0)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	result := make([]IngressLogResponse, len(entries))
	for i, e := range entries {
		result[i] = logEntryDTOToResponse(e)
	}
	return c.JSON(result)
}

// @Summary Delete a log entry
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param entry_id path string true "Log entry ID"
// @Success 204
// @Router /api/v1/ingress/log/{entry_id} [delete].
func (s *Server) handleDeleteIngressLogEntry(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	if err := s.ingress.DeleteLogEntry(c.Context(), claims.WorkspaceID, c.Params("entry_id")); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Retry a log entry
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param entry_id path string true "Log entry ID"
// @Success 202 {object} object{job_id=string}
// @Router /api/v1/ingress/log/{entry_id}/retry [post].
func (s *Server) handleRetryIngressLogEntry(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	jobID, err := s.ingress.RetryLogEntry(c.Context(), claims.WorkspaceID, c.Params("entry_id"))
	if err != nil {
		// ErrInvalidInput here means "entry is not in a retryable state" — return 400 not 422.
		if isInvalidInput(err) {
			return errRes(c, fiber.StatusBadRequest, err.Error())
		}
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{apiJobIDKey: jobID})
}
