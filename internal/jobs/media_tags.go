package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"go.opentelemetry.io/otel/attribute"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/jobspec"
	"damask/server/internal/media/contentmeta"
	"damask/server/internal/queue"
	"damask/server/internal/telemetry"
)

const (
	customFieldTypeText   = "text"
	customFieldTypeNumber = "number"
	mediaTagsSource       = "media_tags"
)

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type mediaTagFieldDef struct {
	key       string
	name      string
	fieldType string
	position  int64
}

var mediaTagFields = []mediaTagFieldDef{
	{"_media_title", "Title", customFieldTypeText, 0},
	{"_media_artist", "Artist", customFieldTypeText, 1},
	{"_media_album", "Album", customFieldTypeText, 2},
	{"_media_album_artist", "Album artist", customFieldTypeText, 3},
	{"_media_date", "Date", customFieldTypeText, 4},
	{"_media_year", "Year", customFieldTypeNumber, 5},
	{"_media_track_number", "Track number", customFieldTypeNumber, 6},
	{"_media_track_total", "Track total", customFieldTypeNumber, 7},
	{"_media_disc_number", "Disc number", customFieldTypeNumber, 8},
	{"_media_disc_total", "Disc total", customFieldTypeNumber, 9},
	{"_media_genre", "Genre", customFieldTypeText, 10},
	{"_media_composer", "Composer", customFieldTypeText, 11},
	{"_media_lyricist", "Lyricist", customFieldTypeText, 12},
	{"_media_comment", "Comment", customFieldTypeText, 13},
	{"_media_lyrics", "Lyrics", customFieldTypeText, 14},
	{"_media_bpm", "BPM", customFieldTypeNumber, 15},
	{"_media_compilation", "Compilation", "boolean", 16},
	{"_media_copyright", "Copyright", customFieldTypeText, 17},
	{"_media_encoder", "Encoder", customFieldTypeText, 18},
	{"_media_encoded_by", "Encoded by", customFieldTypeText, 19},
	{"_media_language", "Language", customFieldTypeText, 20},
	{"_media_container", "Container", customFieldTypeText, 21},
	{"_media_duration_sec", "Duration", customFieldTypeNumber, 22},
	{"_media_overall_bitrate", "Overall bitrate", customFieldTypeNumber, 23},
	{"_media_audio_codec", "Audio codec", customFieldTypeText, 24},
	{"_media_audio_bitrate", "Audio bitrate", customFieldTypeNumber, 25},
	{"_media_sample_rate", "Sample rate", customFieldTypeNumber, 26},
	{"_media_channels", "Channels", customFieldTypeNumber, 27},
	{"_media_channel_layout", "Channel layout", customFieldTypeText, 28},
	{"_media_bits_per_sample", "Bit depth", customFieldTypeNumber, 29},
	{"_media_video_codec", "Video codec", customFieldTypeText, 30},
	{"_media_video_width", "Video width", customFieldTypeNumber, 31},
	{"_media_video_height", "Video height", customFieldTypeNumber, 32},
	{"_media_frame_rate", "Frame rate", customFieldTypeText, 33},
	{"_media_has_cover_art", "Cover art", "boolean", 34},
}

var mediaTagExtract = contentmeta.ExtractAVTags

// EnqueueExtractMediaTagsJob enqueues an extract_media_tags job for an audio or video asset.
func EnqueueExtractMediaTagsJob(ctx context.Context, q queue.JobQueue, workspaceID, assetID string) error {
	payload, _ := json.Marshal(jobspec.ExtractMediaTagsPayload{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
	})
	_, err := q.Enqueue(ctx, workspaceID, queue.JobTypeExtractMediaTags, string(payload))
	return err
}

func (s *JobServer) jobExtractMediaTags(ctx context.Context, job dbgen.Job) (err error) {
	var p jobspec.ExtractMediaTagsPayload
	if err = json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}
	ctx, span := telemetry.StartBackgroundSpan(ctx, "jobs.media_tags.extract",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.String("damask.asset_id", p.AssetID),
		attribute.String("damask.job.type", job.Type),
	)
	defer func() {
		telemetry.EndSpan(span, err)
	}()

	asset, err := s.queries.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
		ID:          p.AssetID,
		WorkspaceID: p.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load asset: %w", err)
	}
	if !strings.HasPrefix(asset.MimeType, "audio/") && !strings.HasPrefix(asset.MimeType, "video/") {
		return nil
	}

	freshCtx, freshSpan := telemetry.StartSpan(ctx, "jobs.media_tags.freshness_check")
	fresh, err := s.mediaTagsFresh(freshCtx, asset)
	telemetry.EndSpan(freshSpan, err)
	if err != nil {
		return fmt.Errorf("check freshness: %w", err)
	}
	if fresh {
		span.SetAttributes(attribute.Bool("damask.media_tags.skipped_fresh", true))
		return nil
	}

	fieldCtx, fieldSpan := telemetry.StartSpan(ctx, "jobs.media_tags.seed_fields")
	fieldIDs, err := s.ensureMediaTagFields(fieldCtx, p.WorkspaceID)
	telemetry.EndSpan(fieldSpan, err)
	if err != nil {
		return fmt.Errorf("ensure media tag fields: %w", err)
	}

	fileCtx, fileSpan := telemetry.StartSpan(ctx, "jobs.media_tags.resolve_file")
	filePath, cleanup, err := s.mediaTagFilePath(fileCtx, asset.StorageKey)
	telemetry.EndSpan(fileSpan, err)
	if err != nil {
		return fmt.Errorf("resolve asset file: %w", err)
	}
	defer cleanup()

	extractCtx, extractSpan := telemetry.StartSpan(ctx, "jobs.media_tags.ffprobe")
	extractSpan.SetAttributes(attribute.String("damask.media_tags.file_path", filePath))
	result, err := mediaTagExtract(extractCtx, s.trf.FFprobePath(), filePath)
	telemetry.EndSpan(extractSpan, err)
	if err != nil {
		return fmt.Errorf("extract media tags: %w", err)
	}
	if result == nil {
		span.SetAttributes(attribute.Bool("damask.media_tags.empty", true))
		return nil
	}
	writeCtx, writeSpan := telemetry.StartSpan(ctx, "jobs.media_tags.upsert_values")
	written, err := s.writeMediaTagValues(writeCtx, asset.ID, fieldIDs, result)
	writeSpan.SetAttributes(attribute.Int("damask.media_tags.values_written", written))
	telemetry.EndSpan(writeSpan, err)
	if err != nil {
		return fmt.Errorf("write media tags: %w", err)
	}
	span.SetAttributes(attribute.Int("damask.media_tags.values_written", written))

	slog.DebugContext(ctx, "media tags extracted",
		"asset_id", asset.ID,
		"artist", ptrStr(result.Artist),
		"codec", firstNonEmptyPtr(result.AudioCodec, result.VideoCodec),
	)
	return nil
}

func (s *JobServer) ensureMediaTagFields(ctx context.Context, workspaceID string) (map[string]string, error) {
	fields, err := s.queries.GetSystemFieldsBySource(ctx, dbgen.GetSystemFieldsBySourceParams{
		WorkspaceID: workspaceID,
		Source:      mediaTagsSource,
	})
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		tx, txErr := s.sqlDB.BeginTx(ctx, nil)
		if txErr != nil {
			return nil, txErr
		}
		defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after read-only queries or commit.
		qtx := s.queries.WithTx(tx)
		for _, fd := range mediaTagFields {
			if e := qtx.InsertSystemFieldDefinition(ctx, dbgen.InsertSystemFieldDefinitionParams{
				ID:          uuid.NewString(),
				WorkspaceID: workspaceID,
				Source:      mediaTagsSource,
				Name:        fd.name,
				Key:         fd.key,
				FieldType:   fd.fieldType,
				Position:    fd.position,
			}); e != nil {
				return nil, e
			}
		}
		if e := tx.Commit(); e != nil {
			return nil, e
		}
		fields, err = s.queries.GetSystemFieldsBySource(ctx, dbgen.GetSystemFieldsBySourceParams{
			WorkspaceID: workspaceID,
			Source:      mediaTagsSource,
		})
		if err != nil {
			return nil, err
		}
	}

	fieldIDs := make(map[string]string, len(fields))
	for _, field := range fields {
		fieldIDs[field.Key] = field.ID
	}
	return fieldIDs, nil
}

func (s *JobServer) mediaTagsFresh(ctx context.Context, asset dbgen.Asset) (bool, error) {
	const q = `
SELECT MAX(afv.updated_at)
FROM asset_field_values afv
JOIN field_definitions fd ON fd.id = afv.field_id
WHERE afv.asset_id = ? AND fd.workspace_id = ? AND fd.source = ?`

	var latest sql.NullString
	if err := s.sqlDB.QueryRowContext(ctx, q, asset.ID, asset.WorkspaceID, mediaTagsSource).Scan(&latest); err != nil {
		return false, err
	}
	if !latest.Valid {
		return false, nil
	}
	fieldTime, err := ParseSQLiteTime(latest.String)
	if err != nil {
		return false, err
	}
	return !fieldTime.Before(asset.UpdatedAt), nil
}

func (s *JobServer) mediaTagFilePath(ctx context.Context, storageKey string) (string, func(), error) {
	type localPathStorage interface {
		LocalPath(key string) string
	}
	if ls, ok := s.storage.(localPathStorage); ok {
		return ls.LocalPath(storageKey), func() {}, nil
	}
	r, err := s.storage.Get(ctx, storageKey)
	if err != nil {
		return "", nil, err
	}
	defer r.Close()
	return writeToTempFile(ctx, r, ".bin")
}

func (s *JobServer) writeMediaTagValues(
	ctx context.Context,
	assetID string,
	fieldIDs map[string]string,
	result *contentmeta.AVTags,
) (int, error) {
	written := 0
	upsert := func(key string, params dbgen.UpsertAssetFieldValueParams) error {
		fieldID, ok := fieldIDs[key]
		if !ok {
			return nil
		}
		params.ID = uuid.NewString()
		params.AssetID = assetID
		params.FieldID = fieldID
		_, err := s.queries.UpsertAssetFieldValue(ctx, params)
		if err == nil {
			written++
		}
		return err
	}
	writeText := func(key string, value *string) error {
		if value == nil {
			return nil
		}
		return upsert(key, dbgen.UpsertAssetFieldValueParams{ValueText: value})
	}
	writeNumber := func(key string, value *float64) error {
		if value == nil {
			return nil
		}
		return upsert(key, dbgen.UpsertAssetFieldValueParams{ValueNumber: value})
	}
	writeInt := func(key string, value *int) error {
		if value == nil {
			return nil
		}
		n := float64(*value)
		return writeNumber(key, &n)
	}

	texts := map[string]*string{
		"_media_title":          result.Title,
		"_media_artist":         result.Artist,
		"_media_album":          result.Album,
		"_media_album_artist":   result.AlbumArtist,
		"_media_date":           result.Date,
		"_media_genre":          result.Genre,
		"_media_composer":       result.Composer,
		"_media_lyricist":       result.Lyricist,
		"_media_comment":        result.Comment,
		"_media_lyrics":         result.Lyrics,
		"_media_copyright":      result.Copyright,
		"_media_encoder":        result.Encoder,
		"_media_encoded_by":     result.EncodedBy,
		"_media_language":       result.Language,
		"_media_container":      result.Container,
		"_media_audio_codec":    result.AudioCodec,
		"_media_channel_layout": result.ChannelLayout,
		"_media_video_codec":    result.VideoCodec,
		"_media_frame_rate":     result.FrameRate,
	}
	for key, value := range texts {
		if err := writeText(key, value); err != nil {
			return written, fmt.Errorf("upsert %s: %w", key, err)
		}
	}

	ints := map[string]*int{
		"_media_year":            result.Year,
		"_media_track_number":    result.TrackNumber,
		"_media_track_total":     result.TrackTotal,
		"_media_disc_number":     result.DiscNumber,
		"_media_disc_total":      result.DiscTotal,
		"_media_overall_bitrate": result.OverallBitrate,
		"_media_audio_bitrate":   result.AudioBitrate,
		"_media_sample_rate":     result.SampleRate,
		"_media_channels":        result.Channels,
		"_media_bits_per_sample": result.BitsPerSample,
		"_media_video_width":     result.VideoWidth,
		"_media_video_height":    result.VideoHeight,
	}
	for key, value := range ints {
		if err := writeInt(key, value); err != nil {
			return written, fmt.Errorf("upsert %s: %w", key, err)
		}
	}

	numbers := map[string]*float64{
		"_media_bpm":          result.BPM,
		"_media_duration_sec": result.DurationSec,
	}
	for key, value := range numbers {
		if err := writeNumber(key, value); err != nil {
			return written, fmt.Errorf("upsert %s: %w", key, err)
		}
	}

	if result.Compilation != nil {
		n := int64(0)
		if *result.Compilation {
			n = 1
		}
		if err := upsert("_media_compilation", dbgen.UpsertAssetFieldValueParams{ValueBoolean: &n}); err != nil {
			return written, fmt.Errorf("upsert _media_compilation: %w", err)
		}
	}
	if result.HasCoverArt {
		n := int64(1)
		if err := upsert("_media_has_cover_art", dbgen.UpsertAssetFieldValueParams{ValueBoolean: &n}); err != nil {
			return written, fmt.Errorf("upsert _media_has_cover_art: %w", err)
		}
	}
	return written, nil
}

func firstNonEmptyPtr(values ...*string) string {
	for _, v := range values {
		if v != nil && *v != "" {
			return *v
		}
	}
	return ""
}
