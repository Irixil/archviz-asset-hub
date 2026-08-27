package transform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

var ErrNoAudioStream = errors.New("no audio stream")

const (
	DefaultLUFS         = -16
	MinLUFS             = -70
	MaxLUFS             = 0
	audioFormatAAC      = "aac"
	audioFormatFLAC     = "flac"
	audioFormatMP3      = "mp3"
	audioFormatOGG      = "ogg"
	audioFormatOpus     = "opus"
	audioFormatWAV      = "wav"
	audioMimeMPEG       = "audio/mpeg"
	audioMimeOGG        = "audio/ogg"
	audioMimeWAV        = "audio/wav"
	audioMimeFLAC       = "audio/flac"
	ffmpegArgAudioCodec = "-c:a"
)

// AudioParams holds parsed, validated parameters for audio transforms.
type AudioParams struct {
	OutputFormat string  `json:"format,omitempty"`
	Bitrate      string  `json:"bitrate,omitempty"`
	Mono         bool    `json:"mono,omitempty"`
	TargetLUFS   float64 `json:"target_lufs,omitempty"`
}

type audioCodec struct {
	codec    string
	ext      string
	mimeType string
	lossless bool
}

var audioCodecs = map[string]audioCodec{
	audioFormatMP3:  {codec: "libmp3lame", ext: ".mp3", mimeType: audioMimeMPEG},
	audioFormatAAC:  {codec: audioFormatAAC, ext: ".m4a", mimeType: "audio/mp4"},
	audioFormatOpus: {codec: "libopus", ext: ".opus", mimeType: audioMimeOGG},
	audioFormatOGG:  {codec: "libvorbis", ext: ".ogg", mimeType: audioMimeOGG},
	audioFormatFLAC: {codec: audioFormatFLAC, ext: ".flac", mimeType: audioMimeFLAC, lossless: true},
	audioFormatWAV:  {codec: "pcm_s16le", ext: ".wav", mimeType: audioMimeWAV, lossless: true},
}

func (t *transformer) AudioWaveform(ctx context.Context, src io.Reader, mimeType string) ([]byte, string, error) {
	ext := MimeToExt(mimeType)

	tmpPath, cleanup, err := writeToTempFile(ctx, src, ext)
	if err != nil {
		return nil, "", fmt.Errorf("temp file: %w", err)
	}
	defer cleanup()

	var buf bytes.Buffer
	var stderr bytes.Buffer
	output := tmpPath + "_thumb" + ".png"

	if !t.ffmpeg.available() {
		return nil, "", errFFmpegUnavailable(t.ffmpeg.configuredPath)
	}
	cmd := t.ffmpeg.commandFFmpeg(ctx,
		"-i",
		tmpPath,
		"-filter_complex",
		"aformat=channel_layouts=mono,showwavespic=s=640x480:colors=#000000",
		"-frames:v",
		"1",
		output,
	)

	cmd.Stdout = &buf
	cmd.Stderr = &stderr

	if err = cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("ffmpeg failed: %w — stderr: %s", err, stderr.String())
	}

	f, err := os.Open(output)
	if err != nil {
		return nil, "", fmt.Errorf("open thumb: %w", err)
	}
	defer f.Close()
	thumbData, err := io.ReadAll(f)
	if err != nil {
		return nil, "", fmt.Errorf("read thumb: %w", err)
	}

	return thumbData, MimeImagePNG, nil
}

// ExtractAudio strips video streams from srcPath and writes the audio stream to dstPath.
func (t *transformer) ExtractAudio(ctx context.Context, srcPath, dstPath string, p AudioParams) error {
	p = defaultAudioParams(p, audioFormatAAC)
	hasAudio, err := t.probeHasAudio(ctx, srcPath)
	if err != nil {
		return err
	}
	if !hasAudio {
		return ErrNoAudioStream
	}

	c, err := codecForFormat(p.OutputFormat)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	args := []string{"-y", "-i", srcPath, "-vn", ffmpegArgAudioCodec, c.codec}
	if !c.lossless {
		args = append(args, "-b:a", p.Bitrate)
	}
	args = append(args, dstPath)
	return t.runFFmpeg(ctx, args...)
}

// TranscodeAudio re-encodes srcPath audio to the requested format and writes it to dstPath.
func (t *transformer) TranscodeAudio(ctx context.Context, srcPath, dstPath string, p AudioParams) error {
	p = defaultAudioParams(p, audioFormatMP3)
	c, err := codecForFormat(p.OutputFormat)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	args := []string{"-y", "-i", srcPath, ffmpegArgAudioCodec, c.codec}
	if !c.lossless {
		args = append(args, "-b:a", p.Bitrate)
	}
	if p.OutputFormat == audioFormatOpus {
		args = append(args, "-vbr", "on")
	}
	if p.Mono {
		args = append(args, "-ac", "1")
	}
	args = append(args, dstPath)
	return t.runFFmpeg(ctx, args...)
}

// NormalizeAudio applies EBU R128 loudness normalization and writes the result to dstPath.
func (t *transformer) NormalizeAudio(ctx context.Context, srcPath, dstPath string, p AudioParams) error {
	p = defaultAudioParams(p, audioFormatMP3)
	if p.TargetLUFS == 0 {
		p.TargetLUFS = DefaultLUFS
	}
	c, err := codecForFormat(p.OutputFormat)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	target := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", p.TargetLUFS), "0"), ".")
	measureFilter := fmt.Sprintf("loudnorm=I=%s:TP=-1.5:LRA=11:print_format=json", target)
	stderr, err := t.runFFmpegOutput(ctx, "-i", srcPath, "-af", measureFilter, "-f", "null", os.DevNull)
	if err == nil {
		if stats, parseErr := parseLoudnormStats(stderr); parseErr == nil {
			applyFilter := fmt.Sprintf(
				"loudnorm=I=%s:TP=-1.5:LRA=11:measured_I=%s:measured_LRA=%s:measured_TP=%s:measured_thresh=%s:offset=%s:linear=true:print_format=summary",
				target,
				stats.InputI,
				stats.InputLRA,
				stats.InputTP,
				stats.InputThresh,
				stats.TargetOffset,
			)
			return t.runNormalizePass(ctx, srcPath, dstPath, c, p, applyFilter)
		}
	}

	fallbackFilter := fmt.Sprintf("loudnorm=I=%s:TP=-1.5:LRA=11:linear=false", target)
	return t.runNormalizePass(ctx, srcPath, dstPath, c, p, fallbackFilter)
}

func defaultAudioParams(p AudioParams, fallbackFormat string) AudioParams {
	p.OutputFormat = strings.ToLower(strings.TrimSpace(p.OutputFormat))
	if p.OutputFormat == "" || p.OutputFormat == "source" {
		p.OutputFormat = fallbackFormat
	}
	if p.Bitrate == "" {
		p.Bitrate = "192k"
	}
	return p
}

func (t *transformer) runNormalizePass(
	ctx context.Context,
	srcPath, dstPath string,
	c audioCodec,
	p AudioParams,
	filter string,
) error {
	args := []string{"-y", "-i", srcPath, "-af", filter, ffmpegArgAudioCodec, c.codec}
	if !c.lossless {
		args = append(args, "-b:a", p.Bitrate)
	}
	args = append(args, dstPath)
	return t.runFFmpeg(ctx, args...)
}

func codecForFormat(format string) (audioCodec, error) {
	c, ok := audioCodecs[strings.ToLower(strings.TrimSpace(format))]
	if !ok {
		return audioCodec{}, fmt.Errorf("unsupported audio format: %s", format)
	}
	return c, nil
}

func AudioExtension(format string) string {
	if c, ok := audioCodecs[strings.ToLower(strings.TrimSpace(format))]; ok {
		return c.ext
	}
	return ".mp3"
}

func AudioMimeType(format string) string {
	if c, ok := audioCodecs[strings.ToLower(strings.TrimSpace(format))]; ok {
		return c.mimeType
	}
	return audioMimeMPEG
}

func AudioFormatFromMimeType(mimeType string) string {
	switch mimeType {
	case audioMimeMPEG, "audio/mp3":
		return audioFormatMP3
	case "audio/aac", "audio/mp4", "audio/x-m4a":
		return "aac"
	case audioMimeOGG, "audio/opus":
		return audioFormatOGG
	case audioMimeFLAC:
		return audioFormatFLAC
	case audioMimeWAV, "audio/x-wav":
		return audioFormatWAV
	default:
		return audioFormatMP3
	}
}

func (t *transformer) probeHasAudio(ctx context.Context, srcPath string) (bool, error) {
	if !t.ffmpeg.ffprobeAvailable() {
		return false, errFFprobeUnavailable(t.ffmpeg.configuredPath)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := t.ffmpeg.commandFFprobe(ctx,
		"-v", "error",
		"-select_streams", "a",
		"-show_entries", "stream=index",
		"-of", "csv=p=0",
		srcPath,
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("ffprobe audio: %w — stderr: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()) != "", nil
}

func (t *transformer) runFFmpeg(ctx context.Context, args ...string) error {
	_, err := t.runFFmpegOutput(ctx, args...)
	return err
}

func (t *transformer) runFFmpegOutput(ctx context.Context, args ...string) (string, error) {
	if !t.ffmpeg.available() {
		return "", errFFmpegUnavailable(t.ffmpeg.configuredPath)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := t.ffmpeg.commandFFmpeg(ctx, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stderr.String(), fmt.Errorf("ffmpeg failed: %w — stderr: %s", err, stderr.String())
	}
	return stderr.String(), nil
}

type loudnormStats struct {
	InputI       string `json:"input_i"`
	InputTP      string `json:"input_tp"`
	InputLRA     string `json:"input_lra"`
	InputThresh  string `json:"input_thresh"`
	TargetOffset string `json:"target_offset"`
}

func parseLoudnormStats(stderr string) (loudnormStats, error) {
	re := regexp.MustCompile(`(?s)\{[^{}]*"input_i"[^{}]*\}`)
	raw := re.FindString(stderr)
	if raw == "" {
		return loudnormStats{}, errors.New("loudnorm stats JSON not found")
	}
	var stats loudnormStats
	if err := json.Unmarshal([]byte(raw), &stats); err != nil {
		return loudnormStats{}, err
	}
	if stats.InputI == "" || stats.InputTP == "" || stats.InputLRA == "" || stats.InputThresh == "" ||
		stats.TargetOffset == "" {
		return loudnormStats{}, errors.New("loudnorm stats incomplete")
	}
	return stats, nil
}
