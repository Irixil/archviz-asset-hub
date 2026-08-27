package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"damask/server/internal/apperr"
	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type oauthRepo struct {
	d *db.DB
}

// NewOAuthRepo returns a repository.OAuthConnectionRepository backed by sqlc-generated queries.
func NewOAuthRepo(d *db.DB) repository.OAuthConnectionRepository {
	return &oauthRepo{d: d}
}

func (r *oauthRepo) List(ctx context.Context, workspaceID string) ([]repository.OAuthConnection, error) {
	rows, err := r.d.RQ.ListOAuthConnectionsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.OAuthConnection, len(rows))
	for i, row := range rows {
		out[i] = toOAuthConnection(row)
	}
	return out, nil
}

func (r *oauthRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.OAuthConnection, error) {
	row, err := r.d.RQ.GetOAuthConnectionByID(ctx, dbgen.GetOAuthConnectionByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.OAuthConnection{}, apperr.ErrNotFound
		}
		return repository.OAuthConnection{}, err
	}
	return toOAuthConnection(row), nil
}

func (r *oauthRepo) GetByProviderUserID(
	ctx context.Context,
	workspaceID, provider, providerUserID string,
) (repository.OAuthConnection, error) {
	row, err := r.d.RQ.GetOAuthConnectionByProviderUserID(ctx, dbgen.GetOAuthConnectionByProviderUserIDParams{
		WorkspaceID:    workspaceID,
		Provider:       provider,
		ProviderUserID: &providerUserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.OAuthConnection{}, apperr.ErrNotFound
		}
		return repository.OAuthConnection{}, err
	}
	return toOAuthConnection(row), nil
}

func (r *oauthRepo) Create(ctx context.Context, c repository.OAuthConnection) error {
	_, err := r.d.WQ.CreateOAuthConnection(ctx, dbgen.CreateOAuthConnectionParams{
		ID:             c.ID,
		WorkspaceID:    c.WorkspaceID,
		CreatedBy:      c.CreatedBy,
		Provider:       c.Provider,
		ProviderUserID: c.ProviderUserID,
		ProviderEmail:  c.ProviderEmail,
		Scopes:         c.Scopes,
		AccessToken:    c.AccessToken,
		RefreshToken:   c.RefreshToken,
		ExpiresAt:      c.ExpiresAt,
	})
	return err
}

func (r *oauthRepo) UpdateTokens(
	ctx context.Context,
	id, accessToken string,
	refreshToken *string,
	expiresAt *string,
) error {
	_, err := r.d.WQ.UpdateOAuthConnectionTokens(ctx, dbgen.UpdateOAuthConnectionTokensParams{
		ID:           id,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	})
	return err
}

func (r *oauthRepo) UpdateTokensAndScopes(
	ctx context.Context,
	id, accessToken string,
	refreshToken *string,
	expiresAt *string,
	scopes string,
) error {
	_, err := r.d.WQ.UpdateOAuthConnectionTokensAndScopes(ctx, dbgen.UpdateOAuthConnectionTokensAndScopesParams{
		ID:           id,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		Scopes:       scopes,
	})
	return err
}

func (r *oauthRepo) Delete(ctx context.Context, workspaceID, id string) error {
	return r.d.WQ.DeleteOAuthConnection(ctx, dbgen.DeleteOAuthConnectionParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func toOAuthConnection(c dbgen.OauthConnection) repository.OAuthConnection {
	return repository.OAuthConnection{
		ID:             c.ID,
		WorkspaceID:    c.WorkspaceID,
		CreatedBy:      c.CreatedBy,
		Provider:       c.Provider,
		ProviderUserID: c.ProviderUserID,
		ProviderEmail:  c.ProviderEmail,
		Scopes:         c.Scopes,
		AccessToken:    c.AccessToken,
		RefreshToken:   c.RefreshToken,
		ExpiresAt:      c.ExpiresAt,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}
