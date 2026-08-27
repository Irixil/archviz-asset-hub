package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"damask/server/internal/audit"
	"damask/server/internal/events"
	"damask/server/internal/repository"
	"damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	workflowRunStatusRunning    = "running"
	workflowRunStatusCompleted  = "completed"
	workflowRunStatusFailed     = "failed"
	workflowRunFailedEvent      = "workflow_run_failed"
	workflowRunStepUpdatedEvent = "workflow_run_step_updated"
	workflowRunStepErrorPort    = "error"
)

type Executor struct {
	deps Deps
}

func NewExecutor(deps Deps) *Executor {
	return &Executor{deps: deps}
}

func (e *Executor) Run(ctx context.Context, runID string) error {
	var err error
	ctx, span := tracer.Start(ctx, "workflow.run", trace.WithAttributes(attribute.String("workflow.run_id", runID)))
	defer telemetry.EndSpan(span, err)

	run, err := e.deps.Runs.GetByID(ctx, runID)
	if err != nil {
		telemetry.EndSpan(span, err)
		return err
	}

	wf, err := e.deps.Workflows.GetByID(ctx, run.WorkspaceID, run.WorkflowID)
	if err != nil {
		telemetry.EndSpan(span, err)
		return err
	}

	span.SetAttributes(
		attribute.String("workflow.id", wf.ID),
		attribute.String("damask.workspace_id", wf.WorkspaceID),
	)

	var graph Graph
	err = json.Unmarshal([]byte(wf.Graph), &graph)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid graph JSON")
		_ = e.deps.Runs.SetFinal(ctx, repository.SetWorkflowRunFinalParams{
			ID:          runID,
			Status:      workflowRunStatusFailed,
			Context:     run.Context,
			Error:       ptr(err.Error()),
			CompletedAt: nowPtr(),
		})
		return err
	}

	rc := NewRunContext(jsonToMap(run.TriggerData))
	rc.Set("workspace_id", wf.WorkspaceID)
	rc.Set("workflow_id", wf.ID)
	rc.Set("workflow_created_by", wf.CreatedBy)

	err = e.deps.Runs.SetStatus(ctx, runID, workflowRunStatusRunning)
	if err != nil {
		return err
	}

	trigger, err := graph.TriggerNode()
	if err != nil {
		return err
	}
	// Mutations performed by workflow nodes carry a trigger depth so the
	// dispatcher refuses to fire triggers from them (recursion guard).
	ctx = WithTriggerDepth(ctx, TriggerDepthFrom(ctx)+1)
	runErr := e.executeNode(ctx, &graph, trigger, rc, runID, wf.WorkspaceID, wf.ID)

	if runErr != nil {
		span.RecordError(runErr)
		span.SetStatus(codes.Error, runErr.Error())
	}
	return e.finalizeRun(ctx, wf, runID, rc, runErr)
}

func (e *Executor) executeNode(
	ctx context.Context,
	g *Graph,
	node GraphNode,
	rc *RunContext,
	runID, workspaceID, workflowID string,
) (err error) {
	ctx, span := tracer.Start(ctx, "workflow.step", trace.WithAttributes(
		attribute.String("workflow.node_id", node.ID),
		attribute.String("workflow.node_type", node.Type),
		attribute.String("workflow.run_id", runID),
	))
	defer func() { telemetry.EndSpan(span, err) }()

	n, err := Build(e.deps, node.Type)
	if err != nil {
		return err
	}

	if ci, ok := n.(ContinuationInjector); ok {
		ci.InjectContinuation(rc, g, node.ID, runID, workflowID, workspaceID)
	}

	stepID := newID()
	step, err := e.deps.Runs.CreateStep(ctx, repository.CreateWorkflowRunStepParams{
		ID:        stepID,
		RunID:     runID,
		NodeID:    node.ID,
		NodeType:  node.Type,
		Status:    workflowRunStatusRunning,
		Attempt:   1,
		InputCtx:  mustJSON(rc),
		StartedAt: nowPtr(),
	})
	if err != nil {
		return err
	}

	policy := retryPolicyFromConfig(node.Config)
	var outPort string
	var updates map[string]any
	var execErr error
	for attempt := 1; ; attempt++ {
		if attempt > 1 {
			_ = e.deps.Runs.IncrementStepAttempt(ctx, step.ID)
			span.AddEvent("retry", trace.WithAttributes(attribute.Int("attempt", attempt)))
		}
		outPort, updates, execErr = n.Execute(ctx, rc, node.Config)
		if execErr == nil || !policy.ShouldRetry(attempt, execErr) {
			break
		}
		time.Sleep(policy.WaitFor(attempt))
	}

	if execErr != nil {
		err = execErr
		span.RecordError(execErr)
		span.SetStatus(codes.Error, execErr.Error())
		_ = e.deps.Runs.SetStepFailed(ctx, stepID, execErr.Error())
		e.publishStepEvent(ctx, workspaceID, rc, runID, workflowID, node.ID, workflowRunStatusFailed, execErr.Error())
		errSuccessors := g.Successors(node.ID, workflowRunStepErrorPort)
		if walkErr := e.walkSuccessors(
			ctx,
			g,
			errSuccessors,
			rc.Clone(),
			runID,
			workspaceID,
			workflowID,
		); walkErr != nil {
			err = walkErr
			return err
		}
		if len(errSuccessors) > 0 {
			err = nil
			return nil
		}
		return err
	}

	rc.Merge(updates)
	rc.Delete(rcKeyContinuation)

	span.SetAttributes(attribute.String("workflow.output_port", outPort))
	_ = e.deps.Runs.SetStepCompleted(ctx, stepID, mustJSON(rc))
	e.publishStepEvent(ctx, workspaceID, rc, runID, workflowID, node.ID, workflowRunStatusCompleted, "")

	err = e.walkSuccessors(ctx, g, g.Successors(node.ID, outPort), rc, runID, workspaceID, workflowID)
	return err
}

func (e *Executor) walkSuccessors(
	ctx context.Context,
	g *Graph,
	successors []GraphNode,
	rc *RunContext,
	runID, workspaceID, workflowID string,
) error {
	switch len(successors) {
	case 0:
		return nil
	case 1:
		return e.executeNode(ctx, g, successors[0], rc, runID, workspaceID, workflowID)
	default:
		// Each parallel branch gets its own snapshot; writes don't propagate to siblings.
		var wg sync.WaitGroup
		var mu sync.Mutex
		var firstErr error
		for _, next := range successors {
			wg.Add(1)
			go func(next GraphNode) {
				defer wg.Done()
				if stepErr := e.executeNode(ctx, g, next, rc.Clone(), runID, workspaceID, workflowID); stepErr != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = stepErr
					}
					mu.Unlock()
				}
			}(next)
		}
		wg.Wait()
		return firstErr
	}
}

// ResumeAt restores a paused workflow run at a specific node, merging additional
// context updates (e.g. variant result data) before executing forward.
func (e *Executor) ResumeAt(ctx context.Context, cont NodeContinuation, updates map[string]any) error {
	var err error
	ctx, span := tracer.Start(ctx, "workflow.resume_at", trace.WithAttributes(
		attribute.String("workflow.run_id", cont.RunID),
		attribute.String("workflow.node_id", cont.NodeID),
	))
	defer telemetry.EndSpan(span, err)

	wf, err := e.deps.Workflows.GetByID(ctx, cont.WorkspaceID, cont.WorkflowID)
	if err != nil {
		telemetry.EndSpan(span, err)
		return err
	}

	var graph Graph
	if err = json.Unmarshal([]byte(wf.Graph), &graph); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid graph JSON")
		return err
	}

	rc := NewRunContext(jsonToMap(cont.ContextJSON))
	rc.Merge(updates)

	byID := nodesByID(graph.Nodes)
	targetNode, found := byID[cont.NodeID]
	if !found {
		err = fmt.Errorf("node %q not found in workflow graph", cont.NodeID)
		telemetry.EndSpan(span, err)
		return err
	}

	// Resumed nodes are still workflow-caused mutations — carry the depth
	// so the dispatcher's recursion guard applies here too.
	ctx = WithTriggerDepth(ctx, TriggerDepthFrom(ctx)+1)
	runErr := e.executeNode(ctx, &graph, targetNode, rc, cont.RunID, cont.WorkspaceID, cont.WorkflowID)

	// Guard against double-write: if the run was already finalised by a
	// portContinued branch in Execute, skip SetFinal here.
	existing, lookupErr := e.deps.Runs.GetByID(ctx, cont.RunID)
	if lookupErr == nil &&
		(existing.Status == workflowRunStatusCompleted || existing.Status == workflowRunStatusFailed) {
		return runErr
	}

	if runErr != nil {
		span.RecordError(runErr)
		span.SetStatus(codes.Error, runErr.Error())
	}
	return e.finalizeRun(ctx, wf, cont.RunID, rc, runErr)
}

// FailAt marks a paused workflow run as failed when the async job it was
// waiting on could not complete (e.g. a permanent OCR/AI/variant failure),
// so the run doesn't stay "running" forever. No-op (returns cause) if the
// run was already finalized by another path.
func (e *Executor) FailAt(ctx context.Context, cont NodeContinuation, cause error) error {
	wf, err := e.deps.Workflows.GetByID(ctx, cont.WorkspaceID, cont.WorkflowID)
	if err != nil {
		return err
	}
	if existing, lookupErr := e.deps.Runs.GetByID(ctx, cont.RunID); lookupErr == nil &&
		(existing.Status == workflowRunStatusCompleted || existing.Status == workflowRunStatusFailed) {
		return cause
	}
	rc := NewRunContext(jsonToMap(cont.ContextJSON))
	return e.finalizeRun(ctx, wf, cont.RunID, rc, cause)
}

func (e *Executor) finalizeRun(
	ctx context.Context,
	wf repository.Workflow,
	runID string,
	rc *RunContext,
	runErr error,
) error {
	status := workflowRunStatusCompleted
	if runErr != nil {
		status = workflowRunStatusFailed
	}
	finalErr := e.deps.Runs.SetFinal(ctx, repository.SetWorkflowRunFinalParams{
		ID:          runID,
		Status:      status,
		Context:     mustJSON(rc),
		Error:       errStringPtr(runErr),
		CompletedAt: nowPtr(),
	})
	if finalErr != nil && runErr == nil {
		runErr = finalErr
	}
	_ = e.deps.Workflows.TouchLastRunAt(ctx, wf.ID)
	if runErr != nil {
		e.reportRunFailure(ctx, wf, runID, rc, runErr)
	}
	return runErr
}

func (e *Executor) reportRunFailure(
	ctx context.Context,
	wf repository.Workflow,
	runID string,
	rc *RunContext,
	runErr error,
) {
	if e.deps.Hub != nil {
		assetID, _ := rcGetString(rc, "asset_id")
		e.deps.Hub.Publish(ctx, wf.WorkspaceID, events.WorkflowRunFailed(assetID, wf.ID, runID, runErr.Error()))
	}

	if e.deps.Audit != nil {
		if assetID, ok := rcGetString(rc, "asset_id"); ok && assetID != "" {
			e.deps.Audit.WriteAsset(ctx, audit.AssetEvent{
				WorkspaceID: wf.WorkspaceID,
				AssetID:     assetID,
				ActorType:   "system",
				EventType:   workflowRunFailedEvent,
				Payload: map[string]any{
					"workflow_id":            wf.ID,
					"workflow_name":          wf.Name,
					"run_id":                 runID,
					workflowRunStepErrorPort: runErr.Error(),
				},
			})
		}
	}

	if e.deps.Mailer != nil && wf.NotifyOnFailureEmail != "" {
		_ = e.deps.Mailer.SendWorkflowRunFailed(ctx, wf.NotifyOnFailureEmail, wf.Name, runErr.Error(), wf.WorkspaceID)
	}
}

func (e *Executor) publishStepEvent(
	ctx context.Context,
	workspaceID string,
	rc *RunContext,
	runID, workflowID, nodeID, status, errMsg string,
) {
	if e.deps.Hub == nil {
		return
	}
	assetID, _ := rcGetString(rc, "asset_id")
	ev := events.WorkflowRunStepUpdated(assetID, workflowID, runID, nodeID, status, errMsg)
	e.deps.Hub.Publish(ctx, workspaceID, ev)
}

func errStringPtr(err error) *string {
	if err == nil {
		return nil
	}
	msg := err.Error()
	return &msg
}
