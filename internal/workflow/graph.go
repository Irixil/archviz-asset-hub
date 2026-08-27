package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

func nodesByID(nodes []GraphNode) map[string]GraphNode {
	m := make(map[string]GraphNode, len(nodes))
	for _, n := range nodes {
		m[n.ID] = n
	}
	return m
}

type GraphNode struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Config   json.RawMessage `json:"config"`
	Position GraphPosition   `json:"position"`
}

type GraphPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type GraphEdge struct {
	FromNode string `json:"from_node"`
	FromPort string `json:"from_port"`
	ToNode   string `json:"to_node"`
	ToPort   string `json:"to_port"`
}

func (g *Graph) Validate() error {
	if len(g.Nodes) == 0 {
		return errors.New("graph must contain at least one node")
	}
	seen := make(map[string]struct{}, len(g.Nodes))
	triggerCount := 0
	for _, node := range g.Nodes {
		if node.ID == "" {
			return errors.New("node id is required")
		}
		if _, exists := seen[node.ID]; exists {
			return fmt.Errorf("duplicate node id %q", node.ID)
		}
		schema, ok := SchemaFor(node.Type)
		if !ok {
			return fmt.Errorf("unknown node type %q", node.Type)
		}
		if schema.Category == nodeCategoryTrigger {
			triggerCount++
		}
		seen[node.ID] = struct{}{}
	}
	if triggerCount != 1 {
		return errors.New("graph must contain exactly one trigger node")
	}
	byID := nodesByID(g.Nodes)
	for _, edge := range g.Edges {
		fromNode, ok := byID[edge.FromNode]
		if !ok {
			return fmt.Errorf("edge references unknown from_node %q", edge.FromNode)
		}
		toNode, ok := byID[edge.ToNode]
		if !ok {
			return fmt.Errorf("edge references unknown to_node %q", edge.ToNode)
		}
		fromSchema, _ := SchemaFor(fromNode.Type)
		toSchema, _ := SchemaFor(toNode.Type)
		if !hasPort(fromSchema.Outputs, edge.FromPort) {
			return fmt.Errorf("edge references unknown output port %q on node %q", edge.FromPort, fromNode.ID)
		}
		if !hasPort(toSchema.Inputs, edge.ToPort) {
			return fmt.Errorf("edge references unknown input port %q on node %q", edge.ToPort, toNode.ID)
		}
	}
	if _, err := g.TopologicalSort(); err != nil {
		return err
	}
	return nil
}

func (g *Graph) TriggerNode() (GraphNode, error) {
	for _, node := range g.Nodes {
		schema, ok := SchemaFor(node.Type)
		if ok && schema.Category == nodeCategoryTrigger {
			return node, nil
		}
	}
	return GraphNode{}, errors.New("trigger node not found")
}

func (g *Graph) Successors(nodeID, fromPort string) []GraphNode {
	byID := nodesByID(g.Nodes)
	var out []GraphNode
	for _, edge := range g.Edges {
		if edge.FromNode == nodeID && edge.FromPort == fromPort {
			if next, ok := byID[edge.ToNode]; ok {
				out = append(out, next)
			}
		}
	}
	return out
}

func (g *Graph) TopologicalSort() ([]GraphNode, error) {
	adj := make(map[string][]string, len(g.Nodes))
	indegree := make(map[string]int, len(g.Nodes))
	byID := nodesByID(g.Nodes)
	for _, node := range g.Nodes {
		indegree[node.ID] = 0
	}
	for _, edge := range g.Edges {
		adj[edge.FromNode] = append(adj[edge.FromNode], edge.ToNode)
		indegree[edge.ToNode]++
	}
	queue := []string{}
	for _, node := range g.Nodes {
		if indegree[node.ID] == 0 {
			queue = append(queue, node.ID)
		}
	}
	sorted := make([]GraphNode, 0, len(g.Nodes))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, byID[id])
		for _, next := range adj[id] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if len(sorted) != len(g.Nodes) {
		return nil, errors.New("graph contains a cycle")
	}
	return sorted, nil
}

func hasPort(ports []Port, id string) bool {
	for _, port := range ports {
		if port.ID == id {
			return true
		}
	}
	return false
}
