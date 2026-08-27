package service

import (
	"context"
	"fmt"

	"damask/server/internal/assetio"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
)

// DuplicateMode controls how DuplicateService.FindDuplicate results are
// enforced by callers (the ingester, the ingress worker).
type DuplicateMode string

const (
	DuplicateModeOff   DuplicateMode = "off"
	DuplicateModeWarn  DuplicateMode = "warn"
	DuplicateModeBlock DuplicateMode = "block"
)

// DuplicateService looks up cross-asset content-hash duplicates within a workspace.
type DuplicateService interface {
	// FindDuplicate looks up an existing version with the same content_hash in
	// the workspace, excluding excludeAssetID (the asset currently being
	// created/uploaded). Returns nil, nil if no match. When multiple matches
	// exist, prefers: current+non-deleted > non-deleted > most recently created
	// (ranking is done by the repository query).
	FindDuplicate(ctx context.Context, workspaceID, contentHash, excludeAssetID string) (*assetio.DuplicateMatch, error)

	// Mode returns the workspace's configured duplicate_detection_mode.
	// Falls back to "warn" for empty/unrecognized values (should not happen
	// post-migration, since the column has a NOT NULL DEFAULT 'warn').
	Mode(ctx context.Context, workspaceID string) (DuplicateMode, error)
}

// DuplicateConflictError is returned by the ingester when
// duplicate_detection_mode is "block" and a matching version was found
// elsewhere in the workspace. The just-created asset has already been rolled
// back by the time this error is returned. Aliased from assetio so the
// ingress package (which cannot import service) can also detect it.
type DuplicateConflictError = assetio.DuplicateConflictError

type duplicateServiceImpl struct {
	versions   repository.VersionRepository
	assets     repository.AssetRepository
	workspaces repository.WorkspaceRepository
	stor       storage.Storage
}

// NewDuplicateService returns a DuplicateService backed by the given dependencies.
func NewDuplicateService(
	versions repository.VersionRepository,
	assets repository.AssetRepository,
	workspaces repository.WorkspaceRepository,
	stor storage.Storage,
) DuplicateService {
	return &duplicateServiceImpl{versions: versions, assets: assets, workspaces: workspaces, stor: stor}
}

func (s *duplicateServiceImpl) Mode(ctx context.Context, workspaceID string) (DuplicateMode, error) {
	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return DuplicateModeWarn, err
	}
	switch mode := DuplicateMode(ws.DuplicateDetectionMode); mode {
	case DuplicateModeOff, DuplicateModeWarn, DuplicateModeBlock:
		return mode, nil
	default:
		return DuplicateModeWarn, nil
	}
}

func (s *duplicateServiceImpl) FindDuplicate(
	ctx context.Context,
	workspaceID, contentHash, excludeAssetID string,
) (*assetio.DuplicateMatch, error) {
	matches, err := s.versions.FindDuplicateVersions(ctx, workspaceID, contentHash, excludeAssetID)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil //nolint:nilnil // no match is not an error
	}
	top := matches[0]

	asset, err := s.assets.GetByID(ctx, workspaceID, top.AssetID)
	if err != nil {
		return nil, fmt.Errorf("load duplicate match asset %q: %w", top.AssetID, err)
	}

	storageAvailable := true
	if _, statErr := s.stor.Stat(ctx, top.StorageKey); statErr != nil {
		storageAvailable = false
	}

	return &assetio.DuplicateMatch{
		AssetID:          top.AssetID,
		VersionID:        top.VersionID,
		VersionNum:       top.VersionNum,
		OriginalFilename: asset.OriginalFilename,
		ProjectID:        asset.ProjectID,
		ThumbnailKey:     asset.ThumbnailKey,
		IsDeletedVersion: top.DeletedAt != nil,
		StorageAvailable: storageAvailable,
		CreatedAt:        top.CreatedAt,
	}, nil
}
