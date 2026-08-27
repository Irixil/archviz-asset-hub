package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

// CollectionDTO is the output of CollectionService methods.
type CollectionDTO struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	CreatedBy   string
	AssetCount  int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateCollectionParams is the input for CollectionService.Create.
type CreateCollectionParams struct {
	Name        string
	Description string
	CreatedBy   string
	AssetIDs    []string
}

func (p *CreateCollectionParams) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("name is required: %w", apperr.ErrInvalidInput)
	}
	return nil
}

// UpdateCollectionParams is the input for CollectionService.Update.
type UpdateCollectionParams struct {
	Name        *string
	Description *string
}

func (p *UpdateCollectionParams) Validate() error {
	if p.Name != nil {
		*p.Name = strings.TrimSpace(*p.Name)
		if *p.Name == "" {
			return fmt.Errorf("name cannot be empty: %w", apperr.ErrInvalidInput)
		}
	}
	return nil
}

type collectionService struct {
	collections repository.CollectionRepository
	assets      repository.AssetRepository
}

// NewCollectionService returns a CollectionService.
func NewCollectionService(
	collections repository.CollectionRepository,
	assets repository.AssetRepository,
) CollectionService {
	return &collectionService{collections: collections, assets: assets}
}

func (s *collectionService) List(ctx context.Context, workspaceID string) ([]*CollectionDTO, error) {
	rows, err := s.collections.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toCollectionDTO)
	return out, nil
}

func (s *collectionService) Get(ctx context.Context, workspaceID, id string) (*CollectionDTO, error) {
	col, err := s.collections.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return toCollectionDTO(col), nil
}

func (s *collectionService) Create(
	ctx context.Context,
	workspaceID string,
	p CreateCollectionParams,
) (*CollectionDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if len(p.AssetIDs) > 0 {
		count, err := s.assets.CountByIDs(ctx, workspaceID, p.AssetIDs)
		if err != nil {
			return nil, err
		}
		if count != int64(len(p.AssetIDs)) {
			return nil, fmt.Errorf("one or more assets do not belong to this workspace: %w", apperr.ErrForbidden)
		}
	}
	col, err := s.collections.Create(ctx, repository.Collection{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        p.Name,
		Description: p.Description,
		CreatedBy:   p.CreatedBy,
	})
	if err != nil {
		return nil, err
	}
	for _, assetID := range p.AssetIDs {
		if addErr := s.collections.AddAsset(ctx, col.ID, assetID); addErr != nil {
			return nil, addErr
		}
	}
	col.AssetCount = int64(len(p.AssetIDs))
	return toCollectionDTO(col), nil
}

func (s *collectionService) Update(
	ctx context.Context,
	workspaceID, id string,
	p UpdateCollectionParams,
) (*CollectionDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	existing, err := s.collections.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if p.Name != nil {
		existing.Name = *p.Name
	}
	if p.Description != nil {
		existing.Description = *p.Description
	}
	updated, err := s.collections.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return toCollectionDTO(updated), nil
}

func (s *collectionService) Delete(ctx context.Context, workspaceID, id string) error {
	if _, err := s.collections.GetByID(ctx, workspaceID, id); err != nil {
		return err
	}
	return s.collections.Delete(ctx, workspaceID, id)
}

func (s *collectionService) AddAsset(ctx context.Context, workspaceID, collectionID, assetID string) error {
	if _, err := s.collections.GetByID(ctx, workspaceID, collectionID); err != nil {
		return err
	}
	return s.collections.AddAsset(ctx, collectionID, assetID)
}

func (s *collectionService) RemoveAsset(ctx context.Context, workspaceID, collectionID, assetID string) error {
	if _, err := s.collections.GetByID(ctx, workspaceID, collectionID); err != nil {
		return err
	}
	return s.collections.RemoveAsset(ctx, collectionID, assetID)
}

func (s *collectionService) ListForAsset(ctx context.Context, workspaceID, assetID string) ([]*CollectionDTO, error) {
	rows, err := s.collections.ListForAsset(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toCollectionDTO)
	return out, nil
}

func (s *collectionService) ListAssets(ctx context.Context, workspaceID, collectionID string) ([]*AssetDTO, error) {
	if _, err := s.collections.GetByID(ctx, workspaceID, collectionID); err != nil {
		return nil, err
	}
	ids, err := s.collections.ListAssetIDs(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	out := make([]*AssetDTO, 0, len(ids))
	for _, id := range ids {
		a, getErr := s.assets.GetByID(ctx, workspaceID, id)
		if getErr != nil {
			continue // asset may have been soft-deleted
		}
		out = append(out, toAssetDTO(a))
	}
	return out, nil
}

func toCollectionDTO(c repository.Collection) *CollectionDTO {
	return &CollectionDTO{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Description: c.Description,
		CreatedBy:   c.CreatedBy,
		AssetCount:  c.AssetCount,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}
