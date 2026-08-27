package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

// ProjectDTO is the output of ProjectService methods.
type ProjectDTO struct {
	ID             string
	WorkspaceID    string
	Name           string
	Description    *string
	Color          *string
	CoverAssetID   *string
	CoverVersionID *string
	AssetCount     int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CreateProjectParams is the input for ProjectService.Create.
type CreateProjectParams struct {
	Name        string
	Description *string
	Color       *string
}

func (p *CreateProjectParams) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("name is required: %w", apperr.ErrInvalidInput)
	}
	return nil
}

// UpdateProjectParams is the input for ProjectService.Update.
// All pointer fields are optional; nil means "keep existing".
type UpdateProjectParams struct {
	Name         *string
	Description  *string
	Color        *string
	CoverAssetID *string
}

func (p *UpdateProjectParams) Validate() error {
	if p.Name != nil {
		*p.Name = strings.TrimSpace(*p.Name)
		if *p.Name == "" {
			return fmt.Errorf("name cannot be empty: %w", apperr.ErrInvalidInput)
		}
	}
	return nil
}

type projectService struct {
	projects repository.ProjectRepository
	audit    audit.Writer
}

// NewProjectService returns a ProjectService.
func NewProjectService(projects repository.ProjectRepository, aw audit.Writer) ProjectService {
	return &projectService{projects: projects, audit: aw}
}

func (s *projectService) Create(ctx context.Context, workspaceID string, p CreateProjectParams) (*ProjectDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	proj, err := s.projects.Create(ctx, repository.Project{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        p.Name,
		Description: p.Description,
		Color:       p.Color,
	})
	if err != nil {
		return nil, err
	}
	dto := toProjectDTO(proj, 0)
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteProject(ctx, audit.ProjectEvent{
		WorkspaceID: workspaceID,
		ProjectID:   dto.ID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventProjectCreated,
		Payload:     audit.ProjectCreatedPayload{V: 1, Name: dto.Name},
	})
	return dto, nil
}

func (s *projectService) Get(ctx context.Context, workspaceID, id string) (*ProjectDTO, error) {
	proj, err := s.projects.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	// Fetch count from the list query (no dedicated count query exists).
	rows, err := s.projects.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	var count int64
	for _, r := range rows {
		if r.ID == id {
			count = r.AssetCount
			break
		}
	}
	return toProjectDTO(proj, count), nil
}

func (s *projectService) List(ctx context.Context, workspaceID string) ([]*ProjectDTO, error) {
	rows, err := s.projects.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]*ProjectDTO, len(rows))
	for i, r := range rows {
		out[i] = toProjectDTO(r.Project, r.AssetCount)
	}
	return out, nil
}

func (s *projectService) Update(
	ctx context.Context,
	workspaceID, id string,
	p UpdateProjectParams,
) (*ProjectDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	existing, err := s.projects.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	prevName := existing.Name
	// Merge: nil fields keep their existing value.
	if p.Name != nil {
		existing.Name = *p.Name
	}
	if p.Description != nil {
		existing.Description = p.Description
	}
	if p.Color != nil {
		existing.Color = p.Color
	}
	if p.CoverAssetID != nil {
		existing.CoverAssetID = p.CoverAssetID
	}
	updated, err := s.projects.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	dto := toProjectDTO(updated, 0)
	if dto.Name != prevName {
		actor := auth.ActorFromCtx(ctx)
		s.audit.WriteProject(ctx, audit.ProjectEvent{
			WorkspaceID: workspaceID,
			ProjectID:   id,
			UserID:      actor.UserID,
			ActorType:   actor.Type,
			EventType:   audit.EventProjectRenamed,
			Payload:     audit.ProjectRenamedPayload{V: 1, Before: prevName, After: dto.Name},
		})
	}
	return dto, nil
}

func (s *projectService) Delete(ctx context.Context, workspaceID, id string) error {
	if _, err := s.projects.GetByID(ctx, workspaceID, id); err != nil {
		return err
	}
	// Nullify project_id on assets, then delete the project.
	// Assets are kept in the library with project_id = NULL (soft orphan by design).
	// SQLite serialises writes so these two ops are effectively atomic.
	if err := s.projects.NullifyAssets(ctx, workspaceID, id); err != nil {
		return err
	}
	if err := s.projects.Delete(ctx, workspaceID, id); err != nil {
		return err
	}
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteProject(ctx, audit.ProjectEvent{
		WorkspaceID: workspaceID,
		ProjectID:   id,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventProjectDeleted,
		Payload:     audit.ProjectDeletedPayload{V: 1},
	})
	return nil
}

func toProjectDTO(p repository.Project, assetCount int64) *ProjectDTO {
	return &ProjectDTO{
		ID:             p.ID,
		WorkspaceID:    p.WorkspaceID,
		Name:           p.Name,
		Description:    p.Description,
		Color:          p.Color,
		CoverAssetID:   p.CoverAssetID,
		CoverVersionID: p.CoverVersionID,
		AssetCount:     assetCount,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}
