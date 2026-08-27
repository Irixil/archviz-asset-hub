package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// Storage is the interface that wraps basic asset storage operations.
// LocalStorage is the default implementation. Swap for S3Storage at config level.
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
	Stat(ctx context.Context, key string) (Info, error)
}

// Info holds metadata about a stored object, as returned by Stat.
type Info struct {
	Size    int64
	ModTime time.Time
}

// ErrNotFound is the sentinel error returned (wrapped) by backends when a
// requested key does not exist.
var ErrNotFound = errors.New("storage: object not found")

// VersionedVariantKey returns the canonical storage path for a variant that
// belongs to a specific asset version.
//
// Format: {workspaceID}/{assetID}/v{versionNum}/variants/{variantType}/{paramsHash}{ext}
//
// paramsHash is the first 8 hex characters of SHA-256(canonical JSON of transform_params).
// ext should include the leading dot, e.g. ".jpg". Pass "" for formats without an extension.
func VersionedVariantKey(workspaceID, assetID string, versionNum int64, variantType, paramsHash, ext string) string {
	return fmt.Sprintf("%s/%s/v%d/variants/%s/%s%s",
		workspaceID, assetID, versionNum, variantType, paramsHash, ext)
}
