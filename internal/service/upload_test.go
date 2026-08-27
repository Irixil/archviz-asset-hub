package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/media/ingest"
	"damask/server/internal/queue"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
)

func newUploadSvcSpy(t *testing.T) (service.UploadService, *spyWriter) {
	t.Helper()
	database, err := dbpkg.Open(t.TempDir() + "/upload_spy.db")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	stor, _ := storage.NewAferoMemoryStorage()
	spy := newSpy()
	q := queue.New(queries, 1)
	ingester := service.NewAssetIngester(
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
	)
	return service.NewUploadService(ingester, spy, nil), spy
}

func newUploadSvc(t *testing.T) service.UploadService {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	q := queue.New(queries, 1)
	ingester := service.NewAssetIngester(
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
	)
	return service.NewUploadService(ingester, audit.NopWriter{}, nil)
}

func waitForTriggerCount(t *testing.T, spy *triggerSpy, want int) { //nolint:unparam // readability
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if spy.count() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d trigger calls, got %d", want, spy.count())
}

// -- Validate inputs --

func TestUploadService_Ingest_EmptyWorkspace(t *testing.T) {
	svc := newUploadSvc(t)
	_, err := svc.Ingest(context.Background(), "", strings.NewReader("data"), service.UploadMeta{
		OriginalFilename: "test.txt",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty workspaceID, got %v", err)
	}
}

func TestUploadService_Ingest_EmptyFilename(t *testing.T) {
	svc := newUploadSvc(t)
	_, err := svc.Ingest(context.Background(), "ws_1", strings.NewReader("data"), service.UploadMeta{
		OriginalFilename: "",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty filename, got %v", err)
	}
}

// -- Happy path --

func TestUploadService_Ingest_OK(t *testing.T) {
	stor, _ := storage.NewAferoMemoryStorage()
	database, err := dbpkg.Open(t.TempDir() + "/upload_test.db")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx := context.Background()
	wsID := "ws_upload"
	userID := "user_upload"
	if _, wsErr := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "test"}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	if _, usrErr := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{ID: userID, Email: "u@t.com", PasswordHash: "x", Name: "t"},
	); usrErr != nil {
		t.Fatalf("seed user: %v", usrErr)
	}

	q2 := queue.New(queries, 1)
	svc := service.NewUploadService(
		service.NewAssetIngester(
			reposqlc.NewAssetRepo(database),
			reposqlc.NewVersionRepo(database),
			stor,
			q2,
			ingest.NewRegistry(transform.NewTransformer()),
			service.NewAutoTagService(
				reposqlc.NewAssetRepo(database),
				reposqlc.NewWorkspaceRepo(database),
				reposqlc.NewAutoTagSuggestionRepo(database),
				q2,
				nil,
				nil,
			),
			nil,
		),
		audit.NopWriter{},
		nil,
	)

	dto, err := svc.Ingest(ctx, wsID, strings.NewReader("fake image bytes"), service.UploadMeta{
		OriginalFilename: "photo.jpg",
		UserID:           userID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.OriginalFilename != "photo.jpg" {
		t.Errorf("OriginalFilename: got %q, want %q", dto.OriginalFilename, "photo.jpg")
	}
	if dto.WorkspaceID != wsID {
		t.Errorf("WorkspaceID: got %q, want %q", dto.WorkspaceID, wsID)
	}
	if dto.ID == "" {
		t.Errorf("expected non-empty asset ID")
	}
}

// --- Audit events ---

func TestUploadService_Ingest_EmitsAuditEvent(t *testing.T) {
	var svc service.UploadService
	_, spy := newUploadSvcSpy(t)

	database, err := dbpkg.Open(t.TempDir() + "/upload_audit.db")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx := context.Background()
	wsID := "ws_audit"
	userID := "user_audit"
	if _, wsErr := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "test"}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	if _, usrErr := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{ID: userID, Email: "a@t.com", PasswordHash: "x", Name: "t"},
	); usrErr != nil {
		t.Fatalf("seed user: %v", usrErr)
	}

	// Rebuild svc with the seeded DB so the workspace FK constraint passes.
	stor, _ := storage.NewAferoMemoryStorage()
	q := queue.New(queries, 1)
	svc = service.NewUploadService(
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
		spy,
		nil,
	)

	_, err = svc.Ingest(ctx, wsID, strings.NewReader("bytes"), service.UploadMeta{
		OriginalFilename: "shot.jpg",
		UserID:           userID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetCreated {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetCreated)
	}
	if e.WorkspaceID != wsID {
		t.Errorf("WorkspaceID: got %q, want %q", e.WorkspaceID, wsID)
	}
}

func TestUploadService_Ingest_DispatchesWorkflowTrigger(t *testing.T) {
	database, err := dbpkg.Open(t.TempDir() + "/upload_trigger.db")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx := context.Background()
	wsID := "ws_trigger"
	userID := "user_trigger"
	if _, wsErr := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "test"}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	if _, usrErr := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{ID: userID, Email: "t@t.com", PasswordHash: "x", Name: "t"},
	); usrErr != nil {
		t.Fatalf("seed user: %v", usrErr)
	}

	stor, _ := storage.NewAferoMemoryStorage()
	q := queue.New(queries, 1)
	triggers := &triggerSpy{}
	svc := service.NewUploadService(
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
		triggers,
	)

	asset, err := svc.Ingest(ctx, wsID, strings.NewReader("trigger-bytes"), service.UploadMeta{
		OriginalFilename: "trigger.jpg",
		UserID:           userID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForTriggerCount(t, triggers, 1)
	call := triggers.last()
	if call.eventType != "trigger.asset_created" {
		t.Fatalf("eventType: got %q", call.eventType)
	}
	if got := call.data["asset_id"]; got != asset.ID {
		t.Fatalf("asset_id: got %v want %s", got, asset.ID)
	}
	if got := call.data["original_filename"]; got != "trigger.jpg" {
		t.Fatalf("original_filename: got %v", got)
	}
}

func TestUploadService_Ingest_TriggerData_NilProjectAndFolder(t *testing.T) {
	database, err := dbpkg.Open(t.TempDir() + "/upload_nil_proj.db")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx := context.Background()
	wsID := "ws_nil_proj"
	userID := "usr_nil_proj"
	if _, wsErr := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "test"}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	if _, usrErr := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{ID: userID, Email: "np@t.com", PasswordHash: "x", Name: "t"},
	); usrErr != nil {
		t.Fatalf("seed user: %v", usrErr)
	}

	stor, _ := storage.NewAferoMemoryStorage()
	q := queue.New(queries, 1)
	triggers := &triggerSpy{}
	svc := service.NewUploadService(
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
		triggers,
	)

	// Upload with no project/folder — ProjectID and FolderID will be nil on the asset.
	_, err = svc.Ingest(ctx, wsID, strings.NewReader("bytes"), service.UploadMeta{
		OriginalFilename: "nilproj.jpg",
		UserID:           userID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForTriggerCount(t, triggers, 1)
	call := triggers.last()
	if got, ok := call.data["project_id"]; !ok || got != "" {
		t.Fatalf("project_id: got %v (ok=%v), want empty string", got, ok)
	}
	if got, ok := call.data["folder_id"]; !ok || got != "" {
		t.Fatalf("folder_id: got %v (ok=%v), want empty string", got, ok)
	}
}
