package jobs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"damask/server/internal/ai"
	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	reposqlc "damask/server/internal/repository/sqlc"
	th "damask/server/internal/testhelpers"
	"damask/server/internal/workflow"

	"github.com/gofiber/fiber/v3"
)

// setupAutoTagAsset registers a user, uploads a JPEG asset, and returns the
// workspace ID, asset ID, and its real storage key (needed to enqueue an
// auto_tag job payload directly).
func setupAutoTagAsset(t *testing.T, env *th.TestEnv) (workspaceID, assetID, storageKey string) {
	t.Helper()
	res := th.Register(t, env, "Auto Tag User", "autotag@test.com", "password123")
	asset := th.UploadAsset(t, env, res.Cookie)
	if e := env.Database.QueryRowContext(
		context.Background(),
		`SELECT storage_key FROM assets WHERE id = ?`, asset.ID,
	).Scan(&storageKey); e != nil {
		t.Fatalf("lookup storage key: %v", e)
	}
	return res.WorkspaceID, asset.ID, storageKey
}

func enqueueAutoTag(
	t *testing.T,
	env *th.TestEnv,
	workspaceID, assetID, storageKey, mimeType, mode string,
) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"workspace_id": workspaceID,
		"asset_id":     assetID,
		"storage_key":  storageKey,
		"mime_type":    mimeType,
		"mode":         mode,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	_, err = env.JobServer.EnqueueForTest(context.Background(), workspaceID, queue.JobTypeAutoTag, string(payload))
	if err != nil {
		t.Fatalf("enqueue auto_tag: %v", err)
	}
}

// enqueueAutoTagFull mirrors enqueueAutoTag but allows setting the
// thumbnail_key / thumbnail_content_type fields directly, for tests that
// exercise the video/PDF thumbnail describe-mime path.
func enqueueAutoTagFull(
	t *testing.T,
	env *th.TestEnv,
	workspaceID, assetID, storageKey, thumbnailKey, thumbnailContentType, mimeType, mode string,
) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"workspace_id":           workspaceID,
		"asset_id":               assetID,
		"storage_key":            storageKey,
		"thumbnail_key":          thumbnailKey,
		"thumbnail_content_type": thumbnailContentType,
		"mime_type":              mimeType,
		"mode":                   mode,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	_, err = env.JobServer.EnqueueForTest(context.Background(), workspaceID, queue.JobTypeAutoTag, string(payload))
	if err != nil {
		t.Fatalf("enqueue auto_tag: %v", err)
	}
}

// setupAutoTagAudioAsset uploads a fake "audio/mpeg" asset (content sniffing
// falls back to the .mp3 extension since the bytes aren't real audio — see
// transform.DetectMimeType) and returns its workspace ID, asset ID, and
// storage key.
func setupAutoTagAudioAsset(t *testing.T, env *th.TestEnv) (workspaceID, assetID, storageKey string) {
	t.Helper()
	res := th.Register(t, env, "Auto Tag Audio User", "autotagaudio@test.com", "password123")
	req := th.BuildUploadRequest(t, "test.mp3", []byte("fake audio bytes"), res.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload audio asset: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload audio asset: expected 201, got %d", resp.StatusCode)
	}
	var asset struct {
		ID string `json:"id"`
	}
	if e := json.NewDecoder(resp.Body).Decode(&asset); e != nil {
		t.Fatalf("decode asset: %v", e)
	}
	if e := env.Database.QueryRowContext(
		context.Background(),
		`SELECT storage_key FROM assets WHERE id = ?`, asset.ID,
	).Scan(&storageKey); e != nil {
		t.Fatalf("lookup storage key: %v", e)
	}
	return res.WorkspaceID, asset.ID, storageKey
}

// audioTagModelServer mocks both OpenRouter endpoints the audio auto-tag
// path needs: speech-to-text transcription and text tagging.
func audioTagModelServer(t *testing.T, transcript, tagsJSON string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio/transcriptions":
			_ = json.NewEncoder(w).Encode(map[string]any{"text": transcript})
		case "/chat/completions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]string{"content": tagsJSON}},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestJobAutoTag_Audio_PendingMode_CreatesSuggestionsAndTranscript(t *testing.T) {
	srv := audioTagModelServer(t, "this is a podcast about go programming", `["podcast","programming"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAudioAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "audio/mpeg", "pending")
	env.JobServer.DrainForTest(context.Background())

	var suggestionCount int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&suggestionCount); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if suggestionCount != 2 {
		t.Fatalf("expected 2 suggestions, got %d", suggestionCount)
	}

	var trackCount int
	var content string
	if e := env.Database.QueryRow(
		`SELECT COUNT(*), COALESCE(MAX(content), '') FROM asset_text_tracks
		 WHERE asset_id = ? AND source = 'audio_transcript'`, assetID,
	).Scan(&trackCount, &content); e != nil {
		t.Fatalf("count text tracks: %v", e)
	}
	if trackCount != 1 {
		t.Fatalf("expected 1 audio_transcript text track, got %d", trackCount)
	}
	if content != "this is a podcast about go programming" {
		t.Fatalf("unexpected transcript content: %q", content)
	}
}

func TestJobAutoTag_Audio_OversizedBySize_SkipsWithoutReadingStorage(t *testing.T) {
	// No httptest server is started — if the job fell through to
	// storage.Get/transcription instead of rejecting on the known size, the
	// provider call would fail with a connection error and the job would be
	// marked failed.
	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL("http://127.0.0.1:1"))
	workspaceID, assetID, storageKey := setupAutoTagAudioAsset(t, env)

	payload, err := json.Marshal(map[string]any{
		"workspace_id": workspaceID,
		"asset_id":     assetID,
		"storage_key":  storageKey,
		"mime_type":    "audio/mpeg",
		"mode":         "pending",
		"size":         ai.MaxTranscribeAudioBytes + 1,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, e := env.JobServer.EnqueueForTest(
		context.Background(),
		workspaceID,
		queue.JobTypeAutoTag,
		string(payload),
	); e != nil {
		t.Fatalf("enqueue auto_tag: %v", e)
	}
	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 0 {
		t.Fatalf("expected no suggestions for an oversized audio file, got %d", count)
	}

	var failed int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE type = 'auto_tag' AND status = 'failed'`,
	).Scan(&failed); e != nil {
		t.Fatalf("count failed jobs: %v", e)
	}
	if failed != 0 {
		t.Fatalf("expected job to skip cleanly on oversized audio (not fail), got %d failed", failed)
	}
}

func TestJobAutoTag_Audio_SilentMode_AppliesTagsDirectly(t *testing.T) {
	srv := audioTagModelServer(t, "a song about the sea", `["music","ocean"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAudioAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "audio/mpeg", "silent")
	env.JobServer.DrainForTest(context.Background())

	var tagCount int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM asset_tags at JOIN tags t ON t.id = at.tag_id WHERE at.asset_id = ?`, assetID,
	).Scan(&tagCount); e != nil {
		t.Fatalf("count asset_tags: %v", e)
	}
	if tagCount != 2 {
		t.Fatalf("expected 2 tags applied directly, got %d", tagCount)
	}
}

// TestJobAutoTag_Audio_M4A_SendsM4AFormatNotAAC guards against regressing to
// transform.AudioFormatFromMimeType, which maps audio/mp4 to "aac" — wrong
// for OpenRouter's STT format enum, which expects "m4a" for an MP4/M4A
// container. See ai.TranscriptionFormatFromMimeType.
func TestJobAutoTag_Audio_M4A_SendsM4AFormatNotAAC(t *testing.T) {
	var gotFormat string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio/transcriptions":
			var body struct {
				InputAudio struct {
					Format string `json:"format"`
				} `json:"input_audio"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			gotFormat = body.InputAudio.Format
			_ = json.NewEncoder(w).Encode(map[string]any{"text": "a voice memo"})
		case "/chat/completions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]string{"content": `["memo"]`}},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAudioAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "audio/mp4", "pending")
	env.JobServer.DrainForTest(context.Background())

	if gotFormat != "m4a" {
		t.Fatalf("expected format %q sent to transcription API, got %q", "m4a", gotFormat)
	}

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 1 {
		t.Fatalf("expected 1 suggestion, got %d", count)
	}
}

func TestJobAutoTag_Audio_UnsupportedMime_SkipsCleanly(t *testing.T) {
	// No httptest server is started — if the job didn't skip on the
	// unmapped mime type, the provider call would fail with a connection
	// error and the job would be marked failed.
	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL("http://127.0.0.1:1"))
	workspaceID, assetID, storageKey := setupAutoTagAudioAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "audio/x-totally-unknown", "pending")
	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 0 {
		t.Fatalf("expected no suggestions for an unsupported audio mime type, got %d", count)
	}

	var failed int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE type = 'auto_tag' AND status = 'failed'`,
	).Scan(&failed); e != nil {
		t.Fatalf("count failed jobs: %v", e)
	}
	if failed != 0 {
		t.Fatalf("expected job to skip cleanly on unsupported audio mime (not fail), got %d failed", failed)
	}
}

func visionModelServer(t *testing.T, response string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": response}},
			},
		})
	}))
}

func TestJobAutoTag_PendingMode_CreatesSuggestions(t *testing.T) {
	srv := visionModelServer(t, `["hero","blue","logo"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "image/jpeg", "pending")
	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 3 {
		t.Fatalf("expected 3 suggestions, got %d", count)
	}
}

func TestJobAutoTag_SilentMode_AppliesTagsDirectly(t *testing.T) {
	srv := visionModelServer(t, `["hero","blue"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "image/jpeg", "silent")
	env.JobServer.DrainForTest(context.Background())

	var suggestionCount int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&suggestionCount); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if suggestionCount != 0 {
		t.Fatalf("expected no suggestions in silent mode, got %d", suggestionCount)
	}

	var tagCount int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM asset_tags at JOIN tags t ON t.id = at.tag_id WHERE at.asset_id = ?`, assetID,
	).Scan(&tagCount); e != nil {
		t.Fatalf("count asset_tags: %v", e)
	}
	if tagCount != 2 {
		t.Fatalf("expected 2 tags applied directly, got %d", tagCount)
	}
}

func TestJobAutoTag_NoProviderConfigured_ExitsCleanly(t *testing.T) {
	env := th.SetupTestApp(t) // no AI provider configured
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "image/jpeg", "pending")
	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 0 {
		t.Fatalf("expected no suggestions without a configured provider, got %d", count)
	}

	var failed int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE type = 'auto_tag' AND status = 'failed'`,
	).Scan(&failed); e != nil {
		t.Fatalf("count failed jobs: %v", e)
	}
	if failed != 0 {
		t.Fatalf("expected job to exit cleanly (not fail), got %d failed", failed)
	}
}

func TestJobAutoTag_IneligibleMime_ExitsCleanly(t *testing.T) {
	srv := visionModelServer(t, `["hero"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "text/plain", "pending")
	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 0 {
		t.Fatalf("expected no suggestions for an ineligible mime type, got %d", count)
	}
}

func TestJobAutoTag_MalformedModelResponse_NoError(t *testing.T) {
	srv := visionModelServer(t, `I think this image shows a logo.`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "image/jpeg", "pending")
	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 0 {
		t.Fatalf("expected no suggestions from an unparsable response, got %d", count)
	}

	var failed int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE type = 'auto_tag' AND status = 'failed'`,
	).Scan(&failed); e != nil {
		t.Fatalf("count failed jobs: %v", e)
	}
	if failed != 0 {
		t.Fatalf("expected job to degrade gracefully (not fail), got %d failed", failed)
	}
}

func TestJobAutoTag_SkipsAlreadyAppliedTags(t *testing.T) {
	srv := visionModelServer(t, `["hero","blue"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	// Seed an existing "hero" tag directly on the asset.
	if _, e := env.Database.Exec(
		`INSERT INTO tags (id, workspace_id, name) VALUES ('tag_hero', ?, 'hero')`, workspaceID,
	); e != nil {
		t.Fatalf("seed tag: %v", e)
	}
	if _, e := env.Database.Exec(
		`INSERT INTO asset_tags (asset_id, tag_id) VALUES (?, 'tag_hero')`, assetID,
	); e != nil {
		t.Fatalf("seed asset_tags: %v", e)
	}

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "image/jpeg", "pending")
	env.JobServer.DrainForTest(context.Background())

	var names []string
	rows, e := env.Database.Query(`SELECT tag_name FROM auto_tag_suggestions WHERE asset_id = ?`, assetID)
	if e != nil {
		t.Fatalf("query suggestions: %v", e)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if e = rows.Scan(&n); e != nil {
			t.Fatalf("scan: %v", e)
		}
		names = append(names, n)
	}
	if len(names) != 1 || names[0] != "blue" {
		t.Fatalf("expected only [blue] as a fresh suggestion, got %v", names)
	}
}

func TestJobAutoTag_Idempotent_WipesStaleSuggestions(t *testing.T) {
	srv := visionModelServer(t, `["fresh"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	if _, e := env.Database.Exec(
		`INSERT INTO auto_tag_suggestions (id, workspace_id, asset_id, tag_name) VALUES (?, ?, ?, ?)`,
		"sug_old", workspaceID, assetID, "stale",
	); e != nil {
		t.Fatalf("seed stale suggestion: %v", e)
	}

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "image/jpeg", "pending")
	env.JobServer.DrainForTest(context.Background())

	var names []string
	rows, e := env.Database.Query(`SELECT tag_name FROM auto_tag_suggestions WHERE asset_id = ?`, assetID)
	if e != nil {
		t.Fatalf("query suggestions: %v", e)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if e = rows.Scan(&n); e != nil {
			t.Fatalf("scan: %v", e)
		}
		names = append(names, n)
	}
	if len(names) != 1 || names[0] != "fresh" {
		t.Fatalf("expected stale suggestion replaced by [fresh], got %v", names)
	}
}

func TestJobAutoTag_LockedVocabulary_VocabInPrompt_ModelRespected(t *testing.T) {
	var sentPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, c := range body.Messages[0].Content {
			if c.Type == "text" {
				sentPrompt = c.Text
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `["hero","sport"]`}},
			},
		})
	}))
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	if _, e := env.Database.Exec(`UPDATE workspaces SET locked_taxonomy = 1 WHERE id = ?`, workspaceID); e != nil {
		t.Fatalf("lock taxonomy: %v", e)
	}
	for _, name := range []string{"hero", "sport"} {
		if _, e := env.Database.Exec(
			`INSERT INTO tags (id, workspace_id, name) VALUES (?, ?, ?)`, "tag_"+name, workspaceID, name,
		); e != nil {
			t.Fatalf("seed tag %s: %v", name, e)
		}
	}

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "image/jpeg", "pending")
	env.JobServer.DrainForTest(context.Background())

	if sentPrompt == "" {
		t.Fatal("expected a prompt to be sent to the model")
	}
	if !strings.Contains(sentPrompt, "hero, sport") && !strings.Contains(sentPrompt, "sport, hero") {
		t.Fatalf("expected vocabulary in prompt, got %q", sentPrompt)
	}

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 2 {
		t.Fatalf("expected 2 suggestions, got %d", count)
	}
}

func TestAutoTag_EnqueuedAutomaticallyOnUploadWhenEnabled(t *testing.T) {
	srv := visionModelServer(t, `["hero"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	res := th.Register(t, env, "Auto Tag Upload User", "autotagupload@test.com", "password123")

	if _, e := env.Database.Exec(
		`UPDATE workspaces SET auto_tag_enabled = 1 WHERE id = ?`, res.WorkspaceID,
	); e != nil {
		t.Fatalf("enable auto-tagging: %v", e)
	}

	asset := th.UploadAsset(t, env, res.Cookie)
	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, asset.ID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 1 {
		t.Fatalf("expected auto_tag to run automatically on upload, got %d suggestions", count)
	}
}

func TestAutoTag_NotEnqueuedOnUploadWhenDisabled(t *testing.T) {
	srv := visionModelServer(t, `["hero"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	res := th.Register(t, env, "Auto Tag Upload User 2", "autotagupload2@test.com", "password123")
	// auto_tag_enabled defaults to 0 — no settings change.

	asset := th.UploadAsset(t, env, res.Cookie)
	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, asset.ID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 0 {
		t.Fatalf("expected no auto_tag run when disabled, got %d suggestions", count)
	}
}

func TestAutoTag_NewVersionUploadDismissesStaleSuggestionsAndReruns(t *testing.T) {
	srv := visionModelServer(t, `["fresh"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	res := th.Register(t, env, "Auto Tag Version User", "autotagversion@test.com", "password123")
	if _, e := env.Database.Exec(
		`UPDATE workspaces SET auto_tag_enabled = 1 WHERE id = ?`, res.WorkspaceID,
	); e != nil {
		t.Fatalf("enable auto-tagging: %v", e)
	}

	asset := th.UploadAsset(t, env, res.Cookie)
	env.JobServer.DrainForTest(context.Background())

	// Seed a stale suggestion as if left over from the first version.
	if _, e := env.Database.Exec(
		`INSERT INTO auto_tag_suggestions (id, workspace_id, asset_id, tag_name) VALUES (?, ?, ?, ?)`,
		"sug_stale", res.WorkspaceID, asset.ID, "stale",
	); e != nil {
		t.Fatalf("seed stale suggestion: %v", e)
	}

	req := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", th.MakeJPEG(150, 150), "", res.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload new version: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	env.JobServer.DrainForTest(context.Background())

	var names []string
	rows, e := env.Database.Query(`SELECT tag_name FROM auto_tag_suggestions WHERE asset_id = ?`, asset.ID)
	if e != nil {
		t.Fatalf("query suggestions: %v", e)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if e = rows.Scan(&n); e != nil {
			t.Fatalf("scan: %v", e)
		}
		names = append(names, n)
	}
	if len(names) != 1 || names[0] != "fresh" {
		t.Fatalf("expected stale suggestion replaced by [fresh] after new version upload, got %v", names)
	}
}

func TestJobAutoTag_VideoThumbnail_UsesRealContentType(t *testing.T) {
	var sentImageURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content []struct {
					Type     string `json:"type"`
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, c := range body.Messages[0].Content {
			if c.Type == "image_url" {
				sentImageURL = c.ImageURL.URL
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `["clip"]`}},
			},
		})
	}))
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	// Reuse a real uploaded JPEG's storage key as a stand-in for a generated
	// video thumbnail — only the declared mime/content-type are under test.
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	enqueueAutoTagFull(t, env, workspaceID, assetID, "", storageKey, "image/png", "video/mp4", "pending")
	env.JobServer.DrainForTest(context.Background())

	if sentImageURL == "" {
		t.Fatal("expected an image_url to be sent to the model")
	}
	if !strings.HasPrefix(sentImageURL, "data:image/png;base64,") {
		t.Fatalf("expected describeMime to use the thumbnail's real content-type (image/png), got url prefix %q",
			sentImageURL[:min(40, len(sentImageURL))])
	}
}

func TestJobAutoTag_ModelOverGeneratesTags_CappedAtEight(t *testing.T) {
	srv := visionModelServer(t, `["a","b","c","d","e","f","g","h","i","j"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "image/jpeg", "pending")
	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM auto_tag_suggestions WHERE asset_id = ?`, assetID,
	).Scan(&count); e != nil {
		t.Fatalf("count suggestions: %v", e)
	}
	if count != 8 {
		t.Fatalf("expected suggestions capped at 8, got %d", count)
	}
}

func TestJobAutoTag_SilentMode_WritesAuditLogEntries(t *testing.T) {
	srv := visionModelServer(t, `["hero","blue"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "image/jpeg", "silent")
	env.JobServer.DrainForTest(context.Background())

	var count int
	if e := env.Database.QueryRow(
		`SELECT COUNT(*) FROM asset_events
		 WHERE asset_id = ? AND event_type = 'asset_tagged' AND actor_type = 'system' AND user_id IS NULL`,
		assetID,
	).Scan(&count); e != nil {
		t.Fatalf("count audit events: %v", e)
	}
	if count != 2 {
		t.Fatalf("expected 2 system-actor asset_tagged audit events for silently applied tags, got %d", count)
	}
}

func TestJobAutoTag_SilentMode_DispatchesWorkflowTrigger(t *testing.T) {
	srv := visionModelServer(t, `["hero","blue"]`)
	defer srv.Close()

	env := th.SetupTestApp(t, th.WithOpenRouterAPIKey("test-key"), th.WithOpenRouterBaseURL(srv.URL))
	workspaceID, assetID, storageKey := setupAutoTagAsset(t, env)

	queries := dbgen.New(env.Database)
	var ownerUserID string
	if e := env.Database.QueryRow(
		`SELECT user_id FROM workspace_members WHERE workspace_id = ? AND role = 'owner'`, workspaceID,
	).Scan(&ownerUserID); e != nil {
		t.Fatalf("lookup workspace owner: %v", e)
	}

	wfRepo := reposqlc.NewWorkflowRepo(&db.DB{Writer: env.Database, Reader: env.Database, WQ: queries, RQ: queries})
	graph, err := json.Marshal(workflow.Graph{
		Nodes: []workflow.GraphNode{{ID: "n1", Type: "trigger.tag_added"}},
		Edges: []workflow.GraphEdge{},
	})
	if err != nil {
		t.Fatalf("marshal graph: %v", err)
	}
	wf, err := wfRepo.Create(context.Background(), repository.CreateWorkflowParams{
		ID:          "wf_autotag_test",
		WorkspaceID: workspaceID,
		Name:        "tag added",
		Enabled:     true,
		TriggerType: "trigger.tag_added",
		Graph:       string(graph),
		CreatedBy:   ownerUserID,
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	enqueueAutoTag(t, env, workspaceID, assetID, storageKey, "image/jpeg", "silent")
	env.JobServer.DrainForTest(context.Background())

	// publishWorkflowTriggerAsync dispatches in a background goroutine, so
	// the workflow_runs row may not exist the instant DrainForTest returns.
	waitForWorkflowRun(t, env, wf.ID)
}

func waitForWorkflowRun(t *testing.T, env *th.TestEnv, workflowID string) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		var runCount int
		if e := env.Database.QueryRow(
			`SELECT COUNT(*) FROM workflow_runs WHERE workflow_id = ?`, workflowID,
		).Scan(&runCount); e != nil {
			t.Fatalf("count workflow runs: %v", e)
		}
		if runCount > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected silent auto-tag to dispatch trigger.tag_added and enqueue a workflow run")
}
