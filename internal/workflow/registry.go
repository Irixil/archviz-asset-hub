package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

type Port struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type NodeSchema struct {
	Type         string          `json:"type"`
	Label        string          `json:"label"`
	Category     string          `json:"category"`
	Description  string          `json:"description"`
	Inputs       []Port          `json:"inputs"`
	Outputs      []Port          `json:"outputs"`
	ConfigSchema json.RawMessage `json:"config_schema"`
}

type Node interface {
	Schema() NodeSchema
	Execute(
		ctx context.Context,
		rc *RunContext,
		cfg json.RawMessage,
	) (outputPort string, updates map[string]any, err error)
}

type Factory func(Deps) Node

type registration struct {
	schema  NodeSchema
	factory Factory
}

var (
	registryMu sync.RWMutex
	registry   = map[string]registration{}
)

func Register(schema NodeSchema, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[schema.Type] = registration{schema: schema, factory: factory}
}

func Build(deps Deps, nodeType string) (Node, error) {
	registryMu.RLock()
	entry, ok := registry[nodeType]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("workflow node type %q is not registered", nodeType)
	}
	return entry.factory(deps), nil
}

func SchemaFor(nodeType string) (NodeSchema, bool) {
	registryMu.RLock()
	entry, ok := registry[nodeType]
	registryMu.RUnlock()
	return entry.schema, ok
}

func NodeSchemas() []NodeSchema {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]NodeSchema, 0, len(registry))
	for _, entry := range registry {
		out = append(out, entry.schema)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].Label < out[j].Label
		}
		if out[i].Category == nodeCategoryTrigger {
			return true
		}
		if out[j].Category == nodeCategoryTrigger {
			return false
		}
		return out[i].Category < out[j].Category
	})
	return out
}
