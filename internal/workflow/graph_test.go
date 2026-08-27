package workflow_test

import (
	"testing"

	"damask/server/internal/workflow"
)

func TestGraphValidateOK(t *testing.T) {
	g := workflow.Graph{
		Nodes: []workflow.GraphNode{
			{ID: "n1", Type: "trigger.manual"},
			{ID: "n2", Type: "action.tag"},
		},
		Edges: []workflow.GraphEdge{{FromNode: "n1", FromPort: "out", ToNode: "n2", ToPort: "in"}},
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestGraphValidateRejectsMissingTrigger(t *testing.T) {
	g := workflow.Graph{
		Nodes: []workflow.GraphNode{{ID: "n1", Type: "action.tag"}},
	}
	if err := g.Validate(); err == nil {
		t.Fatal("Validate() expected error")
	}
}
