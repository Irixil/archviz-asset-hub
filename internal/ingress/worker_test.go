package ingress

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
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

const testAppSecret = "test-app-secret-for-tests!!"

// stubIngester satisfies assetio.Ingester for tests that exercise HandleFetch.
// It inserts a minimal asset row so FK constraints on asset_id pass.
type stubIngester struct{ queries *dbgen.Queries }

func (s stubIngester) IngestFile(
	ctx context.Context,
	workspaceID, _ string,
	_ assetio.IngestFileOpts,
) (assetio.AssetSummary, error) {
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
	}, nil
}

// pollSource is a fakeSource that returns a fixed list of items from Poll
// and serves file content from Fetch.
type pollSource struct {
	typ   string
	items []IngestItem
}

func (p *pollSource) Type() string                     { return p.typ }
func (p *pollSource) Validate(_ context.Context) error { return nil }
func (p *pollSource) Poll(_ context.Context) ([]IngestItem, error) {
	return p.items, nil
}
func (p *pollSource) Fetch(_ context.Context, item IngestItem) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("fetched:" + item.RemoteID)), nil
}

func setupWorkerTest(t *testing.T) (*Worker, *dbgen.Queries) {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	queries, sqlDB := database.WQ, database.Writer
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	cfg := &config.Config{AppSecret: testAppSecret}
	q := queue.New(queries, 1)
	w := NewWorker(
		queries,
		sqlDB,
		stor,
		q,
		cfg,
		audit.New(sqlDB),
		mail.NewMailer(&mail.Config{}),
		stubIngester{queries: queries},
		nil,
	)
	return w, queries
}

// insertWorkspaceAndSource inserts the minimum rows needed for HandleFetch:
// a workspace, a user, and an ingress source with the given label.
func insertWorkspaceAndSource(t *testing.T, queries *dbgen.Queries, label string) (workspaceID, sourceID string) {
	t.Helper()
	ctx := context.Background()
	workspaceID = uuid.NewString()
	userID := uuid.NewString()
	sourceID = uuid.NewString()

	// Insert workspace
	_, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// Insert user (required by ingress_sources.created_by FK)
	_, err = queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID:           userID,
		Email:        "worker-test@example.com",
		PasswordHash: "x",
		Name:         "Worker Test",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Encrypt an empty config for a fake source type
	Register("fake_worker_test", func(_ []byte) (Source, error) {
		return &fakeSource{typ: "fake_worker_test"}, nil
	})
	configJSON, err := EncryptConfig(testAppSecret, []byte(`{}`))
	if err != nil {
		t.Fatalf("encrypt config: %v", err)
	}

	_, err = queries.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID:              sourceID,
		WorkspaceID:     workspaceID,
		CreatedBy:       userID,
		Type:            "fake_worker_test",
		Label:           label,
		Config:          configJSON,
		PublicToken:     uuid.NewString(),
		Enabled:         1,
		PollIntervalMin: 60,
	})
	if err != nil {
		t.Fatalf("create ingress source: %v", err)
	}

	return workspaceID, sourceID
}

// insertWorkspaceAndPollSource is like insertWorkspaceAndSource but registers
// a pollSource that returns items and can serve Fetch content.
func insertWorkspaceAndPollSource(
	t *testing.T,
	queries *dbgen.Queries,
	label string,
	items []IngestItem,
) (workspaceID, sourceID string) {
	t.Helper()
	ctx := context.Background()
	workspaceID = uuid.NewString()
	userID := uuid.NewString()
	sourceID = uuid.NewString()

	_, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	_, err = queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID:           userID,
		Email:        "poll-test@example.com",
		PasswordHash: "x",
		Name:         "Poll Test",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	typ := "fake_poll_source_" + sourceID
	Register(typ, func(_ []byte) (Source, error) {
		return &pollSource{typ: typ, items: items}, nil
	})
	configJSON, err := EncryptConfig(testAppSecret, []byte(`{}`))
	if err != nil {
		t.Fatalf("encrypt config: %v", err)
	}

	_, err = queries.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID:              sourceID,
		WorkspaceID:     workspaceID,
		CreatedBy:       userID,
		Type:            typ,
		Label:           label,
		Config:          configJSON,
		PublicToken:     uuid.NewString(),
		Enabled:         1,
		PollIntervalMin: 60,
	})
	if err != nil {
		t.Fatalf("create ingress source: %v", err)
	}

	return workspaceID, sourceID
}

func TestHandleFetch_TagsAssetWithSourceLabel(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	const label = "my-source-label"
	workspaceID, sourceID := insertWorkspaceAndSource(t, queries, label)

	// Insert a log entry
	entryID := uuid.NewString()
	entry, err := queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: sourceID,
		RemoteID: "remote-1",
		Filename: "test.txt",
	})
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	// Write a real temp file so HandleFetch can use TmpPath
	tmp, err := os.CreateTemp(t.TempDir(), "worker-test-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err = tmp.WriteString("hello world"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	_ = tmp.Close()

	payload, _ := json.Marshal(FetchJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    "remote-1",
		Filename:    "test.txt",
		TmpPath:     tmp.Name(),
	})

	job := dbgen.Job{Payload: string(payload)}
	if err = w.HandleFetch(ctx, job); err != nil {
		t.Fatalf("HandleFetch: %v", err)
	}

	// Verify the log entry was marked imported and has an asset ID
	updated, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if updated.Status != "imported" {
		t.Fatalf("expected status 'imported', got %q", updated.Status)
	}
	if updated.AssetID == nil || *updated.AssetID == "" {
		t.Fatal("expected non-nil asset_id on log entry")
	}

	// Verify the asset has the source label as a tag
	tags, err := queries.GetTagsForAsset(ctx, *updated.AssetID)
	if err != nil {
		t.Fatalf("get tags for asset: %v", err)
	}
	for _, tag := range tags {
		if tag.Name == label {
			return // found
		}
	}
	t.Fatalf("expected tag %q on asset %s, got %v", label, *updated.AssetID, tags)
}

func TestHandleFetch_Idempotent_AlreadyImported(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	workspaceID, sourceID := insertWorkspaceAndSource(t, queries, "label")

	entryID := uuid.NewString()
	entry, err := queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: sourceID,
		RemoteID: "remote-idem",
		Filename: "idem.txt",
	})
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	// Mark it already imported — no real asset needed, just a non-pending status
	if err = queries.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status: "imported",
		ID:     entry.ID,
	}); err != nil {
		t.Fatalf("pre-mark imported: %v", err)
	}

	payload, _ := json.Marshal(FetchJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    "remote-idem",
		Filename:    "idem.txt",
	})

	// HandleFetch should return nil without doing anything
	if err = w.HandleFetch(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandleFetch on already-imported entry should not error: %v", err)
	}

	// Status must still be "imported" (unchanged)
	after, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if after.Status != "imported" {
		t.Fatalf("expected status 'imported', got %q", after.Status)
	}
}

func TestHandleFetch_DenyRule_MarksSkipped(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	workspaceID, sourceID := insertWorkspaceAndSource(t, queries, "deny-label")

	// Add a deny rule matching *.exe filenames
	_, err := queries.CreateIngressRule(ctx, dbgen.CreateIngressRuleParams{
		ID:       uuid.NewString(),
		SourceID: sourceID,
		Position: 1,
		Field:    "filename",
		Operator: "ends_with",
		Value:    ".exe",
		Action:   "deny",
	})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	entryID := uuid.NewString()
	entry, err := queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: sourceID,
		RemoteID: "remote-exe",
		Filename: "malware.exe",
	})
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	tmp, err := os.CreateTemp(t.TempDir(), "worker-deny-*.exe")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	_, _ = tmp.WriteString("MZ")
	_ = tmp.Close()

	payload, _ := json.Marshal(FetchJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    "remote-exe",
		Filename:    "malware.exe",
		TmpPath:     tmp.Name(),
	})

	if err = w.HandleFetch(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandleFetch: %v", err)
	}

	after, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if after.Status != "skipped" {
		t.Fatalf("expected status 'skipped', got %q", after.Status)
	}
	if after.AssetID != nil {
		t.Fatal("expected no asset_id on skipped entry")
	}
}

func TestHandleFetch_PullSource_FetchesViaSource(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	const typ = "fake_pull_fetch"
	Register(typ, func(_ []byte) (Source, error) {
		return &pollSource{typ: typ}, nil
	})

	userID := uuid.NewString()
	workspaceID := uuid.NewString()
	sourceID := uuid.NewString()

	_, _ = queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: workspaceID, Name: "WS"})
	_, _ = queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{ID: userID, Email: "pull@example.com", PasswordHash: "x", Name: "Pull"},
	)
	configJSON, _ := EncryptConfig(testAppSecret, []byte(`{}`))
	_, _ = queries.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID:              sourceID,
		WorkspaceID:     workspaceID,
		CreatedBy:       userID,
		Type:            typ,
		Label:           "pull-label",
		Config:          configJSON,
		PublicToken:     uuid.NewString(),
		Enabled:         1,
		PollIntervalMin: 60,
	})

	entryID := uuid.NewString()
	entry, err := queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: sourceID,
		RemoteID: "pull-remote-1",
		Filename: "pulled.txt",
	})
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	// No TmpPath — worker must call source.Fetch()
	payload, _ := json.Marshal(FetchJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    "pull-remote-1",
		Filename:    "pulled.txt",
	})

	if err = w.HandleFetch(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandleFetch (pull): %v", err)
	}

	after, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if after.Status != "imported" {
		t.Fatalf("expected status 'imported', got %q", after.Status)
	}
	if after.AssetID == nil || *after.AssetID == "" {
		t.Fatal("expected asset_id on pulled entry")
	}
}

// captureSource records the IngestItem passed to Fetch so tests can inspect it.
type captureSource struct {
	typ          string
	items        []IngestItem
	capturedItem *IngestItem
}

func (s *captureSource) Type() string                     { return s.typ }
func (s *captureSource) Validate(_ context.Context) error { return nil }
func (s *captureSource) Poll(_ context.Context) ([]IngestItem, error) {
	return s.items, nil
}
func (s *captureSource) Fetch(_ context.Context, item IngestItem) (io.ReadCloser, error) {
	*s.capturedItem = item
	return io.NopCloser(strings.NewReader("data")), nil
}

func TestHandleFetch_MetaPassedToSource(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	var captured IngestItem
	typ := "meta_capture_" + uuid.NewString()
	src := &captureSource{
		typ: typ,
		items: []IngestItem{{
			RemoteID: "r1",
			Filename: "file.jpg",
			Meta:     map[string]string{"file_id": "abc123", "mime_type": "image/jpeg"},
		}},
		capturedItem: &captured,
	}
	Register(typ, func(_ []byte) (Source, error) { return src, nil })

	userID := uuid.NewString()
	workspaceID := uuid.NewString()
	sourceID := uuid.NewString()
	_, _ = queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: workspaceID, Name: "WS"})
	_, _ = queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{ID: userID, Email: "meta@example.com", PasswordHash: "x", Name: "Meta"},
	)
	configJSON, _ := EncryptConfig(testAppSecret, []byte(`{}`))
	_, _ = queries.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID:              sourceID,
		WorkspaceID:     workspaceID,
		CreatedBy:       userID,
		Type:            typ,
		Label:           "meta-label",
		Config:          configJSON,
		PublicToken:     uuid.NewString(),
		Enabled:         1,
		PollIntervalMin: 60,
	})

	pollPayload, _ := json.Marshal(PollJobPayload{SourceID: sourceID, WorkspaceID: workspaceID})
	if err := w.HandlePoll(ctx, dbgen.Job{Payload: string(pollPayload)}); err != nil {
		t.Fatalf("HandlePoll: %v", err)
	}

	fetchJob, err := queries.ClaimNextJob(ctx, queue.NoExcludedTypes)
	if err != nil {
		t.Fatalf("ClaimNextJob: %v", err)
	}
	if fetchJob.Type != queue.JobTypeIngestFetch {
		t.Fatalf("expected job type %q, got %q", queue.JobTypeIngestFetch, fetchJob.Type)
	}

	if err = w.HandleFetch(ctx, fetchJob); err != nil {
		t.Fatalf("HandleFetch: %v", err)
	}

	if captured.Meta["file_id"] != "abc123" {
		t.Errorf("Meta[file_id] = %q, want %q", captured.Meta["file_id"], "abc123")
	}
	if captured.Meta["mime_type"] != "image/jpeg" {
		t.Errorf("Meta[mime_type] = %q, want %q", captured.Meta["mime_type"], "image/jpeg")
	}
}

func TestHandleFetch_BadPayload_ReturnsError(t *testing.T) {
	t.Parallel()
	w, _ := setupWorkerTest(t)
	ctx := context.Background()

	err := w.HandleFetch(ctx, dbgen.Job{Payload: "not-json"})
	if err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
}

func TestHandlePoll_EnqueuesItemsFromSource(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	items := []IngestItem{
		{RemoteID: "r1", Filename: "file1.jpg"},
		{RemoteID: "r2", Filename: "file2.png"},
	}
	workspaceID, sourceID := insertWorkspaceAndPollSource(t, queries, "poll-label", items)

	payload, _ := json.Marshal(PollJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
	})

	if err := w.HandlePoll(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandlePoll: %v", err)
	}

	log, err := queries.ListIngressSourceLog(ctx, dbgen.ListIngressSourceLogParams{
		SourceID: sourceID,
		Limit:    10,
		Offset:   0,
	})
	if err != nil {
		t.Fatalf("list log: %v", err)
	}
	if len(log) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(log))
	}
}

func TestHandlePoll_DuplicateItem_IsSkipped(t *testing.T) {
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	items := []IngestItem{
		{RemoteID: "dup-1", Filename: "dup.jpg"},
	}
	workspaceID, sourceID := insertWorkspaceAndPollSource(t, queries, "dedup-label", items)

	payload, _ := json.Marshal(PollJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
	})

	// First poll — creates log entry
	if err := w.HandlePoll(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("first HandlePoll: %v", err)
	}

	// Second poll — same remote_id must be deduplicated
	if err := w.HandlePoll(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("second HandlePoll: %v", err)
	}

	log, err := queries.ListIngressSourceLog(ctx, dbgen.ListIngressSourceLogParams{
		SourceID: sourceID,
		Limit:    10,
		Offset:   0,
	})
	if err != nil {
		t.Fatalf("list log: %v", err)
	}
	if len(log) != 1 {
		t.Fatalf("expected 1 log entry after dedup, got %d", len(log))
	}
}

func TestHandlePoll_DisabledSource_DoesNothing(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	workspaceID, sourceID := insertWorkspaceAndPollSource(t, queries, "disabled-label", []IngestItem{
		{RemoteID: "x", Filename: "x.jpg"},
	})

	// Disable the source
	src, err := queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	})
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	_, _ = queries.UpdateIngressSource(ctx, dbgen.UpdateIngressSourceParams{
		Label:           src.Label,
		Config:          src.Config,
		DestFolderID:    src.DestFolderID,
		DestProjectID:   src.DestProjectID,
		Enabled:         0,
		PollIntervalMin: src.PollIntervalMin,
		ID:              src.ID,
		WorkspaceID:     src.WorkspaceID,
	})

	payload, _ := json.Marshal(PollJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
	})

	if err = w.HandlePoll(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandlePoll on disabled source: %v", err)
	}

	log, err := queries.ListIngressSourceLog(ctx, dbgen.ListIngressSourceLogParams{
		SourceID: sourceID,
		Limit:    10,
		Offset:   0,
	})
	if err != nil {
		t.Fatalf("list log: %v", err)
	}
	if len(log) != 0 {
		t.Fatalf("expected no log entries for disabled source, got %d", len(log))
	}
}

func TestHandlePoll_BadPayload_ReturnsError(t *testing.T) {
	t.Parallel()
	w, _ := setupWorkerTest(t)
	ctx := context.Background()

	err := w.HandlePoll(ctx, dbgen.Job{Payload: "not-json"})
	if err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
}

// errorPollSource always returns an error from Poll.
type errorPollSource struct{}

func (e *errorPollSource) Type() string                     { return "error_poll_source" }
func (e *errorPollSource) Validate(_ context.Context) error { return nil }
func (e *errorPollSource) Poll(_ context.Context) ([]IngestItem, error) {
	return nil, errors.New("poll failed")
}
func (e *errorPollSource) Fetch(_ context.Context, _ IngestItem) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

// insertErrorPollSource creates a workspace + source backed by errorPollSource.
func insertErrorPollSource(t *testing.T, queries *dbgen.Queries) (workspaceID, sourceID string) {
	t.Helper()
	ctx := context.Background()
	workspaceID = uuid.NewString()
	userID := uuid.NewString()
	sourceID = uuid.NewString()

	_, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: workspaceID, Name: "Err WS"})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	_, err = queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID: userID, Email: "err-" + sourceID + "@example.com", PasswordHash: "x", Name: "Err User",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	const typ = "error_poll_source"
	Register(typ, func(_ []byte) (Source, error) { return &errorPollSource{}, nil })
	configJSON, err := EncryptConfig(testAppSecret, []byte(`{}`))
	if err != nil {
		t.Fatalf("encrypt config: %v", err)
	}

	_, err = queries.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID, CreatedBy: userID,
		Type: typ, Label: "err-source", Config: configJSON,
		PublicToken: uuid.NewString(), Enabled: 1, PollIntervalMin: 60,
	})
	if err != nil {
		t.Fatalf("create ingress source: %v", err)
	}
	return workspaceID, sourceID
}

func pollSourceN(t *testing.T, w *Worker, _ *dbgen.Queries, workspaceID, sourceID string, n int) {
	t.Helper()
	ctx := context.Background()
	payload, _ := json.Marshal(PollJobPayload{SourceID: sourceID, WorkspaceID: workspaceID})
	job := dbgen.Job{Payload: string(payload)}
	for range n {
		_ = w.HandlePoll(ctx, job) // errors expected; ignore return value
	}
}

func getErrorCount(t *testing.T, queries *dbgen.Queries, workspaceID, sourceID string) int64 {
	t.Helper()
	src, err := queries.GetIngressSource(context.Background(), dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	})
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	return src.ErrorCount
}

// TestErrorCount_IncreasesOnPollFailure verifies that error_count increments
// each time HandlePoll fails, and the source still appears in due list at count=5.
func TestErrorCount_IncreasesOnPollFailure(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	workspaceID, sourceID := insertErrorPollSource(t, queries)

	pollSourceN(t, w, queries, workspaceID, sourceID, 5)

	if count := getErrorCount(t, queries, workspaceID, sourceID); count != 5 {
		t.Fatalf("expected error_count=5 after 5 failures, got %d", count)
	}

	// Source with error_count=5 must still appear in ListDueIngressSources.
	ctx := context.Background()
	// Reset last_polled_at so the source is considered due.
	if _, execErr := w.sqlDB.ExecContext(
		ctx,
		"UPDATE ingress_sources SET last_polled_at = datetime('now', '-2 hours') WHERE id = ?",
		sourceID,
	); execErr != nil {
		t.Fatalf("reset last_polled_at: %v", execErr)
	}
	due, err := queries.ListDueIngressSources(ctx)
	if err != nil {
		t.Fatalf("ListDueIngressSources: %v", err)
	}
	found := false
	for _, s := range due {
		if s.ID == sourceID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("source with error_count=5 must still appear in due list")
	}
}

// TestErrorCount_SkipsSourceAfterSixFailures verifies that a source with
// error_count > 5 is excluded from ListDueIngressSources.
func TestErrorCount_SkipsSourceAfterSixFailures(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	workspaceID, sourceID := insertErrorPollSource(t, queries)

	pollSourceN(t, w, queries, workspaceID, sourceID, 6)

	if count := getErrorCount(t, queries, workspaceID, sourceID); count != 6 {
		t.Fatalf("expected error_count=6 after 6 failures, got %d", count)
	}

	// Reset last_polled_at so the source would be due if not for error_count.
	ctx := context.Background()
	_, execErr := w.sqlDB.ExecContext(ctx,
		"UPDATE ingress_sources SET last_polled_at = datetime('now', '-2 hours') WHERE id = ?", sourceID)
	if execErr != nil {
		t.Fatalf("reset last_polled_at: %v", execErr)
	}

	due, err := queries.ListDueIngressSources(ctx)
	if err != nil {
		t.Fatalf("ListDueIngressSources: %v", err)
	}
	for _, s := range due {
		if s.ID == sourceID {
			t.Fatal("source with error_count=6 must not appear in due list")
		}
	}
}

// TestErrorCount_ResetsOnSuccess verifies that a successful poll resets error_count to 0.
func TestErrorCount_ResetsOnSuccess(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	// Use a poll source with items so HandlePoll succeeds.
	workspaceID, sourceID := insertWorkspaceAndPollSource(t, queries, "reset-label", []IngestItem{
		{RemoteID: "item1", Filename: "a.jpg"},
	})

	// Seed error_count=4 directly via SQL.
	if _, err := w.sqlDB.ExecContext(ctx,
		"UPDATE ingress_sources SET error_count = 4 WHERE id = ?", sourceID); err != nil {
		t.Fatalf("seed error_count: %v", err)
	}

	payload, _ := json.Marshal(PollJobPayload{SourceID: sourceID, WorkspaceID: workspaceID})
	if err := w.HandlePoll(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandlePoll: %v", err)
	}

	if count := getErrorCount(t, queries, workspaceID, sourceID); count != 0 {
		t.Fatalf("expected error_count=0 after successful poll, got %d", count)
	}
}

// TestErrorCount_ResetsOnSourceEdit verifies that updating a source resets error_count.
func TestErrorCount_ResetsOnSourceEdit(t *testing.T) {
	t.Parallel()
	w, queries := setupWorkerTest(t)
	ctx := context.Background()
	workspaceID, sourceID := insertErrorPollSource(t, queries)

	// Seed error_count=6 directly.
	if _, err := w.sqlDB.ExecContext(ctx,
		"UPDATE ingress_sources SET error_count = 6 WHERE id = ?", sourceID); err != nil {
		t.Fatalf("seed error_count: %v", err)
	}

	src, err := queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	})
	if err != nil {
		t.Fatalf("get source: %v", err)
	}

	// Simulate an edit (UpdateIngressSource resets error_count=0).
	_, err = queries.UpdateIngressSource(ctx, dbgen.UpdateIngressSourceParams{
		Label:           src.Label,
		Config:          src.Config,
		DestFolderID:    src.DestFolderID,
		DestProjectID:   src.DestProjectID,
		Enabled:         src.Enabled,
		PollIntervalMin: src.PollIntervalMin,
		ID:              src.ID,
		WorkspaceID:     src.WorkspaceID,
	})
	if err != nil {
		t.Fatalf("UpdateIngressSource: %v", err)
	}

	if count := getErrorCount(t, queries, workspaceID, sourceID); count != 0 {
		t.Fatalf("expected error_count=0 after edit, got %d", count)
	}
}
