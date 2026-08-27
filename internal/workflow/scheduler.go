package workflow

import (
	"context"
	"encoding/json"
	"time"

	"damask/server/internal/repository"
	"damask/server/internal/telemetry"

	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel/attribute"
)

var schedulerTracer = telemetry.Tracer("damask/internal/workflow/scheduler")

type CronScheduler struct {
	workflows  repository.WorkflowRepository
	dispatcher *TriggerDispatcher
	interval   time.Duration
}

func NewCronScheduler(workflows repository.WorkflowRepository, dispatcher *TriggerDispatcher) *CronScheduler {
	return &CronScheduler{workflows: workflows, dispatcher: dispatcher, interval: time.Minute}
}

func (s *CronScheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

func (s *CronScheduler) tick(ctx context.Context) {
	var err error
	ctx, span := schedulerTracer.Start(ctx, "workflow.scheduler.tick")
	defer telemetry.EndSpan(span, err)

	due, err := s.workflows.ListByTrigger(ctx, "trigger.schedule")
	if err != nil {
		span.RecordError(err)
		return
	}
	fired := 0
	for _, wf := range due {
		if !s.IsDue(wf) {
			continue
		}
		_ = s.dispatcher.Dispatch(ctx, "trigger.schedule", map[string]any{
			"workspace_id": wf.WorkspaceID, //nolint:goconst // queue job payloads are opaque to the executor
			"workflow_id":  wf.ID,
		})
		fired++
	}
	span.SetAttributes(attribute.Int("workflow.scheduler.fired", fired))
}

func (s *CronScheduler) IsDue(wf repository.Workflow) bool {
	var graph Graph
	if err := json.Unmarshal([]byte(wf.Graph), &graph); err != nil {
		return false
	}
	trigger, err := graph.TriggerNode()
	if err != nil {
		return false
	}
	var cfg struct {
		Cron string `json:"cron"`
	}
	if unmarshalErr := json.Unmarshal(trigger.Config, &cfg); unmarshalErr != nil || cfg.Cron == "" {
		return false
	}
	sched, err := cron.ParseStandard(cfg.Cron)
	if err != nil {
		return false
	}
	last := time.Time{}
	if wf.LastRunAt != nil {
		last = *wf.LastRunAt
	}
	return !time.Now().Before(sched.Next(last))
}
