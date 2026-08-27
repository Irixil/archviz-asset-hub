package transform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type OCRParams struct {
	Lang         string
	OutputFormat string
}

type OCRResult struct {
	PlainText   string
	FileContent []byte
	ContentType string
	Extension   string
}

func TesseractAvailable() bool {
	_, err := exec.LookPath("tesseract")
	return err == nil
}

// ListOCRLanguages returns the Tesseract language codes actually installed
// on this host (e.g. "eng", "fra"), by parsing `tesseract --list-langs`.
// The "osd" entry (orientation/script detection data, not a real language)
// is filtered out.
func ListOCRLanguages(ctx context.Context) ([]string, error) {
	if !TesseractAvailable() {
		return nil, errors.New("ocr: tesseract not available")
	}

	cmd := exec.CommandContext(ctx, "tesseract", "--list-langs")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ocr: list langs: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	lines := strings.Split(stdout.String(), "\n")
	langs := make([]string, 0, len(lines))
	for _, line := range lines[1:] { // first line is a header, not a language
		line = strings.TrimSpace(line)
		if line == "" || line == "osd" {
			continue
		}
		langs = append(langs, line)
	}
	return langs, nil
}

var SupportedOCRMIMEs = map[string]bool{
	"image/bmp":   true,
	MimeImageJPEG: true,
	MimeImagePNG:  true,
	"image/tiff":  true,
	MimeImageWebP: true,
}

func RunOCR(ctx context.Context, imageData []byte, p OCRParams) (*OCRResult, error) {
	if p.Lang == "" {
		p.Lang = "eng"
	}
	if p.OutputFormat == "" {
		p.OutputFormat = "txt"
	}
	if p.OutputFormat != "txt" && p.OutputFormat != "hocr" {
		return nil, fmt.Errorf("ocr: unsupported output format %q", p.OutputFormat)
	}

	tmpIn, err := os.CreateTemp("", "damask-ocr-*")
	if err != nil {
		return nil, fmt.Errorf("ocr: create temp file: %w", err)
	}
	defer os.Remove(tmpIn.Name())

	if _, err = tmpIn.Write(imageData); err != nil {
		_ = tmpIn.Close()
		return nil, fmt.Errorf("ocr: write temp file: %w", err)
	}
	if err = tmpIn.Close(); err != nil {
		return nil, fmt.Errorf("ocr: close temp file: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tesseract", tmpIn.Name(), "stdout", "-l", p.Lang, p.OutputFormat) //nolint:gosec,golines // arguments should come from config or LookPath, not user input
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		return nil, fmt.Errorf("ocr: tesseract: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	fileBytes := stdout.Bytes()
	plainText := strings.TrimSpace(string(fileBytes))
	contentType := "text/plain; charset=utf-8"
	extension := ".txt"
	if p.OutputFormat == "hocr" {
		plainText = stripHTMLTags(string(fileBytes))
		contentType = "text/html; charset=utf-8"
		extension = ".hocr"
	}

	return &OCRResult{
		PlainText:   plainText,
		FileContent: fileBytes,
		ContentType: contentType,
		Extension:   extension,
	}, nil
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
