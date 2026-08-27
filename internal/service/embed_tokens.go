package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/slug"
	"damask/server/internal/token"
)

const (
	embedTokenLength = 16
	embedSlugMaxLen  = 60
)

// embedSlugFor derives a cosmetic, URL-safe slug from an asset's filename for
// use as a readable prefix on public embed URLs. Purely decorative — see
// ExtractTokenID, which strips it back off before any DB lookup.
func embedSlugFor(filename string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	s := slug.ToSlug(base)
	if len(s) > embedSlugMaxLen {
		s = strings.Trim(s[:embedSlugMaxLen], "-")
	}
	return s
}

// ExtractTokenID strips the cosmetic slug prefix (if any) from a public embed
// URL's :token path param, returning the trailing fixed-length token id used
// for the DB lookup. The slug is decorative only — see embedSlugFor.
func ExtractTokenID(param string) string {
	if len(param) <= embedTokenLength {
		return param
	}
	return param[len(param)-embedTokenLength:]
}

// EmbedTokenDTO is the output of EmbedTokenService methods.
type EmbedTokenDTO struct {
	ID        string
	AssetID   string
	PublicURL string
	ThumbURL  string
	CreatedAt time.Time
	Revoked   bool
}

// ResolvedEmbed carries the file-serving data needed by the public /e/:token endpoints.
type ResolvedEmbed struct {
	StorageKey           string
	ThumbnailKey         *string // nil if not yet generated
	MimeType             string
	ThumbnailContentType string
	Filename             string
	ContentHash          string // hex sha256 — used as ETag value
}

type embedTokenService struct {
	tokens   repository.EmbedTokenRepository
	assets   repository.AssetRepository
	versions repository.VersionRepository
	baseURL  string
}

// NewEmbedTokenService returns an EmbedTokenService.
func NewEmbedTokenService(
	tokens repository.EmbedTokenRepository,
	assets repository.AssetRepository,
	versions repository.VersionRepository,
	baseURL string,
) EmbedTokenService {
	return &embedTokenService{tokens: tokens, assets: assets, versions: versions, baseURL: baseURL}
}

// GetOrCreate tries GetActiveByAssetID first. On ErrNotFound, generates a new
// 16-char base62 id and calls Create. On ErrConflict from Create (race
// condition: another request created the token concurrently), retries
// GetActiveByAssetID once.
func (s *embedTokenService) GetOrCreate(
	ctx context.Context,
	workspaceID, assetID, userID string,
) (*EmbedTokenDTO, error) {
	existing, err := s.tokens.GetActiveByAssetID(ctx, workspaceID, assetID)
	if err == nil {
		return s.toDTO(ctx, existing), nil
	}
	if !errors.Is(err, apperr.ErrNotFound) {
		return nil, err
	}

	id, err := token.NewBase62(embedTokenLength)
	if err != nil {
		return nil, err
	}
	created, err := s.tokens.Create(ctx, repository.CreateEmbedTokenParams{
		ID:          id,
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		CreatedBy:   userID,
	})
	if err == nil {
		return s.toDTO(ctx, created), nil
	}
	if !errors.Is(err, apperr.ErrConflict) {
		return nil, err
	}

	// Race: another request created the token between our GetActiveByAssetID
	// check and our Create call. The partial unique index rejected our insert,
	// so the winner's row must now be visible.
	existing, err = s.tokens.GetActiveByAssetID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, fmt.Errorf("embed token: conflict on create but no active token found: %w", err)
	}
	return s.toDTO(ctx, existing), nil
}

// GetActive returns the active token for an asset, or apperr.ErrNotFound.
func (s *embedTokenService) GetActive(ctx context.Context, workspaceID, assetID string) (*EmbedTokenDTO, error) {
	t, err := s.tokens.GetActiveByAssetID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	return s.toDTO(ctx, t), nil
}

// Revoke first calls GetActiveByAssetID to get the token id, then calls
// repo.Revoke. This double lookup lets us return ErrNotFound clearly when no
// active token exists, without exposing the internal id.
func (s *embedTokenService) Revoke(ctx context.Context, workspaceID, assetID string) error {
	t, err := s.tokens.GetActiveByAssetID(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	return s.tokens.Revoke(ctx, workspaceID, t.ID)
}

// ResolveCurrentFile loads the token, checks revocation, then follows
// assets.current_version_id (a primary-key lookup) rather than scanning
// asset_versions for is_current = 1.
func (s *embedTokenService) ResolveCurrentFile(ctx context.Context, tokenID string) (*ResolvedEmbed, error) {
	t, err := s.tokens.GetByID(ctx, tokenID)
	if err != nil {
		return nil, err
	}
	if t.RevokedAt != nil {
		return nil, ErrGone
	}

	asset, err := s.assets.GetByID(ctx, t.WorkspaceID, t.AssetID)
	if err != nil {
		return nil, err
	}
	if asset.CurrentVersionID == nil {
		return nil, apperr.ErrNotFound
	}
	version, err := s.versions.GetByID(ctx, *asset.CurrentVersionID)
	if err != nil {
		return nil, err
	}

	return &ResolvedEmbed{
		StorageKey:           version.StorageKey,
		ThumbnailKey:         asset.ThumbnailKey,
		MimeType:             version.MimeType,
		ThumbnailContentType: asset.ThumbnailContentType,
		Filename:             asset.OriginalFilename,
		ContentHash:          version.ContentHash,
	}, nil
}

func (s *embedTokenService) toDTO(ctx context.Context, t repository.EmbedToken) *EmbedTokenDTO {
	path := t.ID
	if asset, err := s.assets.GetByID(ctx, t.WorkspaceID, t.AssetID); err == nil {
		if slugPart := embedSlugFor(asset.OriginalFilename); slugPart != "" {
			path = slugPart + "-" + t.ID
		}
	}
	return &EmbedTokenDTO{
		ID:        t.ID,
		AssetID:   t.AssetID,
		PublicURL: s.baseURL + "/e/" + path,
		ThumbURL:  s.baseURL + "/e/" + path + "/thumb",
		CreatedAt: t.CreatedAt,
		Revoked:   t.RevokedAt != nil,
	}
}
