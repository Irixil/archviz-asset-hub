package ai_test

import (
	"testing"

	"damask/server/internal/ai"
)

func TestTranscriptionFormatFromMimeType(t *testing.T) {
	cases := []struct {
		mimeType   string
		wantFormat string
		wantOK     bool
	}{
		{"audio/mpeg", "mp3", true},
		{"audio/mp3", "mp3", true},
		{"audio/wav", "wav", true},
		{"audio/x-wav", "wav", true},
		{"audio/wave", "wav", true},
		{"audio/flac", "flac", true},
		{"audio/x-flac", "flac", true},
		{"audio/mp4", "m4a", true},
		{"audio/x-m4a", "m4a", true},
		{"audio/aac", "aac", true},
		{"audio/ogg", "ogg", true},
		{"audio/opus", "ogg", true},
		{"audio/webm", "webm", true},
		{"audio/x-totally-unknown", "", false},
		{"", "", false},
	}

	for _, c := range cases {
		gotFormat, gotOK := ai.TranscriptionFormatFromMimeType(c.mimeType)
		if gotFormat != c.wantFormat || gotOK != c.wantOK {
			t.Errorf("TranscriptionFormatFromMimeType(%q) = (%q, %v), want (%q, %v)",
				c.mimeType, gotFormat, gotOK, c.wantFormat, c.wantOK)
		}
	}
}
