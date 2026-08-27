package transform

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PDFExtractResolution uses ImageMagick identify to read the dimensions of the first PDF page.
func (t *transformer) PDFExtractResolution(ctx context.Context, srcPath string) (*VideoResolution, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// identify -format "%wx%h" "file.pdf[0]"
	out, err := exec.CommandContext(ctx, //nolint:gosec // not user input
		"identify", "-format", "%wx%h", srcPath+"[0]").Output()
	if err != nil {
		return nil, fmt.Errorf("identify pdf resolution: %w", err)
	}

	parts := strings.SplitN(strings.TrimSpace(string(out)), "x", 2)
	if len(parts) != 2 { //nolint:mnd // expect a format like "612x792".
		return nil, fmt.Errorf("identify pdf resolution: unexpected output: %s", string(out))
	}
	width, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("identify pdf resolution: invalid width: %w", err)
	}
	height, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("identify pdf resolution: invalid height: %w", err)
	}
	return &VideoResolution{Width: width, Height: height}, nil
}

// ExtractPDFText extracts plain text from a PDF using pdftotext (poppler-utils).
func ExtractPDFText(ctx context.Context, pdfBytes []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "damask-pdf-*.pdf")
	if err != nil {
		return "", fmt.Errorf("pdftotext: create temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err = tmpFile.Write(pdfBytes); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("pdftotext: write temp: %w", err)
	}
	_ = tmpFile.Close()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// pdftotext <file> - writes plain text to stdout
	out, err := exec.CommandContext(ctx, "pdftotext", tmpPath, "-").Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ExtractDocumentText converts an office document to plain text using LibreOffice.
func ExtractDocumentText(ctx context.Context, docBytes []byte, mimeType string) (string, error) {
	ext := MimeToExt(mimeType)
	tmpFile, err := os.CreateTemp("", "damask-doc-*"+ext)
	if err != nil {
		return "", fmt.Errorf("soffice txt: create temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err = tmpFile.Write(docBytes); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("soffice txt: write temp: %w", err)
	}
	_ = tmpFile.Close()

	outDir, err := os.MkdirTemp("", "damask-doc-txt-*")
	if err != nil {
		return "", fmt.Errorf("soffice txt: create outdir: %w", err)
	}
	defer os.RemoveAll(outDir)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "soffice", //nolint:gosec // not user input
		"-env:UserInstallation=file://"+outDir+"/.lo",
		"--headless", "--convert-to", "txt:Text", "--outdir", outDir, tmpPath).
		CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("soffice txt: %w: %s", err, strings.TrimSpace(string(out)))
	}

	txtPath, err := findExt(outDir, ".txt")
	if err != nil {
		return "", fmt.Errorf("soffice txt: %w", err)
	}
	content, err := os.ReadFile(txtPath)
	if err != nil {
		return "", fmt.Errorf("soffice txt: read output: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}

func findExt(dir, ext string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ext {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("soffice: no %s output produced", ext)
}

// DocumentThumbnail converts the first page of an office document to PNG using LibreOffice.
func (t *transformer) DocumentThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, error) {
	ext := MimeToExt(mimeType)
	tmpPath, cleanup, err := writeToTempFile(ctx, src, ext)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	outDir, err := os.MkdirTemp("", "damask-doc-thumb-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(outDir)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// soffice cannot reliably convert office documents (DOCX, XLSX, PPTX, HTML, …) directly
	// to PNG. Convert to PDF first, then render the first page as PNG.
	out, err := exec.CommandContext(ctx, "soffice", //nolint:gosec // not user input
		"-env:UserInstallation=file://"+outDir+"/.lo",
		"--headless", "--convert-to", "pdf", "--outdir", outDir, tmpPath).
		CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("soffice doc→pdf: %w: %s", err, strings.TrimSpace(string(out)))
	}
	pdfPath, err := findExt(outDir, ".pdf")
	if err != nil {
		return nil, fmt.Errorf("soffice doc→pdf: %w", err)
	}

	out, err = exec.CommandContext(ctx, "soffice", //nolint:gosec // not user input
		"-env:UserInstallation=file://"+outDir+"/.lo",
		"--headless", "--convert-to", "png", "--outdir", outDir, pdfPath).
		CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("soffice: %w: %s", err, strings.TrimSpace(string(out)))
	}

	pngPath, err := findExt(outDir, ".png")
	if err != nil {
		return nil, err
	}
	return os.ReadFile(pngPath)
}
