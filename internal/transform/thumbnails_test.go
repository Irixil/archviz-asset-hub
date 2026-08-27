package transform_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	"damask/server/internal/storage"
	"damask/server/internal/transform"
)

// createTestJPEG creates a minimal valid JPEG image for testing.
func createTestJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

// createTestPNG creates a minimal valid PNG image for testing (as bytes).
func createTestPNG() []byte {
	// Create a real PNG using image package
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	var buf bytes.Buffer
	// Use default PNG encoder which creates valid checksums
	if err := (&png.Encoder{}).Encode(&buf, img); err != nil {
		// Fallback if encoding fails - this shouldn't happen in tests
		panic(err)
	}
	return buf.Bytes()
}

func TestGenerateThumbnailData(t *testing.T) {
	jpegData := createTestJPEG()
	pngData := createTestPNG()
	plainTextData := []byte("Hello, this is plain text content for thumbnail generation.")

	thumbnailer := transform.NewThumbnailer(&mockTransformer{})

	tests := []struct {
		name       string
		storage    storage.Storage
		mimeType   string
		storageKey string
		wantBytes  bool
		wantExt    string
		wantErr    bool
	}{
		{
			name: "JPEG image generates thumbnail",
			storage: &mockStorage{
				data: map[string][]byte{"test.jpg": jpegData},
			},
			mimeType:   transform.MimeImageJPEG,
			storageKey: "test.jpg",
			wantBytes:  true,
			wantExt:    ".jpg",
			wantErr:    false,
		},
		{
			name: "PNG image generates thumbnail",
			storage: &mockStorage{
				data: map[string][]byte{"test.png": pngData},
			},
			mimeType:   transform.MimeImagePNG,
			storageKey: "test.png",
			wantBytes:  true,
			wantExt:    ".jpg",
			wantErr:    false,
		},
		{
			name: "WebP image generates thumbnail",
			storage: &mockStorage{
				data: map[string][]byte{"test.webp": jpegData}, // reuse JPEG for simplicity
			},
			mimeType:   transform.MimeImageWebP,
			storageKey: "test.webp",
			wantBytes:  true,
			wantExt:    ".jpg",
			wantErr:    false,
		},
		{
			name: "Plain text generates thumbnail",
			storage: &mockStorage{
				data: map[string][]byte{"test.txt": plainTextData},
			},
			mimeType:   transform.MimeTextPlain,
			storageKey: "test.txt",
			wantBytes:  true,
			wantExt:    ".png",
			wantErr:    false,
		},
		{
			name: "HTML text generates thumbnail",
			storage: &mockStorage{
				data: map[string][]byte{"test.html": []byte("<html><body>Test HTML</body></html>")},
			},
			mimeType:   transform.MimeTextHTML,
			storageKey: "test.html",
			wantBytes:  true,
			wantExt:    ".png",
			wantErr:    false,
		},
		{
			name: "Storage.Get fails for image",
			storage: &mockStorage{
				err: errors.New("storage unavailable"),
			},
			mimeType:   transform.MimeImageJPEG,
			storageKey: "missing.jpg",
			wantBytes:  false,
			wantExt:    "",
			wantErr:    true,
		},
		{
			name: "Storage.Get fails for text",
			storage: &mockStorage{
				err: errors.New("access denied"),
			},
			mimeType:   transform.MimeTextPlain,
			storageKey: "missing.txt",
			wantBytes:  false,
			wantExt:    "",
			wantErr:    true,
		},
		{
			name: "Unsupported MIME type returns no error",
			storage: &mockStorage{
				data: map[string][]byte{},
			},
			mimeType:   transform.MimeApplicationOctetStream,
			storageKey: "unknown.bin",
			wantBytes:  false,
			wantExt:    "",
			wantErr:    false,
		},
		{
			name: "Storage.Get fails for video returns error",
			storage: &mockStorage{
				err: errors.New("video not found"),
			},
			mimeType:   transform.MimeVideoMP4,
			storageKey: "missing.mp4",
			wantBytes:  false,
			wantExt:    "",
			wantErr:    true,
		},
		{
			name: "Empty text file generates thumbnail",
			storage: &mockStorage{
				data: map[string][]byte{"empty.txt": []byte("")},
			},
			mimeType:   transform.MimeTextPlain,
			storageKey: "empty.txt",
			wantBytes:  true,
			wantExt:    ".png",
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, gotExt, gotErr := thumbnailer.GenerateThumbnailData(
				context.Background(),
				tt.storage,
				tt.mimeType,
				tt.storageKey,
			)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GenerateThumbnailData() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GenerateThumbnailData() succeeded unexpectedly")
			}
			if (len(gotBytes) > 0) != tt.wantBytes {
				t.Errorf("GenerateThumbnailData() got %v bytes, wantBytes=%v", len(gotBytes), tt.wantBytes)
			}
			if gotExt != tt.wantExt {
				t.Errorf("GenerateThumbnailData() got ext %q, want %q", gotExt, tt.wantExt)
			}
		})
	}
}
