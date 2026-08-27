package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/media/ingest"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
)

func newVersionSvc(t *testing.T) (service.VersionService, *memory.RealVersionRepo) {
	t.Helper()
	repo := memory.NewRealVersionRepo()
	return service.NewVersionService(repo, audit.NopWriter{}), repo
}

func newVersionSvcSpy(t *testing.T) (service.VersionService, *memory.RealVersionRepo, *spyWriter) {
	t.Helper()
	spy := newSpy()
	repo := memory.NewRealVersionRepo()
	return service.NewVersionService(repo, spy), repo, spy
}

// --- List ---

func TestVersionService_List_Empty(t *testing.T) {
	svc, _ := newVersionSvc(t)
	out, err := svc.List(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty list, got %d", len(out))
	}
}

func TestVersionService_List_ByAsset(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(
		repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1"},
		repository.AssetVersion{ID: "v2", AssetID: "ast_2", WorkspaceID: "ws_1"},
	)
	out, err := svc.List(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].ID != "v1" {
		t.Errorf("expected [v1], got %v", out)
	}
}

// --- Get ---

func TestVersionService_Get_NotFound(t *testing.T) {
	svc, _ := newVersionSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVersionService_Get_WrongWorkspace(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_A"})
	_, err := svc.Get(context.Background(), "ws_B", "v1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong workspace, got %v", err)
	}
}

// --- Delete ---

func TestVersionService_Delete_NotFound(t *testing.T) {
	svc, _ := newVersionSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVersionService_Delete_CurrentVersion(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: true,
	})
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "v1")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (current version), got %v", err)
	}
}

func TestVersionService_Delete_AssetMismatch(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_OTHER", WorkspaceID: "ws_1", IsCurrent: false,
	})
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "v1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound (wrong asset), got %v", err)
	}
}

func TestVersionService_Delete_ReferencedAsCover(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: false,
	})
	repo.MarkAsCover("v1")
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "v1")
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict (cover), got %v", err)
	}
}

func TestVersionService_Delete_OK(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: false,
	})
	if err := svc.Delete(context.Background(), "ws_1", "ast_1", "v1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, err := repo.GetByID(context.Background(), "v1")
	if err != nil {
		t.Fatalf("unexpected error getting soft-deleted version: %v", err)
	}
	if v.DeletedAt == nil {
		t.Error("expected DeletedAt to be set after soft-delete")
	}
}

// --- Audit events ---

func TestVersionService_Delete_EmitsAuditEvent(t *testing.T) {
	svc, repo, spy := newVersionSvcSpy(t)
	repo.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: false,
	})
	if err := svc.Delete(context.Background(), "ws_1", "ast_1", "v1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetVersionDeleted {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetVersionDeleted)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}

func TestVersionService_WriteVersionUploaded_EmitsAuditEvent(t *testing.T) {
	svc, _, spy := newVersionSvcSpy(t)
	v := &service.VersionDTO{ID: "v1", VersionNum: 2, Size: 1024}
	svc.WriteVersionUploaded(context.Background(), "ws_1", "ast_1", v, "initial upload")
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetVersionUploaded {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetVersionUploaded)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}

func TestVersionService_WriteVersionRestored_EmitsAuditEvent(t *testing.T) {
	svc, _, spy := newVersionSvcSpy(t)
	svc.WriteVersionRestored(context.Background(), "ws_1", "ast_1", 3, 1)
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetVersionRestored {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetVersionRestored)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}

func TestVersionService_UploadNewVersion_DispatchesWorkflowTrigger(t *testing.T) {
	database, err := dbpkg.Open(t.TempDir() + "/version_upload.db")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx := context.Background()
	wsID := "ws_upload"
	userID := "usr_upload"
	if _, wsErr := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "test"}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	if _, usrErr := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{ID: userID, Email: "u@example.com", PasswordHash: "x", Name: "u"},
	); usrErr != nil {
		t.Fatalf("seed user: %v", usrErr)
	}

	stor, _ := storage.NewAferoMemoryStorage()
	q := queue.New(queries, 1)
	uploadSvc := service.NewUploadService(
		service.NewAssetIngester(
			reposqlc.NewAssetRepo(database),
			reposqlc.NewVersionRepo(database),
			stor,
			q,
			ingest.NewRegistry(transform.NewTransformer()),
			service.NewAutoTagService(
				reposqlc.NewAssetRepo(database),
				reposqlc.NewWorkspaceRepo(database),
				reposqlc.NewAutoTagSuggestionRepo(database),
				q,
				nil,
				nil,
			),
			nil,
		),
		audit.NopWriter{},
		nil,
	)
	asset, err := uploadSvc.Ingest(ctx, wsID, strings.NewReader("first-version"), service.UploadMeta{
		OriginalFilename: "photo.jpg",
		UserID:           userID,
	})
	if err != nil {
		t.Fatalf("seed asset upload: %v", err)
	}

	triggers := &triggerSpy{}
	versionSvc := service.NewVersionService(
		reposqlc.NewVersionRepo(database),
		audit.NopWriter{},
		service.VersionServiceDeps{
			Assets:   reposqlc.NewAssetRepo(database),
			Storage:  stor,
			Queue:    q,
			Media:    ingest.NewRegistry(transform.NewTransformer()),
			Triggers: triggers,
		},
	)

	result, err := versionSvc.UploadNewVersion(ctx, service.UploadAssetVersionParams{
		WorkspaceID: wsID,
		AssetID:     asset.ID,
		Filename:    "photo-v2.jpg",
		ContentType: "image/jpeg",
		Comment:     "second version",
		UserID:      userID,
		Reader:      strings.NewReader("second-version"),
	})
	if err != nil {
		t.Fatalf("UploadNewVersion: %v", err)
	}
	if result.Version == nil || result.Asset == nil {
		t.Fatalf("expected asset and version in result")
	}

	waitForTriggerCount(t, triggers, 1)
	call := triggers.last()
	if call.eventType != "trigger.version_uploaded" {
		t.Fatalf("eventType: got %q", call.eventType)
	}
	if got := call.data["asset_id"]; got != asset.ID {
		t.Fatalf("asset_id: got %v want %s", got, asset.ID)
	}
	if got := call.data["version_id"]; got != result.Version.ID {
		t.Fatalf("version_id: got %v want %s", got, result.Version.ID)
	}
}

func TestVersionService_UploadNewVersion_TriggerData_NilProjectAndFolder(t *testing.T) {
	database, err := dbpkg.Open(t.TempDir() + "/version_nil_proj.db")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx := context.Background()
	wsID := "ws_ver_nil"
	userID := "usr_ver_nil"
	if _, wsErr := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "test"}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	if _, usrErr := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{ID: userID, Email: "vn@example.com", PasswordHash: "x", Name: "vn"},
	); usrErr != nil {
		t.Fatalf("seed user: %v", usrErr)
	}

	stor, _ := storage.NewAferoMemoryStorage()
	q := queue.New(queries, 1)
	// Upload asset without project/folder → both will be nil.
	uploadSvc := service.NewUploadService(
		service.NewAssetIngester(
			reposqlc.NewAssetRepo(database),
			reposqlc.NewVersionRepo(database),
			stor,
			q,
			ingest.NewRegistry(transform.NewTransformer()),
			service.NewAutoTagService(
				reposqlc.NewAssetRepo(database),
				reposqlc.NewWorkspaceRepo(database),
				reposqlc.NewAutoTagSuggestionRepo(database),
				q,
				nil,
				nil,
			),
			nil,
		),
		audit.NopWriter{},
		nil,
	)
	asset, err := uploadSvc.Ingest(ctx, wsID, strings.NewReader("v1"), service.UploadMeta{
		OriginalFilename: "doc.pdf",
		UserID:           userID,
	})
	if err != nil {
		t.Fatalf("seed upload: %v", err)
	}

	triggers := &triggerSpy{}
	versionSvc := service.NewVersionService(
		reposqlc.NewVersionRepo(database),
		audit.NopWriter{},
		service.VersionServiceDeps{
			Assets:   reposqlc.NewAssetRepo(database),
			Storage:  stor,
			Queue:    q,
			Media:    ingest.NewRegistry(transform.NewTransformer()),
			Triggers: triggers,
		},
	)

	_, err = versionSvc.UploadNewVersion(ctx, service.UploadAssetVersionParams{
		WorkspaceID: wsID,
		AssetID:     asset.ID,
		Filename:    "doc-v2.pdf",
		ContentType: "application/pdf",
		UserID:      userID,
		Reader:      strings.NewReader("v2"),
	})
	if err != nil {
		t.Fatalf("UploadNewVersion: %v", err)
	}

	waitForTriggerCount(t, triggers, 1)
	call := triggers.last()
	if call.eventType != "trigger.version_uploaded" {
		t.Fatalf("eventType: got %q", call.eventType)
	}
	if got, ok := call.data["project_id"]; !ok || got != "" {
		t.Fatalf("project_id: got %v (ok=%v), want empty string", got, ok)
	}
	if got, ok := call.data["folder_id"]; !ok || got != "" {
		t.Fatalf("folder_id: got %v (ok=%v), want empty string", got, ok)
	}
}

func TestVersionService_UploadNewVersion_IgnoresDispatchError(t *testing.T) {
	database, err := dbpkg.Open(t.TempDir() + "/version_upload_dispatch_err.db")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx := context.Background()
	wsID := "ws_upload_err"
	userID := "usr_upload_err"
	if _, wsErr := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "test"}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	if _, usrErr := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{ID: userID, Email: "u2@example.com", PasswordHash: "x", Name: "u2"},
	); usrErr != nil {
		t.Fatalf("seed user: %v", usrErr)
	}

	stor, _ := storage.NewAferoMemoryStorage()
	q := queue.New(queries, 1)
	uploadSvc := service.NewUploadService(
		service.NewAssetIngester(
			reposqlc.NewAssetRepo(database),
			reposqlc.NewVersionRepo(database),
			stor,
			q,
			ingest.NewRegistry(transform.NewTransformer()),
			service.NewAutoTagService(
				reposqlc.NewAssetRepo(database),
				reposqlc.NewWorkspaceRepo(database),
				reposqlc.NewAutoTagSuggestionRepo(database),
				q,
				nil,
				nil,
			),
			nil,
		),
		audit.NopWriter{},
		nil,
	)
	asset, err := uploadSvc.Ingest(ctx, wsID, strings.NewReader("first-version"), service.UploadMeta{
		OriginalFilename: "photo.jpg",
		UserID:           userID,
	})
	if err != nil {
		t.Fatalf("seed asset upload: %v", err)
	}

	triggers := &triggerSpy{err: errors.New("boom")}
	versionSvc := service.NewVersionService(
		reposqlc.NewVersionRepo(database),
		audit.NopWriter{},
		service.VersionServiceDeps{
			Assets:   reposqlc.NewAssetRepo(database),
			Storage:  stor,
			Queue:    q,
			Media:    ingest.NewRegistry(transform.NewTransformer()),
			Triggers: triggers,
		},
	)

	if _, uploadErr := versionSvc.UploadNewVersion(ctx, service.UploadAssetVersionParams{
		WorkspaceID: wsID,
		AssetID:     asset.ID,
		Filename:    "photo-v2.jpg",
		ContentType: "image/jpeg",
		UserID:      userID,
		Reader:      strings.NewReader("second-version"),
	}); uploadErr != nil {
		t.Fatalf("UploadNewVersion should ignore dispatch errors: %v", uploadErr)
	}
}

// --- GetCurrentByAsset ---

func TestVersionService_GetCurrentByAsset_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newVersionSvc(t)
	_, err := svc.GetCurrentByAsset(context.Background(), "ast_nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVersionService_GetCurrentByAsset_OK(t *testing.T) {
	t.Parallel()
	svc, repo := newVersionSvc(t)
	repo.Seed(
		repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: true},
		repository.AssetVersion{ID: "v2", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: false},
	)
	dto, err := svc.GetCurrentByAsset(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.ID != "v1" {
		t.Errorf("ID: got %q, want v1", dto.ID)
	}
}

// --- GetFirstByAsset ---

func TestVersionService_GetFirstByAsset_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newVersionSvc(t)
	_, err := svc.GetFirstByAsset(context.Background(), "ast_nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVersionService_GetFirstByAsset_OK(t *testing.T) {
	t.Parallel()
	svc, repo := newVersionSvc(t)
	repo.Seed(
		repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", VersionNum: 1},
		repository.AssetVersion{ID: "v2", AssetID: "ast_1", WorkspaceID: "ws_1", VersionNum: 2},
	)
	dto, err := svc.GetFirstByAsset(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.ID != "v1" {
		t.Errorf("ID: got %q, want v1 (lowest version num)", dto.ID)
	}
}

// --- ListWithVariantCount ---

func TestVersionService_ListWithVariantCount_OK(t *testing.T) {
	t.Parallel()
	svc, repo := newVersionSvc(t)
	repo.Seed(
		repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1"},
		repository.AssetVersion{ID: "v2", AssetID: "ast_1", WorkspaceID: "ws_1"},
	)
	rows, err := svc.ListWithVariantCount(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 versions, got %d", len(rows))
	}
}

// --- GetByHash ---

func TestVersionService_GetByHash_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newVersionSvc(t)
	_, err := svc.GetByHash(context.Background(), "ast_1", "deadbeef")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVersionService_GetByHash_OK(t *testing.T) {
	t.Parallel()
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", ContentHash: "abc123"})
	dto, err := svc.GetByHash(context.Background(), "ast_1", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.ID != "v1" {
		t.Errorf("ID: got %q, want v1", dto.ID)
	}
}

// --- NextVersionNum ---

func TestVersionService_NextVersionNum_Empty(t *testing.T) {
	t.Parallel()
	svc, _ := newVersionSvc(t)
	n, err := svc.NextVersionNum(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 for empty asset, got %d", n)
	}
}

func TestVersionService_NextVersionNum_WithExisting(t *testing.T) {
	t.Parallel()
	svc, repo := newVersionSvc(t)
	repo.Seed(
		repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", VersionNum: 1},
		repository.AssetVersion{ID: "v2", AssetID: "ast_1", WorkspaceID: "ws_1", VersionNum: 2},
	)
	n, err := svc.NextVersionNum(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}
