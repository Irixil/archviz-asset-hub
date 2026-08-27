// Package ingress implements the asset ingestion pipeline.
// Sources fetch items from external systems; the shared pipeline
// deduplicates, downloads, stores, and creates asset records.
package ingress

import (
	"context"
	"fmt"
	"io"
	"time"

	dbgen "damask/server/internal/db/gen"
)

// IngestItem represents a single remote item to be fetched.
type IngestItem struct {
	RemoteID string // source-specific unique key used for dedup
	Filename string
	ModTime  time.Time
	Size     int64
	Meta     map[string]string // source-specific metadata (subject, sender, etc.)
}

// Source is implemented by each ingestor backend.
// Each call to Poll or Fetch MUST use its own connection — no shared state.
type Source interface {
	// Type returns the string type identifier, e.g. "imap", "sftp".
	Type() string

	// Validate tests credentials without side effects.
	// Used by the /test endpoint.
	Validate(ctx context.Context) error

	// Poll returns all items currently available from the remote source.
	Poll(ctx context.Context) ([]IngestItem, error)

	// Fetch returns a streaming reader for one item.
	// The caller must close the returned ReadCloser.
	Fetch(ctx context.Context, item IngestItem) (io.ReadCloser, error)
}

// ConstructorFn builds a Source from its decrypted config JSON.
type ConstructorFn func(configJSON []byte) (Source, error)

var registry = map[string]ConstructorFn{}

// Register adds a source type to the global registry.
// Call from init() in each source sub-package.
func Register(sourceType string, fn ConstructorFn) {
	registry[sourceType] = fn
}

// Build constructs a Source from its type and decrypted config JSON.
func Build(sourceType string, configJSON []byte) (Source, error) {
	fn, ok := registry[sourceType]
	if !ok {
		return nil, fmt.Errorf("unknown ingress source type: %q", sourceType)
	}
	return fn(configJSON)
}

// OnCreateHookFn may mutate the user-provided config map before it is
// encrypted and stored. Return an error to abort source creation.
type OnCreateHookFn func(config map[string]any) (map[string]any, error)

var onCreateRegistry = map[string]OnCreateHookFn{}

// RegisterOnCreate registers a creation hook for a source type.
// Call from init() in each source sub-package that needs it.
func RegisterOnCreate(sourceType string, fn OnCreateHookFn) {
	onCreateRegistry[sourceType] = fn
}

// OnAfterCreateHookFn is run after the source creation.
type OnAfterCreateHookFn func(source dbgen.IngressSource) error

var onAfterCreateRegistry = map[string]OnAfterCreateHookFn{}

// RegisterOnAfterCreate registers a creation hook for a source type.
// Call from init() in each source sub-package that needs it.
func RegisterOnAfterCreate(sourceType string, fn OnAfterCreateHookFn) {
	onAfterCreateRegistry[sourceType] = fn
}

// RunOnCreateHook invokes the OnCreate hook for sourceType if one is registered.
// Returns config unchanged when no hook is found.
func RunOnCreateHook(sourceType string, config map[string]any) (map[string]any, error) {
	fn, ok := onCreateRegistry[sourceType]
	if !ok {
		return config, nil
	}
	return fn(config)
}
