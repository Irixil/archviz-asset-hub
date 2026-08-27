package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"damask/server/internal/assetio"
	"damask/server/internal/audit"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/media/ingest"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
)

// newDuplicateTestUploadSvc wires an UploadService against a real sqlite DB,
// in-memory storage, and a real DuplicateService (dupSvc may be overridden by
// callers that need to simulate a broken dedup lookup).
func newDuplicateTestUploadSvc(
	t *testing.T,
	wsID, mode string,
	dupSvc service.DuplicateService,
) (service.UploadService, storage.Storage, repository.VersionRepository) {
	t.Helper()
	database, err := dbpkg.Open(t.TempDir() + "/dup_test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx := context.Background()
	if _, wsErr := database.WQ.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "test"}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	if mode != "" {
		if modeErr := database.WQ.UpdateWorkspaceDuplicateDetectionMode(
			ctx,
			dbgen.UpdateWorkspaceDuplicateDetectionModeParams{ID: wsID, DuplicateDetectionMode: mode},
		); modeErr != nil {
			t.Fatalf("set dedup mode: %v", modeErr)
		}
	}

	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	q := queue.New(database.WQ, 1)
	assets := reposqlc.NewAssetRepo(database)
	versions := reposqlc.NewVersionRepo(database)
	workspaces := reposqlc.NewWorkspaceRepo(database)
	if dupSvc == nil {
		dupSvc = service.NewDuplicateService(versions, assets, workspaces, stor)
	}
	ingester := service.NewAssetIngester(
		assets, versions, stor, q,
		ingest.NewRegistry(transform.NewTransformer()),
		service.NewAutoTagService(assets, workspaces, reposqlc.NewAutoTagSuggestionRepo(database), q, nil, nil),
		dupSvc,
	)
	return service.NewUploadService(ingester, audit.NopWriter{}, nil), stor, versions
}

// fakeDuplicateService lets a single test simulate a broken Mode() lookup
// without needing a real database in an invalid state.
type fakeDuplicateService struct {
	modeErr error
}

func (f *fakeDuplicateService) Mode(context.Context, string) (service.DuplicateMode, error) {
	return service.DuplicateModeWarn, f.modeErr
}

func (f *fakeDuplicateService) FindDuplicate(
	context.Context, string, string, string,
) (*assetio.DuplicateMatch, error) {
	return nil, nil //nolint:nilnil // fake: no match
}

func TestUploadService_Duplicate_ModeWarn_NoMatch(t *testing.T) {
	svc, _, _ := newDuplicateTestUploadSvc(t, "ws_warn_nomatch", "warn", nil)

	dto, err := svc.Ingest(
		context.Background(),
		"ws_warn_nomatch",
		strings.NewReader("unique bytes 1"),
		service.UploadMeta{
			OriginalFilename: "a.jpg",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.DuplicateOf != nil {
		t.Errorf("expected no duplicate match, got %+v", dto.DuplicateOf)
	}
}

func TestUploadService_Duplicate_ModeWarn_Match(t *testing.T) {
	svc, _, _ := newDuplicateTestUploadSvc(t, "ws_warn_match", "warn", nil)
	ctx := context.Background()

	first, err := svc.Ingest(ctx, "ws_warn_match", strings.NewReader("shared bytes"), service.UploadMeta{
		OriginalFilename: "first.jpg",
	})
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}

	second, err := svc.Ingest(ctx, "ws_warn_match", strings.NewReader("shared bytes"), service.UploadMeta{
		OriginalFilename: "second.jpg",
	})
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if second.DuplicateOf == nil {
		t.Fatal("expected a duplicate match on the second upload")
	}
	if second.DuplicateOf.AssetID != first.ID {
		t.Errorf("DuplicateOf.AssetID: got %q, want %q", second.DuplicateOf.AssetID, first.ID)
	}
	if second.DuplicateOf.OriginalFilename != "first.jpg" {
		t.Errorf("DuplicateOf.OriginalFilename: got %q, want %q", second.DuplicateOf.OriginalFilename, "first.jpg")
	}
}

func TestUploadService_Duplicate_ModeBlock(t *testing.T) {
	svc, stor, _ := newDuplicateTestUploadSvc(t, "ws_block", "block", nil)
	ctx := context.Background()

	first, err := svc.Ingest(ctx, "ws_block", strings.NewReader("blocked bytes"), service.UploadMeta{
		OriginalFilename: "first.jpg",
	})
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}

	_, err = svc.Ingest(ctx, "ws_block", strings.NewReader("blocked bytes"), service.UploadMeta{
		OriginalFilename: "second.jpg",
	})
	var conflictErr *service.DuplicateConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected DuplicateConflictError, got %v", err)
	}
	if conflictErr.Match.AssetID != first.ID {
		t.Errorf("conflict match asset: got %q, want %q", conflictErr.Match.AssetID, first.ID)
	}

	// Only the first upload's storage object should remain — the blocked
	// second upload's asset+version+storage must have been fully rolled back.
	keys, err := stor.List(ctx, "ws_block/")
	if err != nil {
		t.Fatalf("list storage: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected exactly 1 storage object to remain after rollback, got %d: %v", len(keys), keys)
	}
}

func TestUploadService_Duplicate_ModeOff(t *testing.T) {
	svc, _, _ := newDuplicateTestUploadSvc(t, "ws_off", "off", nil)
	ctx := context.Background()

	_, err := svc.Ingest(ctx, "ws_off", strings.NewReader("off-mode bytes"), service.UploadMeta{
		OriginalFilename: "first.jpg",
	})
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}

	second, err := svc.Ingest(ctx, "ws_off", strings.NewReader("off-mode bytes"), service.UploadMeta{
		OriginalFilename: "second.jpg",
	})
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if second.DuplicateOf != nil {
		t.Errorf("expected no duplicate check in off mode, got %+v", second.DuplicateOf)
	}
}

func TestUploadService_Duplicate_MatchesSoftDeletedVersion(t *testing.T) {
	svc, _, versions := newDuplicateTestUploadSvc(t, "ws_soft_deleted", "warn", nil)
	ctx := context.Background()

	first, err := svc.Ingest(ctx, "ws_soft_deleted", strings.NewReader("soft deleted bytes"), service.UploadMeta{
		OriginalFilename: "first.jpg",
	})
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}
	firstVersions, err := versions.ListByAsset(ctx, first.ID)
	if err != nil || len(firstVersions) != 1 {
		t.Fatalf("list first asset versions: %v (versions=%v)", err, firstVersions)
	}
	if softErr := versions.SoftDelete(ctx, firstVersions[0].ID); softErr != nil {
		t.Fatalf("soft delete first version: %v", softErr)
	}

	second, err := svc.Ingest(ctx, "ws_soft_deleted", strings.NewReader("soft deleted bytes"), service.UploadMeta{
		OriginalFilename: "second.jpg",
	})
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if second.DuplicateOf == nil {
		t.Fatal("expected a match against the soft-deleted version")
	}
	if !second.DuplicateOf.IsDeletedVersion {
		t.Errorf("expected IsDeletedVersion=true")
	}
}

func TestUploadService_Duplicate_ModeLookupFails_DefaultsToWarnNotFatal(t *testing.T) {
	fake := &fakeDuplicateService{modeErr: errors.New("boom")}
	svc, _, _ := newDuplicateTestUploadSvc(t, "ws_mode_fail", "", fake)

	dto, err := svc.Ingest(context.Background(), "ws_mode_fail", strings.NewReader("bytes"), service.UploadMeta{
		OriginalFilename: "a.jpg",
	})
	if err != nil {
		t.Fatalf("expected upload to succeed despite mode lookup failure, got: %v", err)
	}
	if dto.ID == "" {
		t.Error("expected asset to be created")
	}
}
