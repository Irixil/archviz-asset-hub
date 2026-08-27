package mockservice

import (
	"context"

	"damask/server/internal/service"
)

type MockWorkflowService struct {
	ListFn               func(ctx context.Context, workspaceID string, params service.ListWorkflowsParams) ([]service.WorkflowDTO, error)
	GetFn                func(ctx context.Context, workspaceID, id string) (*service.WorkflowDTO, error)
	CreateFn             func(ctx context.Context, workspaceID, createdBy string, p service.CreateWorkflowParams) (*service.WorkflowDTO, error)
	UpdateFn             func(ctx context.Context, workspaceID, id string, p service.UpdateWorkflowParams) (*service.WorkflowDTO, error)
	SetEnabledFn         func(ctx context.Context, workspaceID, id string, enabled bool) error
	DeleteFn             func(ctx context.Context, workspaceID, id string) error
	TriggerManualFn      func(ctx context.Context, workspaceID, id, assetID string) (string, error)
	TriggerManualBulkFn  func(ctx context.Context, workspaceID, workflowID string, assetIDs []string) ([]string, error)
	TriggerWebhookFn     func(ctx context.Context, id, token string, body []byte) (string, error)
	GetRunFn             func(ctx context.Context, workspaceID, runID string) (*service.WorkflowRunDTO, error)
	ListRunsFn           func(ctx context.Context, workspaceID, workflowID string, limit int, cursor string) ([]service.WorkflowRunDTO, error)
	ListAllRunsFn        func(ctx context.Context, workspaceID string, limit int, cursor string) ([]service.WorkflowRunDTO, error)
	FindCoveringFn       func(ctx context.Context, workspaceID, assetID, assetProjectID, assetFolderID string) (*service.CoveringWorkflowDTO, error)
	CreateFromVariantsFn func(ctx context.Context, workspaceID string, p service.CreateVariantAutomationParams) (*service.WorkflowDTO, error)
	GetWebhookTokenFn    func(ctx context.Context, workspaceID, id string) (string, error)
	RegenerateTokenFn    func(ctx context.Context, workspaceID, id string) (string, error)
	NodeSchemasFn        func() []service.WorkflowNodeSchema
	TemplatesFn          func() []service.WorkflowTemplateDTO
}

func NewWorkflowService() *MockWorkflowService { return &MockWorkflowService{} }

func (m *MockWorkflowService) List(
	ctx context.Context,
	workspaceID string,
	params service.ListWorkflowsParams,
) ([]service.WorkflowDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID, params)
	}
	return nil, nil
}

func (m *MockWorkflowService) Get(ctx context.Context, workspaceID, id string) (*service.WorkflowDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, id)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockWorkflowService) Create(
	ctx context.Context,
	workspaceID, createdBy string,
	p service.CreateWorkflowParams,
) (*service.WorkflowDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, workspaceID, createdBy, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockWorkflowService) Update(
	ctx context.Context,
	workspaceID, id string,
	p service.UpdateWorkflowParams,
) (*service.WorkflowDTO, error) {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, workspaceID, id, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockWorkflowService) SetEnabled(ctx context.Context, workspaceID, id string, enabled bool) error {
	if m.SetEnabledFn != nil {
		return m.SetEnabledFn(ctx, workspaceID, id, enabled)
	}
	return nil
}

func (m *MockWorkflowService) Delete(ctx context.Context, workspaceID, id string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, id)
	}
	return nil
}

func (m *MockWorkflowService) TriggerManual(ctx context.Context, workspaceID, id, assetID string) (string, error) {
	if m.TriggerManualFn != nil {
		return m.TriggerManualFn(ctx, workspaceID, id, assetID)
	}
	return "", nil
}

func (m *MockWorkflowService) TriggerManualBulk(
	ctx context.Context,
	workspaceID, workflowID string,
	assetIDs []string,
) ([]string, error) {
	if m.TriggerManualBulkFn != nil {
		return m.TriggerManualBulkFn(ctx, workspaceID, workflowID, assetIDs)
	}
	return nil, nil
}

func (m *MockWorkflowService) TriggerWebhook(ctx context.Context, id, token string, body []byte) (string, error) {
	if m.TriggerWebhookFn != nil {
		return m.TriggerWebhookFn(ctx, id, token, body)
	}
	return "", nil
}

func (m *MockWorkflowService) GetRun(ctx context.Context, workspaceID, runID string) (*service.WorkflowRunDTO, error) {
	if m.GetRunFn != nil {
		return m.GetRunFn(ctx, workspaceID, runID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockWorkflowService) ListRuns(
	ctx context.Context,
	workspaceID, workflowID string,
	limit int,
	cursor string,
) ([]service.WorkflowRunDTO, error) {
	if m.ListRunsFn != nil {
		return m.ListRunsFn(ctx, workspaceID, workflowID, limit, cursor)
	}
	return nil, nil
}

func (m *MockWorkflowService) ListAllRuns(
	ctx context.Context,
	workspaceID string,
	limit int,
	cursor string,
) ([]service.WorkflowRunDTO, error) {
	if m.ListAllRunsFn != nil {
		return m.ListAllRunsFn(ctx, workspaceID, limit, cursor)
	}
	return nil, nil
}

func (m *MockWorkflowService) FindCoveringWorkflow(
	ctx context.Context,
	workspaceID, assetID, assetProjectID, assetFolderID string,
) (*service.CoveringWorkflowDTO, error) {
	if m.FindCoveringFn != nil {
		return m.FindCoveringFn(ctx, workspaceID, assetID, assetProjectID, assetFolderID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockWorkflowService) CreateFromVariants(
	ctx context.Context,
	workspaceID string,
	p service.CreateVariantAutomationParams,
) (*service.WorkflowDTO, error) {
	if m.CreateFromVariantsFn != nil {
		return m.CreateFromVariantsFn(ctx, workspaceID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockWorkflowService) GetWebhookToken(ctx context.Context, workspaceID, id string) (string, error) {
	if m.GetWebhookTokenFn != nil {
		return m.GetWebhookTokenFn(ctx, workspaceID, id)
	}
	return "", nil
}

func (m *MockWorkflowService) RegenerateWebhookToken(ctx context.Context, workspaceID, id string) (string, error) {
	if m.RegenerateTokenFn != nil {
		return m.RegenerateTokenFn(ctx, workspaceID, id)
	}
	return "", nil
}

func (m *MockWorkflowService) NodeSchemas() []service.WorkflowNodeSchema {
	if m.NodeSchemasFn != nil {
		return m.NodeSchemasFn()
	}
	return nil
}

func (m *MockWorkflowService) Templates() []service.WorkflowTemplateDTO {
	if m.TemplatesFn != nil {
		return m.TemplatesFn()
	}
	return nil
}
