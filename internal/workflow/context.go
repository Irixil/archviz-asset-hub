// Package workflow manages asset processing workflow state.
package workflow

import (
	"encoding/json"
	"maps"
	"sync"
)

type RunContext struct {
	mu   sync.RWMutex
	data map[string]any
}

func NewRunContext(seed map[string]any) *RunContext {
	data := map[string]any{}
	maps.Copy(data, seed)
	return &RunContext{data: data}
}

func (rc *RunContext) Get(key string) (any, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	v, ok := rc.data[key]
	return v, ok
}

func (rc *RunContext) Set(key string, val any) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.data[key] = val
}

func (rc *RunContext) Delete(key string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	delete(rc.data, key)
}

func (rc *RunContext) Merge(updates map[string]any) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	maps.Copy(rc.data, updates)
}

func (rc *RunContext) Clone() *RunContext {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	clone := make(map[string]any, len(rc.data))
	maps.Copy(clone, rc.data)
	return &RunContext{data: clone}
}

func (rc *RunContext) MarshalJSON() ([]byte, error) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return json.Marshal(rc.data)
}
