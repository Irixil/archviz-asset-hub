package ingress

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	t.Parallel()
	secret := "test-app-secret-for-ingress!!"
	plain := []byte(`{"host":"imap.example.com","password":"s3cr3t"}`)

	enc, err := EncryptConfig(secret, plain)
	if err != nil {
		t.Fatalf("EncryptConfig: %v", err)
	}
	if enc == "" {
		t.Fatal("EncryptConfig returned empty string")
	}

	got, err := DecryptConfig(secret, enc)
	if err != nil {
		t.Fatalf("DecryptConfig: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", got, plain)
	}
}

func TestEncryptProducesUniqueValues(t *testing.T) {
	t.Parallel()
	// Each call uses a fresh random nonce — ciphertexts must differ.
	secret := "test-app-secret-for-ingress!!"
	plain := []byte(`{"key":"value"}`)

	enc1, _ := EncryptConfig(secret, plain)
	enc2, _ := EncryptConfig(secret, plain)
	if enc1 == enc2 {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertext (missing random nonce)")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	t.Parallel()
	plain := []byte(`{"password":"secret"}`)
	enc, _ := EncryptConfig("correct-secret-key-32chars!!!", plain)

	_, err := DecryptConfig("wrong-secret-key-32charssss!!!!", enc)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key, got nil")
	}
}

func TestDecryptTruncatedCiphertext(t *testing.T) {
	t.Parallel()
	secret := "test-app-secret-for-ingress!!"
	enc, _ := EncryptConfig(secret, []byte(`{"x":1}`))

	// Truncate to fewer bytes than the nonce size (12 bytes for GCM).
	truncated := enc[:4]
	_, err := DecryptConfig(secret, truncated)
	if err == nil {
		t.Fatal("expected error on truncated ciphertext, got nil")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	t.Parallel()
	_, err := DecryptConfig("any-secret", "!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error on invalid base64, got nil")
	}
	if !strings.Contains(err.Error(), "base64") {
		t.Fatalf("expected base64 error, got %v", err)
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	t.Parallel()
	secret := "test-app-secret-for-ingress!!"
	enc, err := EncryptConfig(secret, []byte{})
	if err != nil {
		t.Fatalf("EncryptConfig empty: %v", err)
	}
	got, err := DecryptConfig(secret, enc)
	if err != nil {
		t.Fatalf("DecryptConfig empty: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty plaintext, got %q", got)
	}
}
