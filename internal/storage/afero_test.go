package storage_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"damask/server/internal/storage"
)

// TestAferoMemoryStorage_CancelledContext verifies that a pre-cancelled
// context causes every Storage method to return promptly with ctx.Err().
// This is the same treatment applied to the S3-backed afero.Fs, since
// afero.Fs itself has no per-call ctx plumbing (see ROADMAP.73 ST-1.2).
func TestAferoMemoryStorage_CancelledContext(t *testing.T) {
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("NewAferoMemoryStorage: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err = stor.Get(ctx, "some/key"); !errors.Is(err, context.Canceled) {
		t.Errorf("Get: expected context.Canceled, got %v", err)
	}
	if err = stor.Put(ctx, "some/key", strings.NewReader("data")); !errors.Is(err, context.Canceled) {
		t.Errorf("Put: expected context.Canceled, got %v", err)
	}
	if err = stor.Delete(ctx, "some/key"); !errors.Is(err, context.Canceled) {
		t.Errorf("Delete: expected context.Canceled, got %v", err)
	}
	if _, err = stor.List(ctx, "some"); !errors.Is(err, context.Canceled) {
		t.Errorf("List: expected context.Canceled, got %v", err)
	}
	if _, err = stor.Stat(ctx, "some/key"); !errors.Is(err, context.Canceled) {
		t.Errorf("Stat: expected context.Canceled, got %v", err)
	}
}

// TestAferoMemoryStorage_GetNotFound verifies that Get on a missing key
// returns an error satisfying [errors.Is](err, storage.ErrNotFound). The
// MemMapFs.Open miss surfaces as a path error wrapping afero's not-found
// sentinel, which is itself just the standard not-exist sentinel — the same
// one the S3-backed afero.Fs (github.com/fclairamb/afero-s3) surfaces via its
// Stat-then-path-error fallback on a HeadObject/List miss, so both backends
// are detected by the same not-exist check in AferoStorage.Get.
// See ROADMAP.73 ST-2.3.
func TestAferoMemoryStorage_GetNotFound(t *testing.T) {
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("NewAferoMemoryStorage: %v", err)
	}

	_, err = stor.Get(t.Context(), "missing/key")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Get: expected errors.Is(err, storage.ErrNotFound), got %v", err)
	}
}

// TestAferoMemoryStorage_Stat verifies that Stat reports the size of a
// previously written object and a recent, non-zero ModTime. See ROADMAP.73 ST-3.3.
func TestAferoMemoryStorage_Stat(t *testing.T) {
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("NewAferoMemoryStorage: %v", err)
	}

	content := "hello world, this is stat test content"
	before := time.Now().Add(-time.Second)
	if err = stor.Put(t.Context(), "some/key", strings.NewReader(content)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	info, err := stor.Stat(t.Context(), "some/key")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("Stat: Size = %d, want %d", info.Size, len(content))
	}
	if info.ModTime.IsZero() || info.ModTime.Before(before) {
		t.Errorf("Stat: ModTime = %v, want recent non-zero time (after %v)", info.ModTime, before)
	}
}

// TestAferoMemoryStorage_StatNotFound verifies that Stat on a missing key
// returns an error satisfying [errors.Is](err, storage.ErrNotFound). See
// ROADMAP.73 ST-3.3.
func TestAferoMemoryStorage_StatNotFound(t *testing.T) {
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("NewAferoMemoryStorage: %v", err)
	}

	_, err = stor.Stat(t.Context(), "missing/key")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Stat: expected errors.Is(err, storage.ErrNotFound), got %v", err)
	}
}

// TestAferoMemoryStorage_Stat_WorkspaceSegregation proves that a key
// constructed via storage.VersionedVariantKey for one workspace is not
// reachable via Stat under a different workspace's constructed key, even
// though both variants share the same asset/version/type/hash/ext
// components. See ROADMAP.73 ST-3.3 and CLAUDE.md's workspace segregation rule.
func TestAferoMemoryStorage_Stat_WorkspaceSegregation(t *testing.T) {
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("NewAferoMemoryStorage: %v", err)
	}

	const (
		assetID     = "ast_1"
		versionNum  = 1
		variantType = "thumbnail"
		paramsHash  = "abc12345"
		ext         = ".jpg"
	)
	keyA := storage.VersionedVariantKey("ws_a", assetID, versionNum, variantType, paramsHash, ext)
	keyB := storage.VersionedVariantKey("ws_b", assetID, versionNum, variantType, paramsHash, ext)

	if err = stor.Put(t.Context(), keyA, strings.NewReader("workspace-a-content")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, err = stor.Stat(t.Context(), keyB); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf(
			"Stat(workspace B key) = %v, want errors.Is(err, storage.ErrNotFound); "+
				"workspace A's object must not be reachable under workspace B's key",
			err,
		)
	}

	// Sanity check: workspace A's own key is still resolvable.
	if _, err = stor.Stat(t.Context(), keyA); err != nil {
		t.Errorf("Stat(workspace A key): unexpected error %v", err)
	}
}
