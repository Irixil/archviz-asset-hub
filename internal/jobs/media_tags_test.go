package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"damask/server/internal/ai"
	"damask/server/internal/config"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/jobspec"
	"damask/server/internal/mail"
	"damask/server/internal/media/contentmeta"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	repomemory "damask/server/internal/repository/memory"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
	"damask/server/internal/workflow"
)

type noopFieldsSvc struct{}

func (noopFieldsSvc) PurgeExpiredFields(_ context.Context) (int, error) { return 0, nil }

type noopExifSvc struct{}

func (noopExifSvc) ExtractForAsset(_ context.Context, _, _, _ string) error { return nil }

type noopTagSvc struct{}

func (noopTagSvc) ApplyTag(_ context.Context, _, _, _ string) error { return nil }

type noopTextTrackSvc struct{}

func (noopTextTrackSvc) RunOCR(_ context.Context, _, _, _, _, _, _, _, _ string) (string, int, error) {
	return "", 0, nil
}
func (noopTextTrackSvc) RunExtractPDF(_ context.Context, _, _, _, _ string) error         { return nil }
func (noopTextTrackSvc) RunExtractPlain(_ context.Context, _, _, _, _ string) error       { return nil }
func (noopTextTrackSvc) RunExtractDocument(_ context.Context, _, _, _, _, _ string) error { return nil }
func (noopTextTrackSvc) CreateAudioTranscript(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}

// memExportSvc wires two memory repos into the exportService interface for tests.
type memExportSvc struct {
	configs *repomemory.ExportConfigMemoryRepo
	runs    *repomemory.ExportRunMemoryRepo
}

func newMemExportSvc(c *repomemory.ExportConfigMemoryRepo, r *repomemory.ExportRunMemoryRepo) exportService {
	return &memExportSvc{configs: c, runs: r}
}

func (s *memExportSvc) ExecuteRun(_ context.Context, _, _, _ string) error {
	panic("memExportSvc.ExecuteRun not implemented")
}
func (s *memExportSvc) ListDueConfigs(ctx context.Context) ([]repository.ExportConfig, error) {
	return s.configs.ListDue(ctx)
}
func (s *memExportSvc) CreateRun(ctx context.Context, run repository.ExportRun) (repository.ExportRun, error) {
	return s.runs.Create(ctx, run)
}
func (s *memExportSvc) SetConfigLastRun(ctx context.Context, configID string, p repository.ExportRunResult) error {
	return s.configs.SetLastRun(ctx, configID, p)
}

func newMediaTagsJobTestEnv(t *testing.T) (*dbgen.Queries, *sql.DB, *JobServer, queue.JobQueue, storage.Storage) {
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
	q := queue.New(queries, 1)
	trf := transform.NewTransformer()
	tmb := transform.NewThumbnailer(trf)
	js := NewJobServer(Deps{
		Queries:     queries,
		SQLDB:       sqlDB,
		Storage:     stor,
		Hub:         events.NewEventHub(),
		Queue:       q,
		Mailer:      mail.NewMailer(&mail.Config{}),
		Transformer: trf,
		Thumbnailer: tmb,
		Config:      &config.Config{},
		AIAPIKeyResolver: ai.KeyResolver(func(context.Context, string, string) (string, ai.KeySource, error) {
			return "", "", nil
		}),
		WorkspaceRepo: repomemory.NewRealWorkspaceRepo(),
		WorkflowExec:  workflow.NewExecutor(workflow.Deps{}),
		ExportSvc:     newMemExportSvc(repomemory.NewExportConfigRepo(), repomemory.NewExportRunRepo()),
		ExifSvc:       noopExifSvc{},
		TagSvc:        noopTagSvc{},
		FieldSvc:      noopFieldsSvc{},
		TextTrackSvc:  noopTextTrackSvc{},
	})

	if _, err = sqlDB.Exec(`INSERT INTO workspaces (id, name) VALUES ('ws_test', 'Test')`); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return queries, sqlDB, js, q, stor
}

func TestExtractMediaTags_WritesValuesAndSeedsFields(t *testing.T) {
	queries, sqlDB, js, q, stor := newMediaTagsJobTestEnv(t)

	const assetID = "asset-media-tags-1"
	const storageKey = "ws_test/audio/test.mp3"
	if err := stor.Put(t.Context(), storageKey, strings.NewReader("audio")); err != nil {
		t.Fatalf("put asset: %v", err)
	}
	if _, err := queries.CreateAsset(context.Background(), dbgen.CreateAssetParams{
		ID:               assetID,
		WorkspaceID:      "ws_test",
		OriginalFilename: "track.mp3",
		StorageKey:       storageKey,
		MimeType:         "audio/mpeg",
		Size:             5,
	}); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	orig := mediaTagExtract
	t.Cleanup(func() { mediaTagExtract = orig })
	mediaTagExtract = func(_ context.Context, _, _ string) (*contentmeta.AVTags, error) {
		title := "Track"
		codec := "mp3"
		year := 1999
		return &contentmeta.AVTags{
			Title:       &title,
			AudioCodec:  &codec,
			Year:        &year,
			HasCoverArt: true,
		}, nil
	}

	payload, _ := json.Marshal(jobspec.ExtractMediaTagsPayload{AssetID: assetID, WorkspaceID: "ws_test"})
	if _, err := q.Enqueue(
		context.Background(),
		"ws_test",
		queue.JobTypeExtractMediaTags,
		string(payload),
	); err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	js.DrainForTest(context.Background())

	var fieldCount int
	if err := sqlDB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM field_definitions WHERE workspace_id = 'ws_test' AND source = 'media_tags'`,
	).Scan(&fieldCount); err != nil {
		t.Fatalf("count field defs: %v", err)
	}
	if fieldCount == 0 {
		t.Fatal("expected seeded media tag field definitions")
	}

	var title string
	if err := sqlDB.QueryRowContext(context.Background(), `
SELECT afv.value_text
FROM asset_field_values afv
JOIN field_definitions fd ON fd.id = afv.field_id
WHERE afv.asset_id = ? AND fd.key = '_media_title'`, assetID).Scan(&title); err != nil {
		t.Fatalf("read title value: %v", err)
	}
	if title != "Track" {
		t.Fatalf("title = %q, want Track", title)
	}
}

func TestExtractMediaTags_FreshValuesSkipExtractor(t *testing.T) {
	_, sqlDB, js, q, _ := newMediaTagsJobTestEnv(t)

	if _, err := sqlDB.ExecContext(context.Background(), `
INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size, updated_at)
VALUES ('asset-media-tags-2', 'ws_test', 'track.mp3', 'ws_test/audio/fresh.mp3', 'audio/mpeg', 5, datetime('now', '-1 day'))`); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	if _, err := sqlDB.ExecContext(context.Background(), `
INSERT INTO field_definitions (id, workspace_id, source, scope, name, key, field_type, position, created_at, updated_at)
VALUES ('fd-media-title', 'ws_test', 'media_tags', 'asset', 'Title', '_media_title', 'text', 0, datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("insert field def: %v", err)
	}
	if _, err := sqlDB.ExecContext(context.Background(), `
INSERT INTO asset_field_values (id, asset_id, field_id, value_text, updated_at, created_at)
VALUES ('afv-media-title', 'asset-media-tags-2', 'fd-media-title', 'Existing', datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("insert field value: %v", err)
	}

	orig := mediaTagExtract
	t.Cleanup(func() { mediaTagExtract = orig })
	mediaTagExtract = func(context.Context, string, string) (*contentmeta.AVTags, error) {
		t.Fatal("extractor should not be called for fresh values")
		return nil, nil //nolint:nilnil // tests
	}

	payload, _ := json.Marshal(jobspec.ExtractMediaTagsPayload{AssetID: "asset-media-tags-2", WorkspaceID: "ws_test"})
	if _, err := q.Enqueue(
		context.Background(),
		"ws_test",
		queue.JobTypeExtractMediaTags,
		string(payload),
	); err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	js.DrainForTest(context.Background())
}
