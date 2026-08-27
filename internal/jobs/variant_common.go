package jobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/jobspec"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

// variantTransformer executes a media transform given a storage source key
// and returns (data, contentType, error).
type variantTransformer func(ctx context.Context, sourceKey string) ([]byte, string, error)

// variantBuildFn is the unified builder signature for all variant types.
// jobType and sourceMime are the job type constant and the asset MIME type.
// workspaceID is needed by types that look up workspace-specific config or assets.
type variantBuildFn func(
	ctx context.Context, jobType, sourceMime, workspaceID string, params json.RawMessage,
) (variantTransformer, error)

// variantEntry describes a registered variant type.
type variantEntry struct {
	build           variantBuildFn
	canonicalJSON   func(jobType, sourceMime string, params json.RawMessage) (string, error)                                        // nil = use raw params
	useFullFinalize bool                                                                                                            // true = CreateVariantFull (image user-triggered)
	postJobHook     func(s *JobServer, ctx context.Context, p jobspec.VariantJobPayload, variantID, storageKey, contentType string) // optional
}

// variantRegistry returns the map of all registered variant types.
// Called once per generic job dispatch; allocating a small map each time is fine.
func (s *JobServer) variantRegistry() map[string]variantEntry {
	localImage := variantEntry{
		build:           s.imageLocalBuild,
		useFullFinalize: true,
	}
	return map[string]variantEntry{
		// Image - local transforms
		queue.JobTypeImageResize:    localImage,
		queue.JobTypeImageConvert:   localImage,
		queue.JobTypeImageCrop:      localImage,
		queue.JobTypeImageWatermark: localImage,
		queue.JobTypeImageSmartCrop: localImage,

		// Image - ImageRouter
		queue.JobTypeImageBgRemove: {
			build:           s.imageBgRemoveBuild,
			useFullFinalize: true,
		},
		queue.JobTypeImageWithPrompt: {
			build:           s.imageWithPromptBuild,
			useFullFinalize: true,
		},

		// Video
		queue.JobTypeVideoCaptureImage: {
			build:         s.videoCaptureBuild,
			canonicalJSON: videoCaptureCanonical,
			postJobHook:   (*JobServer).videoCapturePostHook,
		},
		queue.JobTypeVideoTranscode: {
			build:         s.videoTranscodeBuild,
			canonicalJSON: videoTranscodeCanonical,
		},
		queue.JobTypeVideoWatermark: {
			build:         s.videoWatermarkBuild,
			canonicalJSON: videoWatermarkCanonical,
		},

		// Audio
		queue.JobTypeExtractAudio:   {build: s.audioBuild, canonicalJSON: audioCanonical},
		queue.JobTypeTranscodeAudio: {build: s.audioBuild, canonicalJSON: audioCanonical},
		queue.JobTypeNormalizeAudio: {build: s.audioBuild, canonicalJSON: audioCanonical},

		// Custom ffmpeg command — works on any MIME type.
		queue.JobTypeCustomFFmpeg: {
			build:         s.customFFmpegBuild,
			canonicalJSON: customFFmpegCanonical,
		},
	}
}

// jobVariant is the single generic handler for all user-triggered variant jobs.
// It fails the paused workflow run (if any) when runVariantJob errors, so a
// run waiting on a create_variant continuation doesn't stay "running" forever.
func (s *JobServer) jobVariant(ctx context.Context, job dbgen.Job) error {
	var p jobspec.VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}
	if err := s.runVariantJob(ctx, job, p); err != nil {
		s.failContinuation(ctx, p.Continuation, err)
		return err
	}
	return nil
}

// runVariantJob runs the build/transform/finalize pipeline for a single
// variant job. p.Continuation is only ever non-nil for useFullFinalize
// (workflow-triggered image) jobs — rebuild-variant jobs never set it.
func (s *JobServer) runVariantJob(ctx context.Context, job dbgen.Job, p jobspec.VariantJobPayload) error {
	reg := s.variantRegistry()
	entry, ok := reg[job.Type]
	if !ok {
		return fmt.Errorf("unknown variant job type: %s", job.Type)
	}

	trf, err := entry.build(ctx, job.Type, p.MimeType, p.WorkspaceID, p.Params)
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, p.StorageKey)
	if err != nil {
		return err
	}

	// Resolve canonical params JSON (some types normalise defaults post-parse).
	paramsJSON := string(p.Params)
	if entry.canonicalJSON != nil {
		paramsJSON, err = entry.canonicalJSON(job.Type, p.MimeType, p.Params)
		if err != nil {
			return fmt.Errorf("canonical params: %w", err)
		}
	}
	paramsHash := CanonicalParamsHash(paramsJSON)

	if entry.useFullFinalize {
		variantID := resolveVariantID(p)
		if e := s.finalizeVariant(
			ctx,
			p,
			variantID,
			job.Type,
			paramsJSON,
			paramsHash,
			data,
			contentType,
		); e != nil {
			return e
		}
		if entry.postJobHook != nil {
			storageKey := storage.VersionedVariantKey(
				p.WorkspaceID,
				p.AssetID,
				p.VersionNum,
				job.Type,
				paramsHash,
				transform.MimeToExt(contentType),
			)
			entry.postJobHook(s, ctx, p, variantID, storageKey, contentType)
		}
		return nil
	}

	ver := assetVersionFromPayload(p)
	if e := s.finalizeRebuildVariant(ctx, ver, job.Type, paramsJSON, paramsHash, data, contentType); e != nil {
		return e
	}
	if entry.postJobHook != nil {
		variantID := resolveVariantID(p)
		storageKey := storage.VersionedVariantKey(
			p.WorkspaceID,
			p.AssetID,
			p.VersionNum,
			job.Type,
			paramsHash,
			transform.MimeToExt(contentType),
		)
		entry.postJobHook(s, ctx, p, variantID, storageKey, contentType)
	}
	return nil
}

// rebuildOneVariant runs the transform for a single variant spec and writes the result.
func (s *JobServer) rebuildVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	variantType, paramsJSON, paramsHash string,
) error {
	reg := s.variantRegistry()
	entry, ok := reg[variantType]
	if !ok {
		return fmt.Errorf("unknown variant type for rebuild: %s", variantType)
	}
	trf, err := entry.build(ctx, variantType, ver.MimeType, ver.WorkspaceID, json.RawMessage(paramsJSON))
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, ver.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeRebuildVariant(ctx, ver, variantType, paramsJSON, paramsHash, data, contentType)
}

// resolveVariantID returns p.VariantID if set, otherwise generates a new UUID.
func resolveVariantID(p jobspec.VariantJobPayload) string {
	if p.VariantID != "" {
		return p.VariantID
	}
	return uuid.NewString()
}

// finalizeVariant stores the transform output, creates the variant DB row (with
// title and is_shared from the payload), then publishes the ready event and
// enqueues the thumbnail job. Used by user-triggered image variant jobs.
func (s *JobServer) finalizeVariant(
	ctx context.Context,
	p jobspec.VariantJobPayload,
	variantID, jobType, paramsJSON, paramsHash string,
	data []byte,
	contentType string,
) error {
	ext := transform.MimeToExt(contentType)
	storageKey := storage.VersionedVariantKey(p.WorkspaceID, p.AssetID, p.VersionNum, jobType, paramsHash, ext)

	if err := s.storage.Put(ctx, storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sum := sha256.Sum256(data)
	contentHash := hex.EncodeToString(sum[:])
	sz := int64(len(data))
	_, err := s.queries.CreateVariantFull(ctx, dbgen.CreateVariantFullParams{
		ID:              variantID,
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.VersionID,
		Type:            jobType,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
		Status:          variantStatusReady,
		Title:           p.Title,
		IsShared:        boolToInt64(p.IsShared),
		ContentHash:     contentHash,
	})
	if err == nil {
		s.publishVariantReady(ctx, p.WorkspaceID, p.AssetID, variantID)
		s.enqueueVariantThumb(ctx, p, variantID, storageKey, contentType)
		if p.Continuation != nil {
			if resumeErr := s.workflowExec.ResumeAt(ctx, *p.Continuation, map[string]any{
				"variant_id":           variantID,
				"variant_content_type": contentType,
				"variant_storage_key":  storageKey,
			}); resumeErr != nil {
				slog.ErrorContext(ctx, "workflow continuation failed after variant ready",
					"run_id", p.Continuation.RunID,
					"node_id", p.Continuation.NodeID,
					"error", resumeErr,
				)
				return resumeErr
			}
		}
	}
	return err
}

// finalizeRebuildVariant stores the transform output, creates the variant DB
// row (no title/is_shared), then publishes the ready event and enqueues the
// thumbnail job. Used by rebuild variant jobs and video/audio user-triggered jobs.
func (s *JobServer) finalizeRebuildVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	variantType, paramsJSON, paramsHash string,
	data []byte,
	contentType string,
) error {
	ext := transform.MimeToExt(contentType)
	storageKey := storage.VersionedVariantKey(
		ver.WorkspaceID,
		ver.AssetID,
		ver.VersionNum,
		variantType,
		paramsHash,
		ext,
	)

	if err := s.storage.Put(ctx, storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sum := sha256.Sum256(data)
	sz := int64(len(data))
	vid := uuid.NewString()
	_, err := s.queries.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              vid,
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            variantType,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
		ContentHash:     hex.EncodeToString(sum[:]),
	})
	if err == nil {
		s.publishVariantReady(ctx, ver.WorkspaceID, ver.AssetID, vid)
		s.enqueueVariantThumbRaw(ctx, ver.WorkspaceID, ver.AssetID, vid, storageKey, contentType)
	}
	return err
}
