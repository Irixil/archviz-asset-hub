//go:build integration

package jobs_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"damask/server/internal/config"
	"damask/server/internal/jobs"
	th "damask/server/internal/testhelpers"
)

// ---- helpers ----

func tinyPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func startFakeImageRouter(t *testing.T, outputPNG []byte, statusCode int) (*httptest.Server, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if statusCode != http.StatusOK {
			http.Error(w, "imagerouter error", statusCode)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(outputPNG)}},
		})
	}))
	return srv, func() {
		srv.Close()
	}
}

// writeDraftMeta writes a .meta sidecar directly into test storage.
func writeDraftMeta(t *testing.T, env *th.TestEnv, workspaceID, userID, nonce, assetID string, age time.Duration) {
	t.Helper()
	metaKey := fmt.Sprintf("scratch/%s/%s/%s.meta", workspaceID, userID, nonce)
	createdAt := time.Now().UTC().Add(-age)
	meta := fmt.Sprintf(`{
		"asset_id":%q,"workspace_id":%q,"user_id":%q,
		"variant_type":"image_with_prompt",
		"transform_params":"{\"prompt\":\"x\",\"model\":\"m\"}",
		"content_type":"image/png",
		"created_at":%q
	}`, assetID, workspaceID, userID, createdAt.Format(time.RFC3339))
	if err := env.Storage.Put(t.Context(), metaKey, bytes.NewReader([]byte(meta))); err != nil {
		t.Fatalf("write meta: %v", err)
	}
}

func writeDraftOutput(t *testing.T, env *th.TestEnv, workspaceID, userID, nonce string) {
	t.Helper()
	outputKey := fmt.Sprintf("scratch/%s/%s/%s", workspaceID, userID, nonce)
	if err := env.Storage.Put(t.Context(), outputKey, bytes.NewReader([]byte("fake-png"))); err != nil {
		t.Fatalf("write draft output: %v", err)
	}
}

func storageKeyExists(env *th.TestEnv, key string) bool {
	rc, err := env.Storage.Get(context.Background(), key)
	if err != nil {
		return false
	}
	rc.Close()
	return true
}

// ---- CreateVariantDraft job tests ----

func TestCreateVariantDraftJob_HappyPath(t *testing.T) {
	outputPNG := tinyPNGBytes(t)
	srv, cleanup := startFakeImageRouter(t, outputPNG, http.StatusOK)
	defer cleanup()

	env := th.SetupTestApp(t, th.WithImageRouterAPIKey("test-key"), th.WithImageRouterBaseURL(srv.URL))
	res := th.Register(t, env, "Draft User", "draft@test.com", "password123")
	assetID := env.UploadTestAsset(t, res.Cookie)

	// Drain thumbnail job from upload.
	env.JobServer.DrainForTest(context.Background())

	const nonce = "deadbeef12345678"
	payload, _ := json.Marshal(jobs.CreateVariantDraftPayload{
		Nonce:       nonce,
		WorkspaceID: res.WorkspaceID,
		UserID:      res.UserID,
		AssetID:     assetID,
		Type:        "image_with_prompt",
		Params:      json.RawMessage(`{"prompt":"make it blue","model":"flux"}`),
	})
	if _, err := env.JobServer.EnqueueForTest(context.Background(), res.WorkspaceID, "create_variant_draft", string(payload)); err != nil {
		t.Fatalf("enqueue draft job: %v", err)
	}
	env.JobServer.DrainForTest(context.Background())

	// Scratch output file should exist.
	scratchKey := fmt.Sprintf("scratch/%s/%s/%s", res.WorkspaceID, res.UserID, nonce)
	if !storageKeyExists(env, scratchKey) {
		t.Errorf("expected scratch output at %s", scratchKey)
	}

	// Meta sidecar should exist.
	metaKey := scratchKey + ".meta"
	if !storageKeyExists(env, metaKey) {
		t.Errorf("expected meta at %s", metaKey)
	}
}

func TestCreateVariantDraftJob_TransformError(t *testing.T) {
	_, cleanup := startFakeImageRouter(t, nil, http.StatusUnprocessableEntity)
	defer cleanup()

	env := th.SetupTestApp(t, th.WithImageRouterAPIKey("test-key"))
	res := th.Register(t, env, "Draft Err", "drafterr@test.com", "password123")
	assetID := env.UploadTestAsset(t, res.Cookie)
	env.JobServer.DrainForTest(context.Background())

	const nonce = "errornonce1234ab"
	payload, _ := json.Marshal(jobs.CreateVariantDraftPayload{
		Nonce:       nonce,
		WorkspaceID: res.WorkspaceID,
		UserID:      res.UserID,
		AssetID:     assetID,
		Type:        "image_with_prompt",
		Params:      json.RawMessage(`{"prompt":"x","model":"flux"}`),
	})
	if _, err := env.JobServer.EnqueueForTest(context.Background(), res.WorkspaceID, "create_variant_draft", string(payload)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	env.JobServer.DrainForTest(context.Background())

	// No scratch output should have been written.
	scratchKey := fmt.Sprintf("scratch/%s/%s/%s", res.WorkspaceID, res.UserID, nonce)
	if storageKeyExists(env, scratchKey) {
		t.Errorf("expected no scratch output after transform error")
	}
}

func TestCreateVariantDraftJob_AssetNotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Draft NF", "draftnf@test.com", "password123")

	const nonce = "notfoundnonce123"
	payload, _ := json.Marshal(jobs.CreateVariantDraftPayload{
		Nonce:       nonce,
		WorkspaceID: res.WorkspaceID,
		UserID:      res.UserID,
		AssetID:     "nonexistent-asset-id",
		Type:        "image_with_prompt",
		Params:      json.RawMessage(`{"prompt":"x","model":"m"}`),
	})
	if _, err := env.JobServer.EnqueueForTest(context.Background(), res.WorkspaceID, "create_variant_draft", string(payload)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// Must not panic and must return nil (error surfaced via hub).
	env.JobServer.DrainForTest(context.Background())
}

// ---- Purge job tests ----

func TestPurgeScratchVariants_DeletesOldFiles(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Purge U", "purge@test.com", "password123")
	wsID := res.WorkspaceID
	userID := res.UserID

	// 3 old nonces (25h), 1 recent (1h).
	for i, nonce := range []string{"old1111111111111", "old2222222222222", "old3333333333333"} {
		_ = i
		writeDraftOutput(t, env, wsID, userID, nonce)
		writeDraftMeta(t, env, wsID, userID, nonce, "ast_1", 25*time.Hour)
	}
	writeDraftOutput(t, env, wsID, userID, "recent11111111111")
	writeDraftMeta(t, env, wsID, userID, "recent11111111111", "ast_1", 1*time.Hour)

	if _, err := env.JobServer.EnqueueForTest(context.Background(), "system", "purge_scratch_variants", "{}"); err != nil {
		t.Fatalf("enqueue purge: %v", err)
	}
	env.JobServer.DrainForTest(context.Background())

	// Old ones should be gone.
	for _, nonce := range []string{"old1111111111111", "old2222222222222", "old3333333333333"} {
		key := fmt.Sprintf("scratch/%s/%s/%s", wsID, userID, nonce)
		if storageKeyExists(env, key) {
			t.Errorf("expected %s to be deleted", key)
		}
	}
	// Recent one should still exist.
	recentKey := fmt.Sprintf("scratch/%s/%s/%s", wsID, userID, "recent11111111111")
	if !storageKeyExists(env, recentKey) {
		t.Errorf("expected recent draft to survive purge")
	}
}

func TestPurgeScratchVariants_LeavesRecentFiles(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "PurgeR U", "purger@test.com", "password123")
	wsID := res.WorkspaceID
	userID := res.UserID

	for _, nonce := range []string{"r1111111111111111", "r2222222222222222"} {
		writeDraftOutput(t, env, wsID, userID, nonce)
		writeDraftMeta(t, env, wsID, userID, nonce, "ast_1", 1*time.Hour)
	}

	if _, err := env.JobServer.EnqueueForTest(context.Background(), "system", "purge_scratch_variants", "{}"); err != nil {
		t.Fatalf("enqueue purge: %v", err)
	}
	env.JobServer.DrainForTest(context.Background())

	for _, nonce := range []string{"r1111111111111111", "r2222222222222222"} {
		key := fmt.Sprintf("scratch/%s/%s/%s", wsID, userID, nonce)
		if !storageKeyExists(env, key) {
			t.Errorf("expected recent draft %s to survive", key)
		}
	}
}

func TestPurgeScratchVariants_OrphanMeta(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Orphan U", "orphan@test.com", "password123")
	wsID := res.WorkspaceID
	userID := res.UserID
	nonce := "orphan1111111111"

	// Write only the meta, no output file.
	writeDraftMeta(t, env, wsID, userID, nonce, "ast_1", 25*time.Hour)

	if _, err := env.JobServer.EnqueueForTest(context.Background(), "system", "purge_scratch_variants", "{}"); err != nil {
		t.Fatalf("enqueue purge: %v", err)
	}
	env.JobServer.DrainForTest(context.Background())

	metaKey := fmt.Sprintf("scratch/%s/%s/%s.meta", wsID, userID, nonce)
	if storageKeyExists(env, metaKey) {
		t.Errorf("expected orphan meta %s to be deleted", metaKey)
	}
}

func TestPurgeScratchVariants_Idempotent(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Idemp U", "idemp@test.com", "password123")
	wsID := res.WorkspaceID
	userID := res.UserID
	nonce := "idemp11111111111"

	writeDraftOutput(t, env, wsID, userID, nonce)
	writeDraftMeta(t, env, wsID, userID, nonce, "ast_1", 25*time.Hour)

	for range 2 {
		if _, err := env.JobServer.EnqueueForTest(context.Background(), "system", "purge_scratch_variants", "{}"); err != nil {
			t.Fatalf("enqueue purge: %v", err)
		}
		env.JobServer.DrainForTest(context.Background())
	}
	// Second run should not error.
}

// ---- Config tests ----

func TestPurgeHourMinute_ValidFormat(t *testing.T) {
	cfg := config.ScratchConfig{PurgeTime: "14:30"}
	h, m := cfg.PurgeHourMinute()
	if h != 14 || m != 30 {
		t.Errorf("expected (14, 30), got (%d, %d)", h, m)
	}
}

func TestPurgeHourMinute_InvalidFormat(t *testing.T) {
	cfg := config.ScratchConfig{PurgeTime: "bad"}
	h, m := cfg.PurgeHourMinute()
	if h != 3 || m != 0 {
		t.Errorf("expected (3, 0) fallback, got (%d, %d)", h, m)
	}
}
