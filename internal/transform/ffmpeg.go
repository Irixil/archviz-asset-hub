package transform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"damask/server/internal/config"
)

type ffmpegRuntime struct {
	configuredPath string
	ffmpegPath     string
	ffprobePath    string
	hwAccel        string
	hwAccelActive  bool
}

func newFFmpegRuntime(cfg config.FFmpegConfig) ffmpegRuntime {
	runtime := ffmpegRuntime{
		configuredPath: strings.TrimSpace(cfg.Path),
		hwAccel:        strings.ToLower(strings.TrimSpace(cfg.HWAccel)),
	}
	runtime.ffmpegPath = resolveBinary(runtime.configuredPath, "ffmpeg")
	runtime.ffprobePath = resolveCompanionProbe(runtime.ffmpegPath)
	runtime.hwAccelActive = runtime.hwAccel != ""

	slog.Info("ffmpeg runtime",
		"ffmpeg_path", logValue(runtime.ffmpegPath),
		"ffprobe_path", logValue(runtime.ffprobePath),
		"hw_accel", logValue(runtime.hwAccel),
		"hw_accel_scope", "video_decode_only",
		"hw_accel_requested", runtime.hwAccel != "",
		"hw_accel_active", runtime.hwAccelActive,
	)

	return runtime
}

func (r ffmpegRuntime) available() bool {
	return r.ffmpegPath != ""
}

func (r ffmpegRuntime) ffprobeAvailable() bool {
	return r.ffprobePath != ""
}

func (r ffmpegRuntime) commandFFmpeg(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, r.ffmpegPath, args...) //nolint:gosec // validated args
}

func (r ffmpegRuntime) commandFFprobe(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, r.ffprobePath, args...) //nolint:gosec // validated args
}

func (r ffmpegRuntime) videoDecodeArgs() []string {
	if !r.hwAccelActive {
		return nil
	}
	return []string{"-hwaccel", r.hwAccel}
}

func (r ffmpegRuntime) withVideoDecode(args ...string) []string {
	if decodeArgs := r.videoDecodeArgs(); len(decodeArgs) > 0 {
		merged := make([]string, 0, len(decodeArgs)+len(args))
		merged = append(merged, decodeArgs...)
		merged = append(merged, args...)
		return merged
	}
	return args
}

func resolveBinary(configuredPath, fallbackName string) string {
	if configuredPath != "" {
		if filepath.IsAbs(configuredPath) {
			if _, err := exec.LookPath(configuredPath); err == nil {
				return configuredPath
			}
			return ""
		}
		if path, err := exec.LookPath(configuredPath); err == nil {
			return path
		}
		return ""
	}
	path, err := exec.LookPath(fallbackName)
	if err != nil {
		return ""
	}
	return path
}

func resolveCompanionProbe(ffmpegPath string) string {
	if ffmpegPath == "" {
		return ""
	}
	if candidate, err := exec.LookPath(filepath.Join(filepath.Dir(ffmpegPath), "ffprobe")); err == nil {
		return candidate
	}
	if candidate, err := exec.LookPath("ffprobe"); err == nil {
		return candidate
	}
	return ""
}

func logValue(v string) string {
	if strings.TrimSpace(v) == "" {
		return "unresolved"
	}
	return v
}

func errFFmpegUnavailable(path string) error {
	if path != "" {
		return fmt.Errorf("ffmpeg not available: FFMPEG_PATH=%q could not be resolved", path)
	}
	return errors.New("ffmpeg not found in PATH")
}

func errFFprobeUnavailable(path string) error {
	if path != "" {
		return fmt.Errorf("ffprobe not available for FFMPEG_PATH=%q", path)
	}
	return errors.New("ffprobe not found in PATH")
}
