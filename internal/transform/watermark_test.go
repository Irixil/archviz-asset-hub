package transform

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func testSourcePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 120, 80))
	for y := range 80 {
		for x := range 120 {
			img.Set(x, y, color.NRGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	return encodePNG(t, img)
}

func testWatermarkPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	for y := range 20 {
		for x := range 40 {
			img.Set(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	return encodePNG(t, img)
}

func TestApplyWatermark_DefaultPosition(t *testing.T) {
	t.Parallel()
	out, err := ApplyWatermark(
		context.Background(),
		bytes.NewReader(testSourcePNG(t)),
		bytes.NewReader(testWatermarkPNG(t)),
		WatermarkParams{},
	)
	if err != nil {
		t.Fatalf("ApplyWatermark: %v", err)
	}
	if out.Bounds().Dx() != 120 || out.Bounds().Dy() != 80 {
		t.Fatalf("unexpected output size: %v", out.Bounds())
	}
}

func TestApplyWatermark_InvalidSourceReturnsError(t *testing.T) {
	t.Parallel()
	_, err := ApplyWatermark(
		context.Background(),
		strings.NewReader("bad"),
		bytes.NewReader(testWatermarkPNG(t)),
		WatermarkParams{},
	)
	if err == nil || !strings.Contains(err.Error(), "decode source image") {
		t.Fatalf("expected source decode error, got %v", err)
	}
}

func TestApplyWatermark_InvalidWatermarkReturnsError(t *testing.T) {
	t.Parallel()
	_, err := ApplyWatermark(
		context.Background(),
		bytes.NewReader(testSourcePNG(t)),
		strings.NewReader("bad"),
		WatermarkParams{},
	)
	if err == nil || !strings.Contains(err.Error(), "decode watermark image") {
		t.Fatalf("expected watermark decode error, got %v", err)
	}
}

func TestApplyWatermark_TilesAcrossImage(t *testing.T) {
	t.Parallel()
	out, err := ApplyWatermark(
		context.Background(),
		bytes.NewReader(testSourcePNG(t)),
		bytes.NewReader(testWatermarkPNG(t)),
		WatermarkParams{
			Opacity: 1,
		},
	)
	if err != nil {
		t.Fatalf("ApplyWatermark: %v", err)
	}
	samples := []image.Point{
		{X: 10, Y: 10},
		{X: 50, Y: 10},
		{X: 10, Y: 40},
		{X: 90, Y: 60},
	}
	for _, sample := range samples {
		pixel := color.NRGBAModel.Convert(out.At(sample.X, sample.Y)).(color.NRGBA)
		if pixel.R == 240 && pixel.G == 240 && pixel.B == 240 {
			t.Fatalf("expected tiled watermark to change pixel at %+v, got %#v", sample, pixel)
		}
	}
}
