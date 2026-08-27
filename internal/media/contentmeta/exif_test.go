package contentmeta

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func loadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("test file not available: %s (%v)", path, err)
	}
	return b
}

func approxEqual(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

func TestExtractImageEXIF_KodakPixpro(t *testing.T) {
	t.Parallel()
	data := loadFile(t, "/home/vincent/Downloads/100_2479.JPG")

	result, err := ExtractImageEXIF(t.Context(), bytes.NewReader(data), true)
	if err != nil {
		t.Fatalf("ExtractImageEXIF returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ExtractImageEXIF returned nil result for a JPEG with EXIF data")
	}

	t.Run("Make", func(t *testing.T) {
		t.Parallel()
		if result.Make == nil {
			t.Fatal("Make is nil")
		}
		if *result.Make == "" {
			t.Error("Make is empty string")
		}
	})

	t.Run("Model", func(t *testing.T) {
		t.Parallel()
		if result.Model == nil {
			t.Fatal("Model is nil")
		}
		if *result.Model == "" {
			t.Error("Model is empty string")
		}
	})

	t.Run("FNumber", func(t *testing.T) {
		t.Parallel()
		if result.FNumber == nil {
			t.Fatal("FNumber is nil — rational tag not decoded")
		}
		want := 3.3
		if !approxEqual(*result.FNumber, want, 0.01) {
			t.Errorf("FNumber = %v, want ~%v", *result.FNumber, want)
		}
	})

	t.Run("FocalLength", func(t *testing.T) {
		t.Parallel()
		if result.FocalLength == nil {
			t.Fatal("FocalLength is nil — rational tag not decoded")
		}
		want := 4.3
		if !approxEqual(*result.FocalLength, want, 0.01) {
			t.Errorf("FocalLength = %v, want ~%v", *result.FocalLength, want)
		}
	})

	t.Run("FocalLength35", func(t *testing.T) {
		t.Parallel()
		if result.FocalLength35 == nil {
			t.Fatal("FocalLength35 is nil — SHORT tag not decoded")
		}
		want := 24.0
		if !approxEqual(*result.FocalLength35, want, 0.5) {
			t.Errorf("FocalLength35 = %v, want ~%v", *result.FocalLength35, want)
		}
	})

	t.Run("ISO", func(t *testing.T) {
		t.Parallel()
		if result.ISO == nil {
			t.Fatal("ISO is nil")
		}
		if *result.ISO <= 0 {
			t.Errorf("ISO = %v, want positive value", *result.ISO)
		}
	})

	t.Run("ExposureTime", func(t *testing.T) {
		t.Parallel()
		if result.ExposureTime == nil {
			t.Fatal("ExposureTime is nil")
		}
		if *result.ExposureTime == "" {
			t.Error("ExposureTime is empty")
		}
	})

	t.Run("TakenAt", func(t *testing.T) {
		t.Parallel()
		if result.TakenAt == nil {
			t.Fatal("TakenAt is nil")
		}
		if result.TakenAt.IsZero() {
			t.Error("TakenAt is zero time")
		}
	})
}

func TestExtractImageEXIF_KodakPixpro_StripGPS(t *testing.T) {
	t.Parallel()
	data := loadFile(t, "/home/vincent/Downloads/100_2479.JPG")

	result, err := ExtractImageEXIF(t.Context(), bytes.NewReader(data), false)
	if err != nil {
		t.Fatalf("ExtractImageEXIF returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ExtractImageEXIF returned nil")
	}
	if result.GPS != nil {
		t.Errorf("GPS should be nil when keepGPS=false, got %+v", result.GPS)
	}
}

func TestExtractImageEXIF_NoEXIF(t *testing.T) {
	t.Parallel()
	result, err := ExtractImageEXIF(t.Context(), bytes.NewReader([]byte{0xFF, 0xD8, 0xFF, 0xE0}), true)
	if err != nil {
		t.Fatalf("unexpected error for no-EXIF data: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for no-EXIF data, got %+v", result)
	}
}

func TestExtractImageEXIF_Panic(t *testing.T) {
	t.Parallel()
	result, err := ExtractImageEXIF(t.Context(), bytes.NewReader(nil), true)
	_ = result
	_ = err
}

func TestExtractImageEXIF_TakenAt_Format(t *testing.T) {
	t.Parallel()
	data := loadFile(t, "/home/vincent/Downloads/100_2479.JPG")
	result, _ := ExtractImageEXIF(t.Context(), bytes.NewReader(data), false)
	if result == nil || result.TakenAt == nil {
		t.Skip("no TakenAt in test file")
	}
	if result.TakenAt.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("TakenAt suspiciously old: %v", result.TakenAt)
	}
}
