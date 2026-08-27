package visualsimilarity

import (
	"image"
	"image/color"
	"testing"
)

func solidImage(c color.Color, w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestCompute_IdenticalImages(t *testing.T) {
	img := solidImage(color.RGBA{R: 128, G: 64, B: 200, A: 255}, 100, 100)

	h1, err := Compute(img)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	h2, err := Compute(img)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	if h1.CentralHash != h2.CentralHash {
		t.Errorf("identical images: central hashes differ: %d vs %d", h1.CentralHash, h2.CentralHash)
	}

	overlap := hashSetOverlap(h1.HashSet, h2.HashSet)
	if !overlap {
		t.Errorf("identical images: hash sets do not overlap")
	}
}

func TestCompute_DifferentImages(t *testing.T) {
	white := solidImage(color.White, 100, 100)
	black := solidImage(color.Black, 100, 100)

	hw, err := Compute(white)
	if err != nil {
		t.Fatalf("Compute white: %v", err)
	}
	hb, err := Compute(black)
	if err != nil {
		t.Fatalf("Compute black: %v", err)
	}

	if hashSetOverlap(hw.HashSet, hb.HashSet) {
		t.Errorf("white vs black: unexpected hash set overlap")
	}
}

func TestMarshalUnmarshalHashSet(t *testing.T) {
	original := []uint64{1, 2, 3, 1<<63 - 1, 0}

	s, err := MarshalHashSet(original)
	if err != nil {
		t.Fatalf("MarshalHashSet: %v", err)
	}

	got, err := UnmarshalHashSet(s)
	if err != nil {
		t.Fatalf("UnmarshalHashSet: %v", err)
	}

	if len(got) != len(original) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(original))
	}
	for i := range original {
		if got[i] != original[i] {
			t.Errorf("index %d: got %d, want %d", i, got[i], original[i])
		}
	}
}

func hashSetOverlap(a, b []uint64) bool {
	m := make(map[uint64]struct{}, len(a))
	for _, v := range a {
		m[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := m[v]; ok {
			return true
		}
	}
	return false
}
