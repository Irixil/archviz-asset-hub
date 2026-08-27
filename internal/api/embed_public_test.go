//go:build integration

package api_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

const embedPublicTestToken = "tok_public_1"

func embedPublicResolved(storageKey, contentHash, mimeType string) *service.ResolvedEmbed {
	return &service.ResolvedEmbed{
		StorageKey:  storageKey,
		MimeType:    mimeType,
		Filename:    "photo.jpg",
		ContentHash: contentHash,
	}
}

func TestPublicEmbedFile_StreamsCurrentVersionFile(t *testing.T) {
	env := testutil.NewTestEnv(t)
	if err := env.Server.StorageForTest().Put(t.Context(), "key/v1", bytes.NewReader([]byte("file-bytes-v1"))); err != nil {
		t.Fatalf("seed storage: %v", err)
	}
	env.EmbedTokens.ResolveFn = func(_ context.Context, tokenID string) (*service.ResolvedEmbed, error) {
		if tokenID != embedPublicTestToken {
			return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		return embedPublicResolved("key/v1", "hash_v1", "image/jpeg"), nil
	}

	req := testutil.AuthRequest(http.MethodGet, "/e/"+embedPublicTestToken, nil, nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "file-bytes-v1" {
		t.Errorf("body = %q, want file-bytes-v1", body)
	}
}

func TestPublicEmbedFile_SetsETagFromContentHash(t *testing.T) {
	env := testutil.NewTestEnv(t)
	if err := env.Server.StorageForTest().Put(t.Context(), "key/v1", bytes.NewReader([]byte("file-bytes"))); err != nil {
		t.Fatalf("seed storage: %v", err)
	}
	env.EmbedTokens.ResolveFn = func(_ context.Context, _ string) (*service.ResolvedEmbed, error) {
		return embedPublicResolved("key/v1", "hash_v1", "image/jpeg"), nil
	}

	req := testutil.AuthRequest(http.MethodGet, "/e/"+embedPublicTestToken, nil, nil)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusOK)
	if etag := resp.Header.Get("ETag"); etag != `"hash_v1"` {
		t.Errorf("ETag = %q, want %q", etag, `"hash_v1"`)
	}
}

func TestPublicEmbedFile_Returns304OnETagMatch(t *testing.T) {
	env := testutil.NewTestEnv(t)
	if err := env.Server.StorageForTest().Put(t.Context(), "key/v1", bytes.NewReader([]byte("file-bytes"))); err != nil {
		t.Fatalf("seed storage: %v", err)
	}
	env.EmbedTokens.ResolveFn = func(_ context.Context, _ string) (*service.ResolvedEmbed, error) {
		return embedPublicResolved("key/v1", "hash_v1", "image/jpeg"), nil
	}

	req := testutil.AuthRequest(http.MethodGet, "/e/"+embedPublicTestToken, nil, nil)
	req.Header.Set("If-None-Match", `"hash_v1"`)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotModified)
}

func TestPublicEmbedFile_StreamsCurrentVersionFileWithSluggedURL(t *testing.T) {
	env := testutil.NewTestEnv(t)
	if err := env.Server.StorageForTest().Put(t.Context(), "key/v1", bytes.NewReader([]byte("file-bytes-v1"))); err != nil {
		t.Fatalf("seed storage: %v", err)
	}
	// Real token IDs are always exactly 16 chars (token.NewBase62(16)) — ExtractTokenID
	// relies on that fixed length to strip a cosmetic slug prefix off the URL param.
	const realToken = "Ab3Xy7PqR9mNsLkT"
	env.EmbedTokens.ResolveFn = func(_ context.Context, tokenID string) (*service.ResolvedEmbed, error) {
		if tokenID != realToken {
			return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
		}
		return embedPublicResolved("key/v1", "hash_v1", "image/jpeg"), nil
	}

	req := testutil.AuthRequest(http.MethodGet, "/e/vacation-photo-"+realToken, nil, nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "file-bytes-v1" {
		t.Errorf("body = %q, want file-bytes-v1", body)
	}
}

func TestPublicEmbedFile_Returns404ForUnknownToken(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.EmbedTokens.ResolveFn = func(_ context.Context, _ string) (*service.ResolvedEmbed, error) {
		return nil, fmt.Errorf("not found: %w", apperr.ErrNotFound)
	}

	req := testutil.AuthRequest(http.MethodGet, "/e/unknown_token", nil, nil)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}

func TestPublicEmbedFile_Returns410ForRevokedToken(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.EmbedTokens.ResolveFn = func(_ context.Context, _ string) (*service.ResolvedEmbed, error) {
		return nil, service.ErrGone
	}

	req := testutil.AuthRequest(http.MethodGet, "/e/"+embedPublicTestToken, nil, nil)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusGone)
}

func TestPublicEmbedFile_ServesNewVersionAfterPromotion(t *testing.T) {
	env := testutil.NewTestEnv(t)
	stor := env.Server.StorageForTest()
	if err := stor.Put(t.Context(), "key/v1", bytes.NewReader([]byte("v1-bytes"))); err != nil {
		t.Fatalf("seed storage: %v", err)
	}
	if err := stor.Put(t.Context(), "key/v2", bytes.NewReader([]byte("v2-bytes"))); err != nil {
		t.Fatalf("seed storage: %v", err)
	}

	current := "key/v1"
	env.EmbedTokens.ResolveFn = func(_ context.Context, _ string) (*service.ResolvedEmbed, error) {
		return embedPublicResolved(current, "hash_"+current, "image/jpeg"), nil
	}

	req1 := testutil.AuthRequest(http.MethodGet, "/e/"+embedPublicTestToken, nil, nil)
	resp1, _ := env.App.Test(req1)
	body1, _ := io.ReadAll(resp1.Body)
	if string(body1) != "v1-bytes" {
		t.Fatalf("first response = %q, want v1-bytes", body1)
	}

	// Promote to a new version — same token URL must now serve the new file.
	current = "key/v2"
	req2 := testutil.AuthRequest(http.MethodGet, "/e/"+embedPublicTestToken, nil, nil)
	resp2, _ := env.App.Test(req2)
	body2, _ := io.ReadAll(resp2.Body)
	if string(body2) != "v2-bytes" {
		t.Fatalf("second response = %q, want v2-bytes", body2)
	}
}

func TestPublicEmbedThumb_StreamsThumbnail(t *testing.T) {
	env := testutil.NewTestEnv(t)
	if err := env.Server.StorageForTest().Put(t.Context(), "thumbs/v1", bytes.NewReader([]byte("thumb-bytes"))); err != nil {
		t.Fatalf("seed storage: %v", err)
	}
	thumbKey := "thumbs/v1"
	env.EmbedTokens.ResolveFn = func(_ context.Context, _ string) (*service.ResolvedEmbed, error) {
		return &service.ResolvedEmbed{
			StorageKey:           "key/v1",
			ThumbnailKey:         &thumbKey,
			ThumbnailContentType: "image/jpeg",
			Filename:             "photo.jpg",
			ContentHash:          "hash_v1",
		}, nil
	}

	req := testutil.AuthRequest(http.MethodGet, "/e/"+embedPublicTestToken+"/thumb", nil, nil)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "thumb-bytes" {
		t.Errorf("body = %q, want thumb-bytes", body)
	}
}

func TestPublicEmbedThumb_Returns202WhenThumbnailPending(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.EmbedTokens.ResolveFn = func(_ context.Context, _ string) (*service.ResolvedEmbed, error) {
		return embedPublicResolved("key/v1", "hash_v1", "image/jpeg"), nil
	}

	req := testutil.AuthRequest(http.MethodGet, "/e/"+embedPublicTestToken+"/thumb", nil, nil)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusAccepted)
	if ra := resp.Header.Get("Retry-After"); ra != "5" {
		t.Errorf("Retry-After = %q, want 5", ra)
	}
}
