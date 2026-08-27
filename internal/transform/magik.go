package transform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
)

func MagikFirstThumbnail(ctx context.Context, src io.Reader, mimeType string) (data []byte, format string, err error) {
	ctx, span := telemetry.StartSpan(ctx, "transform.pdf.first.thumbnail",
		attribute.String("damask.mimeType", mimeType),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		slog.DebugContext(ctx, "magik.firstthumbnail completed", "error", err)
	}()

	ext := MimeToExt(mimeType)

	tmpPath, cleanup, err := writeToTempFile(ctx, src, ext)
	if err != nil {
		return nil, "", fmt.Errorf("temp file: %w", err)
	}
	defer cleanup()

	output := tmpPath + "_thumb" + ".jpg"
	var buf bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, //nolint:gosec // validated args
		"convert",
		"-units", "pixelsperinch",
		"-density", "72",
		tmpPath+"[0]",
		output,
	)

	cmd.Stdout = &buf
	cmd.Stderr = &stderr

	slog.DebugContext(ctx, "running magik command", "command", cmd.String())
	span.SetAttributes(attribute.String("transform.command", cmd.String()))

	if err = cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("convert failed: %w — stderr: %s", err, stderr.String())
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

	return thumbData, MimeImageJPEG, nil
}

// PDFSlideshowThumbnail converts up to 6 PDF pages to JPEG frames via ImageMagick convert.
// Single-page PDFs return a JPEG directly (mimeImageJPEG). Multi-page PDFs are concatenated
// into a silent MP4 slideshow via ffmpeg ("video/mp4").
func (t *transformer) PDFSlideshowThumbnail(
	ctx context.Context,
	src io.Reader,
	mimeType string,
) (data []byte, contentType string, err error) {
	ctx, span := telemetry.StartSpan(ctx, "transform.pdf.slideshow.thumbnail",
		attribute.String("damask.mimeType", mimeType),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		slog.DebugContext(ctx, "magik.pdfslideshowthumbnail completed", "error", err)
	}()

	tmpPDF, cleanup, err := writeToTempFile(ctx, src, ".pdf")
	if err != nil {
		return nil, "", fmt.Errorf("temp pdf: %w", err)
	}
	defer cleanup()

	dir, err := os.MkdirTemp("", "damask-pdf-*")
	if err != nil {
		return nil, "", fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	// convert -density 72 -format jpeg input.pdf[0-5] outdir/page-%d.jpg
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, //nolint:gosec // validated args
		"convert",
		"-density", "72",
		"-format", FormatJPEG,
		tmpPDF+"[0-5]",
		filepath.Join(dir, "page-%d.jpg"),
	)
	cmd.Stderr = &stderr
	span.SetAttributes(attribute.String("transform.command", cmd.String()))

	if err = cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("convert pdf: %w — stderr: %s", err, stderr.String())
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return nil, "", errors.New("no pages extracted from PDF")
	}

	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			n++
		}
	}

	// Single-page PDF: return the JPEG directly — no video needed.
	if n == 1 {
		data, err = os.ReadFile(filepath.Join(dir, entries[0].Name()))
		if err != nil {
			return nil, "", fmt.Errorf("read single page jpeg: %w", err)
		}
		return data, MimeImageJPEG, nil
	}

	return t.buildPDFSlideshowVideo(ctx, dir, entries, n)
}

// buildPDFSlideshowVideo assembles a silent MP4 slideshow from n already-rasterized
// PDF page JPEGs in dir, with a quadratic ease-in slide-transition between pages.
func (t *transformer) buildPDFSlideshowVideo(
	ctx context.Context,
	dir string,
	entries []os.DirEntry,
	n int,
) (data []byte, contentType string, err error) {
	outPath := filepath.Join(dir, "out.mp4")

	// Build ffmpeg args: one -loop 1 -t 3 -i <img> per page, then filter_complex
	// for slide-in transitions using overlay with eased x offset.
	ffArgs := []string{"-y"}
	for _, e := range entries {
		if !e.IsDir() {
			ffArgs = append(ffArgs, "-loop", "1", "-t", "3", "-i", filepath.Join(dir, e.Name()))
		}
	}

	// filter_complex: shift each input's PTS by its index seconds, then chain overlays
	// with a quadratic ease-in slide from offscreen right.
	var fc strings.Builder
	for i := 1; i < n; i++ {
		fmt.Fprintf(&fc, "[%d]setpts=PTS+%d/TB[i%d];\n", i, i, i)
	}
	prev := "[0]"
	for i := 1; i < n; i++ {
		out := fmt.Sprintf("[f%d]", i)
		if i == n-1 {
			out = "[out]"
		}
		shortest := ""
		if i == n-1 {
			shortest = ":shortest=1"
		}
		fmt.Fprintf(&fc, "%s[i%d]overlay=x='if(lt(t,%d),W+1,max(0,W*(1-(t-%d)^2)))':y=0%s %s;\n",
			prev, i, i, i, shortest, out)
		prev = out
	}

	// append scale after the final overlay; can't use -vf alongside -filter_complex + -map
	filterStr := fc.String() + "[out]scale=400:-2[scaled]"
	ffArgs = append(ffArgs,
		"-filter_complex", filterStr,
		"-map", "[scaled]",
		"-r", "30",
		"-c:v", "libx264", "-movflags", "+faststart", "-preset", "fast", "-an",
		outPath,
	)
	if !t.ffmpeg.available() {
		return nil, "", errFFmpegUnavailable(t.ffmpeg.configuredPath)
	}
	ffCmd := t.ffmpeg.commandFFmpeg(ctx, ffArgs...)
	var ffmpegStderr bytes.Buffer
	ffCmd.Stderr = &ffmpegStderr
	if err = ffCmd.Run(); err != nil {
		return nil, "", fmt.Errorf("ffmpeg pdf slideshow: %w — stderr: %s", err, ffmpegStderr.String())
	}

	data, err = os.ReadFile(outPath)
	if err != nil {
		return nil, "", fmt.Errorf("read slideshow: %w", err)
	}
	return data, "video/mp4", nil
}

// ---- OS helpers ----

func writeToTempFile(ctx context.Context, src io.Reader, ext string) (name string, cb func(), err error) {
	ctx, span := telemetry.StartSpan(ctx, "transform.write.tempfile",
		attribute.String("damask.ext", ext),
	)
	defer telemetry.EndSpan(span, err)

	f, err := os.CreateTemp("", "damask-*"+ext)
	if err != nil {
		return "", nil, fmt.Errorf("create temp: %w", err)
	}
	span.SetAttributes(attribute.String("damask.tempfile", f.Name()))
	if _, copyErr := io.Copy(f, src); copyErr != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("copy to temp: %w", copyErr)
	}
	err = f.Close()
	if err != nil {
		return "", nil, fmt.Errorf("close temp: %w", err)
	}
	return f.Name(), func() {
		slog.DebugContext(ctx, "cleaning up temp file", "tempfile", f.Name())
		_ = os.Remove(f.Name())
	}, nil
}
