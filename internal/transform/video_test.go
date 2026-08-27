package transform

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func makeOddVideoFixture(t *testing.T) string {
	t.Helper()

	src := filepath.Join(t.TempDir(), "odd-source.mkv")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-y",
		"-f", "lavfi",
		"-i", "color=c=blue:s=321x241:d=0.5:r=12",
		"-c:v", "ffv1",
		src,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create odd video fixture: %v\n%s", err, string(out))
	}
	return src
}

func TestVideoWatermark_MP4KeepsOutputWithinSourceBounds(t *testing.T) {
	requireFFmpeg(t)

	src := makeOddVideoFixture(t)
	dst := filepath.Join(t.TempDir(), "watermarked.mp4")

	tr := NewTransformer()
	srcRes, err := tr.VideoExtractResolution(context.Background(), src)
	if err != nil {
		t.Fatalf("source resolution: %v", err)
	}

	err = tr.VideoWatermark(context.Background(), src, dst, bytes.NewReader(testWatermarkPNG(t)), VideoWatermarkParams{
		WatermarkAssetID: "wm_1",
		Opacity:          0.5,
		Format:           "mp4",
	})
	if err != nil {
		t.Fatalf("VideoWatermark: %v", err)
	}
	assertFileWritten(t, dst)

	outRes, err := tr.VideoExtractResolution(context.Background(), dst)
	if err != nil {
		t.Fatalf("output resolution: %v", err)
	}
	if outRes.Width > srcRes.Width || outRes.Height > srcRes.Height {
		t.Fatalf(
			"output resolution %dx%d exceeds source %dx%d",
			outRes.Width,
			outRes.Height,
			srcRes.Width,
			srcRes.Height,
		)
	}
	if outRes.Width%2 != 0 || outRes.Height%2 != 0 {
		t.Fatalf("expected even output dimensions for mp4, got %dx%d", outRes.Width, outRes.Height)
	}
}

func TestVideoWatermark_WEBMPreservesSourceResolution(t *testing.T) {
	requireFFmpeg(t)

	src := makeOddVideoFixture(t)
	dst := filepath.Join(t.TempDir(), "watermarked.webm")

	tr := NewTransformer()
	srcRes, err := tr.VideoExtractResolution(context.Background(), src)
	if err != nil {
		t.Fatalf("source resolution: %v", err)
	}

	err = tr.VideoWatermark(context.Background(), src, dst, bytes.NewReader(testWatermarkPNG(t)), VideoWatermarkParams{
		WatermarkAssetID: "wm_1",
		Opacity:          0.5,
		Format:           "webm",
	})
	if err != nil {
		t.Fatalf("VideoWatermark webm: %v", err)
	}
	assertFileWritten(t, dst)

	outRes, err := tr.VideoExtractResolution(context.Background(), dst)
	if err != nil {
		t.Fatalf("output resolution: %v", err)
	}
	if outRes.Width != srcRes.Width || outRes.Height != srcRes.Height {
		t.Fatalf(
			"expected webm output resolution %dx%d, got %dx%d",
			srcRes.Width,
			srcRes.Height,
			outRes.Width,
			outRes.Height,
		)
	}
}
