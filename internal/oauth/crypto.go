// Package oauth provides OAuth state signing and verification.
package oauth

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

const keyLength = 32

func deriveKey(appSecret string) []byte {
	r := hkdf.New(sha256.New, []byte(appSecret), nil, []byte("damask oauth token v1"))
	key := make([]byte, keyLength)
	if _, err := io.ReadFull(r, key); err != nil {
		panic(fmt.Sprintf("oauth/crypto: deriveKey: %v", err))
	}
	return key
}

// EncryptToken encrypts a plain token string using AES-256-GCM.
func EncryptToken(appSecret, plain string) (string, error) {
	block, err := aes.NewCipher(deriveKey(appSecret))
	if err != nil {
		return "", fmt.Errorf("oauth/crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("oauth/crypto: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("oauth/crypto: rand nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// DecryptToken decrypts a value produced by EncryptToken.
func DecryptToken(appSecret, ciphertext string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("oauth/crypto: base64 decode: %w", err)
	}
	block, err := aes.NewCipher(deriveKey(appSecret))
	if err != nil {
		return "", fmt.Errorf("oauth/crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("oauth/crypto: new gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("oauth/crypto: ciphertext too short")
	}
	plain, err := gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("oauth/crypto: decrypt: %w", err)
	}
	return string(plain), nil
}
