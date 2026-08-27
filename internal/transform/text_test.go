package transform_test

import (
	"bytes"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"damask/server/internal/transform"
)

func TestGenerateImageOfText_DefaultFont(t *testing.T) {
	t.Parallel()

	tf := &mockTransformer{}
	tests := []struct {
		name        string
		textContent string
		fgColorHex  string
		bgColorHex  string
		fontSize    float64
		wantErr     bool
	}{
		{
			name:        "multiline text with default font",
			textContent: "Lorem ipsum dolor sit amet",
			fgColorHex:  "#000000",
			bgColorHex:  "#ffffff",
			fontSize:    14,
			wantErr:     false,
		},
		{
			name:        "simple text",
			textContent: "Hello, World!",
			fgColorHex:  "#000000",
			bgColorHex:  "#ffffff",
			fontSize:    16,
			wantErr:     false,
		},
		{
			name:        "custom colors",
			textContent: "Red text on blue",
			fgColorHex:  "#ff0000",
			bgColorHex:  "#0000ff",
			fontSize:    12,
			wantErr:     false,
		},
		{
			name:        "auto font size",
			textContent: "Auto sized text",
			fgColorHex:  "#000000",
			bgColorHex:  "#ffffff",
			fontSize:    0, // auto-calculate
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tf.GenerateImageOfText(t.Context(), transform.ImageOfTextOptions{
				TextContent: tt.textContent,
				FgColorHex:  tt.fgColorHex,
				BgColorHex:  tt.bgColorHex,
				FontSize:    tt.fontSize,
				Width:       400,
				Height:      400,
			})

			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateImageOfText() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && !isPNG(got) {
				t.Error("GenerateImageOfText() did not produce valid PNG")
			}
		})
	}
}

//nolint:tparallel // shared state
func TestGenerateImageOfText_CustomFont(t *testing.T) {
	t.Parallel()
	tf := &mockTransformer{}
	fontPath := "AovelSansRounded-rdDL.ttf"

	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		t.Skipf("Font file %s not found", fontPath)
	}
	ff, _ := os.Open(fontPath)
	defer ff.Close()

	tests := []struct {
		name        string
		textContent string
		fontSize    float64
		wantErr     bool
	}{
		{
			name:        "simple text with custom font",
			textContent: filepath.Base(fontPath) + "\n\nThe quick brown fox jumps over the lazy dog.",
			fontSize:    16,
			wantErr:     false,
		},
		{
			name:        "multiline with custom font",
			textContent: "Line 1\nLine 2\nLine 3",
			fontSize:    14,
			wantErr:     false,
		},
		{
			name:        "custom font with auto sizing",
			textContent: "Auto sized custom",
			fontSize:    0,
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _ = ff.Seek(0, io.SeekStart)
			got, err := tf.GenerateImageOfText(t.Context(), transform.ImageOfTextOptions{
				TextContent: tt.textContent,
				FontSize:    tt.fontSize,
				FontFile:    ff,
				Width:       400,
				Height:      400,
			})

			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateImageOfText() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// os.WriteFile("/tmp/ttf.png", got, os.FileMode(os.O_CREATE))
				if !isPNG(got) {
					t.Error("GenerateImageOfText() did not produce valid PNG")
				}
			}
		})
	}
}

//nolint:tparallel // shared state
func TestGenerateImageOfText_ErrorCases(t *testing.T) {
	t.Parallel()
	tf := transform.NewTransformer()
	tests := []struct {
		name        string
		opts        transform.ImageOfTextOptions
		wantErr     bool
		errContains string
	}{
		{
			name: "invalid font file path",
			opts: transform.ImageOfTextOptions{
				TextContent: "Hello",
				FontFile:    io.NopCloser(bytes.NewReader([]byte("not a real font file"))),
				Width:       400,
				Height:      400,
			},
			wantErr:     true,
			errContains: "read font file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tf.GenerateImageOfText(t.Context(), tt.opts)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateImageOfText() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func isPNG(data []byte) bool {
	_, err := png.Decode(bytes.NewReader(data))
	return err == nil && len(data) > 0
}
