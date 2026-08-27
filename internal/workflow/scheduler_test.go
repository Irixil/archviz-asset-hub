package workflow_test

import (
	"encoding/json"
	"testing"
	"time"

	"damask/server/internal/repository"
	"damask/server/internal/workflow"
)

func makeScheduleWorkflow(cronExpr string) repository.Workflow {
	cfg, _ := json.Marshal(map[string]string{"cron": cronExpr})
	graph, _ := json.Marshal(workflow.Graph{
		Nodes: []workflow.GraphNode{
			{ID: "n1", Type: "trigger.schedule", Config: cfg},
		},
		Edges: []workflow.GraphEdge{},
	})
	return repository.Workflow{
		ID:          "wf_sched",
		WorkspaceID: "ws_1",
		Graph:       string(graph),
	}
}

func TestIsDue_NilLastRunAt_EveryMinute(t *testing.T) {
	s := &workflow.CronScheduler{}
	wf := makeScheduleWorkflow("* * * * *")
	if !s.IsDue(wf) {
		t.Fatal("expected isDue=true when LastRunAt is nil")
	}
}

func TestIsDue_RecentLastRunAt_EveryMinute(t *testing.T) {
	s := &workflow.CronScheduler{}
	wf := makeScheduleWorkflow("* * * * *")
	ago := time.Now().Add(-2 * time.Minute)
	wf.LastRunAt = &ago
	if !s.IsDue(wf) {
		t.Fatal("expected isDue=true when last run was 2 minutes ago and cron is * * * * *")
	}
}

func TestIsDue_TooSoon_DailyAt3am(t *testing.T) {
	s := &workflow.CronScheduler{}
	wf := makeScheduleWorkflow("0 3 * * *")
	ago := time.Now().Add(-30 * time.Second)
	wf.LastRunAt = &ago
	if s.IsDue(wf) {
		t.Fatal("expected isDue=false when last run was 30s ago and cron is daily at 3am")
	}
}

func TestIsDue_InvalidCron(t *testing.T) {
	s := &workflow.CronScheduler{}
	wf := makeScheduleWorkflow("not a cron")
	if s.IsDue(wf) {
		t.Fatal("expected isDue=false for invalid cron expression")
	}
}

func TestIsDue_MissingCronKey(t *testing.T) {
	s := &workflow.CronScheduler{}
	cfg, _ := json.Marshal(map[string]string{"interval_minutes": "5"})
	graph, _ := json.Marshal(workflow.Graph{
		Nodes: []workflow.GraphNode{
			{ID: "n1", Type: "trigger.schedule", Config: cfg},
		},
		Edges: []workflow.GraphEdge{},
	})
	wf := repository.Workflow{
		ID:          "wf_no_cron",
		WorkspaceID: "ws_1",
		Graph:       string(graph),
	}
	if s.IsDue(wf) {
		t.Fatal("expected isDue=false when cron key is absent")
	}
}
