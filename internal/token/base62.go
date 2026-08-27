// Package token provides cryptographically random string generators for
// public, opaque identifiers (e.g. embed tokens).
package token

import (
	"crypto/rand"
	"math/big"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// NewBase62 returns a cryptographically random n-character base62 string.
func NewBase62(n int) (string, error) {
	b := make([]byte, n)
	charsetSize := big.NewInt(int64(len(base62Chars)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, charsetSize)
		if err != nil {
			return "", err
		}
		b[i] = base62Chars[idx.Int64()]
	}
	return string(b), nil
}
