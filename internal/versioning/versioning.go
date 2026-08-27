// Package versioning provides utilities for asset version management.
package versioning

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// HashReader streams r through SHA-256 while counting bytes.
// Returns the hex digest and total byte count.
// The caller is responsible for closing r.
func HashReader(r io.Reader) (hash string, size int64, err error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// HashFile computes the SHA-256 hex digest of the file at path.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash, _, err := HashReader(f)
	return hash, err
}
