package config

import (
	"strings"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", strings.Repeat("j", 32))
	t.Setenv("APP_SECRET", strings.Repeat("a", 32))
	t.Setenv("BASE_URL", "http://localhost:5173")
}

func TestLoad_FFmpegConfigDefaults(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("FFMPEG_PATH", "")
	t.Setenv("FFMPEG_HW_ACCEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.FFmpeg.Path != "" {
		t.Fatalf("FFMPEG_PATH = %q, want empty", cfg.FFmpeg.Path)
	}
	if cfg.FFmpeg.HWAccel != "" {
		t.Fatalf("FFMPEG_HW_ACCEL = %q, want empty", cfg.FFmpeg.HWAccel)
	}
}

func TestLoad_FFmpegConfigNormalizesValues(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("FFMPEG_PATH", " /opt/custom/ffmpeg ")
	t.Setenv("FFMPEG_HW_ACCEL", " CUDA ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.FFmpeg.Path != "/opt/custom/ffmpeg" {
		t.Fatalf("FFMPEG_PATH = %q", cfg.FFmpeg.Path)
	}
	if cfg.FFmpeg.HWAccel != "cuda" {
		t.Fatalf("FFMPEG_HW_ACCEL = %q", cfg.FFmpeg.HWAccel)
	}
}

func TestLoad_InvalidFFmpegHWAccelRejected(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("FFMPEG_HW_ACCEL", "metal")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "FFMPEG_HW_ACCEL") {
		t.Fatalf("expected FFMPEG_HW_ACCEL error, got %v", err)
	}
}
