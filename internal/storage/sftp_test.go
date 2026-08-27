package storage_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"damask/server/internal/storage"
)

// TestSFTPStorage_CancelledContext verifies that a pre-cancelled context
// causes every Storage method to return promptly with ctx.Err(), without
// ever attempting to dial the (bogus) remote server. No real SSH server is
// needed because the ctx.Err() check is the very first thing each method
// does — if the code instead tried to connect first, these calls would hang
// or fail slowly with a dial/timeout error instead of returning immediately
// with [context.Canceled]. See ROADMAP.73 ST-1.2.
func TestSFTPStorage_CancelledContext(t *testing.T) {
	stor, err := storage.NewSFTPStorage(storage.SFTPConfig{
		Host:       "sftp.invalid.example", // never resolves/dials
		Port:       22,
		User:       "someone",
		AuthMethod: "password",
		Password:   "irrelevant",
	})
	if err != nil {
		t.Fatalf("NewSFTPStorage: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)

		if _, getErr := stor.Get(ctx, "some/key"); !errors.Is(getErr, context.Canceled) {
			t.Errorf("Get: expected context.Canceled, got %v", getErr)
		}
		if putErr := stor.Put(ctx, "some/key", strings.NewReader("data")); !errors.Is(putErr, context.Canceled) {
			t.Errorf("Put: expected context.Canceled, got %v", putErr)
		}
		if delErr := stor.Delete(ctx, "some/key"); !errors.Is(delErr, context.Canceled) {
			t.Errorf("Delete: expected context.Canceled, got %v", delErr)
		}
		if _, listErr := stor.List(ctx, "some"); !errors.Is(listErr, context.Canceled) {
			t.Errorf("List: expected context.Canceled, got %v", listErr)
		}
		if _, statErr := stor.Stat(ctx, "some/key"); !errors.Is(statErr, context.Canceled) {
			t.Errorf("Stat: expected context.Canceled, got %v", statErr)
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("storage calls did not return promptly; ctx.Err() check is not happening before connect()")
	}
}
