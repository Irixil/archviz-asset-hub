package workflow

import (
	"encoding/json"
	"testing"

	"damask/server/internal/repository"
)

func makeTagWorkflow(tagValue string) repository.Workflow {
	cfg, _ := json.Marshal(map[string]string{"tag": tagValue})
	graph, _ := json.Marshal(Graph{
		Nodes: []GraphNode{
			{ID: "n1", Type: "trigger.tag_added", Config: cfg},
		},
		Edges: []GraphEdge{},
	})
	return repository.Workflow{
		ID:          "wf_tag",
		WorkspaceID: "ws_1",
		Graph:       string(graph),
	}
}

func TestTriggerMatches_TagAdded_Match(t *testing.T) {
	wf := makeTagWorkflow("photo")
	data := map[string]any{"tag": "photo"}
	if !triggerMatches(wf, data) {
		t.Fatal("expected triggerMatches=true for matching tag")
	}
}

func TestTriggerMatches_TagAdded_NoMatch(t *testing.T) {
	wf := makeTagWorkflow("photo")
	data := map[string]any{"tag": "other"}
	if triggerMatches(wf, data) {
		t.Fatal("expected triggerMatches=false for non-matching tag")
	}
}

func TestTriggerMatches_EmptyConfig_AlwaysMatches(t *testing.T) {
	graph, _ := json.Marshal(Graph{
		Nodes: []GraphNode{
			{ID: "n1", Type: "trigger.tag_added"},
		},
		Edges: []GraphEdge{},
	})
	wf := repository.Workflow{
		ID:    "wf_any",
		Graph: string(graph),
	}
	data := map[string]any{"tag": "anything"}
	if !triggerMatches(wf, data) {
		t.Fatal("expected triggerMatches=true when config is empty")
	}
}
