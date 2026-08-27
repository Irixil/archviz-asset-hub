package oauth

import (
	"testing"
)

func TestEncryptDecryptToken(t *testing.T) {
	secret := "test-secret-32-bytes-long-enough!"
	plain := "ya29.some_access_token"

	enc, err := EncryptToken(secret, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == plain {
		t.Fatal("encrypted value must differ from plaintext")
	}

	got, err := DecryptToken(secret, enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plain {
		t.Fatalf("got %q want %q", got, plain)
	}
}

func TestDecryptToken_WrongSecret(t *testing.T) {
	enc, _ := EncryptToken("secret-a", "token")
	_, err := DecryptToken("secret-b", enc)
	if err == nil {
		t.Fatal("expected error with wrong secret")
	}
}

func TestDecryptToken_Corrupt(t *testing.T) {
	_, err := DecryptToken("secret", "notbase64!!")
	if err == nil {
		t.Fatal("expected error on corrupt input")
	}
}
