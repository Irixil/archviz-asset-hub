package versioning

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, errors.New("read error") }

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestHashReader_KnownInput(t *testing.T) {
	t.Parallel()
	content := []byte("hello damask")
	want := sha256Hex(content)

	got, size, err := HashReader(strings.NewReader(string(content)))
	if err != nil {
		t.Fatalf("HashReader: %v", err)
	}
	if got != want {
		t.Fatalf("hash mismatch: got %q, want %q", got, want)
	}
	if size != int64(len(content)) {
		t.Fatalf("size mismatch: got %d, want %d", size, len(content))
	}
}

func TestHashReader_Empty(t *testing.T) {
	t.Parallel()
	want := sha256Hex([]byte{})

	got, size, err := HashReader(strings.NewReader(""))
	if err != nil {
		t.Fatalf("HashReader empty: %v", err)
	}
	if got != want {
		t.Fatalf("hash mismatch: got %q, want %q", got, want)
	}
	if size != 0 {
		t.Fatalf("expected size 0, got %d", size)
	}
}

func TestHashReader_ReadError(t *testing.T) {
	t.Parallel()
	_, _, err := HashReader(errReader{})
	if err == nil {
		t.Fatal("expected error from failing reader, got nil")
	}
}

func TestHashFile_ValidFile(t *testing.T) {
	t.Parallel()
	content := []byte("file content for hashing")
	dir := t.TempDir()
	path := filepath.Join(dir, "asset.bin")

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	want, _, err := HashReader(strings.NewReader(string(content)))
	if err != nil {
		t.Fatalf("HashReader: %v", err)
	}

	got, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if got != want {
		t.Fatalf("hash mismatch: got %q, want %q", got, want)
	}
}

func TestHashFile_Nonexistent(t *testing.T) {
	t.Parallel()
	_, err := HashFile("/tmp/damask-versioning-nonexistent-file-xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}
