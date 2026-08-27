package workflow

import (
	"context"
	"encoding/json"
	"log/slog"

	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var dispatcherTracer = telemetry.Tracer("damask/internal/workflow/dispatcher")

const WorkflowRunStatusPending = "pending"
const WorkflowRunStatusCompleted = "completed"

type TriggerDispatcher struct {
	workflows repository.WorkflowRepository
	runs      repository.WorkflowRunRepository
	queue     queue.JobQueue
}

func NewTriggerDispatcher(
	workflows repository.WorkflowRepository,
	runs repository.WorkflowRunRepository,
	queue queue.JobQueue,
) *TriggerDispatcher {
	return &TriggerDispatcher{workflows: workflows, runs: runs, queue: queue}
}

func (d *TriggerDispatcher) Dispatch(ctx context.Context, eventType string, data map[string]any) error {
	if depth := TriggerDepthFrom(ctx); depth >= 1 {
		slog.DebugContext(ctx, "workflow trigger suppressed: mutation caused by a workflow run",
			"trigger_type", eventType, "depth", depth)
		return nil
	}

	var err error
	ctx, span := dispatcherTracer.Start(
		ctx,
		"workflow.dispatch",
		trace.WithAttributes(attribute.String("workflow.trigger_type", eventType)),
	)
	defer telemetry.EndSpan(span, err)

	wfs, err := d.workflows.ListByTrigger(ctx, eventType)
	if err != nil {
		return err
	}

	enqueued := 0
	for _, wf := range wfs {
		if !triggerMatches(wf, data) {
			continue
		}
		runID := newID()
		_, createErr := d.runs.Create(ctx, repository.CreateWorkflowRunParams{
			ID:          runID,
			WorkflowID:  wf.ID,
			WorkspaceID: wf.WorkspaceID,
			Status:      WorkflowRunStatusPending,
			TriggerData: mustJSON(data),
			Context:     "{}",
		})
		if createErr != nil {
			span.RecordError(createErr)
			continue
		}
		payload, _ := json.Marshal(RunWorkflowPayload{RunID: runID})
		if _, enqErr := d.queue.Enqueue(ctx, wf.WorkspaceID, queue.JobTypeRunWorkflow, string(payload)); enqErr != nil {
			span.RecordError(enqErr)
			continue
		}
		enqueued++
	}

	span.SetAttributes(
		attribute.Int("workflow.candidate_count", len(wfs)),
		attribute.Int("workflow.enqueued_count", enqueued),
	)
	if enqueued == 0 {
		slog.DebugContext(ctx, "workflow dispatcher matched no workflows", "trigger_type", eventType)
	}
	return nil
}

func triggerMatches(wf repository.Workflow, data map[string]any) bool {
	var graph Graph
	if err := json.Unmarshal([]byte(wf.Graph), &graph); err != nil {
		return false
	}
	trigger, err := graph.TriggerNode()
	if err != nil {
		return false
	}
	var cfg map[string]any
	if len(trigger.Config) == 0 {
		return true
	}
	if unmarshalErr := json.Unmarshal(trigger.Config, &cfg); unmarshalErr != nil {
		return false
	}
	for key, expected := range cfg {
		if key == "cron" || key == "expression" {
			continue
		}
		actual, ok := data[key]
		if !ok || actual != expected {
			return false
		}
	}
	return true
}
