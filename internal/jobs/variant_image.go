package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/jobspec"
	"damask/server/internal/queue"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"go.opentelemetry.io/otel/attribute"
)

const variantStatusReady = "ready"

// enqueueVariantThumb enqueues a generate_variant_thumbnail job after a variant row is created.
func (s *JobServer) enqueueVariantThumb(
	ctx context.Context,
	p jobspec.VariantJobPayload,
	variantID, storageKey, contentType string,
) {
	_ = EnqueueVariantThumbnailJob(ctx, s, VariantThumbnailJobPayload{
		VariantID:   variantID,
		WorkspaceID: p.WorkspaceID,
		AssetID:     p.AssetID,
		StorageKey:  storageKey,
		MimeType:    contentType,
	})
}

// imageLocalBuild is the variantBuildFn for local image transforms (resize, convert, crop, watermark, smart-crop).
func (s *JobServer) imageLocalBuild(
	_ context.Context, jobType, _, workspaceID string,
	params json.RawMessage,
) (variantTransformer, error) {
	return s.imageLocalTransformer(jobType, params, workspaceID)
}

// imageLocalTransformer returns a variantTransformer for local
// image transforms: resize, convert, crop, watermark, smart-crop.
func (s *JobServer) imageLocalTransformer(
	jobType string,
	params json.RawMessage,
	workspaceID string,
) (variantTransformer, error) {
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", jobType),
		)

		rc, err := s.storage.Get(ctx, sourceKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("get file: %w", err)
		}
		defer rc.Close()

		var (
			data        []byte
			contentType string
		)
		switch jobType {
		case queue.JobTypeImageResize:
			var p transform.ResizeParams
			if err = json.Unmarshal(params, &p); err != nil {
				telemetry.EndSpan(span, err)
				return nil, "", fmt.Errorf("parse resize params: %w", err)
			}
			data, contentType, err = s.trf.ImageResize(rc, p)
		case queue.JobTypeImageConvert:
			var p transform.ConvertParams
			if err = json.Unmarshal(params, &p); err != nil {
				telemetry.EndSpan(span, err)
				return nil, "", fmt.Errorf("parse convert params: %w", err)
			}
			data, contentType, err = s.trf.ImageConvert(rc, p)
		case queue.JobTypeImageCrop:
			var p transform.CropParams
			if err = json.Unmarshal(params, &p); err != nil {
				telemetry.EndSpan(span, err)
				return nil, "", fmt.Errorf("parse crop params: %w", err)
			}
			data, contentType, err = s.trf.ImageCrop(rc, p)
		case queue.JobTypeImageWatermark:
			var p transform.WatermarkParams
			if err = json.Unmarshal(params, &p); err != nil {
				telemetry.EndSpan(span, err)
				return nil, "", fmt.Errorf("parse watermark params: %w", err)
			}
			if p.WatermarkAssetID == "" {
				err = errors.New("watermark asset id is required")
				telemetry.EndSpan(span, err)
				return nil, "", err
			}
			wm, werr := s.queries.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
				ID:          p.WatermarkAssetID,
				WorkspaceID: workspaceID,
			})
			if werr != nil {
				telemetry.EndSpan(span, werr)
				return nil, "", fmt.Errorf("get watermark asset: %w", werr)
			}
			wmRC, werr := s.storage.Get(ctx, wm.StorageKey)
			if werr != nil {
				telemetry.EndSpan(span, werr)
				return nil, "", fmt.Errorf("get watermark file: %w", werr)
			}
			data, contentType, err = s.trf.ImageWatermark(rc, wmRC, p)
			_ = wmRC.Close()
		case queue.JobTypeImageSmartCrop:
			var p transform.SmartCropParams
			if err = json.Unmarshal(params, &p); err != nil {
				telemetry.EndSpan(span, err)
				return nil, "", fmt.Errorf("parse smartcrop params: %w", err)
			}
			data, contentType, err = s.trf.ImageSmartCrop(rc, p)
		default:
			err = fmt.Errorf("unknown image job type: %s", jobType)
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("transform: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return data, contentType, nil
	}, nil
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
