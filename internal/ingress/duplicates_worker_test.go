package ingress

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"damask/server/internal/assetio"
	"damask/server/internal/audit"
	"damask/server/internal/config"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/storage"

	"github.com/google/uuid"
)

// dupStubIngester lets a test control exactly what Ingester.IngestFile
// returns, to exercise finalizeImport's duplicate handling without needing a
// real DuplicateService/DB round trip (that's already covered by
// internal/service's duplicates_ingest_test.go).
type dupStubIngester struct {
	queries *dbgen.Queries
	// duplicateOf, if set, is attached to the returned AssetSummary (warn mode).
	duplicateOf *assetio.DuplicateMatch
	// blockErr, if set, is returned instead of creating an asset (block mode).
	blockErr error
}

func (s *dupStubIngester) IngestFile(
	ctx context.Context,
	workspaceID, _ string,
	_ assetio.IngestFileOpts,
) (assetio.AssetSummary, error) {
	if s.blockErr != nil {
		return assetio.AssetSummary{}, s.blockErr
	}
	id := uuid.NewString()
	if _, err := s.queries.CreateAsset(ctx, dbgen.CreateAssetParams{
		ID:               id,
		WorkspaceID:      workspaceID,
		OriginalFilename: "stub.txt",
		StorageKey:       "stub/" + id,
		MimeType:         "text/plain",
	}); err != nil {
		return assetio.AssetSummary{}, err
	}
	return assetio.AssetSummary{
		ID:               id,
		WorkspaceID:      workspaceID,
		OriginalFilename: "stub.txt",
		MimeType:         "text/plain",
		DuplicateOf:      s.duplicateOf,
	}, nil
}

func setupDuplicateWorkerTest(t *testing.T, ingester *dupStubIngester) (*Worker, *dbgen.Queries) {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	queries, sqlDB := database.WQ, database.Writer
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	ingester.queries = queries

	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	cfg := &config.Config{AppSecret: testAppSecret}
	q := queue.New(queries, 1)
	w := NewWorker(queries, sqlDB, stor, q, cfg, audit.New(sqlDB), mail.NewMailer(&mail.Config{}), ingester, nil)
	return w, queries
}

// insertFetchEntry inserts a workspace/source/log-entry triple and a temp
// file, returning the FetchJobPayload JSON ready for HandleFetch.
func insertFetchEntry(
	t *testing.T,
	queries *dbgen.Queries,
	label string,
) (payload []byte, entryID, workspaceID, sourceID string) {
	t.Helper()
	ctx := context.Background()
	workspaceID, sourceID = insertWorkspaceAndSource(t, queries, label)

	entryID = uuid.NewString()
	entry, err := queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: sourceID,
		RemoteID: "remote-dup",
		Filename: "dup.txt",
	})
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	tmp, err := os.CreateTemp(t.TempDir(), "worker-dup-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	_, _ = tmp.WriteString("hello world")
	_ = tmp.Close()

	payload, _ = json.Marshal(FetchJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    "remote-dup",
		Filename:    "dup.txt",
		TmpPath:     tmp.Name(),
	})
	return payload, entryID, workspaceID, sourceID
}

// createExistingAsset inserts a minimal asset row so it can be referenced by
// duplicate_of_asset_id (FK-checked) in tests.
func createExistingAsset(t *testing.T, queries *dbgen.Queries, workspaceID string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := queries.CreateAsset(context.Background(), dbgen.CreateAssetParams{
		ID:               id,
		WorkspaceID:      workspaceID,
		OriginalFilename: "existing.txt",
		StorageKey:       "existing/" + id,
		MimeType:         "text/plain",
	}); err != nil {
		t.Fatalf("create existing asset: %v", err)
	}
	return id
}

func TestIngressWorker_Duplicate_ModeBlock_LogsSkipped(t *testing.T) {
	t.Parallel()
	ingester := &dupStubIngester{}
	w, queries := setupDuplicateWorkerTest(t, ingester)
	payload, entryID, workspaceID, sourceID := insertFetchEntry(t, queries, "dup-block-label")
	existingAssetID := createExistingAsset(t, queries, workspaceID)
	ingester.blockErr = &assetio.DuplicateConflictError{Match: assetio.DuplicateMatch{AssetID: existingAssetID}}

	ctx := context.Background()
	if err := w.HandleFetch(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandleFetch: %v", err)
	}

	after, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if after.Status != ingressStatusSkippedDup {
		t.Fatalf("expected status %q, got %q", ingressStatusSkippedDup, after.Status)
	}
	if after.AssetID != nil {
		t.Errorf("expected no asset_id on a skipped-duplicate entry, got %v", *after.AssetID)
	}
	if after.DuplicateOfAssetID == nil || *after.DuplicateOfAssetID != existingAssetID {
		t.Errorf("expected duplicate_of_asset_id=%q, got %v", existingAssetID, after.DuplicateOfAssetID)
	}

	// A block-mode duplicate is expected behavior, not a source malfunction —
	// it must not count against the source's error budget.
	if count := getErrorCount(t, queries, workspaceID, sourceID); count != 0 {
		t.Errorf("expected error_count=0 after a duplicate skip, got %d", count)
	}
}

func TestIngressWorker_Duplicate_ModeWarn_CreatesFlaggedAsset(t *testing.T) {
	t.Parallel()
	ingester := &dupStubIngester{}
	w, queries := setupDuplicateWorkerTest(t, ingester)
	payload, entryID, workspaceID, _ := insertFetchEntry(t, queries, "dup-warn-label")
	existingAssetID := createExistingAsset(t, queries, workspaceID)
	ingester.duplicateOf = &assetio.DuplicateMatch{AssetID: existingAssetID}

	ctx := context.Background()
	if err := w.HandleFetch(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandleFetch: %v", err)
	}

	after, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if after.Status != "imported" {
		t.Fatalf("expected status 'imported', got %q", after.Status)
	}
	if after.AssetID == nil || *after.AssetID == "" {
		t.Fatal("expected a non-nil asset_id — the asset must still be created in warn mode")
	}
	if after.DuplicateOfAssetID == nil || *after.DuplicateOfAssetID != existingAssetID {
		t.Errorf("expected duplicate_of_asset_id=%q, got %v", existingAssetID, after.DuplicateOfAssetID)
	}
}

func TestIngressWorker_NoDuplicate_Unaffected(t *testing.T) {
	t.Parallel()
	ingester := &dupStubIngester{}
	w, queries := setupDuplicateWorkerTest(t, ingester)
	payload, entryID, _, _ := insertFetchEntry(t, queries, "no-dup-label")

	ctx := context.Background()
	if err := w.HandleFetch(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandleFetch: %v", err)
	}

	after, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if after.Status != "imported" {
		t.Fatalf("expected status 'imported', got %q", after.Status)
	}
	if after.AssetID == nil || *after.AssetID == "" {
		t.Fatal("expected a non-nil asset_id")
	}
	if after.DuplicateOfAssetID != nil {
		t.Errorf("expected no duplicate_of_asset_id, got %v", *after.DuplicateOfAssetID)
	}
}
