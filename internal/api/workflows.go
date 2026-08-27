package api

import (
	"encoding/json"
	"strings"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/service"
	apptelemetry "damask/server/internal/telemetry"

	"github.com/gofiber/fiber/v3"
)

const maxRunPageSize = 20

// -- Request types

// workflowRequest is used for both create and update. PUT is replace-all:
// all three fields are required.
type workflowRequest struct {
	Name                 string `json:"name"`
	Description          string `json:"description"`
	Graph                string `json:"graph"`
	NotifyOnFailureEmail string `json:"notify_on_failure_email"`
}

type toggleWorkflowRequest struct {
	Enabled bool `json:"enabled"`
}

type bulkManualRunRequest struct {
	AssetIDs []string `json:"asset_ids"`
}

// -- Response types

// WorkflowResponse is the API representation of a workflow.
type WorkflowResponse struct {
	ID                   string     `json:"id"`
	WorkspaceID          string     `json:"workspace_id"`
	Name                 string     `json:"name"`
	Description          string     `json:"description"`
	Enabled              bool       `json:"enabled"`
	TriggerType          string     `json:"trigger_type"`
	Graph                string     `json:"graph"`
	NotifyOnFailureEmail string     `json:"notify_on_failure_email"`
	LastRunAt            *time.Time `json:"last_run_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// WorkflowRunStepResponse is the API representation of a single step within a workflow run.
type WorkflowRunStepResponse struct {
	NodeID      string         `json:"node_id"`
	NodeType    string         `json:"node_type"`
	Status      string         `json:"status"`
	Attempt     int            `json:"attempt"`
	InputCtx    map[string]any `json:"input_ctx"`
	OutputCtx   map[string]any `json:"output_ctx"`
	Error       *string        `json:"error,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// WorkflowRunResponse is the API representation of a workflow run.
type WorkflowRunResponse struct {
	ID          string                    `json:"id"`
	WorkflowID  string                    `json:"workflow_id"`
	Status      string                    `json:"status"`
	TriggerData map[string]any            `json:"trigger_data"`
	Error       *string                   `json:"error,omitempty"`
	StartedAt   *time.Time                `json:"started_at,omitempty"`
	CompletedAt *time.Time                `json:"completed_at,omitempty"`
	Steps       []WorkflowRunStepResponse `json:"steps"`
	CreatedAt   time.Time                 `json:"created_at"`
}

// WorkflowNodePortResponse is the API representation of a node port.
type WorkflowNodePortResponse struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// WorkflowNodeSchemaResponse is the API representation of a workflow node schema.
type WorkflowNodeSchemaResponse struct {
	Type         string                     `json:"type"`
	Label        string                     `json:"label"`
	Category     string                     `json:"category"`
	Description  string                     `json:"description"`
	Inputs       []WorkflowNodePortResponse `json:"inputs"`
	Outputs      []WorkflowNodePortResponse `json:"outputs"`
	ConfigSchema map[string]any             `json:"config_schema"`
}

// WorkflowTemplateResponse is the API representation of a workflow template.
type WorkflowTemplateResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TriggerType string `json:"trigger_type"`
	Graph       string `json:"graph"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
}

// WorkflowListRunsResponse wraps a paginated list of workflow runs.
type WorkflowListRunsResponse struct {
	Runs       []WorkflowRunResponse `json:"runs"`
	NextCursor string                `json:"next_cursor"`
}

// WorkflowTriggerResponse is returned when a workflow run is initiated.
type WorkflowTriggerResponse struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

// WorkflowTokenResponse carries a webhook token.
type WorkflowTokenResponse struct {
	Token string `json:"token"`
}

// BulkManualRunResponse is returned by the bulk manual trigger endpoint.
type BulkManualRunResponse struct {
	RunIDs []string `json:"run_ids"`
	Count  int      `json:"count"`
	Error  string   `json:"error,omitempty"`
}

func workflowToResponse(dto service.WorkflowDTO) WorkflowResponse {
	return WorkflowResponse{
		ID:                   dto.ID,
		WorkspaceID:          dto.WorkspaceID,
		Name:                 dto.Name,
		Description:          dto.Description,
		Enabled:              dto.Enabled,
		TriggerType:          dto.TriggerType,
		Graph:                dto.Graph,
		NotifyOnFailureEmail: dto.NotifyOnFailureEmail,
		LastRunAt:            dto.LastRunAt,
		CreatedAt:            dto.CreatedAt,
		UpdatedAt:            dto.UpdatedAt,
	}
}

func workflowRunToResponse(dto service.WorkflowRunDTO) WorkflowRunResponse {
	steps := make([]WorkflowRunStepResponse, len(dto.Steps))
	for i, step := range dto.Steps {
		steps[i] = WorkflowRunStepResponse{
			NodeID:      step.NodeID,
			NodeType:    step.NodeType,
			Status:      step.Status,
			Attempt:     step.Attempt,
			InputCtx:    step.InputCtx,
			OutputCtx:   step.OutputCtx,
			Error:       step.Error,
			StartedAt:   step.StartedAt,
			CompletedAt: step.CompletedAt,
		}
	}
	return WorkflowRunResponse{
		ID:          dto.ID,
		WorkflowID:  dto.WorkflowID,
		Status:      dto.Status,
		TriggerData: dto.TriggerData,
		Error:       dto.Error,
		StartedAt:   dto.StartedAt,
		CompletedAt: dto.CompletedAt,
		Steps:       steps,
		CreatedAt:   dto.CreatedAt,
	}
}

func nodeSchemaToResponse(s service.WorkflowNodeSchema) WorkflowNodeSchemaResponse {
	inputs := make([]WorkflowNodePortResponse, len(s.Inputs))
	for i, p := range s.Inputs {
		inputs[i] = WorkflowNodePortResponse{ID: p.ID, Label: p.Label}
	}
	outputs := make([]WorkflowNodePortResponse, len(s.Outputs))
	for i, p := range s.Outputs {
		outputs[i] = WorkflowNodePortResponse{ID: p.ID, Label: p.Label}
	}
	var configSchema map[string]any
	if len(s.ConfigSchema) > 0 {
		_ = json.Unmarshal(s.ConfigSchema, &configSchema)
	}
	return WorkflowNodeSchemaResponse{
		Type:         s.Type,
		Label:        s.Label,
		Category:     s.Category,
		Description:  s.Description,
		Inputs:       inputs,
		Outputs:      outputs,
		ConfigSchema: configSchema,
	}
}

// @Summary List workflows
// @Success 200 {array} WorkflowResponse
// @Router /api/v1/workflows [get].
func (s *Server) handleListWorkflows(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	apptelemetry.EnrichSpan(c, claims.WorkspaceID, claims.UserID)
	params := service.ListWorkflowsParams{}
	if triggerType := c.Query("trigger_type"); triggerType != "" {
		params.TriggerType = &triggerType
	}
	params.EnabledOnly = c.Query("enabled_only") == "true"
	rows, err := s.workflows.List(c.Context(), claims.WorkspaceID, params)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	out := make([]WorkflowResponse, len(rows))
	for i, row := range rows {
		out[i] = workflowToResponse(row)
	}
	return c.JSON(out)
}

// @Summary Create a workflow
// @Param body body workflowRequest true "Workflow definition"
// @Success 201 {object} WorkflowResponse
// @Router /api/v1/workflows [post].
func (s *Server) handleCreateWorkflow(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	apptelemetry.EnrichSpan(c, claims.WorkspaceID, claims.UserID)
	var req workflowRequest
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	dto, err := s.workflows.Create(c.Context(), claims.WorkspaceID, claims.UserID, service.CreateWorkflowParams{
		Name:                 req.Name,
		Description:          req.Description,
		Graph:                req.Graph,
		NotifyOnFailureEmail: req.NotifyOnFailureEmail,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(workflowToResponse(*dto))
}

// @Summary List workflow node schemas
// @Success 200 {array} WorkflowNodeSchemaResponse
// @Router /api/v1/workflows/node-schemas [get].
func (s *Server) handleGetWorkflowNodeSchemas(c fiber.Ctx) error {
	schemas := s.workflows.NodeSchemas()
	out := make([]WorkflowNodeSchemaResponse, len(schemas))
	for i, s := range schemas {
		out[i] = nodeSchemaToResponse(s)
	}
	return c.JSON(out)
}

// @Summary List workflow templates
// @Success 200 {array} WorkflowTemplateResponse
// @Router /api/v1/workflows/templates [get].
func (s *Server) handleGetWorkflowTemplates(c fiber.Ctx) error {
	templates := s.workflows.Templates()
	out := make([]WorkflowTemplateResponse, len(templates))
	for i, t := range templates {
		out[i] = WorkflowTemplateResponse{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			TriggerType: t.TriggerType,
			Graph:       t.Graph,
			Icon:        t.Icon,
			Category:    t.Category,
		}
	}
	return c.JSON(out)
}

// @Summary Get a workflow
// @Success 200 {object} WorkflowResponse
// @Router /api/v1/workflows/{id} [get].
func (s *Server) handleGetWorkflow(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	dto, err := s.workflows.Get(c.Context(), claims.WorkspaceID, c.Params("id"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(workflowToResponse(*dto))
}

// @Summary Update a workflow
// @Param body body workflowRequest true "Workflow definition"
// @Success 200 {object} WorkflowResponse
// @Router /api/v1/workflows/{id} [put].
func (s *Server) handleUpdateWorkflow(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	var req workflowRequest
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	dto, err := s.workflows.Update(c.Context(), claims.WorkspaceID, c.Params("id"), service.UpdateWorkflowParams{
		Name:                 &req.Name,
		Description:          &req.Description,
		Graph:                &req.Graph,
		NotifyOnFailureEmail: &req.NotifyOnFailureEmail,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(workflowToResponse(*dto))
}

// @Summary Enable or disable a workflow
// @Success 204
// @Router /api/v1/workflows/{id}/enabled [patch].
func (s *Server) handleToggleWorkflow(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	var req toggleWorkflowRequest
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := s.workflows.SetEnabled(c.Context(), claims.WorkspaceID, c.Params("id"), req.Enabled); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Delete a workflow
// @Success 204
// @Router /api/v1/workflows/{id} [delete].
func (s *Server) handleDeleteWorkflow(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if err := s.workflows.Delete(c.Context(), claims.WorkspaceID, c.Params("id")); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Trigger a manual workflow run
// @Success 202 {object} WorkflowTriggerResponse
// @Router /api/v1/workflows/{id}/runs [post].
func (s *Server) handleManualWorkflowRun(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	var req struct {
		AssetID string `json:"asset_id"`
	}
	_ = c.Bind().JSON(&req)
	runID, err := s.workflows.TriggerManual(c.Context(), claims.WorkspaceID, c.Params("id"), req.AssetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.Status(fiber.StatusAccepted).JSON(WorkflowTriggerResponse{RunID: runID, Status: apiStatusPending})
}

// @Summary Bulk trigger manual workflow runs
// @Param body body bulkManualRunRequest true "Asset IDs to trigger"
// @Success 202 {object} BulkManualRunResponse
// @Router /api/v1/workflows/{id}/runs/bulk [post].
func (s *Server) handleBulkManualWorkflowRun(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	var req bulkManualRunRequest
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	runIDs, err := s.workflows.TriggerManualBulk(c.Context(), claims.WorkspaceID, c.Params("id"), req.AssetIDs)
	if err != nil && len(runIDs) == 0 {
		return ErrorStatusResponse(c, err)
	}
	resp := BulkManualRunResponse{
		RunIDs: runIDs,
		Count:  len(runIDs),
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return c.Status(fiber.StatusAccepted).JSON(resp)
}

// @Summary List all workflow runs (paginated)
// @Success 200 {object} WorkflowListRunsResponse
// @Router /api/v1/workflows/runs [get].
func (s *Server) handleListAllWorkflowRuns(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	apptelemetry.EnrichSpan(c, claims.WorkspaceID, claims.UserID)
	rows, err := s.workflows.ListAllRuns(c.Context(), claims.WorkspaceID, maxRunPageSize, c.Query("cursor"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	out := make([]WorkflowRunResponse, len(rows))
	for i, row := range rows {
		out[i] = workflowRunToResponse(row)
	}
	var nextCursor string
	if len(rows) == maxRunPageSize {
		last := rows[len(rows)-1]
		nextCursor = last.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z") + "|" + last.ID
	}
	return c.JSON(WorkflowListRunsResponse{Runs: out, NextCursor: nextCursor})
}

// @Summary List runs for a workflow
// @Success 200 {array} WorkflowRunResponse
// @Router /api/v1/workflows/{id}/runs [get].
func (s *Server) handleListWorkflowRuns(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	workflowID := c.Params("id")
	if _, err := s.workflows.Get(c.Context(), claims.WorkspaceID, workflowID); err != nil {
		return ErrorStatusResponse(c, err)
	}
	rows, err := s.workflows.ListRuns(c.Context(), claims.WorkspaceID, workflowID, maxRunPageSize, c.Query("cursor"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	out := make([]WorkflowRunResponse, len(rows))
	for i, row := range rows {
		out[i] = workflowRunToResponse(row)
	}
	return c.JSON(out)
}

// @Summary Get a specific workflow run
// @Success 200 {object} WorkflowRunResponse
// @Router /api/v1/workflows/{id}/runs/{rid} [get].
func (s *Server) handleGetWorkflowRun(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	run, err := s.workflows.GetRun(c.Context(), claims.WorkspaceID, c.Params("rid"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if run.WorkflowID != c.Params("id") {
		return ErrorStatusResponse(c, fiber.ErrNotFound)
	}
	return c.JSON(workflowRunToResponse(*run))
}

// @Summary Get workflow webhook token
// @Success 200 {object} WorkflowTokenResponse
// @Router /api/v1/workflows/{id}/webhook-token [get].
func (s *Server) handleGetWorkflowWebhookToken(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	token, err := s.workflows.GetWebhookToken(c.Context(), claims.WorkspaceID, c.Params("id"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(WorkflowTokenResponse{Token: token})
}

// @Summary Regenerate workflow webhook token
// @Success 200 {object} WorkflowTokenResponse
// @Router /api/v1/workflows/{id}/webhook-token/regenerate [post].
func (s *Server) handleRegenerateWorkflowWebhookToken(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	token, err := s.workflows.RegenerateWebhookToken(c.Context(), claims.WorkspaceID, c.Params("id"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(WorkflowTokenResponse{Token: token})
}

const webhookBodyLimit = 512 * 1024 // 512 KB

func (s *Server) handleInboundWorkflowWebhook(c fiber.Ctx) error {
	token := strings.TrimSpace(c.Get("X-Workflow-Token"))
	if token == "" {
		authz := strings.TrimSpace(c.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			token = strings.TrimSpace(authz[7:])
		}
	}
	if token == "" {
		return errRes(c, fiber.StatusUnauthorized, "missing webhook token")
	}
	body := c.Body()
	if len(body) > webhookBodyLimit {
		return errRes(c, fiber.StatusRequestEntityTooLarge, "webhook body too large")
	}
	runID, err := s.workflows.TriggerWebhook(c.Context(), c.Params("id"), token, body)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.Status(fiber.StatusAccepted).JSON(WorkflowTriggerResponse{RunID: runID, Status: apiStatusPending})
}
