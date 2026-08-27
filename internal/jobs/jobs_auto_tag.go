package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"damask/server/internal/ai"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/transform"
	"damask/server/internal/workflow"

	"github.com/google/uuid"
)

// autoTagDefaultVisionModel is used for every auto-tag vision call. Unlike
// workflow nodes, auto-tagging has no per-call model picker in the UI, so a
// single inexpensive, vision-capable default is used regardless of provider.
const autoTagDefaultVisionModel = "google/gemini-2.5-flash"

// autoTagDefaultTextModel is used for every auto-tag text-tagging call (e.g.
// transcribed audio). Same underlying model as autoTagDefaultVisionModel
// today, but kept as a separate constant so the two call sites can diverge
// without surprising each other.
const autoTagDefaultTextModel = "google/gemini-2.5-flash"

const autoTagModeSilent = "silent"

// maxAutoTagSuggestions caps the number of tags accepted from a model
// response, matching the "Maximum 8 tags" rule stated in the prompt — models
// don't always respect prompt instructions.
const maxAutoTagSuggestions = 8

// maxAutoTagTranscriptChars bounds how much transcript text is embedded in
// the text-tagging prompt, to stay well within the chat model's context
// window for long recordings.
const maxAutoTagTranscriptChars = 8000

// AutoTagPayload is the payload for the auto_tag job.
type AutoTagPayload struct {
	WorkspaceID          string `json:"workspace_id"`
	AssetID              string `json:"asset_id"`
	AssetVersionID       string `json:"asset_version_id"` // version that triggered this run
	StorageKey           string `json:"storage_key"`      // original file key
	MimeType             string `json:"mime_type"`
	ThumbnailKey         string `json:"thumbnail_key"`          // preferred — smaller than original
	ThumbnailContentType string `json:"thumbnail_content_type"` // actual content-type of ThumbnailKey
	Mode                 string `json:"mode"`                   // "pending" | "silent"
	Size                 int64  `json:"size"`                   // original file size in bytes, if known
	// Continuation, when set, resumes a suspended workflow run once tagging
	// finishes (see action.auto_tag workflow node). Any nil-error outcome —
	// including the many "nothing to do" skip paths below — resumes the run
	// with whatever tags (possibly none) were produced; only a genuine job
	// error fails the continuation.
	Continuation *workflow.NodeContinuation `json:"continuation,omitempty"`
}

// jobAutoTag is the registered handler for queue.JobTypeAutoTag. It delegates
// to runAutoTag for the actual tagging logic, then resumes or fails any
// workflow continuation carried in the payload.
func (s *JobServer) jobAutoTag(ctx context.Context, job dbgen.Job) error {
	var p AutoTagPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("jobAutoTag: unmarshal: %w", err)
	}

	tags, err := s.runAutoTag(ctx, p)
	if err != nil {
		s.failContinuation(ctx, p.Continuation, err)
		return err
	}

	if p.Continuation == nil {
		return nil
	}

	if resumeErr := s.workflowExec.ResumeAt(ctx, *p.Continuation, map[string]any{
		"tags":  tags,
		"count": len(tags),
	}); resumeErr != nil {
		slog.ErrorContext(ctx, "workflow continuation failed after auto-tag ready",
			"run_id", p.Continuation.RunID,
			"node_id", p.Continuation.NodeID,
			"error", resumeErr,
		)
		s.failContinuation(ctx, p.Continuation, resumeErr)
		return fmt.Errorf("jobAutoTag: resume workflow: %w", resumeErr)
	}
	return nil
}

// runAutoTag suggests tags for an asset via a vision-capable AI provider. It
// is idempotent: pending-mode runs wipe any existing suggestions for the
// asset before inserting fresh ones; silent-mode runs only apply tags not
// already present on the asset. Returns the tags actually applied/suggested
// (empty when a "nothing to do" condition was hit — that is not an error).
func (s *JobServer) runAutoTag(ctx context.Context, p AutoTagPayload) ([]string, error) {
	// Workspace guard: the asset must exist in the job's workspace before any
	// tag reads/writes happen.
	if _, err := s.queries.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
		ID:          p.AssetID,
		WorkspaceID: p.WorkspaceID,
	}); err != nil {
		return nil, queue.Permanent(
			fmt.Errorf("auto_tag: asset %s not found in workspace %s", p.AssetID, p.WorkspaceID),
		)
	}

	if !transform.IsAutoTaggable(p.MimeType) {
		slog.WarnContext(ctx, "auto_tag: ineligible mime type, skipping", "mime", p.MimeType, "asset_id", p.AssetID)
		return nil, nil
	}

	if transform.IsAudioMime(p.MimeType) {
		return s.runAutoTagAudio(ctx, p)
	}

	imageKey, describeMime, ok := autoTagImageSource(p)
	if !ok {
		slog.WarnContext(ctx, "auto_tag: no renderable image source available yet, skipping", "asset_id", p.AssetID)
		return nil, nil
	}

	provider, err := s.resolveProvider(ctx, p.WorkspaceID, "", ai.CapVisionTag)
	if err != nil || provider == nil {
		// Not a job failure — the operator just hasn't configured a provider.
		slog.WarnContext(ctx, "auto_tag: no provider configured for vision tagging, skipping",
			"workspace_id", p.WorkspaceID, "asset_id", p.AssetID)
		return nil, nil //nolint:nilerr // see comment above
	}

	ws, err := s.queries.GetWorkspaceByID(ctx, p.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("jobAutoTag: get workspace: %w", err)
	}

	prompt, err := s.buildAutoTagPrompt(ctx, p.WorkspaceID, ws.LockedTaxonomy != 0)
	if err != nil {
		return nil, fmt.Errorf("jobAutoTag: build prompt: %w", err)
	}

	rc, err := s.storage.Get(ctx, imageKey)
	if err != nil {
		return nil, fmt.Errorf("jobAutoTag: read image: %w", err)
	}
	imageBytes, err := io.ReadAll(io.LimitReader(rc, ai.MaxDescribeImageBytes+1))
	_ = rc.Close()
	if err != nil {
		return nil, fmt.Errorf("jobAutoTag: read bytes: %w", err)
	}
	if len(imageBytes) > ai.MaxDescribeImageBytes {
		slog.WarnContext(ctx, "auto_tag: image exceeds max size for tagging, skipping", "asset_id", p.AssetID)
		return nil, nil
	}

	rawResponse, err := provider.DescribeImage(ctx, autoTagDefaultVisionModel, prompt, imageBytes, describeMime)
	if err != nil {
		return nil, fmt.Errorf("jobAutoTag: describe image: %w", err)
	}

	tags, err := parseTagSuggestions(rawResponse)
	if err != nil {
		// Malformed model output is not a job failure — degrade gracefully.
		slog.WarnContext(ctx, "auto_tag: could not parse model response",
			"asset_id", p.AssetID, "raw", rawResponse, "error", err)
		return nil, nil
	}
	if len(tags) == 0 {
		return nil, nil
	}

	return s.finishAutoTag(ctx, p, tags)
}

// runAutoTagAudio handles the audio/* branch of runAutoTag: transcribe the
// asset's audio via a speech-to-text-capable provider, persist the transcript
// as a text track, then tag it via a text-capable provider.
//
//nolint:cyclop // sequential best-effort pipeline; each guard is an independent skip condition
func (s *JobServer) runAutoTagAudio(ctx context.Context, p AutoTagPayload) ([]string, error) {
	transcribeProvider, err := s.resolveProvider(ctx, p.WorkspaceID, "", ai.CapAudioTranscription)
	if err != nil || transcribeProvider == nil {
		// Not a job failure — the operator just hasn't configured a provider.
		slog.WarnContext(ctx, "auto_tag: no provider configured for audio transcription, skipping",
			"workspace_id", p.WorkspaceID, "asset_id", p.AssetID)
		return nil, nil //nolint:nilerr // see comment above
	}

	// Reject up front using the already-known file size when available,
	// rather than buffering up to the limit before finding out it's too big.
	if p.Size > ai.MaxTranscribeAudioBytes {
		slog.WarnContext(ctx, "auto_tag: audio exceeds max size for transcription, skipping", "asset_id", p.AssetID)
		return nil, nil
	}

	rc, err := s.storage.Get(ctx, p.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("runAutoTagAudio: read audio: %w", err)
	}
	audioBytes, err := io.ReadAll(io.LimitReader(rc, ai.MaxTranscribeAudioBytes+1))
	_ = rc.Close()
	if err != nil {
		return nil, fmt.Errorf("runAutoTagAudio: read bytes: %w", err)
	}
	if len(audioBytes) > ai.MaxTranscribeAudioBytes {
		slog.WarnContext(ctx, "auto_tag: audio exceeds max size for transcription, skipping", "asset_id", p.AssetID)
		return nil, nil
	}

	format, ok := ai.TranscriptionFormatFromMimeType(p.MimeType)
	if !ok {
		slog.WarnContext(ctx, "auto_tag: unsupported audio mime type for transcription, skipping",
			"mime", p.MimeType, "asset_id", p.AssetID)
		return nil, nil
	}
	transcript, err := transcribeProvider.TranscribeAudio(ctx, ai.DefaultTranscriptionModel, audioBytes, format)
	if err != nil {
		return nil, fmt.Errorf("runAutoTagAudio: transcribe: %w", err)
	}
	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		return nil, nil
	}

	// Persist the full transcript (unlike promptTranscript below, which is
	// capped for the tagging prompt) — the text track is meant to be a
	// complete record of the audio, not just what's sent to the model.
	if _, ttErr := s.textTrackSvc.CreateAudioTranscript(
		ctx, p.WorkspaceID, p.AssetID, p.AssetVersionID, transcript,
	); ttErr != nil {
		// Best-effort: a failed transcript save shouldn't block tagging.
		slog.WarnContext(ctx, "auto_tag: persist transcript failed", "asset_id", p.AssetID, "error", ttErr)
	}

	tagProvider, err := s.resolveProvider(ctx, p.WorkspaceID, "", ai.CapTextTag)
	if err != nil || tagProvider == nil {
		// Not a job failure — the operator just hasn't configured a provider.
		slog.WarnContext(ctx, "auto_tag: no provider configured for text tagging, skipping",
			"workspace_id", p.WorkspaceID, "asset_id", p.AssetID)
		return nil, nil //nolint:nilerr // see comment above
	}

	ws, err := s.queries.GetWorkspaceByID(ctx, p.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("runAutoTagAudio: get workspace: %w", err)
	}

	promptTranscript := transcript
	if len(promptTranscript) > maxAutoTagTranscriptChars {
		promptTranscript = promptTranscript[:maxAutoTagTranscriptChars]
	}
	prompt, err := s.buildAutoTagAudioPrompt(ctx, p.WorkspaceID, ws.LockedTaxonomy != 0, promptTranscript)
	if err != nil {
		return nil, fmt.Errorf("runAutoTagAudio: build prompt: %w", err)
	}

	rawResponse, err := tagProvider.TagText(ctx, autoTagDefaultTextModel, prompt)
	if err != nil {
		return nil, fmt.Errorf("runAutoTagAudio: tag text: %w", err)
	}

	tags, err := parseTagSuggestions(rawResponse)
	if err != nil {
		// Malformed model output is not a job failure — degrade gracefully.
		slog.WarnContext(ctx, "auto_tag: could not parse model response",
			"asset_id", p.AssetID, "raw", rawResponse, "error", err)
		return nil, nil
	}
	if len(tags) == 0 {
		return nil, nil
	}

	return s.finishAutoTag(ctx, p, tags)
}

// finishAutoTag removes tags already on the asset, then either applies the
// remainder directly (silent mode) or stores them as pending suggestions.
// Shared by both the image/video/PDF and audio auto-tag paths. Returns the
// tags that were actually applied/suggested.
func (s *JobServer) finishAutoTag(ctx context.Context, p AutoTagPayload, tags []string) ([]string, error) {
	tags, err := s.removeExistingAutoTags(ctx, p.AssetID, tags)
	if err != nil {
		return nil, fmt.Errorf("jobAutoTag: remove existing tags: %w", err)
	}
	if len(tags) == 0 {
		return nil, nil
	}

	if p.Mode == autoTagModeSilent {
		return tags, s.applyAutoTags(ctx, p.WorkspaceID, p.AssetID, tags)
	}
	if err = s.storeAutoTagSuggestions(ctx, p, tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// autoTagImageSource picks the storage key to render and the MIME type to
// declare to the vision API. Thumbnails are always preferred over the
// original (cheaper, and the only renderable form for video/PDF inputs,
// which are sent as a thumbnail frame/render rather than the raw file).
func autoTagImageSource(p AutoTagPayload) (storageKey, describeMime string, ok bool) {
	if transform.IsImageMime(p.MimeType) {
		if p.ThumbnailKey != "" {
			return p.ThumbnailKey, p.MimeType, true
		}
		return p.StorageKey, p.MimeType, true
	}
	// video/* and application/pdf have no directly renderable original —
	// only a generated thumbnail can be sent. Its format depends on the
	// source (JPEG, PNG, or an MP4 frame render), so use the content-type
	// recorded when the thumbnail was generated rather than assuming JPEG.
	if p.ThumbnailKey != "" {
		if p.ThumbnailContentType != "" {
			return p.ThumbnailKey, p.ThumbnailContentType, true
		}
		return p.ThumbnailKey, transform.MimeImageJPEG, true
	}
	return "", "", false
}

// removeExistingAutoTags filters out tag names already applied to the asset.
func (s *JobServer) removeExistingAutoTags(
	ctx context.Context,
	assetID string,
	tags []string,
) ([]string, error) {
	existing, err := s.queries.GetTagsForAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	existingNames := make(map[string]bool, len(existing))
	for _, t := range existing {
		existingNames[strings.ToLower(t.Name)] = true
	}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if !existingNames[t] {
			out = append(out, t)
		}
	}
	return out, nil
}

// applyAutoTags applies AI-suggested tags via the real TagService, so the
// normal audit + workflow-trigger side effects fire for silently-applied
// tags too (jobs.tagSvc is a dedicated TagService instance wired with a real
// TriggerDispatcher — see cmd/server/main.go).
//
//nolint:unparam // best-effort: per-tag apply failures are logged, not surfaced
func (s *JobServer) applyAutoTags(ctx context.Context, workspaceID, assetID string, tags []string) error {
	ctx = auth.WithActor(ctx, auth.Actor{Type: audit.ActorTypeSystem})
	for _, name := range tags {
		if err := s.tagSvc.ApplyTag(ctx, workspaceID, assetID, name); err != nil {
			slog.WarnContext(ctx, "auto_tag: apply tag failed", "tag", name, "error", err)
		}
	}
	return nil
}

// storeAutoTagSuggestions wipes stale suggestions for the asset and inserts
// fresh ones, making the job idempotent across re-runs.
func (s *JobServer) storeAutoTagSuggestions(ctx context.Context, p AutoTagPayload, tags []string) error {
	if err := s.queries.DeleteAutoTagSuggestionsByAsset(ctx, dbgen.DeleteAutoTagSuggestionsByAssetParams{
		AssetID:     p.AssetID,
		WorkspaceID: p.WorkspaceID,
	}); err != nil {
		return fmt.Errorf("delete stale suggestions: %w", err)
	}
	var versionID *string
	if p.AssetVersionID != "" {
		versionID = &p.AssetVersionID
	}
	failed := 0
	for _, name := range tags {
		if _, err := s.queries.CreateAutoTagSuggestion(ctx, dbgen.CreateAutoTagSuggestionParams{
			ID:             uuid.NewString(),
			WorkspaceID:    p.WorkspaceID,
			AssetID:        p.AssetID,
			AssetVersionID: versionID,
			TagName:        name,
		}); err != nil {
			slog.WarnContext(ctx, "auto_tag: create suggestion failed", "tag", name, "error", err)
			failed++
		}
	}
	if failed == len(tags) {
		return fmt.Errorf("create suggestion: all %d inserts failed", failed)
	}
	return nil
}

// buildAutoTagPrompt returns the prompt for the auto-tag job. When
// lockedTaxonomy is true, the full workspace vocabulary is embedded in the
// prompt and the model must constrain its output to that list — there is no
// post-filtering step. The model returning [] when no vocabulary tag applies
// is the correct and expected signal.
func (s *JobServer) buildAutoTagPrompt(ctx context.Context, workspaceID string, lockedTaxonomy bool) (string, error) {
	const baseOpen = `You are a digital asset tagging assistant.
Analyse the image and suggest concise, relevant tags.

Rules:
- Return ONLY a JSON array of lowercase strings, e.g. ["logo","blue","hero"].
- Maximum 8 tags.
- Omit any tag you are not confident about.
- No explanations, no markdown fences, no extra text — only the JSON array.`

	const baseLockedFormat = `You are a digital asset tagging assistant.
Analyse the image and suggest relevant tags from the vocabulary below.

Rules:
- Return ONLY a JSON array of lowercase strings chosen from the vocabulary.
- Maximum 8 tags.
- Omit any tag that does not clearly apply.
- Return [] if no vocabulary tag is relevant.
- No explanations, no markdown fences, no extra text — only the JSON array.

Vocabulary:
%s`

	if !lockedTaxonomy {
		return baseOpen, nil
	}

	tags, err := s.queries.ListTagsInWorkspace(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	if len(tags) == 0 {
		return `Return [].`, nil
	}

	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return fmt.Sprintf(baseLockedFormat, strings.Join(names, ", ")), nil
}

// buildAutoTagAudioPrompt returns the prompt for the audio branch of the
// auto-tag job: same rules and locked-taxonomy behavior as
// buildAutoTagPrompt, but tagging is based on a transcript rather than a
// rendered image.
func (s *JobServer) buildAutoTagAudioPrompt(
	ctx context.Context,
	workspaceID string,
	lockedTaxonomy bool,
	transcript string,
) (string, error) {
	const baseOpenAudio = `You are a digital asset tagging assistant.
Analyse the following audio transcript and suggest concise, relevant tags.

Rules:
- Return ONLY a JSON array of lowercase strings, e.g. ["interview","music","tutorial"].
- Maximum 8 tags.
- Omit any tag you are not confident about.
- No explanations, no markdown fences, no extra text — only the JSON array.

Transcript:
%s`

	const baseLockedFormatAudio = `You are a digital asset tagging assistant.
Analyse the following audio transcript and suggest relevant tags from the vocabulary below.

Rules:
- Return ONLY a JSON array of lowercase strings chosen from the vocabulary.
- Maximum 8 tags.
- Omit any tag that does not clearly apply.
- Return [] if no vocabulary tag is relevant.
- No explanations, no markdown fences, no extra text — only the JSON array.

Vocabulary:
%s

Transcript:
%s`

	if !lockedTaxonomy {
		return fmt.Sprintf(baseOpenAudio, transcript), nil
	}

	tags, err := s.queries.ListTagsInWorkspace(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	if len(tags) == 0 {
		return `Return [].`, nil
	}

	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return fmt.Sprintf(baseLockedFormatAudio, strings.Join(names, ", "), transcript), nil
}

// parseTagSuggestions parses the model's JSON array response. Strips
// markdown fences if present (some models add them despite instructions).
// Returns a deduplicated, lowercased, trimmed slice.
func parseTagSuggestions(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil, fmt.Errorf("parseTagSuggestions: %w (raw: %q)", err, raw)
	}

	seen := make(map[string]bool, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	if len(out) > maxAutoTagSuggestions {
		out = out[:maxAutoTagSuggestions]
	}
	return out, nil
}
