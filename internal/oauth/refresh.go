package oauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	dbgen "damask/server/internal/db/gen"

	"golang.org/x/oauth2"
)

// ProviderTokenEndpoint maps provider names to their token endpoint URLs.
// Used when we need to refresh without a full oauth2.Config.
var ProviderTokenEndpoint = map[string]string{
	"google": "https://oauth2.googleapis.com/token",
	"canva":  "https://api.canva.com/rest/v1/oauth/token",
}

// TokenRefresher retrieves valid access tokens for oauth_connections,
// refreshing when tokens are near expiry. Thread-safe per connection.
type TokenRefresher struct {
	queries   *dbgen.Queries
	appSecret string

	locks sync.Map // map[connID string]*sync.Mutex, one entry per connection, never evicted

	mu      sync.Mutex
	configs map[string]*oauth2.Config // per-provider oauth2.Config
}

func NewTokenRefresher(queries *dbgen.Queries, appSecret string) *TokenRefresher {
	return &TokenRefresher{
		queries:   queries,
		appSecret: appSecret,
		configs:   make(map[string]*oauth2.Config),
	}
}

// RegisterProvider registers an oauth2.Config for a provider (e.g. "google", "canva").
func (r *TokenRefresher) RegisterProvider(provider string, cfg *oauth2.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[provider] = cfg
}

func (r *TokenRefresher) lockFor(connID string) *sync.Mutex {
	v, _ := r.locks.LoadOrStore(connID, &sync.Mutex{})
	mu, ok := v.(*sync.Mutex)
	if !ok {
		return nil
	}
	return mu
}

// EnsureFreshToken returns a valid decrypted access token for the connection,
// refreshing when expires_at < now+5min. Updates the DB row on refresh.
func (r *TokenRefresher) EnsureFreshToken(ctx context.Context, workspaceID, connID string) (string, error) {
	mu := r.lockFor(connID)
	if mu == nil {
		return "", errors.New("oauth/refresh: connection lock has unexpected type")
	}
	mu.Lock()
	defer mu.Unlock()

	conn, err := r.queries.GetOAuthConnectionByID(ctx, dbgen.GetOAuthConnectionByIDParams{
		ID:          connID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		slog.ErrorContext(
			ctx,
			"oauth/refresh: cannot get connection",
			"connID",
			connID,
			"workspaceID",
			workspaceID,
			"error",
			err,
		)
		return "", fmt.Errorf("oauth/refresh: get connection: %w", err)
	}

	accessToken, err := DecryptToken(r.appSecret, conn.AccessToken)
	if err != nil {
		return "", fmt.Errorf("oauth/refresh: decrypt access token: %w", err)
	}

	// Check if still fresh (more than 5 minutes remaining).
	if conn.ExpiresAt != nil && *conn.ExpiresAt != "" {
		exp, parseErr := time.Parse(time.RFC3339, *conn.ExpiresAt)
		if parseErr == nil && time.Until(exp) > 5*time.Minute {
			return accessToken, nil
		}
	} else {
		// No expiry recorded — assume token is long-lived regardless of refresh token presence.
		return accessToken, nil
	}

	// Need to refresh.
	if conn.RefreshToken == nil {
		return "", errors.New("oauth/refresh: token expired and no refresh token available")
	}

	refreshToken, err := DecryptToken(r.appSecret, *conn.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("oauth/refresh: decrypt refresh token: %w", err)
	}

	r.mu.Lock()
	cfg, ok := r.configs[conn.Provider]
	r.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("oauth/refresh: no oauth2 config for provider %q", conn.Provider)
	}

	ts := cfg.TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	})
	newToken, err := ts.Token()
	if err != nil {
		return "", fmt.Errorf("oauth/refresh: token refresh failed: %w", err)
	}

	// Persist new tokens.
	encAccess, err := EncryptToken(r.appSecret, newToken.AccessToken)
	if err != nil {
		return "", fmt.Errorf("oauth/refresh: encrypt new access token: %w", err)
	}
	var encRefresh *string
	if newToken.RefreshToken != "" {
		enc, encErr := EncryptToken(r.appSecret, newToken.RefreshToken)
		if encErr != nil {
			return "", fmt.Errorf("oauth/refresh: encrypt new refresh token: %w", encErr)
		}
		encRefresh = &enc
	}
	var expiresAt *string
	if !newToken.Expiry.IsZero() {
		s := newToken.Expiry.UTC().Format(time.RFC3339)
		expiresAt = &s
	}

	_, err = r.queries.UpdateOAuthConnectionTokens(ctx, dbgen.UpdateOAuthConnectionTokensParams{
		AccessToken:  encAccess,
		RefreshToken: encRefresh,
		ExpiresAt:    expiresAt,
		ID:           connID,
	})
	if err != nil {
		return "", fmt.Errorf("oauth/refresh: persist tokens: %w", err)
	}

	return newToken.AccessToken, nil
}
