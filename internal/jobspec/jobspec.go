// Package jobspec is the shared vocabulary between internal/service and
// internal/jobs: the job-type constants persisted in the jobs table and the
// wire payload structs service marshals when enqueuing work for jobs to
// unmarshal and run. It sits below both packages so service can depend on
// the payload/type definitions directly instead of importing internal/jobs,
// which used to force internal/jobs to hand-maintain duplicate interface
// subsets of the real internal/service interfaces just to dodge that import.
//
// Invariant: every payload here carries WorkspaceID, and any handler that
// consumes one must scope all reads/writes to it — jobs run without an
// authenticated request context, so this is the only workspace segregation
// guard they get.
package jobspec

import "encoding/json"

// Job type constants used throughout the application. Values are persisted
// in the jobs table, so they must never change once shipped. internal/queue
// re-exports these under their historical queue.JobType* names so the many
// existing call sites don't need to change.
const (
	JobTypeVersionThumbnail            = "version_thumbnail"
	JobTypeVariantThumbnail            = "generate_variant_thumbnail"
	JobTypeOCRTextTrack                = "ocr_text_track"
	JobTypeAIImageDescriptionTextTrack = "ai_image_description_text_track"
	JobTypeExtractPDFTextTrack         = "document_pdf_extract_text_track"
	JobTypeExtractPlainTextTrack       = "document_plain_extract_text_track"
	JobTypeExtractDocumentTextTrack    = "document_office_extract_text_track"

	JobTypeVideoCaptureImage = "video_capture_image"
	JobTypeVideoTranscode    = "video_transcode"
	JobTypeVideoWatermark    = "video_watermark"
	JobTypeImageResize       = "image_resize"
	JobTypeImageConvert      = "image_convert"
	JobTypeImageCrop         = "image_crop"
	JobTypeImageWatermark    = "image_watermark"
	JobTypeImageBgRemove     = "image_bg_remove"
	JobTypeImageWithPrompt   = "image_with_prompt"
	JobTypeImageSmartCrop    = "image_smart_crop"
	JobTypeExtractAudio      = "video_extract"
	JobTypeTranscodeAudio    = "audio_transcode"
	JobTypeNormalizeAudio    = "audio_normalize"
	JobTypeCustomFFmpeg      = "custom_ffmpeg"

	JobTypeIngestPoll  = "ingest_poll"
	JobTypeIngestFetch = "ingest_fetch"

	JobTypeRebuildVariants = "rebuild_variants"
	JobTypeRunWorkflow     = "run_workflow"

	JobTypeExtractExif      = "extract_exif"
	JobTypeExtractMediaTags = "extract_media_tags"

	JobTypeStackMerge         = "stack_merge"
	JobTypeCreateVariantDraft = "create_variant_draft"

	JobTypeExportRun = "export_run"

	JobTypePurgeDeletedFields      = "purge_deleted_fields"
	JobTypeEnforceVersionRetention = "enforce_version_retention"
	JobTypePurgeVersionStorage     = "purge_version_storage"
	JobTypePurgeAuditLog           = "purge_event_log"
	JobTypePurgeScratchVariants    = "purge_scratch_variants"

	JobTypeVisualSimilarityBackfill = "visual_similarity_backfill"

	JobTypeAutoTag = "auto_tag"
)

// MetaKeyWordCount is the shared meta/context key for OCR and AI image
// description word counts.
const MetaKeyWordCount = "word_count"

// NodeContinuation identifies a paused workflow node run to resume once an
// async job (OCR, AI description, variant creation, ...) completes.
// Defined here rather than in internal/workflow because job payloads that
// embed it must be constructible from internal/service without pulling in
// internal/workflow's own dependency on internal/queue (which would cycle
// back through this package). internal/workflow aliases this type.
type NodeContinuation struct {
	RunID       string `json:"run_id"`
	NodeID      string `json:"node_id"`
	WorkflowID  string `json:"workflow_id"`
	WorkspaceID string `json:"workspace_id"`
	ContextJSON string `json:"context_json"`
}

// VersionThumbnailJobPayload is the payload for version-specific thumbnail generation.
type VersionThumbnailJobPayload struct {
	AssetID     string `json:"asset_id"`
	VersionID   string `json:"version_id"`
	WorkspaceID string `json:"workspace_id"`
	StorageKey  string `json:"storage_key"`
	MimeType    string `json:"mime_type"`
}

// ExtractExifPayload is the payload for the extract_exif job.
type ExtractExifPayload struct {
	AssetID     string `json:"asset_id"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"` // required: field_definitions.created_by and asset_field_values.created_by are NOT NULL
}

// ExtractMediaTagsPayload is the payload for the extract_media_tags job.
type ExtractMediaTagsPayload struct {
	AssetID     string `json:"asset_id"`
	WorkspaceID string `json:"workspace_id"`
}

// ExtractTextPayload is the payload for the extract_text family of jobs.
type ExtractTextPayload struct {
	WorkspaceID string `json:"workspace_id"`
	AssetID     string `json:"asset_id"`
	StorageKey  string `json:"storage_key"`
	MimeType    string `json:"mime_type,omitempty"`
}

// VariantJobPayload is the payload for user-triggered variant creation jobs.
// VersionID and VersionNum identify the asset version the variant is bound to.
type VariantJobPayload struct {
	AssetID      string            `json:"asset_id"`
	WorkspaceID  string            `json:"workspace_id"`
	VersionID    string            `json:"version_id"`
	VersionNum   int64             `json:"version_num"`
	VariantID    string            `json:"variant_id,omitempty"`
	StorageKey   string            `json:"storage_key"`
	MimeType     string            `json:"mime_type"`
	Type         string            `json:"type"`
	Params       json.RawMessage   `json:"params"`
	Title        *string           `json:"title,omitempty"`
	IsShared     bool              `json:"is_shared,omitempty"`
	Continuation *NodeContinuation `json:"continuation,omitempty"`
}
