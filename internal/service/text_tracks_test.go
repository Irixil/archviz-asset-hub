package service_test

import (
	"errors"
	"testing"

	"damask/server/internal/queue"
	"damask/server/internal/service"
)

func TestExtractJobType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		source  string
		want    string
		wantErr error
	}{
		{"extract_pdf", queue.JobTypeExtractPDFTextTrack, nil},
		{"extract_plain", queue.JobTypeExtractPlainTextTrack, nil},
		{"extract_document", queue.JobTypeExtractDocumentTextTrack, nil},
		{"unknown", "", service.ErrUnsupportedTextTrackSource},
		{"", "", service.ErrUnsupportedTextTrackSource},
	}
	for _, c := range cases {
		got, err := service.ExtractJobType(c.source)
		if c.wantErr != nil {
			if !errors.Is(err, c.wantErr) {
				t.Errorf("ExtractJobType(%q): expected %v, got %v", c.source, c.wantErr, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("ExtractJobType(%q): unexpected error: %v", c.source, err)
		}
		if got != c.want {
			t.Errorf("ExtractJobType(%q) = %q, want %q", c.source, got, c.want)
		}
	}
}

func TestReadyTextContent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"  ", " "},
		{"", " "},
		{"hello", "hello"},
		{"  hi  ", "hi"},
	}
	for _, c := range cases {
		got := service.ReadyTextContent(c.in)
		if got != c.want {
			t.Errorf("ReadyTextContent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStringValue(t *testing.T) {
	t.Parallel()

	ptr := func(s string) *string { return &s }

	if service.StringValue(ptr("hello")) != "hello" {
		t.Error("non-empty ptr should return its value")
	}
	if service.StringValue(nil, "default") != "default" {
		t.Error("nil ptr should return fallback")
	}
	if service.StringValue(nil) != "" {
		t.Error("nil ptr with no fallback should return empty string")
	}
	emptyStr := ""
	if service.StringValue(&emptyStr, "fb") != "fb" {
		t.Error("empty ptr should return fallback")
	}
}

func TestStringParam(t *testing.T) {
	t.Parallel()

	params := map[string]any{"key": "value", "blank": "  "}

	if service.StringParam(params, "key", "fb") != "value" {
		t.Error("present key should return its value")
	}
	if service.StringParam(params, "missing", "fb") != "fb" {
		t.Error("missing key should return fallback")
	}
	if service.StringParam(nil, "key", "fb") != "fb" {
		t.Error("nil map should return fallback")
	}
	if service.StringParam(params, "blank", "fb") != "fb" {
		t.Error("whitespace-only value should return fallback")
	}
}
