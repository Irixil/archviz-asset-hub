package transform

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not found")
	}
}

func testdataPath(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("missing fixture %s: %v", path, err)
	}
	return path
}

func outputHasAudioCodec(t *testing.T, path, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx,
		"ffprobe",
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		t.Fatalf("ffprobe codec: %v", err)
	}
	got := strings.TrimSpace(string(out))
	if got != want {
		t.Fatalf("codec = %q, want %q", got, want)
	}
}

func outputChannels(t *testing.T, path string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx,
		"ffprobe",
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=channels",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		t.Fatalf("ffprobe channels: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func assertFileWritten(t *testing.T, path string) {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("output not written: %v", err)
	}
	if st.Size() == 0 {
		t.Fatalf("output is empty")
	}
}

func TestExtractAudio_MP3(t *testing.T) {
	requireFFmpeg(t)
	dst := filepath.Join(t.TempDir(), "out.mp3")
	err := NewTransformer().ExtractAudio(context.Background(), testdataPath(t, "sample_video_with_audio.mp4"), dst, AudioParams{
		OutputFormat: "mp3",
		Bitrate:      "128k",
	})
	if err != nil {
		t.Fatalf("ExtractAudio: %v", err)
	}
	assertFileWritten(t, dst)
	outputHasAudioCodec(t, dst, "mp3")
}

func TestExtractAudio_AAC(t *testing.T) {
	requireFFmpeg(t)
	dst := filepath.Join(t.TempDir(), "out.m4a")
	err := NewTransformer().ExtractAudio(context.Background(), testdataPath(t, "sample_video_with_audio.mp4"), dst, AudioParams{
		OutputFormat: "aac",
		Bitrate:      "128k",
	})
	if err != nil {
		t.Fatalf("ExtractAudio: %v", err)
	}
	assertFileWritten(t, dst)
	outputHasAudioCodec(t, dst, "aac")
}

func TestExtractAudio_NoAudioStream(t *testing.T) {
	requireFFmpeg(t)
	dst := filepath.Join(t.TempDir(), "out.mp3")
	err := NewTransformer().ExtractAudio(context.Background(), testdataPath(t, "sample_video_no_audio.mp4"), dst, AudioParams{
		OutputFormat: "mp3",
	})
	if !errors.Is(err, ErrNoAudioStream) {
		t.Fatalf("error = %v, want ErrNoAudioStream", err)
	}
}

func TestTranscodeAudio_MP3toOpus(t *testing.T) {
	requireFFmpeg(t)
	dst := filepath.Join(t.TempDir(), "out.opus")
	err := NewTransformer().TranscodeAudio(context.Background(), testdataPath(t, "sample_audio_loud.mp3"), dst, AudioParams{
		OutputFormat: "opus",
		Bitrate:      "96k",
	})
	if err != nil {
		t.Fatalf("TranscodeAudio: %v", err)
	}
	assertFileWritten(t, dst)
	outputHasAudioCodec(t, dst, "opus")
}

func TestTranscodeAudio_MonoMixdown(t *testing.T) {
	requireFFmpeg(t)
	dst := filepath.Join(t.TempDir(), "out.mp3")
	err := NewTransformer().TranscodeAudio(context.Background(), testdataPath(t, "sample_audio_loud.mp3"), dst, AudioParams{
		OutputFormat: "mp3",
		Bitrate:      "96k",
		Mono:         true,
	})
	if err != nil {
		t.Fatalf("TranscodeAudio: %v", err)
	}
	assertFileWritten(t, dst)
	if channels := outputChannels(t, dst); channels != "1" {
		t.Fatalf("channels = %s, want 1", channels)
	}
}

func TestTranscodeAudio_BitrateDowngrade(t *testing.T) {
	requireFFmpeg(t)
	dst := filepath.Join(t.TempDir(), "out.mp3")
	err := NewTransformer().TranscodeAudio(context.Background(), testdataPath(t, "sample_audio_loud.mp3"), dst, AudioParams{
		OutputFormat: "mp3",
		Bitrate:      "64k",
	})
	if err != nil {
		t.Fatalf("TranscodeAudio: %v", err)
	}
	assertFileWritten(t, dst)
	outputHasAudioCodec(t, dst, "mp3")
}

func TestNormalizeAudio_TwoPass(t *testing.T) {
	requireFFmpeg(t)
	dst := filepath.Join(t.TempDir(), "out.mp3")
	err := NewTransformer().NormalizeAudio(context.Background(), testdataPath(t, "sample_audio_loud.mp3"), dst, AudioParams{
		OutputFormat: "mp3",
		Bitrate:      "128k",
		TargetLUFS:   DefaultLUFS,
	})
	if err != nil {
		t.Fatalf("NormalizeAudio: %v", err)
	}
	assertFileWritten(t, dst)
	outputHasAudioCodec(t, dst, "mp3")
}

func TestNormalizeAudio_ShortFile(t *testing.T) {
	requireFFmpeg(t)
	tmp := t.TempDir()
	src := filepath.Join(tmp, "short.wav")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx,
		"ffmpeg",
		"-y",
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=0.5",
		src,
	).Run(); err != nil {
		t.Fatalf("create short fixture: %v", err)
	}

	dst := filepath.Join(tmp, "out.wav")
	err := NewTransformer().NormalizeAudio(context.Background(), src, dst, AudioParams{
		OutputFormat: "wav",
		TargetLUFS:   DefaultLUFS,
	})
	if err != nil {
		t.Fatalf("NormalizeAudio: %v", err)
	}
	assertFileWritten(t, dst)
	outputHasAudioCodec(t, dst, "pcm_s16le")
}

func TestCodecForFormat_UnknownFormat(t *testing.T) {
	if _, err := codecForFormat("bad"); err == nil {
		t.Fatal("expected error")
	}
}
