package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/slug"

	"github.com/google/uuid"
)

// FolderDTO is the output of FolderService methods.
type FolderDTO struct {
	ID          string
	WorkspaceID string
	ProjectID   string
	ParentID    *string
	Name        string
	Slug        *string
	Position    int64
	CreatedAt   time.Time
	Children    []*FolderDTO
}

// FolderTreeDTO is a folder with asset count and pre-built children, returned by ListTree.
type FolderTreeDTO struct {
	ID          string
	WorkspaceID string
	ProjectID   string
	ParentID    *string
	Name        string
	Slug        *string
	Position    int64
	CreatedAt   time.Time
	AssetCount  int64
	Children    []*FolderTreeDTO
}

// CreateFolderParams is the input for FolderService.Create.
type CreateFolderParams struct {
	Name     string
	ParentID *string
	Position int64
}

func (p *CreateFolderParams) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("name is required: %w", apperr.ErrInvalidInput)
	}
	return nil
}

// UpdateFolderParams is the input for FolderService.Update.
type UpdateFolderParams struct {
	Name     *string
	Position *int64
}

func (p *UpdateFolderParams) Validate() error {
	if p.Name != nil {
		*p.Name = strings.TrimSpace(*p.Name)
		if *p.Name == "" {
			return fmt.Errorf("name cannot be empty: %w", apperr.ErrInvalidInput)
		}
	}
	return nil
}

type folderService struct {
	folders repository.FolderRepository
}

// NewFolderService returns a FolderService.
func NewFolderService(folders repository.FolderRepository) FolderService {
	return &folderService{folders: folders}
}

func (s *folderService) Create(
	ctx context.Context,
	workspaceID, projectID string,
	p CreateFolderParams,
) (*FolderDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	var parentID *string
	if p.ParentID != nil && *p.ParentID != "" {
		parent, err := s.folders.GetByID(ctx, workspaceID, *p.ParentID)
		if err != nil {
			return nil, err
		}
		if parent.ParentID != nil {
			return nil, fmt.Errorf("max folder depth is 2: %w", apperr.ErrInvalidInput)
		}
		if parent.ProjectID != projectID {
			return nil, fmt.Errorf("parent folder belongs to a different project: %w", apperr.ErrInvalidInput)
		}
		parentID = p.ParentID
	}

	// SQLite does not enforce UNIQUE for NULL parent_id, so check root duplicates here.
	if parentID == nil {
		existing, err := s.folders.ListByProject(ctx, workspaceID, projectID)
		if err != nil {
			return nil, err
		}
		for _, f := range existing {
			if f.ParentID == nil && f.Name == p.Name {
				return nil, fmt.Errorf("a folder with that name already exists here: %w", apperr.ErrConflict)
			}
		}
	}

	sl := slug.ToSlug(p.Name)
	folder, err := s.folders.Create(ctx, repository.Folder{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		ParentID:    parentID,
		Name:        p.Name,
		Slug:        &sl,
		Position:    p.Position,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("a folder with that name already exists here: %w", apperr.ErrConflict)
		}
		return nil, err
	}
	return toFolderDTO(folder), nil
}

func (s *folderService) Get(ctx context.Context, workspaceID, id string) (*FolderDTO, error) {
	folder, err := s.folders.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return toFolderDTO(folder), nil
}

func (s *folderService) List(ctx context.Context, workspaceID, projectID string) ([]*FolderDTO, error) {
	rows, err := s.folders.ListByProject(ctx, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toFolderDTO)
	return out, nil
}

func (s *folderService) ListTree(ctx context.Context, workspaceID, projectID string) ([]*FolderTreeDTO, error) {
	roots, err := s.folders.ListTree(ctx, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	return toFolderTreeDTOs(roots), nil
}

func toFolderTreeDTOs(nodes []repository.FolderTree) []*FolderTreeDTO {
	out := make([]*FolderTreeDTO, len(nodes))
	for i, n := range nodes {
		out[i] = &FolderTreeDTO{
			ID:          n.ID,
			WorkspaceID: n.WorkspaceID,
			ProjectID:   n.ProjectID,
			ParentID:    n.ParentID,
			Name:        n.Name,
			Slug:        n.Slug,
			Position:    n.Position,
			CreatedAt:   n.CreatedAt,
			AssetCount:  n.AssetCount,
			Children:    toFolderTreeDTOs(n.Children),
		}
	}
	return out
}

func (s *folderService) Update(ctx context.Context, workspaceID, id string, p UpdateFolderParams) (*FolderDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	existing, err := s.folders.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if p.Name != nil {
		existing.Name = *p.Name
		sl := slug.ToSlug(*p.Name)
		existing.Slug = &sl
	}
	if p.Position != nil {
		existing.Position = *p.Position
	}
	updated, err := s.folders.Update(ctx, existing)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("a folder with that name already exists here: %w", apperr.ErrConflict)
		}
		return nil, err
	}
	return toFolderDTO(updated), nil
}

func (s *folderService) Delete(ctx context.Context, workspaceID, id string) error {
	if _, err := s.folders.GetByID(ctx, workspaceID, id); err != nil {
		return err
	}
	children, err := s.folders.GetChildren(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	for _, child := range children {
		if nullErr := s.folders.NullifyAssets(ctx, workspaceID, child.ID); nullErr != nil {
			return nullErr
		}
		if delErr := s.folders.Delete(ctx, workspaceID, child.ID); delErr != nil {
			return delErr
		}
	}
	if nullErr := s.folders.NullifyAssets(ctx, workspaceID, id); nullErr != nil {
		return nullErr
	}
	return s.folders.Delete(ctx, workspaceID, id)
}

func toFolderDTO(f repository.Folder) *FolderDTO {
	return &FolderDTO{
		ID:          f.ID,
		WorkspaceID: f.WorkspaceID,
		ProjectID:   f.ProjectID,
		ParentID:    f.ParentID,
		Name:        f.Name,
		Slug:        f.Slug,
		Position:    f.Position,
		CreatedAt:   f.CreatedAt,
		Children:    []*FolderDTO{},
	}
}
