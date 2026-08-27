package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

// ErrGone is returned when a share has expired.
var ErrGone = errors.New("gone")

// PublicAssetDTO is the asset view returned by public share endpoints.
type PublicAssetDTO struct {
	ID               string
	WorkspaceID      string
	ProjectID        *string
	FolderID         *string
	OriginalFilename string
	StorageKey       string
	MimeType         string
	Size             int64
	Width            *int64
	Height           *int64
	ThumbnailKey     *string
	Metadata         *string
	SharedVariants   []SharedVariantDTO
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// PublicAssetFileDTO carries file-serving data for a shared asset download.
type PublicAssetFileDTO struct {
	MimeType         string
	OriginalFilename string
	StorageKey       string
	ContentHash      string
	Size             int64
	VersionCreatedAt string
}

// PublicAssetThumbDTO carries thumbnail-serving data.
type PublicAssetThumbDTO struct {
	ThumbnailKey string
	UpdatedAt    time.Time
}

// ShareCommentDTO is the service-layer representation of a share comment.
type ShareCommentDTO struct {
	ID          string
	ShareID     string
	AssetID     string
	AuthorName  string
	AuthorEmail *string
	Body        string
	CreatedAt   string
}

// CreateShareCommentParams is the input for SharePublicService.CreateComment.
type CreateShareCommentParams struct {
	ShareID     string
	AssetID     string
	AuthorName  string
	AuthorEmail *string
	Body        string
}

type variantMentionResolver interface {
	GetSharedByVariantAndAsset(ctx context.Context, variantID, assetID string) (repository.Variant, error)
}

var variantMentionRe = regexp.MustCompile(`^@([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}) `)

func resolveVariantMention(ctx context.Context, variants variantMentionResolver, body, assetID string) string {
	m := variantMentionRe.FindStringSubmatch(body)
	if m == nil {
		return body
	}
	v, err := variants.GetSharedByVariantAndAsset(ctx, m[1], assetID)
	if err != nil {
		return body
	}
	title := m[1]
	if v.Title != nil {
		title = *v.Title
	}
	return "@" + title + " " + body[len(m[0]):]
}

type sharePublicService struct {
	shares   repository.ShareRepository
	variants variantMentionResolver
	mailer   interface {
		SendCommentPosted(ctx context.Context, workspaceID, assetID, ownerEmail, authorName, shareLabel, body string) error
	}
	users repository.UserRepository
}

// NewSharePublicService returns a SharePublicService.
func NewSharePublicService(
	shares repository.ShareRepository,
	users repository.UserRepository,
	variants variantMentionResolver,
	mailer interface {
		SendCommentPosted(ctx context.Context, workspaceID, assetID, ownerEmail, authorName, shareLabel, body string) error
	},
) SharePublicService {
	return &sharePublicService{shares: shares, users: users, variants: variants, mailer: mailer}
}

func (s *sharePublicService) GetActive(ctx context.Context, shareID string) (*ShareDTO, error) {
	sh, err := s.shares.GetPublic(ctx, shareID)
	if err != nil {
		return nil, err
	}
	if sh.RevokedAt != nil {
		return nil, fmt.Errorf("share revoked: %w", apperr.ErrNotFound)
	}
	if isShareExpiredDomain(sh) {
		return nil, ErrGone
	}
	return toShareDTO(sh), nil
}

func (s *sharePublicService) IncrementViewCount(ctx context.Context, shareID string) error {
	return s.shares.IncrementViewCount(ctx, shareID)
}

func (s *sharePublicService) ListAssets(ctx context.Context, targetType, targetID string) ([]*PublicAssetDTO, error) {
	assets, err := s.shares.ListAssetsByTarget(ctx, targetType, targetID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(assets, toPublicAssetDTO)
	return out, nil
}

func (s *sharePublicService) GetAsset(ctx context.Context, assetID string) (*PublicAssetDTO, error) {
	a, err := s.shares.GetPublicAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return toPublicAssetDTO(a), nil
}

func (s *sharePublicService) GetAssetFile(ctx context.Context, assetID string) (*PublicAssetFileDTO, error) {
	f, err := s.shares.GetPublicAssetFile(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return &PublicAssetFileDTO{
		MimeType:         f.MimeType,
		OriginalFilename: f.OriginalFilename,
		StorageKey:       f.StorageKey,
		ContentHash:      f.ContentHash,
		Size:             f.Size,
		VersionCreatedAt: f.VersionCreatedAt,
	}, nil
}

func (s *sharePublicService) GetAssetThumb(ctx context.Context, assetID string) (*PublicAssetThumbDTO, error) {
	key, updatedAt, err := s.shares.GetPublicAssetThumb(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, fmt.Errorf("thumbnail not ready: %w", apperr.ErrNotFound)
	}
	return &PublicAssetThumbDTO{ThumbnailKey: *key, UpdatedAt: updatedAt}, nil
}

func (s *sharePublicService) IsAssetInTarget(ctx context.Context, targetType, targetID, assetID string) (bool, error) {
	return s.shares.IsAssetInTarget(ctx, targetType, targetID, assetID)
}

func (s *sharePublicService) CreateComment(ctx context.Context, p CreateShareCommentParams) (*ShareCommentDTO, error) {
	c := repository.ShareComment{
		ID:          uuid.NewString(),
		ShareID:     p.ShareID,
		AssetID:     p.AssetID,
		AuthorName:  p.AuthorName,
		AuthorEmail: p.AuthorEmail,
		Body:        p.Body,
	}
	asset, err := s.shares.GetPublicAsset(ctx, p.AssetID)
	if err != nil {
		return nil, err
	}
	created, err := s.shares.CreateComment(ctx, c)
	if err != nil {
		return nil, err
	}

	// Best-effort email notification.
	if sh, shErr := s.shares.GetPublic(ctx, p.ShareID); shErr == nil {
		if owner, ownerErr := s.users.GetByID(ctx, sh.CreatedBy); ownerErr == nil {
			emailBody := resolveVariantMention(ctx, s.variants, p.Body, p.AssetID)
			_ = s.mailer.SendCommentPosted(
				ctx,
				sh.WorkspaceID,
				asset.ID,
				owner.Email,
				p.AuthorName,
				asset.OriginalFilename,
				emailBody,
			)
		}
	}

	return toCommentDTO(created), nil
}

func (s *sharePublicService) ListCommentsByShare(ctx context.Context, shareID string) ([]*ShareCommentDTO, error) {
	rows, err := s.shares.ListCommentsByShare(ctx, shareID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toCommentDTO)
	return out, nil
}

func (s *sharePublicService) ListCommentsByShareAndAsset(
	ctx context.Context,
	shareID, assetID string,
) ([]*ShareCommentDTO, error) {
	rows, err := s.shares.ListCommentsByShareAndAsset(ctx, shareID, assetID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toCommentDTO)
	return out, nil
}

func (s *sharePublicService) DeleteComment(ctx context.Context, shareID, commentID string) error {
	return s.shares.DeleteComment(ctx, shareID, commentID)
}

func (s *sharePublicService) GetOwnerShare(ctx context.Context, workspaceID, shareID string) (*ShareDTO, error) {
	sh, err := s.shares.GetByIDAndWorkspace(ctx, workspaceID, shareID)
	if err != nil {
		return nil, err
	}
	return toShareDTO(sh), nil
}

func isShareExpiredDomain(sh repository.Share) bool {
	if sh.ExpiresAt == nil {
		return false
	}
	t, err := time.Parse("2006-01-02T15:04:05Z", *sh.ExpiresAt)
	if err != nil {
		t, err = time.Parse("2006-01-02 15:04:05", *sh.ExpiresAt)
	}
	return err == nil && time.Now().After(t)
}

func toPublicAssetDTO(a repository.PublicAsset) *PublicAssetDTO {
	return &PublicAssetDTO{
		ID:               a.ID,
		WorkspaceID:      a.WorkspaceID,
		ProjectID:        a.ProjectID,
		FolderID:         a.FolderID,
		OriginalFilename: a.OriginalFilename,
		StorageKey:       a.StorageKey,
		MimeType:         a.MimeType,
		Size:             a.Size,
		Width:            a.Width,
		Height:           a.Height,
		ThumbnailKey:     a.ThumbnailKey,
		Metadata:         a.Metadata,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}

func toCommentDTO(c repository.ShareComment) *ShareCommentDTO {
	return &ShareCommentDTO{
		ID:          c.ID,
		ShareID:     c.ShareID,
		AssetID:     c.AssetID,
		AuthorName:  c.AuthorName,
		AuthorEmail: c.AuthorEmail,
		Body:        c.Body,
		CreatedAt:   c.CreatedAt,
	}
}
