package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"damask/server/internal/apperr"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/workflow"

	"github.com/google/uuid"
)

const (
	WorkflowRunStatusPending  = "pending"
	workflowPortOut           = "out"
	workflowPortMatch         = "match"
	workflowNodeFilterFolder  = "n_filter_folder"
	workflowNodeFilterProject = "n_filter_project"
	workflowNodeFilterMIME    = "n_filter_mime"
	workflowNodeFanout        = "n_fanout"
	maxBulkTriggerAssets      = 500
	workflowTriggerManual     = "manual"
	variantTypeManual         = "manual"
)

type workflowService struct {
	workflows repository.WorkflowRepository
	runs      repository.WorkflowRunRepository
	webhooks  repository.WorkflowWebhookRepository
	assets    repository.AssetRepository
	variants  repository.VariantRepository
	versions  repository.VersionRepository
	queue     queue.JobQueue
}

type WorkflowServiceDeps struct {
	Assets   repository.AssetRepository
	Variants repository.VariantRepository
	Versions repository.VersionRepository
}

func NewWorkflowService(
	workflows repository.WorkflowRepository,
	runs repository.WorkflowRunRepository,
	webhooks repository.WorkflowWebhookRepository,
	queue queue.JobQueue,
	deps WorkflowServiceDeps,
) WorkflowService {
	return &workflowService{
		workflows: workflows,
		runs:      runs,
		webhooks:  webhooks,
		assets:    deps.Assets,
		variants:  deps.Variants,
		versions:  deps.Versions,
		queue:     queue,
	}
}

func (s *workflowService) List(
	ctx context.Context,
	workspaceID string,
	params ListWorkflowsParams,
) ([]WorkflowDTO, error) {
	var (
		rows []repository.Workflow
		err  error
	)
	if params.TriggerType != nil && params.EnabledOnly {
		rows, err = s.workflows.ListEnabledByTrigger(ctx, workspaceID, *params.TriggerType)
	} else {
		rows, err = s.workflows.List(ctx, workspaceID)
	}
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowDTO, 0, len(rows))
	for _, row := range rows {
		if params.TriggerType != nil && !params.EnabledOnly && row.TriggerType != *params.TriggerType {
			continue
		}
		out = append(out, toWorkflowDTO(row))
	}
	return out, nil
}

func (s *workflowService) Get(ctx context.Context, workspaceID, id string) (*WorkflowDTO, error) {
	row, err := s.workflows.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	dto := toWorkflowDTO(row)
	return &dto, nil
}

func (s *workflowService) Create(
	ctx context.Context,
	workspaceID, createdBy string,
	p CreateWorkflowParams,
) (*WorkflowDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	triggerType, err := extractTriggerType(p.Graph)
	if err != nil {
		return nil, err
	}
	row, err := s.workflows.Create(ctx, repository.CreateWorkflowParams{
		ID:                   uuid.NewString(),
		WorkspaceID:          workspaceID,
		Name:                 p.Name,
		Description:          p.Description,
		Enabled:              true,
		TriggerType:          triggerType,
		TriggerConfig:        defaultWorkflowTriggerConfig(p.TriggerConfig),
		Graph:                p.Graph,
		NotifyOnFailureEmail: p.NotifyOnFailureEmail,
		CreatedBy:            createdBy,
	})
	if err != nil {
		return nil, err
	}
	dto := toWorkflowDTO(row)
	return &dto, nil
}

func (s *workflowService) Update(
	ctx context.Context,
	workspaceID, id string,
	p UpdateWorkflowParams,
) (*WorkflowDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	update := repository.UpdateWorkflowParams{
		ID:                   id,
		WorkspaceID:          workspaceID,
		Name:                 p.Name,
		Description:          p.Description,
		Graph:                p.Graph,
		NotifyOnFailureEmail: p.NotifyOnFailureEmail,
	}
	if p.Graph != nil {
		triggerType, err := extractTriggerType(*p.Graph)
		if err != nil {
			return nil, err
		}
		update.TriggerType = &triggerType
	}
	row, err := s.workflows.Update(ctx, update)
	if err != nil {
		return nil, err
	}
	dto := toWorkflowDTO(row)
	return &dto, nil
}

func (s *workflowService) SetEnabled(ctx context.Context, workspaceID, id string, enabled bool) error {
	return s.workflows.SetEnabled(ctx, workspaceID, id, enabled)
}

func (s *workflowService) Delete(ctx context.Context, workspaceID, id string) error {
	return s.workflows.Delete(ctx, workspaceID, id)
}

func (s *workflowService) TriggerManual(ctx context.Context, workspaceID, id, assetID string) (string, error) {
	wf, err := s.workflows.GetByID(ctx, workspaceID, id)
	if err != nil {
		return "", err
	}
	if !wf.Enabled {
		return "", fmt.Errorf("workflow is disabled: %w", apperr.ErrConflict)
	}
	triggerData := map[string]any{"trigger": workflowTriggerManual}
	if assetID != "" {
		triggerData, err = s.manualAssetTriggerData(ctx, workspaceID, assetID)
		if err != nil {
			return "", err
		}
	}
	return s.enqueueRun(ctx, wf, triggerData)
}

func (s *workflowService) TriggerManualBulk(
	ctx context.Context,
	workspaceID, workflowID string,
	assetIDs []string,
) ([]string, error) {
	if len(assetIDs) == 0 {
		return nil, fmt.Errorf("asset_ids must not be empty: %w", apperr.ErrInvalidInput)
	}
	if len(assetIDs) > maxBulkTriggerAssets {
		return nil, fmt.Errorf("asset_ids exceeds maximum of %d: %w", maxBulkTriggerAssets, apperr.ErrInvalidInput)
	}
	wf, err := s.workflows.GetByID(ctx, workspaceID, workflowID)
	if err != nil {
		return nil, err
	}
	if !wf.Enabled {
		return nil, fmt.Errorf("workflow is disabled: %w", apperr.ErrConflict)
	}
	if wf.TriggerType != "trigger.manual" {
		return nil, fmt.Errorf("workflow is not manually triggered: %w", apperr.ErrConflict)
	}
	runIDs := make([]string, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		triggerData, tdErr := s.manualAssetTriggerData(ctx, workspaceID, assetID)
		if tdErr != nil {
			return runIDs, tdErr
		}
		runID, runErr := s.enqueueRun(ctx, wf, triggerData)
		if runErr != nil {
			return runIDs, runErr
		}
		runIDs = append(runIDs, runID)
	}
	return runIDs, nil
}

func (s *workflowService) TriggerWebhook(ctx context.Context, id, token string, body []byte) (string, error) {
	wfs, err := s.workflows.ListByTrigger(ctx, "trigger.webhook")
	if err != nil {
		return "", err
	}
	var wf repository.Workflow
	found := false
	for _, candidate := range wfs {
		if candidate.ID == id {
			wf = candidate
			found = true
			break
		}
	}
	if !found {
		return "", apperr.ErrNotFound
	}
	tokenHash, err := s.webhooks.GetTokenHash(ctx, id)
	if err != nil {
		return "", err
	}
	if subtle.ConstantTimeCompare([]byte(workflow.Sha256Hex(token)), []byte(tokenHash)) != 1 {
		return "", fmt.Errorf("invalid webhook token: %w", apperr.ErrForbidden)
	}
	payload := map[string]any{"trigger": "webhook", "raw_body": string(body)}
	var bodyFields map[string]any
	if json.Unmarshal(body, &bodyFields) == nil {
		payload["body"] = bodyFields
	}
	return s.enqueueRun(ctx, wf, payload)
}

func (s *workflowService) GetRun(ctx context.Context, workspaceID, runID string) (*WorkflowRunDTO, error) {
	run, err := s.runs.GetByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.WorkspaceID != workspaceID {
		return nil, apperr.ErrNotFound
	}
	steps, err := s.runs.ListSteps(ctx, runID)
	if err != nil {
		return nil, err
	}
	dto := toWorkflowRunDTO(run, steps)
	return &dto, nil
}

func (s *workflowService) ListRuns(
	ctx context.Context,
	workspaceID, workflowID string,
	limit int,
	cursor string,
) ([]WorkflowRunDTO, error) {
	rows, err := s.runs.List(ctx, workspaceID, workflowID, limit, cursor)
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowRunDTO, len(rows))
	for i, row := range rows {
		out[i] = toWorkflowRunDTO(row, nil)
	}
	return out, nil
}

func (s *workflowService) ListAllRuns(
	ctx context.Context,
	workspaceID string,
	limit int,
	cursor string,
) ([]WorkflowRunDTO, error) {
	rows, err := s.runs.List(ctx, workspaceID, "", limit, cursor)
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowRunDTO, len(rows))
	for i, row := range rows {
		out[i] = toWorkflowRunDTO(row, nil)
	}
	return out, nil
}

// GetWebhookToken returns an existing plaintext token if one was just generated
// this session, or generates a fresh one. Because only the hash is persisted,
// the plaintext is only available immediately after (re)generation — callers
// that missed it must call RegenerateWebhookToken to get a new one.
func (s *workflowService) GetWebhookToken(ctx context.Context, workspaceID, id string) (string, error) {
	if _, err := s.workflows.GetByID(ctx, workspaceID, id); err != nil {
		return "", err
	}
	_, err := s.webhooks.GetTokenHash(ctx, id)
	if err == nil {
		// Token already exists; plaintext is not stored — caller must regenerate.
		return "", nil
	}
	if !errors.Is(err, apperr.ErrNotFound) {
		return "", err
	}
	return s.regenerateWebhookToken(ctx, id)
}

func (s *workflowService) RegenerateWebhookToken(ctx context.Context, workspaceID, id string) (string, error) {
	if _, err := s.workflows.GetByID(ctx, workspaceID, id); err != nil {
		return "", err
	}
	return s.regenerateWebhookToken(ctx, id)
}

func (s *workflowService) NodeSchemas() []WorkflowNodeSchema {
	nodes := workflow.NodeSchemas()
	out := make([]WorkflowNodeSchema, len(nodes))
	for i, node := range nodes {
		out[i] = WorkflowNodeSchema{
			Type:         node.Type,
			Label:        node.Label,
			Category:     node.Category,
			Description:  node.Description,
			Inputs:       toWorkflowNodePorts(node.Inputs),
			Outputs:      toWorkflowNodePorts(node.Outputs),
			ConfigSchema: node.ConfigSchema,
		}
	}
	return out
}

func (s *workflowService) Templates() []WorkflowTemplateDTO {
	templates := workflow.Templates()
	out := make([]WorkflowTemplateDTO, len(templates))
	for i, tpl := range templates {
		out[i] = WorkflowTemplateDTO{
			ID:          tpl.ID,
			Name:        tpl.Name,
			Description: tpl.Description,
			TriggerType: tpl.TriggerType,
			Graph:       tpl.Graph,
			Icon:        tpl.Icon,
			Category:    tpl.Category,
		}
	}
	return out
}

func (s *workflowService) FindCoveringWorkflow(
	ctx context.Context,
	workspaceID, assetID, assetProjectID, assetFolderID string,
) (*CoveringWorkflowDTO, error) {
	return findCoveringWorkflowDTO(ctx, s.workflows, workspaceID, assetID, assetProjectID, assetFolderID)
}

func (s *workflowService) CreateFromVariants(
	ctx context.Context,
	workspaceID string,
	p CreateVariantAutomationParams,
) (*WorkflowDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if s.assets == nil || s.variants == nil {
		return nil, errors.New("variant automation dependencies not configured")
	}
	assetRow, err := s.assets.GetByID(ctx, workspaceID, p.AssetID)
	if err != nil {
		return nil, err
	}
	asset := toAssetDTO(assetRow)
	variantRows, err := s.variants.ListByAsset(ctx, workspaceID, p.AssetID)
	if err != nil {
		return nil, err
	}
	automatable := make([]*VariantDTO, 0, len(variantRows))
	for i, v := range variantRows {
		if v.Type == variantTypeManual {
			continue
		}
		automatable = append(automatable, toVariantDTO(v, i+1))
	}
	if len(automatable) == 0 {
		return nil, fmt.Errorf("no automatable variants found: %w", apperr.ErrConflict)
	}

	triggerConfig := buildVariantAutomationTriggerConfig(p.Scope, asset)
	graph := buildVariantAutomationGraph(asset.MimeType, p.Scope, asset, automatable)
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		return nil, fmt.Errorf("graph serialisation: %w", err)
	}

	row, err := s.workflows.Create(ctx, repository.CreateWorkflowParams{
		ID:            uuid.NewString(),
		WorkspaceID:   workspaceID,
		Name:          buildVariantAutomationName(p.Scope, asset),
		Description:   "Creates matching variants whenever a new version is uploaded.",
		Enabled:       false,
		TriggerType:   "trigger.version_uploaded",
		TriggerConfig: string(triggerConfig),
		Graph:         string(graphJSON),
		CreatedBy:     p.CreatedBy,
	})
	if err != nil {
		return nil, err
	}
	dto := toWorkflowDTO(row)
	return &dto, nil
}

func (s *workflowService) enqueueRun(
	ctx context.Context,
	wf repository.Workflow,
	triggerData map[string]any,
) (string, error) {
	runID := uuid.NewString()
	_, err := s.runs.Create(ctx, repository.CreateWorkflowRunParams{
		ID:          runID,
		WorkflowID:  wf.ID,
		WorkspaceID: wf.WorkspaceID,
		Status:      WorkflowRunStatusPending,
		TriggerData: mustJSONMap(triggerData),
		Context:     "{}",
	})
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(workflow.RunWorkflowPayload{RunID: runID})
	if _, enqErr := s.queue.Enqueue(ctx, wf.WorkspaceID, queue.JobTypeRunWorkflow, string(payload)); enqErr != nil {
		return "", enqErr
	}
	return runID, nil
}

func (s *workflowService) manualAssetTriggerData(
	ctx context.Context,
	workspaceID, assetID string,
) (map[string]any, error) {
	if s.assets == nil || s.versions == nil {
		return nil, fmt.Errorf(
			"workflowService: Assets and Versions deps required for manual trigger: %w",
			apperr.ErrInternal,
		)
	}
	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	ver, err := s.versions.GetCurrentByAsset(ctx, assetID)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return nil, fmt.Errorf("asset %q has no current version: %w", assetID, apperr.ErrInvalidInput)
		}
		return nil, err
	}
	projectID := ""
	if asset.ProjectID != nil {
		projectID = *asset.ProjectID
	}
	folderID := ""
	if asset.FolderID != nil {
		folderID = *asset.FolderID
	}
	return workflowAssetTrigger{
		AssetID:          asset.ID,
		WorkspaceID:      asset.WorkspaceID,
		ProjectID:        projectID,
		FolderID:         folderID,
		MimeType:         asset.MimeType,
		Size:             asset.Size,
		OriginalFilename: asset.OriginalFilename,
		VersionID:        ver.ID,
		VersionNum:       ver.VersionNum,
		StorageKey:       ver.StorageKey,
	}.toMap(), nil
}

func findCoveringWorkflowDTO(
	ctx context.Context,
	workflows repository.WorkflowRepository,
	workspaceID, assetID, assetProjectID, assetFolderID string,
) (*CoveringWorkflowDTO, error) {
	wf, err := workflows.FindCoveringWorkflow(ctx, workspaceID, assetID, assetProjectID, assetFolderID)
	if errors.Is(err, apperr.ErrNotFound) {
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	scope := "workspace"
	var cfg struct {
		ProjectID string `json:"project_id"`
		FolderID  string `json:"folder_id"`
		AssetID   string `json:"asset_id"`
	}
	_ = json.Unmarshal([]byte(defaultWorkflowTriggerConfig(wf.TriggerConfig)), &cfg)
	switch {
	case cfg.FolderID != "":
		scope = "folder"
	case cfg.ProjectID != "":
		scope = string(AutomationScopeProject)
	case cfg.AssetID != "":
		scope = string(AutomationScopeAsset)
	}
	return &CoveringWorkflowDTO{ID: wf.ID, Name: wf.Name, Scope: scope}, nil
}

func buildVariantAutomationGraph(
	assetMIME string,
	scope AutomationScope,
	asset *AssetDTO,
	variants []*VariantDTO,
) workflow.Graph {
	triggerNode := workflow.GraphNode{
		ID:       "n_trigger",
		Type:     "trigger.version_uploaded",
		Config:   json.RawMessage(`{}`),
		Position: workflow.GraphPosition{X: 25, Y: 25}, //nolint:mnd // coordinates are arbitrary
	}

	if scope == AutomationScopeAsset {
		triggerNode.Config = json.RawMessage(`{"asset_id":"` + asset.ID + `"}`)
	}

	nodes := []workflow.GraphNode{triggerNode}
	edges := []workflow.GraphEdge{}
	prevID := "n_trigger"
	prevPort := workflowPortOut

	if scope != AutomationScopeAsset {
		if scope == AutomationScopeFolder && asset.FolderID != nil && *asset.FolderID != "" {
			folderCfg, _ := json.Marshal(map[string]string{"folder_id": *asset.FolderID})
			nodes = append(nodes, workflow.GraphNode{
				ID:       workflowNodeFilterFolder,
				Type:     "filter.folder",
				Config:   folderCfg,
				Position: workflow.GraphPosition{X: 188, Y: 173}, //nolint:mnd // coordinates are arbitrary
			})
			edges = append(
				edges,
				workflow.GraphEdge{
					FromNode: prevID,
					FromPort: prevPort,
					ToNode:   workflowNodeFilterFolder,
					ToPort:   "in",
				},
			)
			prevID = workflowNodeFilterFolder
			prevPort = workflowPortMatch
		} else if (scope == AutomationScopeProject || scope == AutomationScopeFolder) && asset.ProjectID != nil && *asset.ProjectID != "" {
			projectCfg, _ := json.Marshal(map[string]string{"key": "project_id", "value": *asset.ProjectID})
			nodes = append(nodes, workflow.GraphNode{
				ID:       workflowNodeFilterProject,
				Type:     "filter.expression",
				Config:   projectCfg,
				Position: workflow.GraphPosition{X: 188, Y: 173}, //nolint:mnd // coordinates are arbitrary
			})
			edges = append(
				edges,
				workflow.GraphEdge{
					FromNode: prevID,
					FromPort: prevPort,
					ToNode:   workflowNodeFilterProject,
					ToPort:   "in",
				},
			)
			prevID = workflowNodeFilterProject
			prevPort = workflowPortMatch
		}
	}

	mimeCfg, _ := json.Marshal(map[string]string{"prefix": mimePrefix(assetMIME)})
	nodes = append(nodes, workflow.GraphNode{
		ID:       workflowNodeFilterMIME,
		Type:     "filter.mime",
		Config:   mimeCfg,
		Position: workflow.GraphPosition{X: 325, Y: 337}, //nolint:mnd // coordinates are arbitrary
	})
	edges = append(
		edges,
		workflow.GraphEdge{FromNode: prevID, FromPort: prevPort, ToNode: workflowNodeFilterMIME, ToPort: "in"},
	)
	prevID = workflowNodeFilterMIME
	prevPort = workflowPortMatch

	if len(variants) == 1 {
		nodes = append(nodes, workflow.GraphNode{
			ID:       "n_variant_0",
			Type:     "action.create_variant",
			Config:   variantAutomationNodeConfig(variants[0]),
			Position: workflow.GraphPosition{X: 700, Y: 161}, //nolint:mnd // coordinates are arbitrary
		})
		edges = append(
			edges,
			workflow.GraphEdge{FromNode: prevID, FromPort: prevPort, ToNode: "n_variant_0", ToPort: "in"},
		)
		return workflow.Graph{Nodes: nodes, Edges: edges}
	}

	nodes = append(nodes, workflow.GraphNode{
		ID:       workflowNodeFanout,
		Type:     "control.fan_out",
		Config:   json.RawMessage(`{}`),
		Position: workflow.GraphPosition{X: 700, Y: 161}, //nolint:mnd // coordinates are arbitrary
	})
	edges = append(
		edges,
		workflow.GraphEdge{FromNode: prevID, FromPort: prevPort, ToNode: workflowNodeFanout, ToPort: "in"},
	)
	spread := 160.0
	startY := 263.0 - float64(len(variants)-1)*spread/2 //nolint:mnd // coordinates are arbitrary
	for i, v := range variants {
		nodeID := fmt.Sprintf("n_variant_%d", i)
		nodes = append(nodes, workflow.GraphNode{
			ID:     nodeID,
			Type:   "action.create_variant",
			Config: variantAutomationNodeConfig(v),
			Position: workflow.GraphPosition{
				X: 1033, //nolint:mnd // coordinates are arbitrary
				Y: startY + float64(i)*spread,
			},
		})
		edges = append(
			edges,
			workflow.GraphEdge{FromNode: workflowNodeFanout, FromPort: workflowPortOut, ToNode: nodeID, ToPort: "in"},
		)
	}
	return workflow.Graph{Nodes: nodes, Edges: edges}
}

func variantAutomationNodeConfig(v *VariantDTO) json.RawMessage {
	params := json.RawMessage(`{}`)
	if v.TransformParams != nil && strings.TrimSpace(*v.TransformParams) != "" &&
		json.Valid([]byte(*v.TransformParams)) {
		params = json.RawMessage(*v.TransformParams)
	}
	b, _ := json.Marshal(map[string]any{"type": v.Type, "params": params})
	return b
}

func mimePrefix(mime string) string {
	if i := strings.Index(mime, "/"); i > 0 {
		return mime[:i+1]
	}
	return mime
}

func buildVariantAutomationTriggerConfig(scope AutomationScope, asset *AssetDTO) json.RawMessage {
	switch scope {
	case AutomationScopeWorkspace:
		return json.RawMessage(`{}`)
	case AutomationScopeAsset:
		b, _ := json.Marshal(map[string]string{"asset_id": asset.ID})
		return b
	case AutomationScopeFolder:
		if asset.FolderID != nil && *asset.FolderID != "" {
			b, _ := json.Marshal(map[string]string{"folder_id": *asset.FolderID})
			return b
		}
		fallthrough
	case AutomationScopeProject:
		if asset.ProjectID != nil && *asset.ProjectID != "" {
			b, _ := json.Marshal(map[string]string{"project_id": *asset.ProjectID})
			return b
		}
	}
	return json.RawMessage(`{}`)
}

func buildVariantAutomationName(scope AutomationScope, asset *AssetDTO) string {
	switch scope {
	case AutomationScopeWorkspace:
		return "Variant automation - all uploads"
	case AutomationScopeFolder:
		if asset.FolderID != nil && *asset.FolderID != "" {
			return fmt.Sprintf("Variant automation - folder %s", *asset.FolderID)
		}
		fallthrough
	case AutomationScopeProject:
		if asset.ProjectID != nil && *asset.ProjectID != "" {
			return fmt.Sprintf("Variant automation - project %s", *asset.ProjectID)
		}
	case AutomationScopeAsset:
		return fmt.Sprintf("Variant automation - asset %s", asset.OriginalFilename)
	}
	return "Variant automation - all uploads"
}

func defaultWorkflowTriggerConfig(v string) string {
	if strings.TrimSpace(v) == "" {
		return "{}"
	}
	return v
}

func (s *workflowService) regenerateWebhookToken(ctx context.Context, id string) (string, error) {
	token, err := workflow.NewToken()
	if err != nil {
		return "", err
	}
	if upsertErr := s.webhooks.Upsert(ctx, id, workflow.Sha256Hex(token)); upsertErr != nil {
		return "", upsertErr
	}
	return token, nil
}

func extractTriggerType(raw string) (string, error) {
	var graph workflow.Graph
	if err := json.Unmarshal([]byte(raw), &graph); err != nil {
		return "", fmt.Errorf("graph is not valid JSON: %w", apperr.ErrInvalidInput)
	}
	if err := graph.Validate(); err != nil {
		return "", fmt.Errorf("graph: %w: %w", err, apperr.ErrInvalidInput)
	}
	trigger, err := graph.TriggerNode()
	if err != nil {
		return "", fmt.Errorf("graph: %w: %w", err, apperr.ErrInvalidInput)
	}
	return trigger.Type, nil
}

func toWorkflowDTO(row repository.Workflow) WorkflowDTO {
	return WorkflowDTO{
		ID:                   row.ID,
		WorkspaceID:          row.WorkspaceID,
		Name:                 row.Name,
		Description:          row.Description,
		Enabled:              row.Enabled,
		TriggerType:          row.TriggerType,
		Graph:                row.Graph,
		NotifyOnFailureEmail: row.NotifyOnFailureEmail,
		LastRunAt:            row.LastRunAt,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}

func toWorkflowRunDTO(run repository.WorkflowRun, steps []repository.WorkflowRunStep) WorkflowRunDTO {
	dto := WorkflowRunDTO{
		ID:          run.ID,
		WorkflowID:  run.WorkflowID,
		Status:      run.Status,
		TriggerData: parseMap(run.TriggerData),
		Error:       run.Error,
		StartedAt:   run.StartedAt,
		CompletedAt: run.CompletedAt,
		CreatedAt:   run.CreatedAt,
	}
	for _, step := range steps {
		dto.Steps = append(dto.Steps, WorkflowRunStepDTO{
			NodeID:      step.NodeID,
			NodeType:    step.NodeType,
			Status:      step.Status,
			Attempt:     step.Attempt,
			InputCtx:    parseMap(step.InputCtx),
			OutputCtx:   parseMapPtr(step.OutputCtx),
			Error:       step.Error,
			StartedAt:   step.StartedAt,
			CompletedAt: step.CompletedAt,
		})
	}
	return dto
}

func parseMap(raw string) map[string]any {
	var out map[string]any
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func parseMapPtr(raw *string) map[string]any {
	if raw == nil {
		return map[string]any{}
	}
	return parseMap(*raw)
}

func mustJSONMap(v map[string]any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func toWorkflowNodePorts(in []workflow.Port) []WorkflowNodePort {
	out := make([]WorkflowNodePort, len(in))
	for i, port := range in {
		out[i] = WorkflowNodePort{ID: port.ID, Label: port.Label}
	}
	return out
}
