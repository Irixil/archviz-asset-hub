package transform

import (
	"context"
	"strings"
	"testing"
)

func TestTesseractAvailableDoesNotPanic(_ *testing.T) {
	_ = TesseractAvailable()
}

func TestRunOCRRejectsUnsupportedFormat(t *testing.T) {
	_, err := RunOCR(context.Background(), []byte("fake"), OCRParams{OutputFormat: "pdf"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunOCRSkipsWhenUnavailable(t *testing.T) {
	if TesseractAvailable() {
		t.Skip("tesseract present")
	}
	_, err := RunOCR(context.Background(), []byte("fake"), OCRParams{})
	if err == nil {
		t.Fatal("expected error when tesseract is unavailable")
	}
}

func TestStripHTMLTagsRemovesTags(t *testing.T) {
	got := stripHTMLTags(`<html><body><p>Hello <strong>world</strong></p></body></html>`)
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") {
		t.Fatalf("missing text: %q", got)
	}
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Fatalf("tags not removed: %q", got)
	}
}

func TestStripHTMLTagsCollapsesWhitespace(t *testing.T) {
	got := stripHTMLTags("<p>foo</p>\n\t  <p>bar</p>")
	if got != "foo bar" {
		t.Fatalf("got %q", got)
	}
}
