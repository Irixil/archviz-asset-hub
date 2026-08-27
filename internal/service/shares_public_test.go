package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

// --- stubs ---

// userRepoWithOwner wraps memory.UserRepo but returns a fixed user for GetByID.
type userRepoWithOwner struct {
	*memory.UserRepo

	ownerEmail string
}

func (r *userRepoWithOwner) GetByID(_ context.Context, _ string) (repository.User, error) {
	return repository.User{ID: "usr_owner", Email: r.ownerEmail}, nil
}

type stubVariantResolver struct{}

func (s stubVariantResolver) GetSharedByVariantAndAsset(_ context.Context, _, _ string) (repository.Variant, error) {
	return repository.Variant{}, apperr.ErrNotFound
}

type spyCommentMailer struct {
	to string
}

func (m *spyCommentMailer) SendCommentPosted(_ context.Context, _, _, ownerEmail, _, _, _ string) error {
	m.to = ownerEmail
	return nil
}

// shareRepoWithAsset wraps memory.ShareRepo and returns a fixed PublicAsset for GetPublicAsset.
type shareRepoWithAsset struct {
	*memory.ShareRepo

	asset repository.PublicAsset
}

func (r *shareRepoWithAsset) GetPublicAsset(_ context.Context, _ string) (repository.PublicAsset, error) {
	return r.asset, nil
}

// newPublicSvc builds a SharePublicService wired with real memory share repo.
func newPublicSvc(t *testing.T) (service.SharePublicService, *memory.ShareRepo) {
	t.Helper()
	shares := memory.NewRealShareRepo()
	users := &userRepoWithOwner{UserRepo: memory.NewUserRepo(), ownerEmail: "owner@example.com"}
	svc := service.NewSharePublicService(shares, users, stubVariantResolver{}, &spyCommentMailer{})
	return svc, shares
}

// newPublicSvcWithAsset builds a service where GetPublicAsset returns a fixed asset.
func newPublicSvcWithAsset(
	t *testing.T,
	ownerEmail string,
	asset repository.PublicAsset,
) (service.SharePublicService, *memory.ShareRepo) {
	t.Helper()
	base := memory.NewRealShareRepo()
	wrapped := &shareRepoWithAsset{ShareRepo: base, asset: asset}
	users := &userRepoWithOwner{UserRepo: memory.NewUserRepo(), ownerEmail: ownerEmail}
	svc := service.NewSharePublicService(wrapped, users, stubVariantResolver{}, &spyCommentMailer{})
	return svc, base
}

func seedShare(shares *memory.ShareRepo, sh repository.Share) {
	if sh.ID == "" {
		sh.ID = "sh_1"
	}
	if sh.WorkspaceID == "" {
		sh.WorkspaceID = "ws_1"
	}
	if sh.CreatedBy == "" {
		sh.CreatedBy = "usr_owner"
	}
	shares.Seed(sh)
}

func TestIsShareExpiredDomain(t *testing.T) {
	t.Parallel()

	// nil ExpiresAt — never expires
	if service.IsShareExpiredDomain(repository.Share{}) {
		t.Error("nil ExpiresAt should not be expired")
	}

	// future RFC3339
	future := time.Now().Add(24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	if service.IsShareExpiredDomain(repository.Share{ExpiresAt: &future}) {
		t.Error("future date should not be expired")
	}

	// past RFC3339
	past := time.Now().Add(-24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	if !service.IsShareExpiredDomain(repository.Share{ExpiresAt: &past}) {
		t.Error("past RFC3339 date should be expired")
	}

	// past space-delimited format
	pastSpace := time.Now().Add(-24 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	if !service.IsShareExpiredDomain(repository.Share{ExpiresAt: &pastSpace}) {
		t.Error("past space-delimited date should be expired")
	}

	// malformed date
	bad := "not-a-date"
	if service.IsShareExpiredDomain(repository.Share{ExpiresAt: &bad}) {
		t.Error("malformed date should not be expired")
	}
}

func TestToPublicAssetDTO(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	w := int64(1920)
	h := int64(1080)
	thumb := "thumb/key"
	meta := `{"foo":"bar"}`
	proj := "proj_1"
	folder := "folder_1"

	in := repository.PublicAsset{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		ProjectID:        &proj,
		FolderID:         &folder,
		OriginalFilename: "photo.jpg",
		StorageKey:       "files/photo.jpg",
		MimeType:         "image/jpeg",
		Size:             1024,
		Width:            &w,
		Height:           &h,
		ThumbnailKey:     &thumb,
		Metadata:         &meta,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	got := service.ToPublicAssetDTO(in)
	if got.ID != in.ID || got.WorkspaceID != in.WorkspaceID || got.ProjectID != in.ProjectID {
		t.Errorf("ID/WorkspaceID/ProjectID mismatch: %+v", got)
	}
	if got.FolderID != in.FolderID || got.OriginalFilename != in.OriginalFilename {
		t.Errorf("FolderID/OriginalFilename mismatch: %+v", got)
	}
	if got.StorageKey != in.StorageKey || got.MimeType != in.MimeType || got.Size != in.Size {
		t.Errorf("StorageKey/MimeType/Size mismatch: %+v", got)
	}
	if got.Width != in.Width || got.Height != in.Height || got.ThumbnailKey != in.ThumbnailKey {
		t.Errorf("Width/Height/ThumbnailKey mismatch: %+v", got)
	}
	if got.Metadata != in.Metadata {
		t.Errorf("Metadata mismatch: %+v", got)
	}
}

func TestToCommentDTO(t *testing.T) {
	t.Parallel()

	email := "user@example.com"
	in := repository.ShareComment{
		ID:          "cmt_1",
		ShareID:     "share_1",
		AssetID:     "ast_1",
		AuthorName:  "Alice",
		AuthorEmail: &email,
		Body:        "Nice photo!",
		CreatedAt:   "2024-01-01T00:00:00Z",
	}

	got := service.ToCommentDTO(in)
	if got.ID != in.ID || got.ShareID != in.ShareID || got.AssetID != in.AssetID {
		t.Errorf("ID/ShareID/AssetID mismatch: %+v", got)
	}
	if got.AuthorName != in.AuthorName || got.AuthorEmail != in.AuthorEmail {
		t.Errorf("AuthorName/AuthorEmail mismatch: %+v", got)
	}
	if got.Body != in.Body || got.CreatedAt != in.CreatedAt {
		t.Errorf("Body/CreatedAt mismatch: %+v", got)
	}
}

// --- SharePublicService integration tests ---

func TestSharePublicSvc_GetActive_OK(t *testing.T) {
	t.Parallel()
	svc, shares := newPublicSvc(t)
	seedShare(shares, repository.Share{TargetType: "asset", TargetID: "ast_1"})

	dto, err := svc.GetActive(context.Background(), "sh_1")
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if dto.ID != "sh_1" {
		t.Errorf("expected ID sh_1, got %q", dto.ID)
	}
}

func TestSharePublicSvc_GetActive_Revoked(t *testing.T) {
	t.Parallel()
	svc, shares := newPublicSvc(t)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	seedShare(shares, repository.Share{RevokedAt: &now})

	_, err := svc.GetActive(context.Background(), "sh_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for revoked share, got %v", err)
	}
}

func TestSharePublicSvc_GetActive_Expired(t *testing.T) {
	t.Parallel()
	svc, shares := newPublicSvc(t)
	past := time.Now().Add(-24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	seedShare(shares, repository.Share{ExpiresAt: &past})

	_, err := svc.GetActive(context.Background(), "sh_1")
	if !errors.Is(err, service.ErrGone) {
		t.Fatalf("expected ErrGone for expired share, got %v", err)
	}
}

func TestSharePublicSvc_GetActive_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newPublicSvc(t)

	_, err := svc.GetActive(context.Background(), "sh_unknown")
	if err == nil {
		t.Fatal("expected error for unknown share")
	}
}

func TestSharePublicSvc_IncrementViewCount(t *testing.T) {
	t.Parallel()
	svc, shares := newPublicSvc(t)
	seedShare(shares, repository.Share{})

	if err := svc.IncrementViewCount(context.Background(), "sh_1"); err != nil {
		t.Fatalf("IncrementViewCount: %v", err)
	}
}

func TestSharePublicSvc_GetOwnerShare_OK(t *testing.T) {
	t.Parallel()
	svc, shares := newPublicSvc(t)
	seedShare(shares, repository.Share{})

	dto, err := svc.GetOwnerShare(context.Background(), "ws_1", "sh_1")
	if err != nil {
		t.Fatalf("GetOwnerShare: %v", err)
	}
	if dto.ID != "sh_1" || dto.WorkspaceID != "ws_1" {
		t.Errorf("unexpected dto: %+v", dto)
	}
}

func TestSharePublicSvc_GetOwnerShare_WrongWorkspace(t *testing.T) {
	t.Parallel()
	svc, shares := newPublicSvc(t)
	seedShare(shares, repository.Share{})

	_, err := svc.GetOwnerShare(context.Background(), "ws_other", "sh_1")
	if err == nil {
		t.Fatal("expected error for wrong workspace")
	}
}

func TestSharePublicSvc_CreateComment_OK(t *testing.T) {
	t.Parallel()
	asset := repository.PublicAsset{ID: "ast_1", WorkspaceID: "ws_1", OriginalFilename: "photo.jpg"}
	svc, shares := newPublicSvcWithAsset(t, "owner@example.com", asset)
	seedShare(shares, repository.Share{})

	dto, err := svc.CreateComment(context.Background(), service.CreateShareCommentParams{
		ShareID:    "sh_1",
		AssetID:    "ast_1",
		AuthorName: "Bob",
		Body:       "Great photo!",
	})
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if dto.ID == "" {
		t.Error("expected non-empty comment ID")
	}
	if dto.AuthorName != "Bob" || dto.Body != "Great photo!" {
		t.Errorf("unexpected dto: %+v", dto)
	}
}

func TestSharePublicSvc_ListCommentsByShare(t *testing.T) {
	t.Parallel()
	svc, shares := newPublicSvc(t)
	seedShare(shares, repository.Share{})

	// memory repo returns nil — just verify no error
	comments, err := svc.ListCommentsByShare(context.Background(), "sh_1")
	if err != nil {
		t.Fatalf("ListCommentsByShare: %v", err)
	}
	_ = comments
}
