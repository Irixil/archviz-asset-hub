package ingress

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const appSecretLength = 32 // must be at least 32 bytes for AES-256

// deriveKey produces a 32-byte AES-256 key using HKDF-SHA256 with a fixed
// info string. HKDF provides domain separation and is the idiomatic Go KDF.
func deriveKey(appSecret string) []byte {
	r := hkdf.New(sha256.New, []byte(appSecret), nil, []byte("damask ingress config v1"))
	key := make([]byte, appSecretLength)
	if _, err := io.ReadFull(r, key); err != nil {
		panic(fmt.Sprintf("ingress/crypto: deriveKey: %v", err))
	}
	return key
}

// EncryptConfig encrypts plaintext config JSON with AES-256-GCM.
// Returns a base64url string: nonce || ciphertext+tag (no padding).
func EncryptConfig(appSecret string, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(deriveKey(appSecret))
	if err != nil {
		return "", fmt.Errorf("ingress/crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("ingress/crypto: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("ingress/crypto: rand nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// DecryptConfig decrypts a value produced by EncryptConfig.
func DecryptConfig(appSecret string, ciphertext string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("ingress/crypto: base64 decode: %w", err)
	}
	block, err := aes.NewCipher(deriveKey(appSecret))
	if err != nil {
		return nil, fmt.Errorf("ingress/crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("ingress/crypto: new gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return nil, errors.New("ingress/crypto: ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("ingress/crypto: decrypt: %w", err)
	}
	return plaintext, nil
}

// GenerateToken returns a cryptographically random URL-safe base64 string
// derived from n random bytes.
func GenerateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("ingress/crypto: generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
