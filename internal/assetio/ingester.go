// Package assetio handles asset ingestion and I/O operations.
package assetio

import (
	"context"
	"fmt"
	"time"

	"damask/server/internal/apperr"
)

// FieldInheritanceFunc is called after asset creation to copy project field values.
type FieldInheritanceFunc func(ctx context.Context, workspaceID, assetID, projectID, userID string)

// IngestFileOpts holds optional destination fields for Ingester.IngestFile.
type IngestFileOpts struct {
	ProjectID     *string
	FolderID      *string
	UserID        string
	InheritFields FieldInheritanceFunc
	// OriginalName overrides the filename derived from filePath.
	OriginalName string
}

// DuplicateMatch describes an existing version elsewhere in the workspace that
// shares content_hash with a just-ingested file.
type DuplicateMatch struct {
	AssetID          string
	VersionID        string
	VersionNum       int64
	OriginalFilename string
	ProjectID        *string
	ThumbnailKey     *string
	// IsDeletedVersion is true when the matched version has deleted_at set.
	IsDeletedVersion bool
	// StorageAvailable is best-effort: false if the matched version's storage
	// object could not be found (e.g. purged by a retention job).
	StorageAvailable bool
	CreatedAt        time.Time
}

// DuplicateConflictError is returned by Ingester.IngestFile /
// IngestFileWithDetails when duplicate_detection_mode is "block" and a
// matching version was found elsewhere in the workspace. The just-created
// asset has already been rolled back by the time this error is returned.
// Defined here (rather than in the service package that constructs it) so
// both the service and ingress packages can reference it without an import
// cycle (service already imports ingress).
type DuplicateConflictError struct {
	Match DuplicateMatch
}

func (e *DuplicateConflictError) Error() string {
	return fmt.Sprintf("duplicate content: matches existing asset %s", e.Match.AssetID)
}

func (e *DuplicateConflictError) Unwrap() error {
	return apperr.ErrConflict
}

// AssetSummary is the minimal asset data returned by Ingester.IngestFile.
type AssetSummary struct {
	ID               string
	WorkspaceID      string
	StorageKey       string
	MimeType         string
	OriginalFilename string
	// DuplicateOf is set when a content-hash duplicate was found elsewhere in
	// the workspace and the workspace's duplicate_detection_mode is "warn".
	DuplicateOf *DuplicateMatch
}

// Ingester handles low-level asset creation from a file path.
type Ingester interface {
	IngestFile(ctx context.Context, workspaceID, filePath string, opts IngestFileOpts) (AssetSummary, error)
}
