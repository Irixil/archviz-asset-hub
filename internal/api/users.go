package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

const (
	maxAvatarBytes        = 5 << 20
	forgotPasswordLimit   = 5
	forgotPasswordWindow  = 15 * time.Minute
	passwordResetTokenTTL = time.Hour
	emailChangeTokenTTL   = 24 * time.Hour
)

var forgotPasswordLimiter = newEmailRateLimiter(forgotPasswordLimit, forgotPasswordWindow)

type emailRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string][]time.Time
}

func newEmailRateLimiter(limit int, window time.Duration) *emailRateLimiter {
	return &emailRateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string][]time.Time),
	}
}

func (l *emailRateLimiter) Allow(email string) bool {
	keyBytes := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	key := hex.EncodeToString(keyBytes[:])
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	current := l.entries[key]
	kept := current[:0]
	for _, ts := range current {
		if now.Sub(ts) < l.window {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= l.limit {
		l.entries[key] = append([]time.Time(nil), kept...)
		return false
	}
	kept = append(kept, now)
	l.entries[key] = append([]time.Time(nil), kept...)
	return true
}

func resolveAvatarURL(baseURL string, dto *MeResponse) *string {
	if dto.AvatarStorageKey != nil && *dto.AvatarStorageKey != "" {
		resolved := strings.TrimRight(baseURL, "/") + "/api/v1/users/" + dto.ID + "/avatar"
		return &resolved
	}
	if dto.AvatarURL != nil && *dto.AvatarURL != "" {
		return dto.AvatarURL
	}
	return nil
}

func meResponseFromDTO(baseURL string, dto *service.OIDCUserDTO) MeResponse {
	resp := MeResponse{
		ID:               dto.ID,
		Name:             dto.Name,
		DisplayName:      dto.DisplayName,
		Email:            dto.Email,
		AvatarURL:        dto.AvatarURL,
		AvatarStorageKey: dto.AvatarStorageKey,
		HasPassword:      strings.Contains(dto.AuthMethods, "password"),
		AuthMethods:      dto.AuthMethods,
		OIDCLinked:       dto.OIDCLinked,
		GoogleLinked:     dto.GoogleLinked,
		CanvaLinked:      dto.CanvaLinked,
		PendingEmail:     dto.PendingEmail,
	}
	resp.AvatarURL = resolveAvatarURL(baseURL, &resp)
	return resp
}

func (s *Server) handleUpdateMe(c fiber.Ctx) error {
	req, ok := decodeAndValidate(c, &UpdateMeRequest{})
	if !ok {
		return nil
	}
	claims := auth.GetClaims(c)
	dto, err := s.users.UpdateProfile(c.Context(), claims.UserID, req.DisplayName)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(meResponseFromDTO(s.cfg.BaseURL.String(), dto))
}

func (s *Server) handleUploadAvatar(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	fh, err := c.FormFile("avatar")
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "avatar field is required")
	}
	if fh.Size > maxAvatarBytes {
		return errRes(c, fiber.StatusRequestEntityTooLarge, "avatar_too_large")
	}
	file, err := fh.Open()
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not open uploaded file")
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxAvatarBytes+1))
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not read uploaded file")
	}
	if int64(len(data)) > maxAvatarBytes {
		return errRes(c, fiber.StatusRequestEntityTooLarge, "avatar_too_large")
	}
	dto, err := s.users.UploadAvatar(c.Context(), claims.UserID, data)
	if err != nil {
		if errors.Is(err, service.ErrUnsupportedAvatarType) {
			return errRes(c, fiber.StatusUnsupportedMediaType, "unsupported_avatar_type")
		}
		if errors.Is(err, service.ErrAvatarStorage) {
			return errRes(c, fiber.StatusInternalServerError, "could not store avatar")
		}
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(meResponseFromDTO(s.cfg.BaseURL.String(), dto))
}

func (s *Server) handleDeleteAvatar(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if err := s.users.DeleteAvatar(c.Context(), claims.UserID); err != nil {
		if errors.Is(err, service.ErrAvatarStorage) {
			return errRes(c, fiber.StatusInternalServerError, "could not delete avatar")
		}
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleGetAvatar(c fiber.Ctx) error {
	dto, err := s.users.GetProfile(c.Context(), c.Params("id"))
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "avatar not found")
	}
	if dto.AvatarStorageKey != nil && *dto.AvatarStorageKey != "" {
		var rc io.ReadCloser
		rc, err = s.storage.Get(c.Context(), *dto.AvatarStorageKey)
		if err != nil {
			return errRes(c, fiber.StatusNotFound, "avatar not found")
		}
		c.Set("Content-Type", "image/webp")
		c.Set("Cache-Control", "public, max-age=86400")
		return c.SendStream(rc)
	}
	if dto.AvatarURL != nil && *dto.AvatarURL != "" {
		return c.Redirect().To(*dto.AvatarURL)
	}
	return errRes(c, fiber.StatusNotFound, "avatar not found")
}

func (s *Server) handleForgotPassword(c fiber.Ctx) error {
	req, ok := decodeAndValidate(c, &ForgotPasswordRequest{})
	if !ok {
		return nil
	}
	if !forgotPasswordLimiter.Allow(req.Email) {
		return c.SendStatus(fiber.StatusNoContent)
	}
	dto, err := s.users.GetProfileByEmail(c.Context(), req.Email)
	if err == nil && strings.Contains(dto.AuthMethods, "password") {
		token, signErr := auth.SignActionToken([]byte(s.cfg.AppSecret), auth.ActionTokenClaims{
			Sub:     dto.ID,
			Purpose: auth.PurposePasswordReset,
			Email:   dto.Email,
		}, passwordResetTokenTTL)
		if signErr == nil {
			_ = s.mailer.SendPasswordReset(c.Context(), dto.Email, token)
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleResetPassword(c fiber.Ctx) error {
	req, ok := decodeAndValidate(c, &ResetPasswordRequest{})
	if !ok {
		return nil
	}
	claims, err := auth.VerifyActionToken([]byte(s.cfg.AppSecret), req.Token)
	if err != nil {
		if errors.Is(err, auth.ErrTokenExpired) || errors.Is(err, auth.ErrTokenInvalid) {
			return errRes(c, fiber.StatusBadRequest, "invalid_or_expired_token")
		}
		return errRes(c, fiber.StatusBadRequest, "invalid_or_expired_token")
	}
	if claims.Purpose != auth.PurposePasswordReset {
		return errRes(c, fiber.StatusBadRequest, "invalid_or_expired_token")
	}
	hash, err := s.bcryptHash(req.Password)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not hash password")
	}
	if err = s.users.ResetPassword(c.Context(), claims.Sub, hash); err != nil {
		if errors.Is(err, apperr.ErrInvalidInput) {
			return errRes(c, fiber.StatusBadRequest, "invalid_or_expired_token")
		}
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleChangePassword(c fiber.Ctx) error {
	req, ok := decodeAndValidate(c, &ChangePasswordRequest{})
	if !ok {
		return nil
	}
	hash, err := s.bcryptHash(req.NewPassword)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not hash password")
	}
	claims := auth.GetClaims(c)
	if err = s.users.ChangePassword(c.Context(), claims.UserID, req.CurrentPassword, hash); err != nil {
		if errors.Is(err, apperr.ErrInvalidInput) {
			return errRes(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleRequestEmailChange(c fiber.Ctx) error {
	req, ok := decodeAndValidate(c, &RequestEmailChangeRequest{})
	if !ok {
		return nil
	}
	claims := auth.GetClaims(c)
	dto, err := s.users.GetProfile(c.Context(), claims.UserID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if err = s.users.RequestEmailChange(c.Context(), claims.UserID, req.Email); err != nil {
		return ErrorStatusResponse(c, err)
	}
	token, err := auth.SignActionToken([]byte(s.cfg.AppSecret), auth.ActionTokenClaims{
		Sub:     claims.UserID,
		Purpose: auth.PurposeEmailChange,
		Email:   req.Email,
	}, emailChangeTokenTTL)
	if err == nil {
		_ = s.mailer.SendEmailChangeConfirmation(c.Context(), dto.Email, req.Email, token)
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"pending_email": req.Email})
}

func (s *Server) handleCancelEmailChange(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if err := s.users.CancelEmailChange(c.Context(), claims.UserID); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleConfirmEmailChange(c fiber.Ctx) error {
	token := c.Query("token")
	claims, err := auth.VerifyActionToken([]byte(s.cfg.AppSecret), token)
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid_or_expired_token")
	}
	if claims.Purpose != auth.PurposeEmailChange {
		return errRes(c, fiber.StatusBadRequest, "invalid_or_expired_token")
	}
	if _, err = s.users.ConfirmEmailChange(c.Context(), claims.Sub, claims.Email); err != nil {
		if errors.Is(err, apperr.ErrInvalidInput) {
			return errRes(c, fiber.StatusBadRequest, "invalid_or_expired_token")
		}
		return ErrorStatusResponse(c, err)
	}
	return c.Redirect().To("/library/settings/account?email_changed=1")
}

func (s *Server) handleDeleteMe(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	dto, err := s.users.GetProfile(c.Context(), claims.UserID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	req := &DeleteMeRequest{}
	if strings.Contains(dto.AuthMethods, "password") {
		if err = c.Bind().Body(req); err != nil {
			return errRes(c, fiber.StatusBadRequest, "invalid request body")
		}
	}
	avatarKey := dto.AvatarStorageKey
	hardDelete := strings.EqualFold(strings.TrimSpace(os.Getenv("USER_HARD_DELETE")), "true")
	if err = s.users.DeleteAccount(c.Context(), claims.UserID, req.Password, hardDelete); err != nil {
		return ErrorStatusResponse(c, err)
	}
	if avatarKey != nil && *avatarKey != "" {
		_ = s.storage.Delete(c.Context(), *avatarKey)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
