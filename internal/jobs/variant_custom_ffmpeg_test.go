package jobs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	th "damask/server/internal/testhelpers"
)

func requireFFmpegForJobsTest(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not found")
	}
}

func uploadCustomFFmpegVideoAsset(t *testing.T, env *th.TestEnv, cookie *http.Cookie) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "transform", "testdata", "sample_video_with_audio.mp4"))
	if err != nil {
		t.Fatalf("read sample video: %v", err)
	}
	req := th.BuildUploadRequest(t, "clip.mp4", data, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload video asset: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload video asset: expected 201, got %d", resp.StatusCode)
	}
	var a struct {
		ID string `json:"id"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&a); decodeErr != nil {
		t.Fatalf("decode video asset: %v", decodeErr)
	}
	return a.ID
}

func TestCustomFFmpegJob_HappyPath_CreatesVariant(t *testing.T) {
	requireFFmpegForJobsTest(t)

	env := th.SetupTestApp(t)
	res := th.Register(t, env, "FFmpeg Worker", "ffmpegworker@test.com", "password123")
	assetID := uploadCustomFFmpegVideoAsset(t, env, res.Cookie)
	env.JobServer.DrainForTest(context.Background())

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(map[string]any{
			"type":   "custom_ffmpeg",
			"params": map[string]any{"command": "ffmpeg -i {input} -t 1 -c copy -f mp4 {output}"},
		}), res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	env.JobServer.DrainForTest(context.Background())

	var storageKey string
	row := env.Database.QueryRow(`SELECT storage_key FROM variants WHERE type = 'custom_ffmpeg' LIMIT 1`)
	if scanErr := row.Scan(&storageKey); scanErr != nil {
		t.Fatalf("load custom_ffmpeg variant: %v", scanErr)
	}
	if storageKey == "" {
		t.Fatal("expected non-empty storage key")
	}

	rc, err := env.Storage.Get(t.Context(), storageKey)
	if err != nil {
		t.Fatalf("read stored variant: %v", err)
	}
	rc.Close()
}

func TestCustomFFmpegJob_BlacklistedCommand_NoVariantCreated(t *testing.T) {
	requireFFmpegForJobsTest(t)

	env := th.SetupTestApp(t)
	res := th.Register(t, env, "FFmpeg Worker 2", "ffmpegworker2@test.com", "password123")
	assetID := uploadCustomFFmpegVideoAsset(t, env, res.Cookie)
	env.JobServer.DrainForTest(context.Background())

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(map[string]any{
			"type":   "custom_ffmpeg",
			"params": map[string]any{"command": "ffmpeg -i {input} -c copy {output}; rm -rf /"},
		}), res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for blacklisted command, got %d", resp.StatusCode)
	}

	var count int
	countRow := env.Database.QueryRow(`SELECT COUNT(*) FROM variants WHERE type = 'custom_ffmpeg'`)
	if scanErr := countRow.Scan(&count); scanErr != nil {
		t.Fatalf("count variants: %v", scanErr)
	}
	if count != 0 {
		t.Fatalf("expected no variant created, got %d", count)
	}
}

func TestCustomFFmpegJob_WithAssetRef_HappyPath(t *testing.T) {
	requireFFmpegForJobsTest(t)

	env := th.SetupTestApp(t)
	res := th.Register(t, env, "FFmpeg Ref Worker", "ffmpegrefworker@test.com", "password123")
	assetID := uploadCustomFFmpegVideoAsset(t, env, res.Cookie)
	refAssetID := uploadCustomFFmpegVideoAsset(t, env, res.Cookie)
	env.JobServer.DrainForTest(context.Background())

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(map[string]any{
			"type": "custom_ffmpeg",
			"params": map[string]any{
				"command": "ffmpeg -i {input} -i {asset:" + refAssetID +
					"} -filter_complex amix=inputs=2 -t 1 {output}",
			},
		}), res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	env.JobServer.DrainForTest(context.Background())

	var storageKey string
	row := env.Database.QueryRow(
		`SELECT storage_key FROM variants WHERE type = 'custom_ffmpeg' ORDER BY created_at DESC LIMIT 1`)
	if scanErr := row.Scan(&storageKey); scanErr != nil {
		t.Fatalf("load custom_ffmpeg variant: %v", scanErr)
	}
	if storageKey == "" {
		t.Fatal("expected non-empty storage key")
	}

	rc, err := env.Storage.Get(t.Context(), storageKey)
	if err != nil {
		t.Fatalf("read stored variant: %v", err)
	}
	rc.Close()
}
