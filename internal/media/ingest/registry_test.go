package ingest

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"damask/server/internal/transform"
)

func TestRegistry_SupportsExpectedMimeTypes(t *testing.T) {
	r := NewRegistry(transform.NewTransformer())

	if !r.Supports("image/png") {
		t.Fatal("expected image/png to be supported")
	}
	if !r.Supports("video/mp4") {
		t.Fatal("expected video/mp4 to be supported")
	}
	if !r.Supports("audio/mpeg") {
		t.Fatal("expected audio/mpeg to be supported by default handler")
	}
	if r.Supports("application/x-unknown") {
		t.Fatal("did not expect application/x-unknown to be supported")
	}
}

func TestRegistry_ExtractMeta_ImageDimensions(t *testing.T) {
	r := NewRegistry(transform.NewTransformer())

	path := filepath.Join(t.TempDir(), "tiny.png")
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	if e := png.Encode(f, img); e != nil {
		_ = f.Close()
		t.Fatalf("encode png: %v", e)
	}
	if e := f.Close(); e != nil {
		t.Fatalf("close png: %v", e)
	}

	meta, err := r.ExtractMeta(context.Background(), path, "image/png")
	if err != nil {
		t.Fatalf("ExtractMeta() error = %v", err)
	}
	if meta.Width == nil || *meta.Width != 3 {
		t.Fatalf("Width = %v, want 3", meta.Width)
	}
	if meta.Height == nil || *meta.Height != 2 {
		t.Fatalf("Height = %v, want 2", meta.Height)
	}
}
