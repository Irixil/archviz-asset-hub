// Package contentmeta extracts metadata from media content.
package contentmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type AVTags struct {
	Title       *string
	Artist      *string
	AlbumArtist *string
	Album       *string
	Date        *string
	Year        *int
	TrackNumber *int
	TrackTotal  *int
	DiscNumber  *int
	DiscTotal   *int
	Genre       *string
	Composer    *string
	Lyricist    *string
	Comment     *string
	Lyrics      *string
	BPM         *float64
	Compilation *bool
	Copyright   *string
	Encoder     *string
	EncodedBy   *string
	Language    *string

	Container      *string
	DurationSec    *float64
	OverallBitrate *int

	AudioCodec    *string
	AudioBitrate  *int
	SampleRate    *int
	Channels      *int
	ChannelLayout *string
	BitsPerSample *int

	VideoCodec  *string
	VideoWidth  *int
	VideoHeight *int
	FrameRate   *string

	HasCoverArt bool
}

type ffprobeOut struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecName     string     `json:"codec_name"`
	CodecType     string     `json:"codec_type"`
	SampleRate    string     `json:"sample_rate"`
	Channels      int        `json:"channels"`
	ChannelLayout string     `json:"channel_layout"`
	BitsPerSample ffprobeInt `json:"bits_per_raw_sample"`
	BitRate       string     `json:"bit_rate"`
	Width         int        `json:"width"`
	Height        int        `json:"height"`
	RFrameRate    string     `json:"r_frame_rate"`
	Disposition   struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
	Tags map[string]string `json:"tags"`
}

type ffprobeFormat struct {
	FormatName string            `json:"format_name"`
	Duration   string            `json:"duration"`
	BitRate    string            `json:"bit_rate"`
	Tags       map[string]string `json:"tags"`
}

type ffprobeInt int

func (v *ffprobeInt) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*v = 0
		return nil
	}

	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*v = ffprobeInt(n)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		*v = 0
		return nil
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*v = ffprobeInt(n)
	return nil
}

func ExtractAVTags(ctx context.Context, ffprobePath, filePath string) (*AVTags, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx,
		ffprobePath, "-v", "quiet",
		"-print_format", "json",
		"-show_format", "-show_streams",
		filePath,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("mediameta: ffprobe: %w", err)
	}

	var probe ffprobeOut
	if err = json.Unmarshal(out, &probe); err != nil {
		return nil, fmt.Errorf("mediameta: parse: %w", err)
	}

	r := &AVTags{}
	if probe.Format.FormatName != "" {
		r.Container = ptr(probe.Format.FormatName)
	}
	if d, parseErr := strconv.ParseFloat(probe.Format.Duration, 64); parseErr == nil && d > 0 {
		r.DurationSec = ptr(d)
	}
	if br, parseErr := strconv.Atoi(probe.Format.BitRate); parseErr == nil {
		r.OverallBitrate = ptr(br)
	}

	r.applyTags(probe.Format.Tags)
	for _, s := range probe.Streams {
		if s.CodecType == "audio" {
			r.applyTags(s.Tags)
			break
		}
	}

	for _, s := range probe.Streams {
		applyStreamMeta(r, s)
	}

	if r.isEmpty() {
		return nil, nil //nolint:nilnil // empty result is not an error
	}
	return r, nil
}

func applyStreamMeta(r *AVTags, s ffprobeStream) {
	isCoverArt := s.Disposition.AttachedPic == 1 ||
		(s.CodecType == "video" && (s.CodecName == "mjpeg" || s.CodecName == "png"))
	switch {
	case s.CodecType == "audio" && r.AudioCodec == nil:
		applyAudioStreamMeta(r, s)
	case s.CodecType == "video" && !isCoverArt && r.VideoCodec == nil:
		applyVideoStreamMeta(r, s)
	case isCoverArt:
		r.HasCoverArt = true
	}
}

func applyAudioStreamMeta(r *AVTags, s ffprobeStream) {
	r.AudioCodec = ptr(s.CodecName)
	if br, err := strconv.Atoi(s.BitRate); err == nil {
		r.AudioBitrate = ptr(br)
	}
	if sr, err := strconv.Atoi(s.SampleRate); err == nil && sr > 0 {
		r.SampleRate = ptr(sr)
	}
	if s.Channels > 0 {
		r.Channels = ptr(s.Channels)
	}
	if s.ChannelLayout != "" {
		r.ChannelLayout = ptr(s.ChannelLayout)
	}
	if s.BitsPerSample > 0 {
		r.BitsPerSample = ptr(int(s.BitsPerSample))
	}
}

func applyVideoStreamMeta(r *AVTags, s ffprobeStream) {
	r.VideoCodec = ptr(s.CodecName)
	if s.Width > 0 {
		r.VideoWidth = ptr(s.Width)
	}
	if s.Height > 0 {
		r.VideoHeight = ptr(s.Height)
	}
	if s.RFrameRate != "" && s.RFrameRate != "0/0" {
		r.FrameRate = ptr(s.RFrameRate)
	}
}

func tagSetText(tags map[string]string, dst **string, keys ...string) {
	if *dst != nil {
		return
	}
	for _, k := range keys {
		if v := strings.TrimSpace(tags[k]); v != "" {
			*dst = ptr(v)
			return
		}
	}
}

func tagSetInt(tags map[string]string, dst **int, keys ...string) {
	if *dst != nil {
		return
	}
	for _, k := range keys {
		raw := strings.TrimSpace(tags[k])
		if i := strings.IndexByte(raw, '/'); i >= 0 {
			raw = raw[:i]
		}
		if v, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			*dst = ptr(v)
			return
		}
	}
}

func tagSetTotal(tags map[string]string, dst **int, keys ...string) {
	if *dst != nil {
		return
	}
	for _, k := range keys {
		raw := strings.TrimSpace(tags[k])
		if _, after, ok := strings.Cut(raw, "/"); ok {
			if v, err := strconv.Atoi(strings.TrimSpace(after)); err == nil {
				*dst = ptr(v)
				return
			}
		}
	}
}

func (r *AVTags) applyTags(tags map[string]string) {
	tagSetText(tags, &r.Title, "title", "TITLE", "TIT2")
	tagSetText(tags, &r.Artist, "artist", "ARTIST", "TPE1")
	tagSetText(tags, &r.AlbumArtist, "album_artist", "ALBUMARTIST", "album artist", "TPE2")
	tagSetText(tags, &r.Album, "album", "ALBUM", "TALB")
	tagSetText(tags, &r.Date, "date", "DATE", "TDRC", "year", "TYER")
	tagSetText(tags, &r.Genre, "genre", "GENRE", "TCON")
	tagSetText(tags, &r.Composer, "composer", "COMPOSER", "TCOM")
	tagSetText(tags, &r.Lyricist, "lyricist", "LYRICIST", "TEXT")
	tagSetText(tags, &r.Comment, "comment", "COMMENT", "COMM")
	tagSetText(tags, &r.Lyrics, "lyrics", "LYRICS", "USLT", "unsyncedlyrics")
	tagSetText(tags, &r.Copyright, "copyright", "COPYRIGHT", "TCOP")
	tagSetText(tags, &r.Encoder, "encoder", "ENCODER", "TSSE")
	tagSetText(tags, &r.EncodedBy, "encoded_by", "ENCODED_BY", "TENC")
	tagSetText(tags, &r.Language, "language", "LANGUAGE", "TLAN")

	tagSetInt(tags, &r.TrackNumber, "track", "TRACK", "TRCK", "tracknumber")
	tagSetTotal(tags, &r.TrackTotal, "track", "TRACK", "TRCK", "tracktotal")
	tagSetInt(tags, &r.DiscNumber, "disc", "DISC", "TPOS", "discnumber")
	tagSetTotal(tags, &r.DiscTotal, "disc", "DISC", "TPOS", "disctotal")

	if r.Year == nil && r.Date != nil && len(*r.Date) >= 4 {
		if y, err := strconv.Atoi((*r.Date)[:4]); err == nil && y > 1000 {
			r.Year = ptr(y)
		}
	}
	if r.BPM == nil {
		for _, k := range []string{"bpm", "BPM", "TBPM", "BEATSPERMINUTE"} {
			if v, err := strconv.ParseFloat(strings.TrimSpace(tags[k]), 64); err == nil {
				r.BPM = ptr(v)
				break
			}
		}
	}
	if r.Compilation == nil {
		for _, k := range []string{"compilation", "COMPILATION", "TCMP"} {
			raw := strings.TrimSpace(tags[k])
			switch strings.ToLower(raw) {
			case "1", "true", "yes":
				r.Compilation = ptr(true)
				return
			case "0", "false", "no":
				r.Compilation = ptr(false)
				return
			}
		}
	}
}
