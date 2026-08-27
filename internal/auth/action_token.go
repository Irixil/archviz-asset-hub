package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type ActionTokenPurpose string

const (
	PurposePasswordReset ActionTokenPurpose = "password_reset"
	PurposeEmailChange   ActionTokenPurpose = "email_change"
)

type ActionTokenClaims struct {
	Sub     string             `json:"sub"`
	Purpose ActionTokenPurpose `json:"purpose"`
	Email   string             `json:"email"`
	Exp     int64              `json:"exp"`
}

var ErrTokenExpired = errors.New("token expired")
var ErrTokenInvalid = errors.New("token invalid")

func SignActionToken(secret []byte, claims ActionTokenClaims, ttl time.Duration) (string, error) {
	if ttl > 0 {
		claims.Exp = time.Now().Add(ttl).Unix()
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(encodedPayload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedPayload + "." + signature, nil
}

func VerifyActionToken(secret []byte, token string) (ActionTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 { //nolint:mnd // expect a format like "payload.signature".
		return ActionTokenClaims{}, ErrTokenInvalid
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(actual, expected) {
		return ActionTokenClaims{}, ErrTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ActionTokenClaims{}, ErrTokenInvalid
	}
	var claims ActionTokenClaims
	if err = json.Unmarshal(payload, &claims); err != nil {
		return ActionTokenClaims{}, ErrTokenInvalid
	}
	if time.Now().Unix() > claims.Exp {
		return ActionTokenClaims{}, ErrTokenExpired
	}
	return claims, nil
}
