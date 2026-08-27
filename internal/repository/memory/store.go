package memory

import (
	"fmt"
	"sync"

	"damask/server/internal/apperr"
)

// mapStore is a generic, goroutine-safe map store for test repos.
// T must have ID and WorkspaceID string fields accessible via the key/workspace
// extractor functions passed to each helper.
// Embed it in concrete repo structs and delegate the mechanical methods.
type mapStore[T any] struct {
	mu    sync.RWMutex
	items map[string]T
}

func newMapStore[T any]() mapStore[T] {
	return mapStore[T]{items: make(map[string]T)}
}

// seed populates the store. keyFn extracts the map key (always ID).
func (s *mapStore[T]) seed(items []T, keyFn func(T) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range items {
		s.items[keyFn(item)] = item
	}
}

// get returns the item for id, filtered by workspaceID.
func (s *mapStore[T]) get(name, id, workspaceID string, workspaceFn func(T) string) (T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	if !ok || workspaceFn(item) != workspaceID {
		var zero T
		return zero, fmt.Errorf("%s %q: %w", name, id, apperr.ErrNotFound)
	}
	return item, nil
}

// del removes the item for id, checking workspaceID scope.
func (s *mapStore[T]) del(name, id, workspaceID string, workspaceFn func(T) string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok || workspaceFn(item) != workspaceID {
		return fmt.Errorf("%s %q: %w", name, id, apperr.ErrNotFound)
	}
	delete(s.items, id)
	return nil
}

// put writes item unconditionally (used by Create).
func (s *mapStore[T]) put(key string, item T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = item
}

// putChecked writes item only if an existing entry for key belongs to workspaceID.
// Used by Update to guard against cross-workspace writes.
func (s *mapStore[T]) putChecked(name, key, workspaceID string, workspaceFn func(T) string, item T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.items[key]
	if !ok || workspaceFn(existing) != workspaceID {
		return fmt.Errorf("%s %q: %w", name, key, apperr.ErrNotFound)
	}
	s.items[key] = item
	return nil
}

// mutate reads-locks, calls fn with a copy, then write-locks and stores the result.
// fn receives the current value and returns the mutated value plus an error.
func (s *mapStore[T]) mutate(name, id, workspaceID string, workspaceFn func(T) string, fn func(T) (T, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok || workspaceFn(item) != workspaceID {
		return fmt.Errorf("%s %q: %w", name, id, apperr.ErrNotFound)
	}
	updated, err := fn(item)
	if err != nil {
		return err
	}
	s.items[id] = updated
	return nil
}

// all returns a snapshot of every item, for use by List* methods.
func (s *mapStore[T]) all() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]T, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	return out
}
