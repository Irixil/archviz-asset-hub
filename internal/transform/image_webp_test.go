package transform

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/HugoSmits86/nativewebp"
)

func makeTestPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, 16, 12))
	for y := range 12 {
		for x := range 16 {
			img.Set(x, y, color.NRGBA{
				R: uint8(x * 10),
				G: uint8(y * 15),
				B: 120,
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestEncodeImage_WebP(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 8))
	data, contentType, err := encodeImage(img, "webp", 42)
	if err != nil {
		t.Fatalf("encodeImage: %v", err)
	}
	if contentType != "image/webp" {
		t.Fatalf("contentType = %q, want image/webp", contentType)
	}

	decoded, err := nativewebp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("nativewebp.Decode: %v", err)
	}
	if got := decoded.Bounds().Dx(); got != 10 {
		t.Fatalf("decoded width = %d, want 10", got)
	}
	if got := decoded.Bounds().Dy(); got != 8 {
		t.Fatalf("decoded height = %d, want 8", got)
	}
}

func TestTransformer_ImageResize_WebP(t *testing.T) {
	tfm := &transformer{}

	data, contentType, err := tfm.ImageResize(bytes.NewReader(makeTestPNG(t)), ResizeParams{
		Width:   8,
		Height:  6,
		Fit:     "contain",
		Format:  "webp",
		Quality: 23,
	})
	if err != nil {
		t.Fatalf("ImageResize: %v", err)
	}
	if contentType != "image/webp" {
		t.Fatalf("contentType = %q, want image/webp", contentType)
	}

	decoded, err := nativewebp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("nativewebp.Decode: %v", err)
	}
	if decoded.Bounds().Dx() <= 0 || decoded.Bounds().Dy() <= 0 {
		t.Fatalf("decoded bounds = %v, want non-zero", decoded.Bounds())
	}
}
