// Package auth implements token creation/validation using HMAC-SHA256 JWTs
// (same security model as Paseto v4 local for a single-key setup).
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const appSecretLength = 32

// Claims is the payload stored in every workspace auth token.
type Claims struct {
	jwt.RegisteredClaims

	UserID      string `json:"user_id"`
	WorkspaceID string `json:"workspace_id"`
	IsDemo      bool   `json:"is_demo,omitempty"`
}

// ShareClaims is the payload stored in a short-lived share session token.
// It is issued by POST /shared/:id/access and used only on /shared/ public routes.
// It must never grant workspace-level access.
type ShareClaims struct {
	jwt.RegisteredClaims

	ShareID       string `json:"share_id"`
	TargetType    string `json:"target_type"`
	TargetID      string `json:"target_id"`
	AllowComments bool   `json:"allow_comments"`
	AllowDownload bool   `json:"allow_download"`
	VisitorName   string `json:"visitor_name"`
}

// Maker creates and verifies tokens.
type Maker struct {
	secret []byte
}

// NewMaker returns a Maker using the provided secret key.
func NewMaker(secret string) (*Maker, error) {
	if len(secret) < appSecretLength {
		return nil, errors.New("JWT_SECRET must be at least 32 characters")
	}
	return &Maker{secret: []byte(secret)}, nil
}

// CreateToken issues a signed token valid for the given duration.
func (m *Maker) CreateToken(userID, workspaceID string, duration time.Duration) (string, error) {
	claims := &Claims{
		UserID:      userID,
		WorkspaceID: workspaceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// CreateDemoToken issues a signed token for the demo user. The token carries
// is_demo=true so middleware can apply demo-specific restrictions.
func (m *Maker) CreateDemoToken(userID, workspaceID string, duration time.Duration) (string, error) {
	claims := &Claims{
		UserID:      userID,
		WorkspaceID: workspaceID,
		IsDemo:      true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// CreateShareToken issues a signed share session token valid for the given duration.
func (m *Maker) CreateShareToken(
	shareID, targetType, targetID string,
	allowComments, allowDownload bool,
	visitorName string,
	duration time.Duration,
) (string, error) {
	claims := &ShareClaims{
		ShareID:       shareID,
		TargetType:    targetType,
		TargetID:      targetID,
		AllowComments: allowComments,
		AllowDownload: allowDownload,
		VisitorName:   visitorName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// VerifyShareToken parses and validates a share session token, returning its claims.
func (m *Maker) VerifyShareToken(tokenStr string) (*ShareClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &ShareClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*ShareClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid share token")
	}
	return claims, nil
}

// VerifyToken parses and validates a token string, returning its claims.
func (m *Maker) VerifyToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
