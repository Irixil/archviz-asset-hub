package token_test

import (
	"strings"
	"testing"

	"damask/server/internal/token"
)

func TestNewBase62_Length(t *testing.T) {
	for _, n := range []int{1, 8, 16, 32} {
		s, err := token.NewBase62(n)
		if err != nil {
			t.Fatalf("NewBase62(%d): unexpected error: %v", n, err)
		}
		if len(s) != n {
			t.Errorf("NewBase62(%d): got length %d, want %d", n, len(s), n)
		}
	}
}

func TestNewBase62_Charset(t *testing.T) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	s, err := token.NewBase62(64)
	if err != nil {
		t.Fatalf("NewBase62: unexpected error: %v", err)
	}
	for _, c := range s {
		if !strings.ContainsRune(alphabet, c) {
			t.Errorf("NewBase62: character %q not in base62 alphabet", c)
		}
	}
}

func TestNewBase62_Uniqueness(t *testing.T) {
	const iterations = 1000
	seen := make(map[string]bool, iterations)
	for range iterations {
		s, err := token.NewBase62(16)
		if err != nil {
			t.Fatalf("NewBase62: unexpected error: %v", err)
		}
		if seen[s] {
			t.Fatalf("NewBase62: duplicate value generated: %q", s)
		}
		seen[s] = true
	}
}
