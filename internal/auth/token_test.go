package auth

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "test-secret-key-must-be-32chars!!"

func newTestMaker(t *testing.T) *Maker {
	t.Helper()
	m, err := NewMaker(testSecret)
	if err != nil {
		t.Fatalf("NewMaker: %v", err)
	}
	return m
}

func TestMaker_SecretTooShort(t *testing.T) {
	t.Parallel()
	_, err := NewMaker("tooshort")
	if err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestMaker_CreateAndVerify_RoundTrip(t *testing.T) {
	t.Parallel()
	m := newTestMaker(t)

	tok, err := m.CreateToken("usr_1", "ws_1", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	claims, err := m.VerifyToken(tok)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if claims.UserID != "usr_1" {
		t.Errorf("UserID: got %q, want %q", claims.UserID, "usr_1")
	}
	if claims.WorkspaceID != "ws_1" {
		t.Errorf("WorkspaceID: got %q, want %q", claims.WorkspaceID, "ws_1")
	}
	if claims.IsDemo {
		t.Error("IsDemo should be false for regular token")
	}
}

func TestMaker_CreateDemo_IsDemo(t *testing.T) {
	t.Parallel()
	m := newTestMaker(t)

	tok, err := m.CreateDemoToken("usr_demo", "ws_demo", time.Hour)
	if err != nil {
		t.Fatalf("CreateDemoToken: %v", err)
	}

	claims, err := m.VerifyToken(tok)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if !claims.IsDemo {
		t.Error("expected IsDemo=true for demo token")
	}
	if claims.UserID != "usr_demo" || claims.WorkspaceID != "ws_demo" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestMaker_VerifyToken_Expired(t *testing.T) {
	t.Parallel()
	m := newTestMaker(t)

	tok, err := m.CreateToken("usr_1", "ws_1", -time.Minute)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if _, vErr := m.VerifyToken(tok); vErr == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestMaker_VerifyToken_WrongSecret(t *testing.T) {
	t.Parallel()
	m := newTestMaker(t)

	tok, err := m.CreateToken("usr_1", "ws_1", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	other, err := NewMaker("another-secret-key-must-be-32chars!!")
	if err != nil {
		t.Fatalf("NewMaker: %v", err)
	}
	if _, vErr := other.VerifyToken(tok); vErr == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestMaker_VerifyToken_Tampered(t *testing.T) {
	t.Parallel()
	m := newTestMaker(t)

	tok, err := m.CreateToken("usr_1", "ws_1", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	// Replace the payload segment (second dot-delimited part).
	parts := strings.SplitN(tok, ".", 3)
	if len(parts) != 3 {
		t.Fatal("unexpected JWT format")
	}
	tampered := parts[0] + ".ZmFrZXBheWxvYWQ." + parts[2]

	if _, vErr := m.VerifyToken(tampered); vErr == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestMaker_ShareToken_RoundTrip(t *testing.T) {
	t.Parallel()
	m := newTestMaker(t)

	tok, err := m.CreateShareToken("sh_1", "asset", "ast_1", true, false, "Alice", time.Hour)
	if err != nil {
		t.Fatalf("CreateShareToken: %v", err)
	}

	claims, err := m.VerifyShareToken(tok)
	if err != nil {
		t.Fatalf("VerifyShareToken: %v", err)
	}
	if claims.ShareID != "sh_1" {
		t.Errorf("ShareID: got %q, want %q", claims.ShareID, "sh_1")
	}
	if claims.TargetType != "asset" {
		t.Errorf("TargetType: got %q, want %q", claims.TargetType, "asset")
	}
	if claims.TargetID != "ast_1" {
		t.Errorf("TargetID: got %q, want %q", claims.TargetID, "ast_1")
	}
	if !claims.AllowComments {
		t.Error("expected AllowComments=true")
	}
	if claims.AllowDownload {
		t.Error("expected AllowDownload=false")
	}
	if claims.VisitorName != "Alice" {
		t.Errorf("VisitorName: got %q, want %q", claims.VisitorName, "Alice")
	}
}

func TestMaker_ShareToken_Expired(t *testing.T) {
	t.Parallel()
	m := newTestMaker(t)

	tok, err := m.CreateShareToken("sh_1", "asset", "ast_1", false, false, "", -time.Minute)
	if err != nil {
		t.Fatalf("CreateShareToken: %v", err)
	}

	if _, vErr := m.VerifyShareToken(tok); vErr == nil {
		t.Fatal("expected error for expired share token")
	}
}
