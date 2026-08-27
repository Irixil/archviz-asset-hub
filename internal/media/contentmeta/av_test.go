package contentmeta

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFakeFFprobe(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ffprobe")
	script := "#!/bin/sh\ncat <<'JSON'\n" + body + "\nJSON\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}
	return path
}

func TestExtractAVTags_MP3FullTags(t *testing.T) {
	t.Parallel()
	ffprobe := writeFakeFFprobe(t, `{
  "format": {
    "format_name": "mp3",
    "duration": "123.4",
    "bit_rate": "192000",
    "tags": {
      "title": "Song",
      "artist": "Artist",
      "album": "Album",
      "track": "3/12",
      "date": "2001-02-03",
      "lyrics": "hello world"
    }
  },
  "streams": [
    {
      "codec_name": "mp3",
      "codec_type": "audio",
      "sample_rate": "44100",
      "channels": 2,
      "channel_layout": "stereo",
      "bit_rate": "192000"
    },
    {
      "codec_name": "mjpeg",
      "codec_type": "video",
      "disposition": { "attached_pic": 1 }
    }
  ]
}`)

	got, err := ExtractAVTags(context.Background(), ffprobe, "ignored.mp3")
	if err != nil {
		t.Fatalf("ExtractAVTags() error = %v", err)
	}
	if got == nil || got.Title == nil || *got.Title != "Song" {
		t.Fatalf("expected title Song, got %+v", got)
	}
	if got.TrackNumber == nil || *got.TrackNumber != 3 || got.TrackTotal == nil || *got.TrackTotal != 12 {
		t.Fatalf("expected track 3/12, got %+v", got)
	}
	if got.Year == nil || *got.Year != 2001 {
		t.Fatalf("expected year 2001, got %+v", got.Year)
	}
	if !got.HasCoverArt {
		t.Fatal("expected cover art")
	}
}

func TestExtractAVTags_StreamLevelVorbisTags(t *testing.T) {
	t.Parallel()
	ffprobe := writeFakeFFprobe(t, `{
  "format": { "format_name": "ogg", "duration": "10.0", "bit_rate": "128000", "tags": {} },
  "streams": [
    {
      "codec_name": "vorbis",
      "codec_type": "audio",
      "sample_rate": "48000",
      "channels": 2,
      "tags": {
        "TITLE": "Vorbis Song",
        "ARTIST": "Vorbis Artist",
        "TRACK": "5/9"
      }
    }
  ]
}`)

	got, err := ExtractAVTags(context.Background(), ffprobe, "ignored.ogg")
	if err != nil {
		t.Fatalf("ExtractAVTags() error = %v", err)
	}
	if got == nil || got.Title == nil || *got.Title != "Vorbis Song" {
		t.Fatalf("expected stream-level title, got %+v", got)
	}
	if got.TrackNumber == nil || *got.TrackNumber != 5 || got.TrackTotal == nil || *got.TrackTotal != 9 {
		t.Fatalf("expected track 5/9, got %+v", got)
	}
}

func TestExtractAVTags_VideoStreamAndFrameRate(t *testing.T) {
	t.Parallel()
	ffprobe := writeFakeFFprobe(t, `{
  "format": {
    "format_name": "mov,mp4,m4a,3gp,3g2,mj2",
    "duration": "42.0",
    "bit_rate": "640000",
    "tags": { "title": "Clip" }
  },
  "streams": [
    {
      "codec_name": "h264",
      "codec_type": "video",
      "width": 1920,
      "height": 1080,
      "r_frame_rate": "30000/1001"
    }
  ]
}`)

	got, err := ExtractAVTags(context.Background(), ffprobe, "ignored.mp4")
	if err != nil {
		t.Fatalf("ExtractAVTags() error = %v", err)
	}
	if got == nil || got.VideoCodec == nil || *got.VideoCodec != "h264" {
		t.Fatalf("expected h264, got %+v", got)
	}
	if got.VideoWidth == nil || *got.VideoWidth != 1920 || got.VideoHeight == nil || *got.VideoHeight != 1080 {
		t.Fatalf("expected 1920x1080, got %+v", got)
	}
	if got.FrameRate == nil || *got.FrameRate != "30000/1001" {
		t.Fatalf("expected frame rate, got %+v", got.FrameRate)
	}
}

func TestExtractAVTags_NoUsefulTagsReturnsNil(t *testing.T) {
	t.Parallel()
	ffprobe := writeFakeFFprobe(t, `{
  "format": { "format_name": "", "duration": "", "bit_rate": "", "tags": {} },
  "streams": []
}`)

	got, err := ExtractAVTags(context.Background(), ffprobe, "ignored.wav")
	if err != nil {
		t.Fatalf("ExtractAVTags() error = %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil result, got %+v", got)
	}
}

func TestExtractAVTags_StringBitsPerRawSampleOnCoverArt(t *testing.T) {
	ffprobe := writeFakeFFprobe(t, `{
  "format": {
    "format_name": "mp3",
    "duration": "178.032000",
    "bit_rate": "238512",
    "tags": {
      "title": "Semispheres",
      "artist": "Siddhartha Barnhoorn"
    }
  },
  "streams": [
    {
      "codec_name": "mp3",
      "codec_type": "audio",
      "sample_rate": "48000",
      "channels": 2,
      "channel_layout": "stereo",
      "bit_rate": "235160"
    },
    {
      "codec_name": "mjpeg",
      "codec_type": "video",
      "width": 640,
      "height": 640,
      "bits_per_raw_sample": "8",
      "disposition": { "attached_pic": 1 }
    }
  ]
}`)

	got, err := ExtractAVTags(context.Background(), ffprobe, "ignored.mp3")
	if err != nil {
		t.Fatalf("ExtractAVTags() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected tags, got nil")
	}
	if got.Artist == nil || *got.Artist != "Siddhartha Barnhoorn" {
		t.Fatalf("expected artist tag, got %+v", got)
	}
	if !got.HasCoverArt {
		t.Fatalf("expected cover art, got %+v", got)
	}
}
