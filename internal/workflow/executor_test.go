package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"damask/server/internal/audit"
	"damask/server/internal/events"
	"damask/server/internal/mail"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"

	"github.com/gofiber/fiber/v3"
)

type testHub struct {
	published []events.Event
}

func (h *testHub) Subscribe(string, string) (<-chan events.Event, []events.Event, func()) {
	return nil, nil, func() {}
}
func (h *testHub) Publish(_ context.Context, _ string, ev events.Event) {
	h.published = append(h.published, ev)
}
func (h *testHub) EventHandler(fiber.Ctx) error { return nil }
func (h *testHub) DropCount() uint64            { return 0 }

type testAuditWriter struct {
	assetEvents []audit.AssetEvent
}

func (w *testAuditWriter) WriteAsset(_ context.Context, e audit.AssetEvent) {
	w.assetEvents = append(w.assetEvents, e)
}
func (w *testAuditWriter) WriteAssetAsync(e audit.AssetEvent) {
	w.assetEvents = append(w.assetEvents, e)
}
func (w *testAuditWriter) WriteProject(context.Context, audit.ProjectEvent) {}

type testMailer struct {
	to       string
	workflow string
	errMsg   string
}

func (m *testMailer) SendInvite(context.Context, string, string, string, string) error { return nil }
func (m *testMailer) SendWelcome(context.Context, string, string, string) error        { return nil }
func (m *testMailer) SendInviteAccepted(context.Context, string, string, string, string, string) error {
	return nil
}
func (m *testMailer) SendIngressSourceAdded(context.Context, string, string, string) error {
	return nil
}
func (m *testMailer) SendIngressSourceFailed(context.Context, string, string, string, string) error {
	return nil
}
func (m *testMailer) SendIngressSourceDisabled(context.Context, string, string, string, string) error {
	return nil
}
func (m *testMailer) SendCommentPosted(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (m *testMailer) SendPasswordReset(context.Context, string, string) error { return nil }
func (m *testMailer) SendEmailChangeConfirmation(context.Context, string, string, string) error {
	return nil
}
func (m *testMailer) SendWorkflowRunFailed(_ context.Context, to, workflowName, errMsg, _ string) error {
	m.to = to
	m.workflow = workflowName
	m.errMsg = errMsg
	return nil
}

var _ mail.Mailer = (*testMailer)(nil)

type failingNode struct{}

func (f failingNode) Schema() NodeSchema {
	return NodeSchema{
		Type:        "action.fail_test",
		Label:       "Fail",
		Category:    "action",
		Description: "Fails for tests.",
		Inputs:      []Port{{ID: "in", Label: "In"}},
		Outputs:     []Port{{ID: "out", Label: "Out"}, {ID: "error", Label: "Error"}},
	}
}

func (f failingNode) Execute(context.Context, *RunContext, json.RawMessage) (string, map[string]any, error) {
	return "", nil, errors.New("boom")
}

func TestExecutorReportsRunFailures(t *testing.T) {
	Register(failingNode{}.Schema(), func(Deps) Node { return failingNode{} })

	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	hub := &testHub{}
	auditWriter := &testAuditWriter{}
	mailer := &testMailer{}

	graph := `{"nodes":[{"id":"trigger","type":"trigger.manual","config":{},"position":{"x":0,"y":0}},{"id":"fail","type":"action.fail_test","config":{},"position":{"x":1,"y":1}}],"edges":[{"from_node":"trigger","from_port":"out","to_node":"fail","to_port":"in"}]}`

	workflows.Seed(repository.Workflow{
		ID:                   "wf_1",
		WorkspaceID:          "ws_1",
		Name:                 "Failure Workflow",
		Enabled:              true,
		TriggerType:          "trigger.manual",
		Graph:                graph,
		NotifyOnFailureEmail: "ops@example.com",
		CreatedBy:            "usr_1",
	})
	_, _ = runs.Create(context.Background(), repository.CreateWorkflowRunParams{
		ID:          "run_1",
		WorkflowID:  "wf_1",
		WorkspaceID: "ws_1",
		Status:      WorkflowRunStatusPending,
		TriggerData: `{"asset_id":"asset_1"}`,
		Context:     `{}`,
	})

	exec := NewExecutor(Deps{
		Workflows: workflows,
		Runs:      runs,
		Hub:       hub,
		Audit:     auditWriter,
		Mailer:    mailer,
	})

	err := exec.Run(context.Background(), "run_1")
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}

	run, err := runs.GetByID(context.Background(), "run_1")
	if err != nil {
		t.Fatalf("GetByID() unexpected error: %v", err)
	}
	if run.Status != "failed" {
		t.Fatalf("expected run status failed, got %q", run.Status)
	}
	if len(hub.published) != 3 {
		t.Fatalf("expected trigger step, failed step, and workflow failure events, got %+v", hub.published)
	}
	step0 := decodeStepUpdated(t, hub.published[0])
	if hub.published[0].Type != "workflow_run_step_updated" || step0.NodeID != "trigger" ||
		step0.Status != "completed" {
		t.Fatalf("expected trigger completion event, got %+v", hub.published[0])
	}
	step1 := decodeStepUpdated(t, hub.published[1])
	if hub.published[1].Type != "workflow_run_step_updated" || step1.NodeID != "fail" ||
		step1.Status != "failed" {
		t.Fatalf("expected failed step update event, got %+v", hub.published[1])
	}
	if hub.published[2].Type != "workflow_run_failed" {
		t.Fatalf("expected workflow failure event, got %+v", hub.published[2])
	}
	if len(auditWriter.assetEvents) != 1 || auditWriter.assetEvents[0].EventType != "workflow_run_failed" {
		t.Fatalf("expected workflow failure audit event, got %+v", auditWriter.assetEvents)
	}
	if mailer.to != "ops@example.com" || mailer.workflow != "Failure Workflow" || mailer.errMsg != "boom" {
		t.Fatalf("expected workflow failure email, got %+v", mailer)
	}
}

func TestExecutorFailAt_MarksRunFailed(t *testing.T) {
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	hub := &testHub{}
	auditWriter := &testAuditWriter{}
	mailer := &testMailer{}

	graph := `{"nodes":[{"id":"trigger","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`

	workflows.Seed(repository.Workflow{
		ID:                   "wf_1",
		WorkspaceID:          "ws_1",
		Name:                 "Continuation Workflow",
		Enabled:              true,
		TriggerType:          "trigger.manual",
		Graph:                graph,
		NotifyOnFailureEmail: "ops@example.com",
		CreatedBy:            "usr_1",
	})
	_, _ = runs.Create(context.Background(), repository.CreateWorkflowRunParams{
		ID:          "run_1",
		WorkflowID:  "wf_1",
		WorkspaceID: "ws_1",
		Status:      workflowRunStatusRunning,
		TriggerData: `{"asset_id":"asset_1"}`,
		Context:     `{"asset_id":"asset_1"}`,
	})

	exec := NewExecutor(Deps{
		Workflows: workflows,
		Runs:      runs,
		Hub:       hub,
		Audit:     auditWriter,
		Mailer:    mailer,
	})

	cont := NodeContinuation{
		RunID:       "run_1",
		NodeID:      "would_be_next",
		WorkflowID:  "wf_1",
		WorkspaceID: "ws_1",
		ContextJSON: `{"asset_id":"asset_1"}`,
	}
	cause := errors.New("ocr permanently failed")
	err := exec.FailAt(context.Background(), cont, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("expected FailAt to return the cause error, got %v", err)
	}

	run, err := runs.GetByID(context.Background(), "run_1")
	if err != nil {
		t.Fatalf("GetByID() unexpected error: %v", err)
	}
	if run.Status != "failed" {
		t.Fatalf("expected run status failed, got %q", run.Status)
	}
	if len(hub.published) != 1 || hub.published[0].Type != "workflow_run_failed" {
		t.Fatalf("expected one workflow failure event, got %+v", hub.published)
	}
	if len(auditWriter.assetEvents) != 1 || auditWriter.assetEvents[0].EventType != "workflow_run_failed" {
		t.Fatalf("expected workflow failure audit event, got %+v", auditWriter.assetEvents)
	}
	if mailer.errMsg != "ocr permanently failed" {
		t.Fatalf("expected workflow failure email, got %+v", mailer)
	}
}

func TestExecutorFailAt_NoOpIfAlreadyFinalized(t *testing.T) {
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	hub := &testHub{}
	auditWriter := &testAuditWriter{}
	mailer := &testMailer{}

	graph := `{"nodes":[{"id":"trigger","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`

	workflows.Seed(repository.Workflow{
		ID:                   "wf_1",
		WorkspaceID:          "ws_1",
		Name:                 "Continuation Workflow",
		Enabled:              true,
		TriggerType:          "trigger.manual",
		Graph:                graph,
		NotifyOnFailureEmail: "ops@example.com",
		CreatedBy:            "usr_1",
	})
	_, _ = runs.Create(context.Background(), repository.CreateWorkflowRunParams{
		ID:          "run_1",
		WorkflowID:  "wf_1",
		WorkspaceID: "ws_1",
		Status:      workflowRunStatusRunning,
		TriggerData: `{}`,
		Context:     `{}`,
	})
	if err := runs.SetFinal(context.Background(), repository.SetWorkflowRunFinalParams{
		ID:          "run_1",
		Status:      "completed",
		Context:     `{}`,
		CompletedAt: nowPtr(),
	}); err != nil {
		t.Fatalf("SetFinal() unexpected error: %v", err)
	}

	exec := NewExecutor(Deps{
		Workflows: workflows,
		Runs:      runs,
		Hub:       hub,
		Audit:     auditWriter,
		Mailer:    mailer,
	})

	cont := NodeContinuation{RunID: "run_1", NodeID: "would_be_next", WorkflowID: "wf_1", WorkspaceID: "ws_1"}
	cause := errors.New("resume failed after run already completed")
	err := exec.FailAt(context.Background(), cont, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("expected FailAt to return the cause error, got %v", err)
	}

	run, err := runs.GetByID(context.Background(), "run_1")
	if err != nil {
		t.Fatalf("GetByID() unexpected error: %v", err)
	}
	if run.Status != "completed" {
		t.Fatalf("expected run status to remain completed, got %q", run.Status)
	}
	if len(hub.published) != 0 {
		t.Fatalf("expected no events published for an already-finalized run, got %+v", hub.published)
	}
	if len(auditWriter.assetEvents) != 0 {
		t.Fatalf("expected no audit events for an already-finalized run, got %+v", auditWriter.assetEvents)
	}
	if mailer.errMsg != "" {
		t.Fatalf("expected no failure email for an already-finalized run, got %+v", mailer)
	}
}

func decodeStepUpdated(t *testing.T, ev events.Event) events.WorkflowRunStepUpdatedPayload {
	t.Helper()
	var p events.WorkflowRunStepUpdatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		t.Fatalf("decode workflow_run_step_updated payload: %v", err)
	}
	return p
}
