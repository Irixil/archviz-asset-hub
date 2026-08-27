package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/jobspec"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"go.opentelemetry.io/otel/attribute"
)

// videoCaptureBuild is the variantBuildFn for video frame capture.
func (s *JobServer) videoCaptureBuild(
	_ context.Context, _, _, _ string, params json.RawMessage,
) (variantTransformer, error) {
	return s.videoCaptureTransformer(params)
}

// videoCaptureCanonical returns canonical JSON for video capture params.
func videoCaptureCanonical(_, _ string, params json.RawMessage) (string, error) {
	var p transform.VideoThumbnailParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	b, err := json.Marshal(p)
	return string(b), err
}

// videoCapturePostHook updates the asset thumbnail if none exists yet.
func (s *JobServer) videoCapturePostHook(ctx context.Context, p jobspec.VariantJobPayload, _, storageKey, _ string) {
	asset, err := s.queries.GetAssetByID(ctx, dbgen.GetAssetByIDParams{ID: p.AssetID, WorkspaceID: p.WorkspaceID})
	if err != nil {
		// Asset gone or not in this workspace — never write a bare-ID update.
		return
	}
	if asset.ThumbnailKey != nil {
		return
	}
	if updErr := s.queries.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: &storageKey,
		ID:           p.AssetID,
	}); updErr == nil {
		s.hub.Publish(ctx, p.WorkspaceID, events.ThumbnailReady(p.AssetID, storageKey))
	}
}

// videoCaptureTransformer returns a variantTransformer for video frame capture.
func (s *JobServer) videoCaptureTransformer(params json.RawMessage) (variantTransformer, error) {
	if !s.trf.FFmpegAvailable() {
		return nil, errors.New("ffmpeg not found in PATH")
	}
	var p transform.VideoThumbnailParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "video_capture_image"),
		)
		rc, err := s.storage.Get(ctx, sourceKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		defer rc.Close()
		tmpPath, cleanup, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("write temp: %w", err)
		}
		defer cleanup()
		data, err := s.trf.VideoExtractThumbnail(ctx, tmpPath, p)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("extract thumbnail: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return data, transform.MimeImageJPEG, nil
	}, nil
}

// videoTranscodeBuild is the variantBuildFn for video transcoding.
func (s *JobServer) videoTranscodeBuild(
	_ context.Context, _, _, _ string, params json.RawMessage,
) (variantTransformer, error) {
	return s.videoTranscodeTransformer(params)
}

// videoTranscodeCanonical returns canonical JSON for transcode params.
func videoTranscodeCanonical(_, _ string, params json.RawMessage) (string, error) {
	var p transform.TranscodeParams
	_ = json.Unmarshal(params, &p)
	if p.Format == "" {
		p.Format = transform.FormatMP4
	}
	b, err := json.Marshal(p)
	return string(b), err
}

// videoTranscodeTransformer returns a variantTransformer for video transcoding.
func (s *JobServer) videoTranscodeTransformer(params json.RawMessage) (variantTransformer, error) {
	if !s.trf.FFmpegAvailable() {
		return nil, errors.New("ffmpeg not found in PATH")
	}
	var p transform.TranscodeParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("parse transcode params: %w", err)
		}
	}
	if p.Format == "" {
		p.Format = transform.FormatMP4
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "video_transcode"),
		)
		rc, err := s.storage.Get(ctx, sourceKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		defer rc.Close()
		srcPath, cleanSrc, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("write src temp: %w", err)
		}
		defer cleanSrc()
		ext := transform.FormatExtension(p.Format)
		dstPath := srcPath + "_out" + ext
		defer os.Remove(dstPath)
		if e := s.trf.VideoTranscode(ctx, srcPath, dstPath, p); e != nil {
			telemetry.EndSpan(span, e)
			return nil, "", fmt.Errorf("transcode: %w", e)
		}
		data, err := os.ReadFile(dstPath)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("read output: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return data, transform.FormatVideoMimeType(p.Format), nil
	}, nil
}

// videoWatermarkBuild is the variantBuildFn for video watermarking.
func (s *JobServer) videoWatermarkBuild(
	ctx context.Context, _, _, workspaceID string, params json.RawMessage,
) (variantTransformer, error) {
	return s.videoWatermarkTransformer(ctx, workspaceID, params)
}

// videoWatermarkCanonical returns canonical JSON for video watermark params.
func videoWatermarkCanonical(_, _ string, params json.RawMessage) (string, error) {
	var p transform.VideoWatermarkParams
	_ = json.Unmarshal(params, &p)
	p.Normalize()
	b, err := json.Marshal(p)
	return string(b), err
}

// videoWatermarkTransformer returns a variantTransformer for video watermarking.
func (s *JobServer) videoWatermarkTransformer(
	ctx context.Context, workspaceID string, params json.RawMessage,
) (variantTransformer, error) {
	if !s.trf.FFmpegAvailable() {
		return nil, errors.New("ffmpeg not found in PATH")
	}
	var p transform.VideoWatermarkParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("parse watermark params: %w", err)
		}
	}
	p.Normalize()
	if p.WatermarkAssetID == "" {
		return nil, errors.New("watermark asset id is required")
	}
	wm, err := s.queries.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
		ID:          p.WatermarkAssetID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("get watermark asset: %w", err)
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "video_watermark"),
		)
		rc, rcErr := s.storage.Get(ctx, sourceKey)
		if rcErr != nil {
			telemetry.EndSpan(span, rcErr)
			return nil, "", rcErr
		}
		defer rc.Close()
		srcPath, cleanSrc, srcErr := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if srcErr != nil {
			telemetry.EndSpan(span, srcErr)
			return nil, "", fmt.Errorf("write src temp: %w", srcErr)
		}
		defer cleanSrc()
		wmRC, wmRCErr := s.storage.Get(ctx, wm.StorageKey)
		if wmRCErr != nil {
			telemetry.EndSpan(span, wmRCErr)
			return nil, "", fmt.Errorf("get watermark file: %w", wmRCErr)
		}
		defer wmRC.Close()
		ext := transform.FormatExtension(p.Format)
		dstPath := srcPath + "_wm" + ext
		defer os.Remove(dstPath)
		if wmErr := s.trf.VideoWatermark(ctx, srcPath, dstPath, wmRC, p); wmErr != nil {
			telemetry.EndSpan(span, wmErr)
			return nil, "", fmt.Errorf("watermark: %w", wmErr)
		}
		data, readErr := os.ReadFile(dstPath)
		if readErr != nil {
			telemetry.EndSpan(span, readErr)
			return nil, "", fmt.Errorf("read output: %w", readErr)
		}
		telemetry.EndSpan(span, nil)
		return data, transform.FormatVideoMimeType(p.Format), nil
	}, nil
}

// assetVersionFromPayload constructs a minimal AssetVersion from a jobspec.VariantJobPayload,
// so user-triggered video/audio jobs can reuse finalizeRebuildVariant.
func assetVersionFromPayload(p jobspec.VariantJobPayload) dbgen.AssetVersion {
	return dbgen.AssetVersion{
		ID:          p.VersionID,
		WorkspaceID: p.WorkspaceID,
		AssetID:     p.AssetID,
		VersionNum:  p.VersionNum,
		StorageKey:  p.StorageKey,
		MimeType:    p.MimeType,
	}
}
