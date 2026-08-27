package workflow

import (
	"context"
	"sync"
	"testing"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
)

// countingQueue is a JobQueue that records enqueued jobs per workspace.
type countingQueue struct {
	mu       sync.Mutex
	enqueued map[string]int // workspaceID -> count
}

func newCountingQueue() *countingQueue {
	return &countingQueue{enqueued: map[string]int{}}
}

func (q *countingQueue) Register(string, queue.HandlerFunc) {}
func (q *countingQueue) Start(context.Context)              {}
func (q *countingQueue) Stop()                              {}

func (q *countingQueue) Enqueue(_ context.Context, workspaceID, jobType, payload string) (dbgen.Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.enqueued[workspaceID]++
	return dbgen.Job{ID: "job_test", WorkspaceID: workspaceID, Type: jobType, Payload: payload}, nil
}

func (q *countingQueue) count(workspaceID string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.enqueued[workspaceID]
}

func seedTagAddedWorkflow(t *testing.T, workflows *memory.WorkflowRepo, id, workspaceID string) {
	t.Helper()
	// The trigger config pins workspace_id so this workflow only matches
	// dispatches carrying its own workspace in the trigger data.
	graph := `{"nodes":[{"id":"trigger","type":"trigger.tag_added",` +
		`"config":{"workspace_id":"` + workspaceID + `"}}],"edges":[]}`
	workflows.Seed(repository.Workflow{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        "On tag added",
		Enabled:     true,
		TriggerType: "trigger.tag_added",
		Graph:       graph,
		CreatedBy:   "usr_1",
	})
}

func TestDispatch_DepthZero_Fires(t *testing.T) {
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	q := newCountingQueue()
	seedTagAddedWorkflow(t, workflows, "wf_1", "ws_1")

	d := NewTriggerDispatcher(workflows, runs, q)
	err := d.Dispatch(context.Background(), "trigger.tag_added", map[string]any{"workspace_id": "ws_1", "tag": "photo"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got := q.count("ws_1"); got != 1 {
		t.Fatalf("expected 1 enqueued run for depth-0 dispatch, got %d", got)
	}
}

func TestDispatch_WorkflowDepth_Suppressed(t *testing.T) {
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	q := newCountingQueue()
	seedTagAddedWorkflow(t, workflows, "wf_1", "ws_1")

	d := NewTriggerDispatcher(workflows, runs, q)
	ctx := WithTriggerDepth(context.Background(), 1)
	err := d.Dispatch(ctx, "trigger.tag_added", map[string]any{"workspace_id": "ws_1", "tag": "photo"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got := q.count("ws_1"); got != 0 {
		t.Fatalf("expected workflow-caused dispatch to be suppressed, got %d enqueued", got)
	}
	if rs, _ := runs.List(context.Background(), "ws_1", "wf_1", 10, ""); len(rs) != 0 {
		t.Fatalf("expected no workflow runs created, got %d", len(rs))
	}
}

// redispatchingTagManager mimics the real TagService: applying a tag publishes
// trigger.tag_added through the dispatcher using the caller's context.
type redispatchingTagManager struct {
	dispatcher *TriggerDispatcher
}

func (m redispatchingTagManager) AddToAsset(ctx context.Context, workspaceID, assetID, tagName string) (string, error) {
	if err := m.dispatcher.Dispatch(ctx, "trigger.tag_added", map[string]any{
		"workspace_id": workspaceID,
		"asset_id":     assetID,
		"tag":          tagName,
	}); err != nil {
		return "", err
	}
	return tagName, nil
}

// TestExecutor_SetTagAction_DoesNotRetrigger is the loop regression test: a
// workflow whose "set tag" action applies a tag through a trigger-publishing
// TagService must not re-publish trigger.tag_added and enqueue a new run.
func TestExecutor_SetTagAction_DoesNotRetrigger(t *testing.T) {
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	q := newCountingQueue()
	dispatcher := NewTriggerDispatcher(workflows, runs, q)

	graph := `{"nodes":[` +
		`{"id":"trigger","type":"trigger.tag_added","config":{}},` +
		`{"id":"tag","type":"action.tag","config":{"name":"processed"}}],` +
		`"edges":[{"from_node":"trigger","from_port":"out","to_node":"tag","to_port":"in"}]}`
	workflows.Seed(repository.Workflow{
		ID:          "wf_1",
		WorkspaceID: "ws_1",
		Name:        "Tag cascade",
		Enabled:     true,
		TriggerType: "trigger.tag_added",
		Graph:       graph,
		CreatedBy:   "usr_1",
	})
	_, err := runs.Create(context.Background(), repository.CreateWorkflowRunParams{
		ID:          "run_1",
		WorkflowID:  "wf_1",
		WorkspaceID: "ws_1",
		Status:      WorkflowRunStatusPending,
		TriggerData: `{"asset_id":"asset_1","tag":"photo"}`,
		Context:     `{}`,
	})
	if err != nil {
		t.Fatalf("seed run: %v", err)
	}

	exec := NewExecutor(Deps{
		Workflows: workflows,
		Runs:      runs,
		Queue:     q,
		Hub:       &testHub{},
		Audit:     &testAuditWriter{},
		Mailer:    &testMailer{},
		Tags:      redispatchingTagManager{dispatcher: dispatcher},
	})
	if runErr := exec.Run(context.Background(), "run_1"); runErr != nil {
		t.Fatalf("executor run: %v", runErr)
	}

	if got := q.count("ws_1"); got != 0 {
		t.Fatalf("workflow set-tag re-published trigger.tag_added: %d runs enqueued (loop regression)", got)
	}
	run, err := runs.GetByID(context.Background(), "run_1")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if run.Status != "completed" {
		t.Fatalf("expected run to complete, got status %q", run.Status)
	}
}

// TestTriggerDepth_ConcurrentWorkspaces verifies the depth guard is carried
// per-request in context, not in shared dispatcher state: a workflow-caused
// mutation in workspace A running concurrently with a user-initiated one in
// workspace B suppresses only A.
func TestTriggerDepth_ConcurrentWorkspaces(t *testing.T) {
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	q := newCountingQueue()
	seedTagAddedWorkflow(t, workflows, "wf_a", "ws_a")
	seedTagAddedWorkflow(t, workflows, "wf_b", "ws_b")

	d := NewTriggerDispatcher(workflows, runs, q)

	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		ctx := WithTriggerDepth(context.Background(), 1) // workflow-caused, ws_a
		for range iterations {
			_ = d.Dispatch(ctx, "trigger.tag_added", map[string]any{"workspace_id": "ws_a"})
		}
	}()
	go func() {
		defer wg.Done()
		ctx := context.Background() // user-initiated, ws_b
		for range iterations {
			_ = d.Dispatch(ctx, "trigger.tag_added", map[string]any{"workspace_id": "ws_b"})
		}
	}()
	wg.Wait()

	if got := q.count("ws_a"); got != 0 {
		t.Fatalf("workspace A (workflow-caused) fired %d triggers, want 0", got)
	}
	// Both workflows have an empty trigger config, so they match any dispatch;
	// each ws_b dispatch enqueues one run per matching workflow. What matters
	// here is that ws_b kept firing while ws_a stayed suppressed.
	if got := q.count("ws_b"); got == 0 {
		t.Fatal("workspace B (user-initiated) triggers were wrongly suppressed")
	}
}
