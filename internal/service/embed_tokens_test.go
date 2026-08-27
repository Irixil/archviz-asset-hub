package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/token"
)

const embedTestBaseURL = "https://app.example.com"
const embedTestWorkspaceID = "ws_1"
const embedTestAssetID = "ast_1"

func newEmbedTokenSvc(
	t *testing.T,
) (service.EmbedTokenService, *memory.EmbedTokenRepo, *memory.AssetRepo, *memory.RealVersionRepo) {
	t.Helper()
	tokens := memory.NewEmbedTokenRepo()
	assets := memory.NewAssetRepo()
	versions := memory.NewRealVersionRepo()
	svc := service.NewEmbedTokenService(tokens, assets, versions, embedTestBaseURL)
	return svc, tokens, assets, versions
}

func seedEmbedAsset(assets *memory.AssetRepo, versions *memory.RealVersionRepo) {
	versionID := embedTestAssetID + "_v1"
	assets.Seed(repository.Asset{
		ID:                   embedTestAssetID,
		WorkspaceID:          embedTestWorkspaceID,
		OriginalFilename:     "photo.jpg",
		StorageKey:           "key/" + embedTestAssetID,
		MimeType:             "image/jpeg",
		ThumbnailContentType: "image/jpeg",
		CurrentVersionID:     &versionID,
	})
	versions.Seed(repository.AssetVersion{
		ID:          versionID,
		AssetID:     embedTestAssetID,
		WorkspaceID: embedTestWorkspaceID,
		VersionNum:  1,
		StorageKey:  "key/" + embedTestAssetID,
		ContentHash: "abc123",
		MimeType:    "image/jpeg",
		Size:        1024,
		IsCurrent:   true,
	})
}

func TestGetOrCreate_CreatesTokenOnFirstCall(t *testing.T) {
	svc, _, assets, versions := newEmbedTokenSvc(t)
	seedEmbedAsset(assets, versions)

	dto, err := svc.GetOrCreate(context.Background(), embedTestWorkspaceID, embedTestAssetID, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.ID == "" {
		t.Error("expected non-empty token ID")
	}
	if dto.AssetID != embedTestAssetID {
		t.Errorf("expected asset_id %s, got %s", embedTestAssetID, dto.AssetID)
	}
}

func TestGetOrCreate_ReturnsExistingTokenOnSecondCall(t *testing.T) {
	svc, tokens, assets, versions := newEmbedTokenSvc(t)
	seedEmbedAsset(assets, versions)

	first, err := svc.GetOrCreate(context.Background(), embedTestWorkspaceID, embedTestAssetID, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := svc.GetOrCreate(context.Background(), embedTestWorkspaceID, embedTestAssetID, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("expected idempotent token, got %s then %s", first.ID, second.ID)
	}

	all, err := tokens.GetActiveByAssetID(context.Background(), embedTestWorkspaceID, embedTestAssetID)
	if err != nil {
		t.Fatalf("expected exactly one active token: %v", err)
	}
	if all.ID != first.ID {
		t.Errorf("stored token mismatch: %s vs %s", all.ID, first.ID)
	}
}

func TestGetOrCreate_PublicURLContainsTokenID(t *testing.T) {
	svc, _, assets, versions := newEmbedTokenSvc(t)
	seedEmbedAsset(assets, versions)

	dto, err := svc.GetOrCreate(context.Background(), embedTestWorkspaceID, embedTestAssetID, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := embedTestBaseURL + "/e/photo-" + dto.ID
	if dto.PublicURL != want {
		t.Errorf("expected public_url %q, got %q", want, dto.PublicURL)
	}
}

func TestGetOrCreate_ThumbURLContainsTokenID(t *testing.T) {
	svc, _, assets, versions := newEmbedTokenSvc(t)
	seedEmbedAsset(assets, versions)

	dto, err := svc.GetOrCreate(context.Background(), embedTestWorkspaceID, embedTestAssetID, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := embedTestBaseURL + "/e/photo-" + dto.ID + "/thumb"
	if dto.ThumbURL != want {
		t.Errorf("expected thumb_url %q, got %q", want, dto.ThumbURL)
	}
	if !strings.Contains(dto.ThumbURL, dto.ID) {
		t.Errorf("expected thumb_url to contain token ID")
	}
}

func TestGetOrCreate_PublicURLContainsSlugFromFilename(t *testing.T) {
	svc, _, assets, versions := newEmbedTokenSvc(t)
	versionID := embedTestAssetID + "_v1"
	assets.Seed(repository.Asset{
		ID:                   embedTestAssetID,
		WorkspaceID:          embedTestWorkspaceID,
		OriginalFilename:     "Vacation Photo.jpg",
		StorageKey:           "key/" + embedTestAssetID,
		MimeType:             "image/jpeg",
		ThumbnailContentType: "image/jpeg",
		CurrentVersionID:     &versionID,
	})
	versions.Seed(repository.AssetVersion{
		ID:          versionID,
		AssetID:     embedTestAssetID,
		WorkspaceID: embedTestWorkspaceID,
		VersionNum:  1,
		StorageKey:  "key/" + embedTestAssetID,
		ContentHash: "abc123",
		MimeType:    "image/jpeg",
		Size:        1024,
		IsCurrent:   true,
	})

	dto, err := svc.GetOrCreate(context.Background(), embedTestWorkspaceID, embedTestAssetID, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := embedTestBaseURL + "/e/vacation-photo-" + dto.ID
	if dto.PublicURL != want {
		t.Errorf("expected public_url %q, got %q", want, dto.PublicURL)
	}
}

func TestGetOrCreate_PublicURLFallsBackToBareIDWhenAssetMissing(t *testing.T) {
	svc, tokens, _, _ := newEmbedTokenSvc(t)

	id, err := token.NewBase62(16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, createErr := tokens.Create(context.Background(), repository.CreateEmbedTokenParams{
		ID:          id,
		WorkspaceID: embedTestWorkspaceID,
		AssetID:     embedTestAssetID,
		CreatedBy:   "user_1",
	}); createErr != nil {
		t.Fatalf("unexpected error: %v", createErr)
	}

	dto, err := svc.GetActive(context.Background(), embedTestWorkspaceID, embedTestAssetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := embedTestBaseURL + "/e/" + dto.ID
	if dto.PublicURL != want {
		t.Errorf("expected public_url %q, got %q", want, dto.PublicURL)
	}
}

func TestExtractTokenID_StripsSlugPrefix(t *testing.T) {
	id := "Ab3Xy7PqR9mNsLkT"
	got := service.ExtractTokenID("vacation-photo-" + id)
	if got != id {
		t.Errorf("expected %q, got %q", id, got)
	}
}

func TestExtractTokenID_ReturnsRawWhenNoSlug(t *testing.T) {
	id := "Ab3Xy7PqR9mNsLkT"
	got := service.ExtractTokenID(id)
	if got != id {
		t.Errorf("expected %q, got %q", id, got)
	}
}

func TestRevoke_RevokesActiveToken(t *testing.T) {
	svc, tokens, assets, versions := newEmbedTokenSvc(t)
	seedEmbedAsset(assets, versions)

	dto, err := svc.GetOrCreate(context.Background(), embedTestWorkspaceID, embedTestAssetID, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = svc.Revoke(context.Background(), embedTestWorkspaceID, embedTestAssetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = tokens.GetActiveByAssetID(context.Background(), embedTestWorkspaceID, embedTestAssetID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("expected no active token after revoke, got %v", err)
	}

	tok, err := tokens.GetByID(context.Background(), dto.ID)
	if err != nil {
		t.Fatalf("expected revoked token to still exist: %v", err)
	}
	if tok.RevokedAt == nil {
		t.Error("expected revoked_at to be set")
	}
}

func TestRevoke_ReturnsNotFoundWhenNoneActive(t *testing.T) {
	svc, _, _, _ := newEmbedTokenSvc(t)
	err := svc.Revoke(context.Background(), embedTestWorkspaceID, embedTestAssetID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestResolveCurrentFile_ReturnsFileMetadata(t *testing.T) {
	svc, _, assets, versions := newEmbedTokenSvc(t)
	seedEmbedAsset(assets, versions)

	dto, err := svc.GetOrCreate(context.Background(), embedTestWorkspaceID, embedTestAssetID, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved, err := svc.ResolveCurrentFile(context.Background(), dto.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Filename != "photo.jpg" {
		t.Errorf("expected filename photo.jpg, got %s", resolved.Filename)
	}
	if resolved.ContentHash != "abc123" {
		t.Errorf("expected content hash abc123, got %s", resolved.ContentHash)
	}
	if resolved.MimeType != "image/jpeg" {
		t.Errorf("expected mime type image/jpeg, got %s", resolved.MimeType)
	}
}

func TestResolveCurrentFile_ReturnsGoneWhenRevoked(t *testing.T) {
	svc, _, assets, versions := newEmbedTokenSvc(t)
	seedEmbedAsset(assets, versions)

	dto, err := svc.GetOrCreate(context.Background(), embedTestWorkspaceID, embedTestAssetID, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = svc.Revoke(context.Background(), embedTestWorkspaceID, embedTestAssetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.ResolveCurrentFile(context.Background(), dto.ID)
	if !errors.Is(err, service.ErrGone) {
		t.Errorf("expected ErrGone, got %v", err)
	}
}

func TestResolveCurrentFile_ReturnsNotFoundForUnknownToken(t *testing.T) {
	svc, _, _, _ := newEmbedTokenSvc(t)
	_, err := svc.ResolveCurrentFile(context.Background(), "unknown_token")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
