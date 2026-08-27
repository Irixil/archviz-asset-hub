package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"damask/server/internal/apperr"
	"damask/server/internal/jobspec"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"
	"damask/server/internal/workflow"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

const textTrackSourceManual = "manual"
const textTrackSourceOCR = "ocr"
const textTrackSourceAIImageDescription = "ai_image_description"
const textTrackSourceExtractPDF = "extract_pdf"
const textTrackSourceExtractPlain = "extract_plain"
const textTrackSourceExtractDocument = "extract_document"
const textTrackSourceAudioTranscript = "audio_transcript"

var ErrUnsupportedTextTrackSource = errors.New("unsupported text track source")

func extractJobType(source string) (string, error) {
	switch source {
	case textTrackSourceExtractPDF:
		return queue.JobTypeExtractPDFTextTrack, nil
	case textTrackSourceExtractPlain:
		return queue.JobTypeExtractPlainTextTrack, nil
	case textTrackSourceExtractDocument:
		return queue.JobTypeExtractDocumentTextTrack, nil
	default:
		return "", ErrUnsupportedTextTrackSource
	}
}

type textTrackService struct {
	repo    repository.TextTrackRepository
	queue   queue.JobQueue
	storage storage.Storage
}

func NewTextTrackService(repo repository.TextTrackRepository, q queue.JobQueue, stor storage.Storage) TextTrackService {
	return &textTrackService{repo: repo, queue: q, storage: stor}
}

func (s *textTrackService) List(ctx context.Context, workspaceID, assetID string) (out []TextTrackDTO, err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.text_tracks.list",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
	)
	defer func() {
		if out != nil {
			span.SetAttributes(attribute.Int("damask.text_tracks.result_count", len(out)))
		}
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track list failed",
				"workspace_id",
				workspaceID,
				"asset_id",
				assetID,
				"error",
				err,
			)
		}
	}()

	rows, err := s.repo.List(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	out = mapDTOs(rows, toTextTrackDTO)
	return out, nil
}

func (s *textTrackService) Get(ctx context.Context, workspaceID, trackID string) (dto TextTrackDTO, err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.text_tracks.get",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		if dto.ID != "" {
			span.SetAttributes(
				attribute.String("damask.asset_id", dto.AssetID),
				attribute.String("damask.text_track.source", dto.Source),
				attribute.String("damask.text_track.status", dto.Status),
			)
		}
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track get failed",
				"workspace_id",
				workspaceID,
				"track_id",
				trackID,
				"error",
				err,
			)
		}
	}()

	row, err := s.repo.Get(ctx, workspaceID, trackID)
	if err != nil {
		return TextTrackDTO{}, err
	}
	dto = toTextTrackDTO(row)
	return dto, nil
}

func (s *textTrackService) Create(ctx context.Context, p CreateTextTrackParams) (dto TextTrackDTO, err error) {
	p.Source = strings.TrimSpace(p.Source)
	ctx, span := telemetry.StartSpan(ctx, "service.text_tracks.create",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.String("damask.asset_id", p.AssetID),
		attribute.String("damask.text_track.source", p.Source),
	)
	defer func() {
		if dto.ID != "" {
			span.SetAttributes(
				attribute.String("damask.text_track_id", dto.ID),
				attribute.String("damask.text_track.status", dto.Status),
			)
		}
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track create failed",
				"workspace_id",
				p.WorkspaceID,
				"asset_id",
				p.AssetID,
				"source",
				p.Source,
				"error",
				err,
			)
		}
	}()

	if p.Source == "" {
		return TextTrackDTO{}, fmt.Errorf("source is required: %w", apperr.ErrInvalidInput)
	}

	if isExtractTextTrackSource(p.Source) {
		if stringParam(p.Params, "storage_key", "") == "" {
			return TextTrackDTO{}, fmt.Errorf("missing storage key: %w", apperr.ErrInvalidInput)
		}
		return s.enqueueExtract(ctx, span, p)
	}

	content, status, err := validateTextTrackSourceParams(p.Source, p.Params, p.InitialContent)
	if err != nil {
		return TextTrackDTO{}, err
	}

	var meta *string
	if len(p.Params) > 0 {
		b, marshalErr := json.Marshal(p.Params)
		if marshalErr != nil {
			return TextTrackDTO{}, fmt.Errorf("marshal params: %w", marshalErr)
		}
		s := string(b)
		meta = &s
	}
	createdBy := nilIfEmpty(p.CreatedBy)

	row, err := s.repo.Create(ctx, repository.CreateTextTrackParams{
		ID:             uuid.NewString(),
		WorkspaceID:    p.WorkspaceID,
		AssetID:        p.AssetID,
		AssetVersionID: p.AssetVersionID,
		Source:         p.Source,
		Lang:           p.Lang,
		Content:        content,
		Meta:           meta,
		Status:         status,
		CreatedBy:      createdBy,
	})
	if err != nil {
		return TextTrackDTO{}, err
	}

	dto = toTextTrackDTO(row)
	switch p.Source {
	case textTrackSourceManual, textTrackSourceAudioTranscript:
		return s.finalizeManualTrack(ctx, row, content, dto)
	case textTrackSourceOCR:
		jobID, enqErr := s.enqueueOCR(ctx, p, row.ID)
		if enqErr != nil {
			return TextTrackDTO{}, enqErr
		}
		span.SetAttributes(attribute.String("damask.job_id", jobID))
		return dto, nil
	case textTrackSourceAIImageDescription:
		jobID, enqErr := s.enqueueAIImageDescription(ctx, p, row.ID)
		if enqErr != nil {
			return TextTrackDTO{}, enqErr
		}
		span.SetAttributes(attribute.String("damask.job_id", jobID))
		return dto, nil
	}

	return TextTrackDTO{}, ErrUnsupportedTextTrackSource
}

func isExtractTextTrackSource(source string) bool {
	switch source {
	case textTrackSourceExtractPDF, textTrackSourceExtractPlain, textTrackSourceExtractDocument:
		return true
	default:
		return false
	}
}

// validateTextTrackSourceParams checks the per-source required params for
// non-extract sources and returns the initial content/status to persist.
// Extract sources are handled separately in Create since they short-circuit
// to enqueueExtract instead of creating the row here.
func validateTextTrackSourceParams(
	source string,
	params map[string]any,
	initialContent string,
) (content, status string, err error) {
	status = WorkflowRunStatusPending
	switch source {
	case textTrackSourceManual, textTrackSourceAudioTranscript:
		status = variantStatusReady
		content = readyTextContent(initialContent)
	case textTrackSourceOCR:
		if stringParam(params, "storage_key", "") == "" {
			return "", "", fmt.Errorf("missing OCR storage key: %w", apperr.ErrInvalidInput)
		}
	case textTrackSourceAIImageDescription:
		if stringParam(params, "storage_key", "") == "" {
			return "", "", fmt.Errorf(
				"missing storage key for ai_image_description: %w",
				apperr.ErrInvalidInput,
			)
		}
		if stringParam(params, "model", "") == "" {
			return "", "", fmt.Errorf("missing model for ai_image_description: %w", apperr.ErrInvalidInput)
		}
	default:
		return "", "", ErrUnsupportedTextTrackSource
	}
	return content, status, nil
}

// CreateAudioTranscript persists a ready-to-use audio transcript text track.
// Unlike OCR/AI-image-description, transcription already happened by the time
// this is called, so the track is created directly in the "ready" state.
func (s *textTrackService) CreateAudioTranscript(
	ctx context.Context,
	workspaceID, assetID, assetVersionID, transcript string,
) (trackID string, err error) {
	var versionID *string
	if assetVersionID != "" {
		versionID = &assetVersionID
	}
	dto, err := s.Create(ctx, CreateTextTrackParams{
		WorkspaceID:    workspaceID,
		AssetID:        assetID,
		AssetVersionID: versionID,
		Source:         textTrackSourceAudioTranscript,
		InitialContent: transcript,
	})
	if err != nil {
		return "", err
	}
	return dto.ID, nil
}

func (s *textTrackService) finalizeManualTrack(
	ctx context.Context,
	row repository.TextTrack,
	content string,
	dto TextTrackDTO,
) (TextTrackDTO, error) {
	lang := ""
	if row.Lang != nil {
		lang = *row.Lang
	}
	if ftsErr := s.repo.InsertFTS(ctx, repository.InsertTextFTSParams{
		TrackID:     row.ID,
		AssetID:     row.AssetID,
		WorkspaceID: row.WorkspaceID,
		Source:      row.Source,
		Lang:        lang,
		Content:     content,
	}); ftsErr != nil {
		return TextTrackDTO{}, ftsErr
	}
	return dto, nil
}

func (s *textTrackService) enqueueOCR(ctx context.Context, p CreateTextTrackParams, trackID string) (string, error) {
	payload, err := json.Marshal(struct {
		WorkspaceID    string                     `json:"workspace_id"`
		AssetID        string                     `json:"asset_id"`
		TrackID        string                     `json:"track_id"`
		AssetVersionID string                     `json:"asset_version_id"`
		StorageKey     string                     `json:"storage_key"`
		MimeType       string                     `json:"mime_type"`
		Lang           string                     `json:"lang"`
		OutputFormat   string                     `json:"output_format"`
		Continuation   *workflow.NodeContinuation `json:"continuation,omitempty"`
	}{
		WorkspaceID:    p.WorkspaceID,
		AssetID:        p.AssetID,
		TrackID:        trackID,
		AssetVersionID: stringValue(p.AssetVersionID),
		StorageKey:     stringParam(p.Params, "storage_key", ""),
		MimeType:       stringParam(p.Params, "mime_type", ""),
		Lang:           stringValue(p.Lang, "eng"),
		OutputFormat:   stringParam(p.Params, "output_format", "txt"),
		Continuation:   p.WorkflowContinuation,
	})
	if err != nil {
		return "", fmt.Errorf("marshal OCR payload: %w", err)
	}
	job, err := s.queue.Enqueue(ctx, p.WorkspaceID, queue.JobTypeOCRTextTrack, string(payload))
	if err != nil {
		return "", err
	}
	slog.DebugContext(ctx, "text track OCR enqueued",
		"workspace_id", p.WorkspaceID, "asset_id", p.AssetID, "track_id", trackID, "job_id", job.ID)
	return job.ID, nil
}

func (s *textTrackService) enqueueAIImageDescription(
	ctx context.Context,
	p CreateTextTrackParams,
	trackID string,
) (string, error) {
	payload, err := json.Marshal(struct {
		WorkspaceID  string                     `json:"workspace_id"`
		AssetID      string                     `json:"asset_id"`
		TrackID      string                     `json:"track_id"`
		StorageKey   string                     `json:"storage_key"`
		MimeType     string                     `json:"mime_type"`
		Model        string                     `json:"model"`
		Prompt       string                     `json:"prompt"`
		Lang         string                     `json:"lang"`
		Continuation *workflow.NodeContinuation `json:"continuation,omitempty"`
	}{
		WorkspaceID:  p.WorkspaceID,
		AssetID:      p.AssetID,
		TrackID:      trackID,
		StorageKey:   stringParam(p.Params, "storage_key", ""),
		MimeType:     stringParam(p.Params, "mime_type", ""),
		Model:        stringParam(p.Params, "model", ""),
		Prompt:       stringParam(p.Params, "prompt", ""),
		Lang:         stringValue(p.Lang, "en"),
		Continuation: p.WorkflowContinuation,
	})
	if err != nil {
		return "", fmt.Errorf("marshal ai_image_description payload: %w", err)
	}
	job, err := s.queue.Enqueue(ctx, p.WorkspaceID, queue.JobTypeAIImageDescriptionTextTrack, string(payload))
	if err != nil {
		return "", err
	}
	slog.DebugContext(ctx, "text track ai_image_description enqueued",
		"workspace_id", p.WorkspaceID, "asset_id", p.AssetID, "track_id", trackID, "job_id", job.ID)
	return job.ID, nil
}

func (s *textTrackService) enqueueExtract(
	ctx context.Context,
	span interface{ SetAttributes(...attribute.KeyValue) },
	p CreateTextTrackParams,
) (TextTrackDTO, error) {
	jobType, err := extractJobType(p.Source)
	if err != nil {
		return TextTrackDTO{}, err
	}
	payload, err := json.Marshal(struct {
		WorkspaceID string `json:"workspace_id"`
		AssetID     string `json:"asset_id"`
		StorageKey  string `json:"storage_key"`
		MimeType    string `json:"mime_type,omitempty"`
	}{
		WorkspaceID: p.WorkspaceID,
		AssetID:     p.AssetID,
		StorageKey:  stringParam(p.Params, "storage_key", ""),
		MimeType:    stringParam(p.Params, "mime_type", ""),
	})
	if err != nil {
		return TextTrackDTO{}, fmt.Errorf("marshal extract payload: %w", err)
	}
	job, err := s.queue.Enqueue(ctx, p.WorkspaceID, jobType, string(payload))
	if err != nil {
		return TextTrackDTO{}, err
	}
	span.SetAttributes(attribute.String("damask.job_id", job.ID))
	slog.DebugContext(ctx, "text track extract enqueued",
		"workspace_id", p.WorkspaceID,
		"asset_id", p.AssetID,
		"source", p.Source,
		"job_id", job.ID,
	)
	return TextTrackDTO{
		WorkspaceID: p.WorkspaceID,
		AssetID:     p.AssetID,
		Source:      p.Source,
		Status:      WorkflowRunStatusPending,
	}, nil
}

func (s *textTrackService) Delete(ctx context.Context, workspaceID, trackID string) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.text_tracks.delete",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track delete failed",
				"workspace_id",
				workspaceID,
				"track_id",
				trackID,
				"error",
				err,
			)
		}
	}()

	track, err := s.Get(ctx, workspaceID, trackID)
	if err != nil {
		return err
	}
	span.SetAttributes(
		attribute.String("damask.asset_id", track.AssetID),
		attribute.String("damask.text_track.source", track.Source),
		attribute.Bool("damask.text_track.has_file", track.StorageKey != nil),
	)
	if track.StorageKey != nil && s.storage != nil {
		if delErr := s.storage.Delete(ctx, *track.StorageKey); delErr != nil {
			slog.WarnContext(
				ctx,
				"text track storage delete failed",
				"track_id",
				trackID,
				"storage_key",
				*track.StorageKey,
				"error",
				delErr,
			)
		}
	}
	if ftsErr := s.repo.DeleteFTS(ctx, trackID); ftsErr != nil {
		return ftsErr
	}
	if delErr := s.repo.Delete(ctx, workspaceID, trackID); delErr != nil {
		return delErr
	}
	return nil
}

func toTextTrackDTO(row repository.TextTrack) TextTrackDTO {
	meta := map[string]any{}
	if row.Meta != nil && *row.Meta != "" {
		_ = json.Unmarshal([]byte(*row.Meta), &meta)
	}
	return TextTrackDTO{
		ID:             row.ID,
		WorkspaceID:    row.WorkspaceID,
		AssetID:        row.AssetID,
		AssetVersionID: row.AssetVersionID,
		Source:         row.Source,
		Lang:           row.Lang,
		Content:        row.Content,
		StorageKey:     row.StorageKey,
		ContentType:    row.ContentType,
		Meta:           meta,
		Status:         row.Status,
		Error:          row.Error,
		CreatedBy:      row.CreatedBy,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func readyTextContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return " "
	}
	return content
}

func stringValue(value *string, fallback ...string) string {
	if value != nil && *value != "" {
		return *value
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}

func stringParam(params map[string]any, key, fallback string) string {
	if params != nil {
		if value, ok := params[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return fallback
}

func (s *textTrackService) RunExtractPDF(
	ctx context.Context,
	workspaceID, assetID, trackID, storageKey string,
) (err error) {
	ctx, span := telemetry.StartBackgroundSpan(ctx, "service.text_tracks.extract_pdf",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "text track PDF extract job failed",
				"workspace_id", workspaceID,
				"asset_id", assetID,
				"track_id", trackID,
				"error", err,
			)
		}
	}()

	rc, err := s.storage.Get(ctx, storageKey)
	if err != nil {
		return fmt.Errorf("RunExtractPDF: read source: %w", err)
	}
	pdfBytes, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("RunExtractPDF: read bytes: %w", err)
	}

	text, err := transform.ExtractPDFText(ctx, pdfBytes)
	if err != nil {
		_ = s.repo.SetFailed(ctx, workspaceID, trackID, err.Error())
		return fmt.Errorf("RunExtractPDF: extract: %w", err)
	}

	return s.writeExtractedText(ctx, workspaceID, assetID, trackID, "pdf", text)
}

func (s *textTrackService) RunExtractPlain(
	ctx context.Context,
	workspaceID, assetID, trackID, storageKey string,
) (err error) {
	ctx, span := telemetry.StartBackgroundSpan(ctx, "service.text_tracks.extract_plain",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "text track plain extract job failed",
				"workspace_id", workspaceID,
				"asset_id", assetID,
				"track_id", trackID,
				"error", err,
			)
		}
	}()

	rc, err := s.storage.Get(ctx, storageKey)
	if err != nil {
		return fmt.Errorf("RunExtractPlain: read source: %w", err)
	}
	textBytes, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("RunExtractPlain: read bytes: %w", err)
	}

	return s.writeExtractedText(ctx, workspaceID, assetID, trackID, "plain", string(textBytes))
}

func (s *textTrackService) RunExtractDocument(
	ctx context.Context,
	workspaceID, assetID, trackID, storageKey, mimeType string,
) (err error) {
	ctx, span := telemetry.StartBackgroundSpan(ctx, "service.text_tracks.extract_document",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "text track document extract job failed",
				"workspace_id", workspaceID,
				"asset_id", assetID,
				"track_id", trackID,
				"error", err,
			)
		}
	}()

	rc, err := s.storage.Get(ctx, storageKey)
	if err != nil {
		return fmt.Errorf("RunExtractDocument: read source: %w", err)
	}
	docBytes, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("RunExtractDocument: read bytes: %w", err)
	}

	text, err := transform.ExtractDocumentText(ctx, docBytes, mimeType)
	if err != nil {
		_ = s.repo.SetFailed(ctx, workspaceID, trackID, err.Error())
		return fmt.Errorf("RunExtractDocument: extract: %w", err)
	}

	return s.writeExtractedText(ctx, workspaceID, assetID, trackID, assetTypeDocument, text)
}

func (s *textTrackService) writeExtractedText(
	ctx context.Context,
	workspaceID, assetID, trackID, source, text string,
) error {
	content := readyTextContent(text)
	wordCount := len(strings.Fields(text))
	metaBytes, _ := json.Marshal(map[string]any{"word_count": wordCount})
	meta := string(metaBytes)

	if err := s.repo.SetReady(ctx, workspaceID, trackID, repository.SetTextTrackReadyParams{
		Content:     content,
		StorageKey:  nil,
		ContentType: nil,
		Meta:        &meta,
	}); err != nil {
		return fmt.Errorf("writeExtractedText: mark ready: %w", err)
	}

	if err := s.repo.InsertFTS(ctx, repository.InsertTextFTSParams{
		TrackID:     trackID,
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		Source:      source,
		Lang:        "",
		Content:     content,
	}); err != nil {
		slog.WarnContext(ctx, "text track FTS insert failed", "track_id", trackID, "error", err)
	}
	return nil
}

func (s *textTrackService) RunOCR(
	ctx context.Context,
	workspaceID, assetID, trackID, _, storageKey, mimeType, lang, outputFormat string,
) (text string, wordCount int, err error) {
	ctx, span := telemetry.StartBackgroundSpan(ctx, "service.text_tracks.ocr",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.asset_mime_type", mimeType),
		attribute.String("damask.text_track_id", trackID),
		attribute.String("damask.text_track.lang", lang),
		attribute.String("damask.text_track.output_format", outputFormat),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track OCR job failed",
				"workspace_id", workspaceID,
				"asset_id", assetID,
				"track_id", trackID,
				"error", err,
			)
		}
	}()

	fail := func(formatErr error) (string, int, error) {
		_ = s.repo.SetFailed(ctx, workspaceID, trackID, formatErr.Error())
		return "", 0, formatErr
	}

	_, readSpan := telemetry.StartSpan(ctx, "service.text_tracks.ocr.read_source",
		attribute.String("damask.storage_key", storageKey),
	)
	rc, err := s.storage.Get(ctx, storageKey)
	telemetry.EndSpan(readSpan, err)
	if err != nil {
		return fail(fmt.Errorf("RunOCR: read source: %w", err))
	}
	imageBytes, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return fail(fmt.Errorf("RunOCR: read bytes: %w", err))
	}
	span.SetAttributes(attribute.Int("damask.source_bytes", len(imageBytes)))

	ocrCtx, ocrSpan := telemetry.StartSpan(ctx, "service.text_tracks.ocr.run")
	result, err := transform.RunOCR(ocrCtx, imageBytes, transform.OCRParams{
		Lang:         lang,
		OutputFormat: outputFormat,
	})
	telemetry.EndSpan(ocrSpan, err)
	if err != nil {
		return fail(fmt.Errorf("RunOCR: OCR: %w", err))
	}

	var resultStorageKey *string
	var contentType *string
	if outputFormat == "hocr" {
		key := fmt.Sprintf("%s/%s/text-tracks/%s%s", workspaceID, assetID, trackID, result.Extension)
		_, storeSpan := telemetry.StartSpan(ctx, "service.text_tracks.ocr.store_companion",
			attribute.String("damask.storage_key", key),
		)
		if err = s.storage.Put(ctx, key, bytes.NewReader(result.FileContent)); err != nil {
			telemetry.EndSpan(storeSpan, err)
			return fail(fmt.Errorf("RunOCR: store companion file: %w", err))
		}
		telemetry.EndSpan(storeSpan, nil)
		resultStorageKey = &key
		ct := result.ContentType
		contentType = &ct
	}

	plainText := readyTextContent(result.PlainText)
	wordCount = len(strings.Fields(result.PlainText))
	metaBytes, _ := json.Marshal(map[string]any{
		"lang":                   lang,
		"model":                  "tesseract",
		"output_format":          outputFormat,
		jobspec.MetaKeyWordCount: wordCount,
	})
	meta := string(metaBytes)

	writeCtx, writeSpan := telemetry.StartSpan(ctx, "service.text_tracks.ocr.mark_ready")
	if err = s.repo.SetReady(writeCtx, workspaceID, trackID, repository.SetTextTrackReadyParams{
		Content:     plainText,
		StorageKey:  resultStorageKey,
		ContentType: contentType,
		Meta:        &meta,
	}); err != nil {
		telemetry.EndSpan(writeSpan, err)
		return fail(fmt.Errorf("RunOCR: update track: %w", err))
	}
	telemetry.EndSpan(writeSpan, nil)

	ftsCtx, ftsSpan := telemetry.StartSpan(ctx, "service.text_tracks.ocr.index_fts")
	if err = s.repo.InsertFTS(ftsCtx, repository.InsertTextFTSParams{
		TrackID:     trackID,
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		Source:      textTrackSourceOCR,
		Lang:        lang,
		Content:     plainText,
	}); err != nil {
		telemetry.EndSpan(ftsSpan, err)
		slog.WarnContext(ctx, "text track FTS insert failed", "track_id", trackID, "error", err)
	} else {
		telemetry.EndSpan(ftsSpan, nil)
	}
	span.SetAttributes(
		attribute.Int("damask.text_track.word_count", wordCount),
		attribute.Bool("damask.text_track.has_file", resultStorageKey != nil),
	)
	slog.DebugContext(
		ctx,
		"text track OCR completed",
		"workspace_id", workspaceID,
		"asset_id", assetID,
		"track_id", trackID,
		"word_count", wordCount,
		"output_format", outputFormat,
	)

	return plainText, wordCount, nil
}
