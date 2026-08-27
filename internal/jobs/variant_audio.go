package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"damask/server/internal/queue"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"go.opentelemetry.io/otel/attribute"
)

// audioBuild is the variantBuildFn for all audio variant types.
func (s *JobServer) audioBuild(
	_ context.Context, jobType, sourceMime, _ string, params json.RawMessage,
) (variantTransformer, error) {
	return s.audioTransformer(jobType, sourceMime, params)
}

// audioCanonical returns canonical JSON for audio params (post-normalization).
func audioCanonical(jobType, sourceMime string, params json.RawMessage) (string, error) {
	var p transform.AudioParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	p = normalizeAudioJobParams(jobType, sourceMime, p)
	b, err := json.Marshal(p)
	return string(b), err
}

// audioTransformer returns a variantTransformer for the given audio job type.
func (s *JobServer) audioTransformer(jobType, sourceMime string, params json.RawMessage) (variantTransformer, error) {
	if !s.trf.FFmpegAvailable() {
		return nil, errors.New("ffmpeg not found in PATH")
	}
	var p transform.AudioParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("parse audio params: %w", err)
		}
	}
	p = normalizeAudioJobParams(jobType, sourceMime, p)
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", jobType),
		)

		rc, err := s.storage.Get(ctx, sourceKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		defer rc.Close()

		srcExt := filepath.Ext(sourceKey)
		srcPath, cleanSrc, err := writeToTempFile(ctx, rc, srcExt)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("write src temp: %w", err)
		}
		defer cleanSrc()

		ext := transform.AudioExtension(p.OutputFormat)
		dstPath := srcPath + "_out" + ext
		defer os.Remove(dstPath)

		if _, err = s.runAudioVariant(ctx, jobType, srcPath, dstPath, p); err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		data, err := os.ReadFile(dstPath)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("read output: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return data, transform.AudioMimeType(p.OutputFormat), nil
	}, nil
}

// runAudioVariant dispatches the ffmpeg audio operation for variantType.
func (s *JobServer) runAudioVariant(
	ctx context.Context,
	variantType, srcPath, dstPath string,
	params transform.AudioParams,
) (string, error) {
	var err error
	switch variantType {
	case queue.JobTypeExtractAudio:
		err = s.trf.ExtractAudio(ctx, srcPath, dstPath, params)
	case queue.JobTypeTranscodeAudio:
		err = s.trf.TranscodeAudio(ctx, srcPath, dstPath, params)
	case queue.JobTypeNormalizeAudio:
		err = s.trf.NormalizeAudio(ctx, srcPath, dstPath, params)
	default:
		return "", fmt.Errorf("unknown audio variant type: %s", variantType)
	}
	if errors.Is(err, transform.ErrNoAudioStream) {
		return "", fmt.Errorf("extract audio: %w", err)
	}
	if err != nil {
		return "", fmt.Errorf("audio transform: %w", err)
	}
	b, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshal audio params: %w", err)
	}
	return string(b), nil
}

func normalizeAudioJobParams(jobType, sourceMime string, p transform.AudioParams) transform.AudioParams {
	if p.OutputFormat == "" || p.OutputFormat == "source" {
		switch jobType {
		case queue.JobTypeExtractAudio:
			p.OutputFormat = "aac"
		case queue.JobTypeTranscodeAudio:
			p.OutputFormat = "mp3"
		case queue.JobTypeNormalizeAudio:
			p.OutputFormat = transform.AudioFormatFromMimeType(sourceMime)
		}
	}
	if p.Bitrate == "" {
		p.Bitrate = "192k"
	}
	if p.TargetLUFS == 0 {
		p.TargetLUFS = transform.DefaultLUFS
	}
	return p
}
