package transform

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"damask/server/internal/config"
)

func resolvedFFmpegPath(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found")
	}
	return path
}

func resolvedFFprobePath(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not found")
	}
	return path
}

func TestNewTransformer_FFmpegPathFallsBackToPATH(t *testing.T) {
	t.Parallel()
	ffmpegPath := resolvedFFmpegPath(t)
	ffprobePath := resolvedFFprobePath(t)

	tr := NewTransformer().(*transformer)
	if !tr.FFmpegAvailable() {
		t.Fatal("expected ffmpeg to be available from PATH")
	}
	if tr.ffmpeg.ffmpegPath != ffmpegPath {
		t.Fatalf("ffmpeg path = %q, want %q", tr.ffmpeg.ffmpegPath, ffmpegPath)
	}
	if tr.ffmpeg.ffprobePath != ffprobePath {
		t.Fatalf("ffprobe path = %q, want %q", tr.ffmpeg.ffprobePath, ffprobePath)
	}
}

func TestNewTransformer_FFmpegPathResolvesExplicitBinary(t *testing.T) {
	t.Parallel()
	ffmpegPath := resolvedFFmpegPath(t)
	ffprobePath := resolvedFFprobePath(t)

	tr := NewTransformer(config.FFmpegConfig{Path: ffmpegPath}).(*transformer)
	if tr.ffmpeg.ffmpegPath != ffmpegPath {
		t.Fatalf("ffmpeg path = %q, want %q", tr.ffmpeg.ffmpegPath, ffmpegPath)
	}
	if filepath.Base(tr.ffmpeg.ffprobePath) != filepath.Base(ffprobePath) {
		t.Fatalf("ffprobe path = %q, want basename %q", tr.ffmpeg.ffprobePath, filepath.Base(ffprobePath))
	}
}

func TestNewTransformer_InvalidFFmpegPathDisablesAvailability(t *testing.T) {
	t.Parallel()
	tr := NewTransformer(config.FFmpegConfig{Path: "/definitely/missing/ffmpeg"}).(*transformer)
	if tr.FFmpegAvailable() {
		t.Fatal("expected ffmpeg to be unavailable")
	}
	_, err := tr.VideoExtractThumbnail(
		context.Background(),
		testdataPath(t, "sample_video_with_audio.mp4"),
		VideoThumbnailParams{},
	)
	if err == nil || !strings.Contains(err.Error(), "FFMPEG_PATH") {
		t.Fatalf("expected actionable FFMPEG_PATH error, got %v", err)
	}
}

func TestFFmpegRuntime_WithVideoDecodeAddsHWAccelArgs(t *testing.T) {
	t.Parallel()
	tr := NewTransformer(config.FFmpegConfig{HWAccel: "cuda"}).(*transformer)
	args := tr.ffmpeg.withVideoDecode("-y", "-i", "input.mp4")
	if len(args) < 2 || args[0] != "-hwaccel" || args[1] != "cuda" {
		t.Fatalf("expected hwaccel args prefix, got %v", args)
	}
}

func TestFFmpegRuntime_WithoutHWAccelLeavesArgsUnchanged(t *testing.T) {
	t.Parallel()
	tr := NewTransformer().(*transformer)
	args := []string{"-y", "-i", "input.mp4"}
	withDecode := tr.ffmpeg.withVideoDecode(args...)
	if strings.Join(withDecode, " ") != strings.Join(args, " ") {
		t.Fatalf("expected unchanged args, got %v", withDecode)
	}
}
