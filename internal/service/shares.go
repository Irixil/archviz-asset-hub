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
	"golang.org/x/crypto/bcrypt"
)

// ShareDTO is the output of ShareService methods.
type ShareDTO struct {
	ID            string
	WorkspaceID   string
	CreatedBy     string
	Label         string
	TargetType    string
	TargetID      string
	PasswordHash  *string
	ExpiresAt     *string
	AllowComments bool
	AllowDownload bool
	ViewCount     int64
	CreatedAt     time.Time
	RevokedAt     *string
}

// CreateShareParams is the input for ShareService.Create.
type CreateShareParams struct {
	CreatedBy     string
	Label         string
	TargetType    string
	TargetID      string
	Password      *string
	ExpiresInDays *int
	AllowComments bool
	AllowDownload bool
}

func (p *CreateShareParams) Validate() error {
	p.Label = strings.TrimSpace(p.Label)
	validTargets := map[string]bool{
		string(AutomationScopeAsset):   true,
		string(AutomationScopeProject): true,
		"collection":                   true,
	}
	if !validTargets[p.TargetType] {
		return fmt.Errorf("target_type must be asset, project, or collection: %w", apperr.ErrInvalidInput)
	}
	if p.TargetID == "" {
		return fmt.Errorf("target_id is required: %w", apperr.ErrInvalidInput)
	}
	return nil
}

// UpdateShareParams is the input for ShareService.Update.
type UpdateShareParams struct {
	Label         *string
	Password      *string
	ClearPassword bool
	ExpiresAt     *string
	ClearExpiry   bool
	AllowComments *bool
	AllowDownload *bool
}

type shareService struct {
	shares     repository.ShareRepository
	audit      audit.Writer
	bcryptCost int
}

// ShareServiceOption configures a shareService.
type ShareServiceOption func(*shareService)

// WithShareBcryptCost overrides the bcrypt work factor. Use bcrypt.MinCost in tests.
func WithShareBcryptCost(cost int) ShareServiceOption {
	return func(s *shareService) { s.bcryptCost = cost }
}

// NewShareService returns a ShareService.
func NewShareService(shares repository.ShareRepository, aw audit.Writer, opts ...ShareServiceOption) ShareService {
	svc := &shareService{shares: shares, audit: aw, bcryptCost: bcrypt.DefaultCost}
	for _, o := range opts {
		o(svc)
	}
	return svc
}

func (s *shareService) List(ctx context.Context, workspaceID string) ([]*ShareDTO, error) {
	rows, err := s.shares.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toShareDTO)
	return out, nil
}

func (s *shareService) Get(ctx context.Context, workspaceID, id string) (*ShareDTO, error) {
	sh, err := s.shares.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return toShareDTO(sh), nil
}

func (s *shareService) Create(ctx context.Context, workspaceID string, p CreateShareParams) (*ShareDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	var passwordHash *string
	if p.Password != nil && *p.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*p.Password), s.bcryptCost)
		if err != nil {
			return nil, err
		}
		h := string(hash)
		passwordHash = &h
	}

	var expiresAt *string
	if p.ExpiresInDays != nil && *p.ExpiresInDays > 0 {
		t := time.Now().UTC().Add(time.Duration(*p.ExpiresInDays) * 24 * time.Hour)
		formatted := t.Format("2006-01-02 15:04:05")
		expiresAt = &formatted
	}

	sh, err := s.shares.Create(ctx, repository.Share{
		ID:            uuid.NewString(),
		WorkspaceID:   workspaceID,
		CreatedBy:     p.CreatedBy,
		Label:         p.Label,
		TargetType:    p.TargetType,
		TargetID:      p.TargetID,
		PasswordHash:  passwordHash,
		ExpiresAt:     expiresAt,
		AllowComments: p.AllowComments,
		AllowDownload: p.AllowDownload,
	})
	if err != nil {
		return nil, err
	}
	dto := toShareDTO(sh)
	if dto.TargetType == "asset" {
		actor := auth.ActorFromCtx(ctx)
		s.audit.WriteAsset(ctx, audit.AssetEvent{
			WorkspaceID: workspaceID,
			AssetID:     dto.TargetID,
			UserID:      actor.UserID,
			ActorType:   actor.Type,
			EventType:   audit.EventAssetShared,
			Payload: audit.AssetSharedPayload{
				V:          1,
				ShareID:    dto.ID,
				TargetType: dto.TargetType,
				ExpiresAt:  dto.ExpiresAt,
			},
		})
	}
	return dto, nil
}

func (s *shareService) Update(ctx context.Context, workspaceID, id string, p UpdateShareParams) (*ShareDTO, error) {
	existing, err := s.shares.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}

	if p.Label != nil {
		existing.Label = *p.Label
	}
	//nolint:nestif // clean me some day
	if p.ClearPassword {
		existing.PasswordHash = nil
	} else if p.Password != nil {
		if *p.Password == "" {
			existing.PasswordHash = nil
		} else {
			hash, hashErr := bcrypt.GenerateFromPassword([]byte(*p.Password), s.bcryptCost)
			if hashErr != nil {
				return nil, hashErr
			}
			h := string(hash)
			existing.PasswordHash = &h
		}
	}
	if p.ClearExpiry {
		existing.ExpiresAt = nil
	} else if p.ExpiresAt != nil {
		existing.ExpiresAt = p.ExpiresAt
	}
	if p.AllowComments != nil {
		existing.AllowComments = *p.AllowComments
	}
	if p.AllowDownload != nil {
		existing.AllowDownload = *p.AllowDownload
	}

	updated, err := s.shares.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return toShareDTO(updated), nil
}

func (s *shareService) Revoke(ctx context.Context, workspaceID, id string) error {
	sh, err := s.shares.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if revokeErr := s.shares.Revoke(ctx, workspaceID, id); revokeErr != nil {
		return revokeErr
	}
	if sh.TargetType == "asset" {
		actor := auth.ActorFromCtx(ctx)
		s.audit.WriteAsset(ctx, audit.AssetEvent{
			WorkspaceID: workspaceID,
			AssetID:     sh.TargetID,
			UserID:      actor.UserID,
			ActorType:   actor.Type,
			EventType:   audit.EventAssetShareRevoked,
			Payload:     audit.AssetShareRevokedPayload{V: 1, ShareID: sh.ID},
		})
	}
	return nil
}

func toShareDTO(sh repository.Share) *ShareDTO {
	return &ShareDTO{
		ID:            sh.ID,
		WorkspaceID:   sh.WorkspaceID,
		CreatedBy:     sh.CreatedBy,
		Label:         sh.Label,
		TargetType:    sh.TargetType,
		TargetID:      sh.TargetID,
		PasswordHash:  sh.PasswordHash,
		ExpiresAt:     sh.ExpiresAt,
		AllowComments: sh.AllowComments,
		AllowDownload: sh.AllowDownload,
		ViewCount:     sh.ViewCount,
		CreatedAt:     sh.CreatedAt,
		RevokedAt:     sh.RevokedAt,
	}
}
