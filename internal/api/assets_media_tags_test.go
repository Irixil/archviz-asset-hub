//go:build integration

package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	th "damask/server/internal/testhelpers"
)

func TestPostAsset_AudioMime_EnqueuesExtractMediaTags(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	data, err := os.ReadFile(filepath.Join("..", "transform", "testdata", "sample_audio_loud.mp3"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	req := th.BuildUploadRequest(t, "track.mp3", data, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var count int
	if err := env.Database.QueryRow(`SELECT COUNT(*) FROM jobs WHERE workspace_id = ? AND type = 'extract_media_tags'`, owner.WorkspaceID).Scan(&count); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if count == 0 {
		t.Fatal("expected extract_media_tags job")
	}
}

func TestPostAsset_VideoMime_EnqueuesExtractMediaTags(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	data, err := os.ReadFile(filepath.Join("..", "transform", "testdata", "sample_video_no_audio.mp4"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	req := th.BuildUploadRequest(t, "clip.mp4", data, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var count int
	if err := env.Database.QueryRow(`SELECT COUNT(*) FROM jobs WHERE workspace_id = ? AND type = 'extract_media_tags'`, owner.WorkspaceID).Scan(&count); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if count == 0 {
		t.Fatal("expected extract_media_tags job")
	}
}

func TestPostAsset_ImageMime_DoesNotEnqueueExtractMediaTags(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	req := th.BuildUploadRequest(t, "image.jpg", th.MakeJPEG(20, 20), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var count int
	if err := env.Database.QueryRow(`SELECT COUNT(*) FROM jobs WHERE workspace_id = ? AND type = 'extract_media_tags'`, owner.WorkspaceID).Scan(&count); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no extract_media_tags job, got %d", count)
	}
}
