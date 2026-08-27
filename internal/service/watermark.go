package service

import (
	"context"
	"errors"
	"fmt"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

var ErrNoWatermarkAsset = errors.New(
	"no watermark asset found: upload an image named '*watermark*' to this folder, project, or workspace",
)

type watermarkService struct {
	assets repository.AssetRepository
}

func NewWatermarkService(
	assets repository.AssetRepository,
	folders repository.FolderRepository,
) WatermarkService {
	_ = folders
	return &watermarkService{assets: assets}
}

func (s *watermarkService) ResolveWatermarkAsset(
	ctx context.Context,
	workspaceID, assetID string,
) (*WatermarkAssetDTO, error) {
	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}

	if asset.FolderID != nil {
		row, folderErr := s.assets.FindWatermarkInFolder(ctx, workspaceID, *asset.FolderID)
		if folderErr == nil {
			return toWatermarkAssetDTO(row, "folder"), nil
		}
		if !errors.Is(folderErr, apperr.ErrNotFound) {
			return nil, fmt.Errorf("find folder watermark: %w", folderErr)
		}
	}

	if asset.ProjectID != nil {
		row, projectErr := s.assets.FindWatermarkInProject(ctx, workspaceID, *asset.ProjectID)
		if projectErr == nil {
			return toWatermarkAssetDTO(row, "project"), nil
		}
		if !errors.Is(projectErr, apperr.ErrNotFound) {
			return nil, fmt.Errorf("find project watermark: %w", projectErr)
		}
	}

	row, err := s.assets.FindWatermarkInWorkspace(ctx, workspaceID)
	if err == nil {
		return toWatermarkAssetDTO(row, "workspace"), nil
	}
	if errors.Is(err, apperr.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrNoWatermarkAsset, apperr.ErrInvalidInput)
	}
	return nil, fmt.Errorf("find workspace watermark: %w", err)
}

func toWatermarkAssetDTO(asset repository.Asset, scope string) *WatermarkAssetDTO {
	return &WatermarkAssetDTO{
		ID:         asset.ID,
		Name:       asset.OriginalFilename,
		StorageKey: asset.StorageKey,
		MimeType:   asset.MimeType,
		Scope:      scope,
	}
}
