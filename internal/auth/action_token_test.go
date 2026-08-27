package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSignAndVerify_RoundTrip(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret-key-must-be-32chars!!")
	token, err := SignActionToken(secret, ActionTokenClaims{
		Sub:     "usr_1",
		Purpose: PurposePasswordReset,
		Email:   "alice@example.com",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	claims, err := VerifyActionToken(secret, token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Sub != "usr_1" || claims.Email != "alice@example.com" || claims.Purpose != PurposePasswordReset {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret-key-must-be-32chars!!")
	token, err := SignActionToken(secret, ActionTokenClaims{
		Sub:     "usr_1",
		Purpose: PurposePasswordReset,
		Email:   "alice@example.com",
	}, -time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, e := VerifyActionToken(secret, token); !errors.Is(e, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", e)
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret-key-must-be-32chars!!")
	token, err := SignActionToken(secret, ActionTokenClaims{
		Sub:     "usr_1",
		Purpose: PurposePasswordReset,
		Email:   "alice@example.com",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, e := VerifyActionToken(
		[]byte("another-secret-key-must-be-32chars"),
		token,
	); !errors.Is(
		e,
		ErrTokenInvalid,
	) {
		t.Fatalf("expected ErrTokenInvalid, got %v", e)
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret-key-must-be-32chars!!")
	token, err := SignActionToken(secret, ActionTokenClaims{
		Sub:     "usr_1",
		Purpose: PurposePasswordReset,
		Email:   "alice@example.com",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	tampered := "ZmFrZQ." + token[strings.Index(token, ".")+1:]
	if _, e := VerifyActionToken(secret, tampered); !errors.Is(e, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestVerify_PurposeMismatch(t *testing.T) {
	t.Parallel()
	secret := []byte("test-secret-key-must-be-32chars!!")
	token, err := SignActionToken(secret, ActionTokenClaims{
		Sub:     "usr_1",
		Purpose: PurposeEmailChange,
		Email:   "alice@example.com",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := VerifyActionToken(secret, token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Purpose == PurposePasswordReset {
		t.Fatal("expected purpose mismatch")
	}
}
