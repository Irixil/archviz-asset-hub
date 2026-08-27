package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/config"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/oauth2"
)

const (
	oidcStateCookie = "oidc_state"
	oidcPKCECookie  = "oidc_pkce"
	stateCookieTTL  = 10 * 60 // 10 minutes
)

// --- state / PKCE helpers ---

func generateRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func (s *Server) setStateCookie(c fiber.Ctx, state string) {
	c.Cookie(&fiber.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		MaxAge:   stateCookieTTL,
		HTTPOnly: true,
		SameSite: apiSameSiteLax,
		Path:     "/",
		Secure:   s.cfg.AppEnv != appEnvDevelopment,
	})
}

func (s *Server) setPKCECookie(c fiber.Ctx, verifier string) {
	c.Cookie(&fiber.Cookie{
		Name:     oidcPKCECookie,
		Value:    verifier,
		MaxAge:   stateCookieTTL,
		HTTPOnly: true,
		SameSite: apiSameSiteLax,
		Path:     "/",
		Secure:   s.cfg.AppEnv != appEnvDevelopment,
	})
}

func (s *Server) clearOIDCCookies(c fiber.Ctx) {
	for _, name := range []string{oidcStateCookie, oidcPKCECookie} {
		c.Cookie(&fiber.Cookie{Name: name, Value: "", MaxAge: -1, Path: "/"})
	}
}

// initiateOAuth starts an OAuth flow: sets state+PKCE cookies, returns the redirect URL.
func (s *Server) initiateOAuth(c fiber.Ctx, oauth2Cfg oauth2.Config, extraOpts ...oauth2.AuthCodeOption) error {
	state, err := generateRandom(nonceLength)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate state")
	}
	verifier, err := generateRandom(verifierLength)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate pkce verifier")
	}
	s.setStateCookie(c, state)
	s.setPKCECookie(c, verifier)

	opts := append([]oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", pkceChallenge(verifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}, extraOpts...)

	return c.Redirect().To(oauth2Cfg.AuthCodeURL(state, opts...))
}

// validateOAuthCallback validates state + exchanges code. Returns (token, error).
func (s *Server) validateOAuthCallback(c fiber.Ctx, oauth2Cfg oauth2.Config) (*oauth2.Token, error) {
	stateParam := c.Query("state")
	stateCookie := c.Cookies(oidcStateCookie)
	if stateParam == "" || stateCookie == "" || !hmac.Equal([]byte(stateParam), []byte(stateCookie)) {
		s.clearOIDCCookies(c)
		return nil, errors.New("state mismatch")
	}
	if errParam := c.Query(apiErrorKey); errParam != "" {
		s.clearOIDCCookies(c)
		return nil, fmt.Errorf("provider error: %s", errParam)
	}

	verifier := c.Cookies(oidcPKCECookie)
	s.clearOIDCCookies(c)

	token, err := oauth2Cfg.Exchange(c.Context(), c.Query("code"),
		oauth2.SetAuthURLParam("code_verifier", verifier),
	)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %w", err)
	}
	return token, nil
}

// --- OIDC handlers ---

// handleOIDCLogin initiates an OIDC login flow.
//
// @Summary Initiate OIDC login
// @Description Redirects the browser to the configured OIDC provider's authorization endpoint. Sets short-lived <code>oidc_state</code> and <code>oidc_pkce</code> cookies (10 min TTL) for CSRF and PKCE verification. Returns 503 if OIDC is not configured.
// @Tags Auth
// @Success 302 {string} string "Redirect to OIDC provider"
// @Failure 503 {object} ErrorResponse "OIDC not configured"
// @Router /auth/oidc/login [get]
// GET /auth/oidc/login.
func (s *Server) handleOIDCLogin(c fiber.Ctx) error {
	rt := config.GetOIDCRuntime()
	if rt == nil {
		return errRes(c, fiber.StatusServiceUnavailable, "OIDC not configured")
	}
	return s.initiateOAuth(c, rt.OAuth2Config)
}

// handleOIDCCallback handles the OIDC provider redirect.
//
// @Summary OIDC callback
// @Description Receives the authorization code from the OIDC provider. Validates state (CSRF) and PKCE, exchanges the code for tokens, verifies the ID token, upserts the user, issues a session cookie, and redirects to <code>/</code>. On any error redirects to <code>/login?error=...</code>.
// @Tags Auth
// @Param code query string true "Authorization code from provider"
// @Param state query string true "State parameter (CSRF token)"
// @Success 302 {string} string "Redirect to / on success, or /login?error=... on failure"
// @Router /auth/oidc/callback [get]
// GET /auth/oidc/callback.
func (s *Server) handleOIDCCallback(c fiber.Ctx) error {
	rt := config.GetOIDCRuntime()
	if rt == nil {
		return c.Redirect().To("/login?error=oidc_error")
	}

	return s.handleOIDCProviderCallback(c, rt, false)
}

type oidcUserClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (s *Server) handleOIDCProviderCallback(c fiber.Ctx, rt *config.OIDCRuntime, isGoogle bool) error {
	token, err := s.validateOAuthCallback(c, rt.OAuth2Config)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return c.Redirect().To("/login?error=oidc_exchange")
	}
	idToken, err := rt.Verifier.Verify(c.Context(), rawIDToken)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}

	var claims oidcUserClaims
	if err = idToken.Claims(&claims); err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}
	if !claims.EmailVerified {
		return c.Redirect().To("/login?error=email_not_verified")
	}

	dto, err := s.users.UpsertOIDCUser(c.Context(), service.UpsertOIDCUserParams{
		Issuer:    idToken.Issuer,
		Sub:       claims.Sub,
		Email:     claims.Email,
		Name:      claims.Name,
		AvatarURL: claims.Picture,
		IsGoogle:  isGoogle,
	})
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}

	jwtToken, err := s.auth.CreateToken(dto.ID, dto.WorkspaceID, sessionDuration)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange")
	}
	s.setAuthCookie(c, jwtToken)
	return c.Redirect().To("/")
}

// --- Google handlers ---

// handleGoogleLogin initiates a Google OAuth2 login flow.
//
// @Summary Initiate Google login
// @Description Redirects the browser to Google's OAuth2 authorization endpoint with <code>prompt=select_account</code>. Sets short-lived <code>oidc_state</code> and <code>oidc_pkce</code> cookies (10 min TTL). Returns 503 if Google OAuth is not configured.
// @Tags Auth
// @Success 302 {string} string "Redirect to Google OAuth2"
// @Failure 503 {object} ErrorResponse "Google login not configured"
// @Router /auth/google/login [get]
// GET /auth/google/login.
func (s *Server) handleGoogleLogin(c fiber.Ctx) error {
	rt := config.GetGoogleRuntime()
	if rt == nil {
		return errRes(c, fiber.StatusServiceUnavailable, "Google login not configured")
	}
	return s.initiateOAuth(c, rt.OAuth2Config,
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
}

// handleGoogleCallback handles the Google OAuth2 provider redirect.
//
// @Summary Google OAuth2 callback
// @Description Receives the authorization code from Google. Validates state and PKCE, exchanges the code, verifies the ID token, upserts the user (marked as Google-linked), issues a session cookie, and redirects to <code>/</code>. On any error redirects to <code>/login?error=...</code>.
// @Tags Auth
// @Param code query string true "Authorization code from Google"
// @Param state query string true "State parameter (CSRF token)"
// @Success 302 {string} string "Redirect to / on success, or /login?error=... on failure"
// @Router /auth/google/callback [get]
// GET /auth/google/callback.
func (s *Server) handleGoogleCallback(c fiber.Ctx) error {
	rt := config.GetGoogleRuntime()
	if rt == nil {
		return c.Redirect().To("/login?error=oidc_error")
	}

	return s.handleOIDCProviderCallback(c, rt, true)
}

// --- Canva handlers ---

const canvaAuthURL = "https://www.canva.com/api/oauth/authorize"
const canvaTokenURL = "https://api.canva.com/rest/v1/oauth/token" //nolint:gosec // canva api endpoints
const canvaMeURL = "https://api.canva.com/rest/v1/users/me"

func (s *Server) canvaOAuth2Config() oauth2.Config {
	return oauth2.Config{
		ClientID:     s.cfg.Canva.ClientID,
		ClientSecret: s.cfg.Canva.ClientSecret,
		RedirectURL:  s.cfg.BaseURL.String() + "/auth/canva/callback",
		Endpoint: oauth2.Endpoint{
			AuthURL:  canvaAuthURL,
			TokenURL: canvaTokenURL,
		},
		Scopes: []string{"profile:read"},
	}
}

// handleCanvaLogin initiates a Canva OAuth2 login flow.
//
// @Summary Initiate Canva login
// @Description Redirects the browser to Canva's OAuth2 authorization endpoint requesting the <code>profile:read</code> scope. Sets short-lived <code>oidc_state</code> and <code>oidc_pkce</code> cookies (10 min TTL). Returns 503 if Canva OAuth is not configured.
// @Tags Auth
// @Success 302 {string} string "Redirect to Canva OAuth2"
// @Failure 503 {object} ErrorResponse "Canva login not configured"
// @Router /auth/canva/login [get]
// GET /auth/canva/login.
func (s *Server) handleCanvaLogin(c fiber.Ctx) error {
	if s.cfg.Canva.ClientID == "" {
		return errRes(c, fiber.StatusServiceUnavailable, "Canva login not configured")
	}
	return s.initiateOAuth(c, s.canvaOAuth2Config())
}

// handleCanvaCallback handles the Canva OAuth2 provider redirect.
//
// @Summary Canva OAuth2 callback
// @Description Receives the authorization code from Canva. Validates state and PKCE, exchanges the code, fetches the user profile from the Canva API (<code>/rest/v1/users/me</code>), upserts the user, issues a session cookie, and redirects to <code>/</code>. On any error redirects to <code>/login?error=oidc_exchange&error_step=...</code>.
// @Tags Auth
// @Param code query string true "Authorization code from Canva"
// @Param state query string true "State parameter (CSRF token)"
// @Success 302 {string} string "Redirect to / on success, or /login?error=... on failure"
// @Router /auth/canva/callback [get]
// GET /auth/canva/callback.
func (s *Server) handleCanvaCallback(c fiber.Ctx) error {
	if s.cfg.Canva.ClientID == "" {
		return c.Redirect().To("/login?error=oidc_error")
	}

	token, err := s.validateOAuthCallback(c, s.canvaOAuth2Config())
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=validate_token")
	}

	// Fetch user profile from Canva.
	meCtx, meCancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer meCancel()
	req, _ := http.NewRequestWithContext(meCtx, http.MethodGet, canvaMeURL, nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=user_profile")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=user_profile_response")
	}

	var me struct {
		Profile struct {
			UserID      string `json:"user_id"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
		} `json:"profile"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=user_profile_decode")
	}

	dto, err := s.users.UpsertCanvaUser(c.Context(), service.UpsertCanvaUserParams{
		CanvaID:   me.Profile.UserID,
		Email:     me.Profile.Email,
		Name:      me.Profile.DisplayName,
		AvatarURL: "",
	})
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=upsert_user")
	}

	jwtToken, err := s.auth.CreateToken(dto.ID, dto.WorkspaceID, sessionDuration)
	if err != nil {
		return c.Redirect().To("/login?error=oidc_exchange&error_step=create_token")
	}
	s.setAuthCookie(c, jwtToken)
	return c.Redirect().To("/")
}

// --- me handler ---

type MeResponse struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	DisplayName      string  `json:"display_name"`
	Email            string  `json:"email"`
	AvatarURL        *string `json:"avatar_url,omitempty"`
	AvatarStorageKey *string `json:"-"`
	HasPassword      bool    `json:"has_password"`
	AuthMethods      string  `json:"auth_methods"`
	OIDCLinked       bool    `json:"oidc_linked"`
	GoogleLinked     bool    `json:"google_linked"`
	CanvaLinked      bool    `json:"canva_linked"`
	PendingEmail     *string `json:"pending_email,omitempty"`
}

// handleGetMe returns the authenticated user's profile.
//
// @Summary Get current user profile
// @Description Returns the profile of the currently authenticated user including their linked OAuth providers (OIDC, Google, Canva).
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} MeResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 500 {object} ErrorResponse "Could not load user"
// @Router /auth/me [get]
// GET /auth/me.
func (s *Server) handleGetMe(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	dto, err := s.users.GetProfile(c.Context(), claims.UserID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load user")
	}
	return c.JSON(meResponseFromDTO(s.cfg.BaseURL.String(), dto))
}

// --- unlink handlers ---

// handleUnlinkOIDC unlinks the OIDC provider from the current user's account.
//
// @Summary Unlink OIDC provider
// @Description Removes the OIDC login method from the current user's account. Fails with 422 if OIDC is the user's only auth method (would lock them out).
// @Tags Auth
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ErrorResponse "Cannot unlink — only auth method remaining"
// @Failure 500 {object} ErrorResponse "Could not unlink"
// @Router /auth/oidc/link [delete]
// DELETE /auth/oidc/link.
func (s *Server) handleUnlinkOIDC(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	dto, err := s.users.UnlinkProvider(c.Context(), claims.UserID, "oidc")
	if err != nil {
		return oidcUnlinkErr(c, err)
	}
	_ = dto
	return c.SendStatus(fiber.StatusNoContent)
}

// handleUnlinkGoogle unlinks the Google provider from the current user's account.
//
// @Summary Unlink Google provider
// @Description Removes the Google login method from the current user's account. Fails with 422 if Google is the user's only auth method (would lock them out).
// @Tags Auth
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ErrorResponse "Cannot unlink — only auth method remaining"
// @Failure 500 {object} ErrorResponse "Could not unlink"
// @Router /auth/google/link [delete]
// DELETE /auth/google/link.
func (s *Server) handleUnlinkGoogle(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	dto, err := s.users.UnlinkProvider(c.Context(), claims.UserID, "google")
	if err != nil {
		return oidcUnlinkErr(c, err)
	}
	_ = dto
	return c.SendStatus(fiber.StatusNoContent)
}

// handleUnlinkCanva unlinks the Canva provider from the current user's account.
//
// @Summary Unlink Canva provider
// @Description Removes the Canva login method from the current user's account. Fails with 422 if Canva is the user's only auth method (would lock them out).
// @Tags Auth
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ErrorResponse "Cannot unlink — only auth method remaining"
// @Failure 500 {object} ErrorResponse "Could not unlink"
// @Router /auth/canva/link [delete]
// DELETE /auth/canva/link.
func (s *Server) handleUnlinkCanva(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	dto, err := s.users.UnlinkProvider(c.Context(), claims.UserID, "canva")
	if err != nil {
		return oidcUnlinkErr(c, err)
	}
	_ = dto
	return c.SendStatus(fiber.StatusNoContent)
}

func oidcUnlinkErr(c fiber.Ctx, err error) error {
	if isInvalidInput(err) {
		return errRes(c, fiber.StatusUnprocessableEntity, err.Error())
	}
	return errRes(c, fiber.StatusInternalServerError, "could not unlink")
}

// --- signed state helpers for /integrations OAuth ---

type oauthState struct {
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	Nonce       string `json:"nonce"`
	RedirectTo  string `json:"redirect_to,omitempty"`
}

func signState(payload oauthState, secret string) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(enc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig, nil
}

func verifyState(raw, secret string) (oauthState, error) {
	parts := strings.SplitN(raw, ".", 2)
	//nolint:mnd // fixed number of parts.
	if len(parts) != 2 {
		return oauthState{}, errors.New("invalid state format")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return oauthState{}, errors.New("state signature invalid")
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oauthState{}, err
	}
	var s oauthState
	if err = json.Unmarshal(b, &s); err != nil {
		return oauthState{}, err
	}
	return s, nil
}
