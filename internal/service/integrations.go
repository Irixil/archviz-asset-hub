package service

import (
	"context"
	"encoding/json"

	"damask/server/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// ConnectionDTO is the public shape of an OAuth connection (no tokens).
type ConnectionDTO struct {
	ID            string
	Provider      string
	ProviderEmail string
	Scopes        []string
	ConnectedAt   string
}

// UpsertConnectionParams is the input for IntegrationService.UpsertConnection.
type UpsertConnectionParams struct {
	WorkspaceID    string
	UserID         string
	Provider       string
	ProviderUserID string
	ProviderEmail  string
	Token          *oauth2.Token
	Scopes         []string
	EncryptToken   func(plain string) (string, error)
}

type integrationService struct {
	oauth repository.OAuthConnectionRepository
}

// NewIntegrationService returns an IntegrationService.
func NewIntegrationService(oauth repository.OAuthConnectionRepository) IntegrationService {
	return &integrationService{oauth: oauth}
}

func (s *integrationService) ListConnections(ctx context.Context, workspaceID string) ([]*ConnectionDTO, error) {
	rows, err := s.oauth.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toConnectionDTO)
	return out, nil
}

func (s *integrationService) DeleteConnection(ctx context.Context, workspaceID, id string) error {
	if _, err := s.oauth.GetByID(ctx, workspaceID, id); err != nil {
		return err
	}
	return s.oauth.Delete(ctx, workspaceID, id)
}

func (s *integrationService) UpsertConnection(ctx context.Context, p UpsertConnectionParams) error {
	encAccess, err := p.EncryptToken(p.Token.AccessToken)
	if err != nil {
		return err
	}
	var encRefresh *string
	if p.Token.RefreshToken != "" {
		enc, encErr := p.EncryptToken(p.Token.RefreshToken)
		if encErr != nil {
			return encErr
		}
		encRefresh = &enc
	}
	var expiresAt *string
	if !p.Token.Expiry.IsZero() {
		s := p.Token.Expiry.UTC().Format("2006-01-02T15:04:05Z")
		expiresAt = &s
	}
	scopesJSON, _ := json.Marshal(p.Scopes)

	existing, err := s.oauth.GetByProviderUserID(ctx, p.WorkspaceID, p.Provider, p.ProviderUserID)
	if err == nil {
		return s.oauth.UpdateTokensAndScopes(ctx, existing.ID, encAccess, encRefresh, expiresAt, string(scopesJSON))
	}

	providerUserID := p.ProviderUserID
	var providerEmail *string
	if p.ProviderEmail != "" {
		providerEmail = &p.ProviderEmail
	}
	return s.oauth.Create(ctx, repository.OAuthConnection{
		ID:             uuid.NewString(),
		WorkspaceID:    p.WorkspaceID,
		CreatedBy:      p.UserID,
		Provider:       p.Provider,
		ProviderUserID: &providerUserID,
		ProviderEmail:  providerEmail,
		Scopes:         string(scopesJSON),
		AccessToken:    encAccess,
		RefreshToken:   encRefresh,
		ExpiresAt:      expiresAt,
	})
}

func toConnectionDTO(c repository.OAuthConnection) *ConnectionDTO {
	var scopes []string
	_ = json.Unmarshal([]byte(c.Scopes), &scopes)
	email := ""
	if c.ProviderEmail != nil {
		email = *c.ProviderEmail
	}
	return &ConnectionDTO{
		ID:            c.ID,
		Provider:      c.Provider,
		ProviderEmail: email,
		Scopes:        scopes,
		ConnectedAt:   c.CreatedAt,
	}
}
