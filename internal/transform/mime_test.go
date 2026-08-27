package transform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectMimeType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  []byte
		want     string
	}{
		{
			name:     "jpeg by magic bytes",
			filename: "photo.jpg",
			content:  []byte("\xff\xd8\xff\xe0" + "fake jpeg content"),
			want:     "image/jpeg",
		},
		{
			name:     "png by magic bytes",
			filename: "image.png",
			content:  []byte("\x89PNG\r\n\x1a\n" + "fake png content"),
			want:     "image/png",
		},
		{
			name:     "docx by extension fallback",
			filename: "document.docx",
			// PK zip magic — sniffed as application/zip without fallback
			content: []byte("PK\x03\x04fake zip content"),
			want:    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		},
		{
			name:     "xlsx by extension fallback",
			filename: "spreadsheet.xlsx",
			content:  []byte("PK\x03\x04fake zip content"),
			want:     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		},
		{
			name:     "pptx by extension fallback",
			filename: "presentation.pptx",
			content:  []byte("PK\x03\x04fake zip content"),
			want:     "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		},
		{
			name:     "odt by extension fallback",
			filename: "document.odt",
			content:  []byte("PK\x03\x04fake zip content"),
			want:     "application/vnd.oasis.opendocument.text",
		},
		{
			name:     "csv by extension fallback",
			filename: "data.csv",
			// plain text content — sniffed as text/plain without fallback
			content: []byte("name,age\nalice,30\n"),
			want:    "text/csv",
		},
		{
			name:     "unknown extension keeps sniffed type",
			filename: "file.unknownxyz",
			content:  []byte("PK\x03\x04fake zip content"),
			want:     "application/zip",
		},
		{
			name:     "no extension keeps sniffed type",
			filename: "Makefile",
			content:  []byte("all:\n\techo hello\n"),
			want:     "text/plain",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.filename)
			if err := os.WriteFile(path, tc.content, 0644); err != nil {
				t.Fatalf("write file: %v", err)
			}
			got, err := DetectMimeType(path)
			if err != nil {
				t.Fatalf("DetectMimeType: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
