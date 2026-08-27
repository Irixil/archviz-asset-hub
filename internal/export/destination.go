// Package export implements the project export feature.
package export

import (
	"context"
	"fmt"
	"io"
)

// Destination is the write side of an external storage location.
// Implementations must be safe for concurrent use.
type Destination interface {
	Type() string

	// Write uploads the content of r to remotePath at the destination.
	// contentHash is the sha256 hex of the bytes (for logging / future
	// server-side dedup). Implementations should create intermediate
	// directories if the protocol requires it.
	Write(ctx context.Context, remotePath string, r io.Reader, size int64, contentHash string) error

	// ReadManifest fetches the sidecar manifest at remotePath.
	// Returns (nil, nil) if the file does not exist — this is normal on
	// the first run.
	ReadManifest(ctx context.Context, remotePath string) ([]byte, error)

	// WriteManifest atomically overwrites the sidecar manifest.
	WriteManifest(ctx context.Context, remotePath string, data []byte) error

	// Validate checks that credentials are valid and the destination path
	// is writable. Must not leave any side effects (clean up probe files).
	Validate(ctx context.Context) error
}

// ConstructorFn builds a destination from decrypted config JSON.
type ConstructorFn func(configJSON []byte) (Destination, error)

var registry = map[string]ConstructorFn{}

// Register registers a destination constructor for the given type.
func Register(destType string, fn ConstructorFn) { registry[destType] = fn }

// NewDestination constructs a destination from the given type and config JSON.
func NewDestination(destType string, configJSON []byte) (Destination, error) {
	fn, ok := registry[destType]
	if !ok {
		return nil, fmt.Errorf("unknown export destination type: %s", destType)
	}
	return fn(configJSON)
}
