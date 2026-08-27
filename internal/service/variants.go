package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	"damask/server/internal/systemtags"
	apptelemetry "damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

var (
	ErrInvalidVariantType = errors.New("invalid variant type")
	ErrInvalidVariantReq  = errors.New("invalid variant request")
)

const (
	maxVariantTitleLength = 255
	defaultAudioBitrate   = "192k"
	variantStatusReady    = "ready"
)

type invalidVariantInputError string

func (e invalidVariantInputError) Error() string { return string(e) }

func (e invalidVariantInputError) Unwrap() error { return apperr.ErrInvalidInput }

type invalidVariantRequestError string

func (e invalidVariantRequestError) Error() string { return string(e) }

func (e invalidVariantRequestError) Unwrap() error { return ErrInvalidVariantReq }

// VariantDTO is the output of VariantService methods.
type VariantDTO struct {
	ID                   string
	WorkspaceID          string
	AssetVersionID       string
	Type                 string
	StorageKey           string
	TransformParams      *string
	Size                 *int64
	Status               string
	ThumbnailKey         *string
	ThumbnailContentType string
	Title                string
	IsShared             bool
	CreatedAt            time.Time
}

type UpdateVariantsSharingParams struct {
	WorkspaceID string
	AssetID     string
	Updates     map[string]bool
}

type SharedVariantDTO struct {
	VariantDTO

	AssetID string
}

type variantService struct {
	variants   repository.VariantRepository
	assets     repository.AssetRepository
	workflows  repository.WorkflowRepository
	tags       TagService
	audit      audit.Writer
	actions    VariantActionsStore
	queue      queue.JobQueue
	storage    storage.Storage
	invalidate StorageInvalidator
}

// NewVariantService returns a VariantService.
func NewVariantService(
	variants repository.VariantRepository,
	assets repository.AssetRepository,
	tags TagService,
	aw audit.Writer,
) VariantService {
	return &variantService{variants: variants, assets: assets, tags: tags, audit: aw}
}

type VariantServiceDeps struct {
	Actions    VariantActionsStore
	Queue      queue.JobQueue
	Storage    storage.Storage
	Workflows  repository.WorkflowRepository
	Invalidate StorageInvalidator
}

// NewVariantServiceWithDeps returns a VariantService with the extra dependencies
// required by advanced variant actions such as promote and rerun.
func NewVariantServiceWithDeps(
	variants repository.VariantRepository,
	assets repository.AssetRepository,
	tags TagService,
	aw audit.Writer,
	deps VariantServiceDeps,
) VariantService {
	return &variantService{
		variants:   variants,
		assets:     assets,
		workflows:  deps.Workflows,
		tags:       tags,
		audit:      aw,
		actions:    deps.Actions,
		queue:      deps.Queue,
		storage:    deps.Storage,
		invalidate: deps.Invalidate,
	}
}

func (s *variantService) List(ctx context.Context, p ListVariantsParams) (*ListVariantsResult, error) {
	rows, err := s.variants.ListByAsset(ctx, p.WorkspaceID, p.AssetID)
	if err != nil {
		return nil, err
	}
	out := make([]*VariantDTO, len(rows))
	for i, r := range rows {
		out[i] = toVariantDTO(r, i+1)
	}
	result := &ListVariantsResult{Variants: out}
	if s.workflows != nil {
		wf, wfErr := findCoveringWorkflowDTO(
			ctx,
			s.workflows,
			p.WorkspaceID,
			p.AssetID,
			p.AssetProjectID,
			p.AssetFolderID,
		)
		if wfErr != nil && !errors.Is(wfErr, apperr.ErrNotFound) {
			return nil, wfErr
		}
		result.CoveringWorkflow = wf
	}
	return result, nil
}

func (s *variantService) Get(ctx context.Context, workspaceID, id string) (*VariantDTO, error) {
	v, err := s.variants.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return toVariantDTO(v, 1), nil
}

func AutoTitle(variantType string, position int) string {
	return fmt.Sprintf("%s #%d", variantType, position)
}

func ResolvedTitle(v repository.Variant, position int) string {
	if v.Title != nil && *v.Title != "" {
		return *v.Title
	}
	return AutoTitle(v.Type, position)
}

func (s *variantService) PrepareCreate(
	ctx context.Context,
	p PrepareCreateVariantParams,
) (PreparedCreateVariant, error) {
	params := json.RawMessage("{}")
	if len(p.Params) > 0 {
		params = p.Params
	}

	if !validVariantType(p.Type) {
		return PreparedCreateVariant{}, ErrInvalidVariantType
	}

	switch {
	case requiresVideoAsset(p.Type) && !strings.HasPrefix(p.AssetMimeType, "video/"):
		if p.Type == queue.JobTypeExtractAudio {
			return PreparedCreateVariant{}, invalidVariantInputError("asset_not_video")
		}
		return PreparedCreateVariant{}, invalidVariantRequestError("video transforms require a video asset")
	case requiresImageAsset(p.Type) && !strings.HasPrefix(p.AssetMimeType, "image/"):
		return PreparedCreateVariant{}, invalidVariantRequestError("image transforms require an image asset")
	case requiresAudioAsset(p.Type) && !strings.HasPrefix(p.AssetMimeType, "audio/"):
		return PreparedCreateVariant{}, invalidVariantInputError("asset_not_audio")
	}

	if (p.Type == queue.JobTypeImageBgRemove || p.Type == queue.JobTypeImageWithPrompt) && !p.ImageRouterConfigured {
		return PreparedCreateVariant{}, invalidVariantInputError("imagerouter_not_configured")
	}

	meta := func(typ string, prm json.RawMessage) PreparedCreateVariant {
		return PreparedCreateVariant{Type: typ, Params: prm, Title: p.Title, IsShared: p.IsShared}
	}

	if handler, ok := variantParamPreparers[p.Type]; ok {
		normalized, err := handler(s, ctx, p, params)
		if err != nil {
			return PreparedCreateVariant{}, err
		}
		return meta(p.Type, normalized), nil
	}

	normalized, err := prepareAudioVariantParams(p.Type, p.AssetMimeType, params)
	if err != nil {
		return PreparedCreateVariant{}, err
	}

	return meta(p.Type, normalized), nil
}

// variantParamPreparer normalizes and validates the raw request params for
// one variant type, filling in workspace-level defaults (e.g. default model).
type variantParamPreparer func(
	s *variantService,
	ctx context.Context,
	p PrepareCreateVariantParams,
	raw json.RawMessage,
) (json.RawMessage, error)

// variantParamPreparers maps variant types with dedicated param handling to
// their preparer. Types not present here (e.g. audio transforms) fall back
// to prepareAudioVariantParams in PrepareCreate.
var variantParamPreparers = map[string]variantParamPreparer{
	queue.JobTypeImageWatermark: (*variantService).prepareImageWatermarkParams,
	queue.JobTypeVideoWatermark: (*variantService).prepareVideoWatermarkParams,
	queue.JobTypeImageBgRemove: func(_ *variantService, _ context.Context, p PrepareCreateVariantParams, raw json.RawMessage) (json.RawMessage, error) {
		return prepareImageRouterBgRemoveParams(raw, p.DefaultBgRemoveModel)
	},
	queue.JobTypeImageWithPrompt: func(_ *variantService, _ context.Context, p PrepareCreateVariantParams, raw json.RawMessage) (json.RawMessage, error) {
		return prepareImageRouterPromptParams(raw, p.DefaultImageModel)
	},
	queue.JobTypeCustomFFmpeg: func(_ *variantService, _ context.Context, _ PrepareCreateVariantParams, raw json.RawMessage) (json.RawMessage, error) {
		return prepareCustomFFmpegParams(raw)
	},
}

// prepareCustomFFmpegParams validates the user-supplied command and returns
// canonical params JSON. Validation errors are mapped to short codes the
// frontend can map to translated messages.
func prepareCustomFFmpegParams(raw json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, invalidVariantInputError("invalid command params")
	}
	p.Command = strings.TrimSpace(p.Command)
	if p.Command == "" {
		return nil, invalidVariantInputError("command_required")
	}
	if err := transform.ValidateCustomCommand(p.Command); err != nil {
		return nil, customFFmpegValidationError(err)
	}
	return marshalRaw(p), nil
}

// customFFmpegValidationError maps a transform.ValidateCustomCommand error to
// a short, stable code suitable for frontend i18n lookups.
func customFFmpegValidationError(err error) error {
	switch {
	case errors.Is(err, transform.ErrCustomFFmpegNoInputToken):
		return invalidVariantInputError("missing_input_token")
	case errors.Is(err, transform.ErrCustomFFmpegNoOutputToken):
		return invalidVariantInputError("missing_output_token")
	case errors.Is(err, transform.ErrCustomFFmpegBlacklisted):
		return invalidVariantInputError("command_blacklisted")
	default:
		return invalidVariantInputError(err.Error())
	}
}

func (s *variantService) prepareImageWatermarkParams(
	ctx context.Context,
	p PrepareCreateVariantParams,
	raw json.RawMessage,
) (json.RawMessage, error) {
	var params transform.WatermarkParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, invalidVariantInputError("invalid watermark params")
	}
	params.WatermarkAssetID = ""
	params.Normalize()

	wm, err := s.resolveSystemTagAsset(ctx, p.WorkspaceID, p.AssetID, systemtags.Watermark)
	if err != nil {
		return nil, err
	}
	params.WatermarkAssetID = wm.ID
	return marshalRaw(params), nil
}

func (s *variantService) prepareVideoWatermarkParams(
	ctx context.Context,
	p PrepareCreateVariantParams,
	raw json.RawMessage,
) (json.RawMessage, error) {
	var params transform.VideoWatermarkParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, invalidVariantInputError("invalid watermark params")
	}
	params.WatermarkAssetID = ""
	params.Normalize()

	wm, err := s.resolveSystemTagAsset(ctx, p.WorkspaceID, p.AssetID, systemtags.Watermark)
	if err != nil {
		return nil, err
	}
	params.WatermarkAssetID = wm.ID
	return marshalRaw(params), nil
}

func (s *variantService) resolveSystemTagAsset(
	ctx context.Context,
	workspaceID, assetID, tagName string,
) (*AssetDTO, error) {
	if s.tags == nil {
		return nil, errors.New("tag service unavailable")
	}

	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}

	resolved, err := s.tags.ResolveSystemTag(ctx, workspaceID, tagName, SystemTagScope{
		FolderID:  asset.FolderID,
		ProjectID: asset.ProjectID,
	})
	if errors.Is(err, apperr.ErrNotFound) || resolved == nil {
		return nil, invalidVariantInputError("no_watermark_asset")
	}
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func (s *variantService) Create(ctx context.Context, p CreateVariantParams) (dto *VariantDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.variants.create",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.String("damask.asset_id", p.AssetID),
		attribute.String("damask.version_id", p.AssetVersionID),
		attribute.String("damask.variant.type", p.Type),
	)
	defer func() {
		if dto != nil {
			span.SetAttributes(attribute.String("damask.variant_id", dto.ID))
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"variant create failed",
				"workspace_id",
				p.WorkspaceID,
				"asset_id",
				p.AssetID,
				"type",
				p.Type,
				"error",
				err,
			)
		}
	}()

	v, err := s.variants.Create(ctx, repository.Variant{
		ID:              p.ID,
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.AssetVersionID,
		Type:            p.Type,
		StorageKey:      p.StorageKey,
		TransformParams: p.TransformParams,
		Size:            p.Size,
	})
	if err != nil {
		return nil, err
	}
	dto = toVariantDTO(v, 1)
	// Only emit audit for manual uploads (job-queued variants are audited via WriteVariantQueued).
	if p.AssetID != "" {
		actor := auth.ActorFromCtx(ctx)
		s.audit.WriteAsset(ctx, audit.AssetEvent{
			WorkspaceID: p.WorkspaceID,
			AssetID:     p.AssetID,
			UserID:      actor.UserID,
			ActorType:   actor.Type,
			EventType:   audit.EventAssetVariantCreated,
			Payload:     audit.AssetVariantCreatedPayload{V: 1, Type: dto.Type},
		})
	}
	if s.invalidate != nil {
		s.invalidate.Invalidate(p.WorkspaceID)
	}
	return dto, nil
}

// CommitDraft persists a pre-generated scratch file as a permanent variant row.
func (s *variantService) CommitDraft(ctx context.Context, p CommitDraftParams) (dto *VariantDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.variants.commit_draft",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.String("damask.asset_id", p.AssetID),
		attribute.String("damask.variant.type", p.VariantType),
	)
	defer apptelemetry.EndSpan(span, err)

	v, err := s.variants.Create(ctx, repository.Variant{
		ID:              uuid.NewString(),
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.AssetVersionID,
		Type:            p.VariantType,
		StorageKey:      p.StorageKey,
		TransformParams: p.TransformParams,
		Status:          variantStatusReady,
		Title:           p.Title,
	})
	if err != nil {
		return nil, err
	}
	dto = toVariantDTO(v, 1)
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: p.WorkspaceID,
		AssetID:     p.AssetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVariantCreated,
		Payload:     audit.AssetVariantCreatedPayload{V: 1, Type: dto.Type},
	})
	if s.invalidate != nil {
		s.invalidate.Invalidate(p.WorkspaceID)
	}
	return dto, nil
}

func (s *variantService) UpdateTitle(ctx context.Context, workspaceID, variantID, title string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.variants.update_title",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.variant_id", variantID),
	)
	defer apptelemetry.EndSpan(span, err)

	trimmed := strings.TrimSpace(title)
	if len(trimmed) > maxVariantTitleLength {
		return apperr.ErrInvalidInput
	}
	var value *string
	if trimmed != "" {
		value = &trimmed
	}
	return s.variants.UpdateTitle(ctx, workspaceID, variantID, value)
}

func (s *variantService) UpdateSharing(ctx context.Context, p UpdateVariantsSharingParams) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.variants.update_sharing",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.String("damask.asset_id", p.AssetID),
	)
	defer apptelemetry.EndSpan(span, err)

	if len(p.Updates) == 0 {
		return apperr.ErrInvalidInput
	}

	rows, err := s.variants.ListByAsset(ctx, p.WorkspaceID, p.AssetID)
	if err != nil {
		return err
	}
	valid := make(map[string]struct{}, len(rows))
	for _, v := range rows {
		valid[v.ID] = struct{}{}
	}

	toShare := make([]string, 0)
	toUnshare := make([]string, 0)
	for id, isShared := range p.Updates {
		if _, ok := valid[id]; !ok {
			return apperr.ErrNotFound
		}
		if isShared {
			toShare = append(toShare, id)
		} else {
			toUnshare = append(toUnshare, id)
		}
	}

	if len(toShare) > 0 {
		if shareErr := s.variants.UpdateSharedBatch(ctx, p.WorkspaceID, toShare, true); shareErr != nil {
			return shareErr
		}
	}
	if len(toUnshare) > 0 {
		if unshareErr := s.variants.UpdateSharedBatch(ctx, p.WorkspaceID, toUnshare, false); unshareErr != nil {
			return unshareErr
		}
	}
	return nil
}

func (s *variantService) ListSharedByAssets(ctx context.Context, assetIDs []string) ([]SharedVariantDTO, error) {
	rows, err := s.variants.ListSharedByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, err
	}
	out := make([]SharedVariantDTO, len(rows))
	currentAssetID := ""
	position := 0
	for i, row := range rows {
		if row.AssetID != currentAssetID {
			currentAssetID = row.AssetID
			position = 1
		} else {
			position++
		}
		out[i] = SharedVariantDTO{
			VariantDTO: *toVariantDTO(row.Variant, position),
			AssetID:    row.AssetID,
		}
	}
	return out, nil
}

func (s *variantService) GetSharedForShare(ctx context.Context, variantID, assetID string) (*VariantDTO, error) {
	v, err := s.variants.GetSharedByVariantAndAsset(ctx, variantID, assetID)
	if err != nil {
		return nil, err
	}
	return toVariantDTO(v, 1), nil
}

func validVariantType(variantType string) bool {
	switch variantType {
	case queue.JobTypeImageResize,
		queue.JobTypeImageWatermark,
		queue.JobTypeImageConvert,
		queue.JobTypeImageCrop,
		queue.JobTypeVideoCaptureImage,
		queue.JobTypeVideoTranscode,
		queue.JobTypeVideoWatermark,
		queue.JobTypeImageBgRemove,
		queue.JobTypeImageWithPrompt,
		queue.JobTypeImageSmartCrop,
		queue.JobTypeExtractAudio,
		queue.JobTypeTranscodeAudio,
		queue.JobTypeNormalizeAudio,
		queue.JobTypeCustomFFmpeg:
		return true
	default:
		return false
	}
}

func requiresVideoAsset(variantType string) bool {
	return variantType == queue.JobTypeVideoCaptureImage ||
		variantType == queue.JobTypeVideoTranscode ||
		variantType == queue.JobTypeVideoWatermark ||
		variantType == queue.JobTypeExtractAudio
}

func requiresImageAsset(variantType string) bool {
	return variantType == queue.JobTypeImageResize ||
		variantType == queue.JobTypeImageConvert ||
		variantType == queue.JobTypeImageCrop ||
		variantType == queue.JobTypeImageWatermark ||
		variantType == queue.JobTypeImageSmartCrop ||
		variantType == queue.JobTypeImageBgRemove ||
		variantType == queue.JobTypeImageWithPrompt
}

func requiresAudioAsset(variantType string) bool {
	return variantType == queue.JobTypeTranscodeAudio ||
		variantType == queue.JobTypeNormalizeAudio
}

//nolint:cyclop // per-audio-variant-type defaults and validation, not decomposable without losing clarity
func prepareAudioVariantParams(variantType, mimeType string, raw json.RawMessage) (json.RawMessage, error) {
	var p transform.AudioParams
	switch variantType {
	case queue.JobTypeExtractAudio:
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, invalidVariantInputError("invalid audio params")
		}
		if p.OutputFormat == "" {
			p.OutputFormat = "aac"
		}
		if p.Bitrate == "" {
			p.Bitrate = defaultAudioBitrate
		}
		if err := validateAudioBitrateAndFormat(p, "aac", "mp3", "opus", "flac"); err != nil {
			return nil, err
		}
	case queue.JobTypeTranscodeAudio:
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, invalidVariantInputError("invalid audio params")
		}
		if p.OutputFormat == "" {
			return nil, invalidVariantInputError("format is required")
		}
		if p.Bitrate == "" {
			p.Bitrate = defaultAudioBitrate
		}
		if err := validateAudioBitrateAndFormat(p, "mp3", "aac", "opus", "ogg", "flac", "wav"); err != nil {
			return nil, err
		}
	case queue.JobTypeNormalizeAudio:
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, invalidVariantInputError("invalid audio params")
		}
		if p.OutputFormat == "" {
			p.OutputFormat = "source"
		}
		if p.OutputFormat == "source" {
			p.OutputFormat = transform.AudioFormatFromMimeType(mimeType)
		}
		if p.TargetLUFS == 0 {
			p.TargetLUFS = transform.DefaultLUFS
		}
		if p.TargetLUFS < transform.MinLUFS || p.TargetLUFS > transform.MaxLUFS {
			return nil, invalidVariantInputError("target_lufs must be between -70 and 0")
		}
		if err := validateAudioBitrateAndFormat(p, "mp3", "aac", "wav", "ogg", "flac"); err != nil {
			return nil, err
		}
	default:
		return raw, nil
	}
	return marshalRaw(p), nil
}

func validateAudioBitrateAndFormat(p transform.AudioParams, allowedFormats ...string) error {
	if p.Bitrate != "" && !isAllowedAudioBitrate(p.Bitrate) {
		return invalidVariantInputError("unsupported audio bitrate")
	}
	if !isAllowedAudioFormat(p.OutputFormat, allowedFormats...) {
		return invalidVariantInputError("unsupported audio format")
	}
	return nil
}

func marshalRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func prepareImageRouterBgRemoveParams(raw json.RawMessage, defaultModel string) (json.RawMessage, error) {
	var params struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, invalidVariantInputError("invalid image background removal params")
	}
	params.Model = strings.TrimSpace(params.Model)
	params.Prompt = strings.TrimSpace(params.Prompt)
	if params.Model == "" {
		params.Model = defaultModel
	}
	if params.Model == "" {
		return nil, invalidVariantInputError("model is required")
	}
	return marshalRaw(params), nil
}

func prepareImageRouterPromptParams(raw json.RawMessage, defaultModel string) (json.RawMessage, error) {
	var params struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, invalidVariantInputError("invalid image prompt params")
	}
	params.Prompt = strings.TrimSpace(params.Prompt)
	params.Model = strings.TrimSpace(params.Model)
	if _, content := transform.StripLeadingDescription(params.Prompt); content == "" {
		return nil, invalidVariantRequestError("prompt_required")
	}
	if params.Model == "" {
		params.Model = defaultModel
	}
	if params.Model == "" {
		return nil, invalidVariantInputError("model is required")
	}
	return marshalRaw(params), nil
}

func isAllowedAudioFormat(format string, allowed ...string) bool {
	for _, a := range allowed {
		if strings.EqualFold(format, a) {
			return true
		}
	}
	return false
}

func isAllowedAudioBitrate(bitrate string) bool {
	switch bitrate {
	case "64k", "96k", "128k", defaultAudioBitrate, "256k", "320k":
		return true
	default:
		return false
	}
}

// WriteVariantQueued emits asset_variant_created for job-queued variants (before the job runs).
func (s *variantService) WriteVariantQueued(ctx context.Context, workspaceID, assetID, variantType string) {
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVariantCreated,
		Payload:     audit.AssetVariantCreatedPayload{V: 1, Type: variantType},
	})
}

// WriteVariantDownloadedAsync emits asset_variant_downloaded in a background goroutine.
func (s *variantService) WriteVariantDownloadedAsync(
	workspaceID, assetID, variantID, variantType, shareID, visitorName string,
) {
	var payloadShareID *string
	if shareID != "" {
		payloadShareID = &shareID
	}
	var payloadVisitorName *string
	if visitorName != "" {
		payloadVisitorName = &visitorName
	}
	actorType := audit.ActorTypeUser
	if shareID != "" {
		actorType = audit.ActorTypeSystem
	}
	s.audit.WriteAssetAsync(audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		ActorType:   actorType,
		EventType:   audit.EventAssetVariantDownloaded,
		Payload: audit.AssetVariantDownloadedPayload{
			V:           1,
			VariantID:   variantID,
			Type:        variantType,
			ShareID:     payloadShareID,
			VisitorName: payloadVisitorName,
		},
	})
}

// Delete deletes a variant. Only variants attached to the asset's current version may be deleted.
func (s *variantService) Delete(ctx context.Context, workspaceID, assetID, variantID string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.variants.delete",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.variant_id", variantID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"variant delete failed",
				"workspace_id",
				workspaceID,
				"asset_id",
				assetID,
				"variant_id",
				variantID,
				"error",
				err,
			)
		}
	}()

	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	v, err := s.variants.GetByID(ctx, workspaceID, variantID)
	if err != nil {
		return err
	}
	if asset.CurrentVersionID == nil || v.AssetVersionID != *asset.CurrentVersionID {
		return fmt.Errorf("variant belongs to a previous version: %w", apperr.ErrInvalidInput)
	}
	if delErr := s.variants.Delete(ctx, workspaceID, variantID); delErr != nil {
		return delErr
	}
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVariantDeleted,
		Payload:     audit.AssetVariantDeletedPayload{V: 1, VariantID: variantID, Type: v.Type},
	})
	if s.invalidate != nil {
		s.invalidate.Invalidate(workspaceID)
	}
	return nil
}

const (
	paramHistoryLimit = 10
	// paramHistoryFetchLimit over-fetches raw rows before deduplication: several variant
	// types store transform_params with whatever key order the client sent (see
	// internal/jobs/variant_common.go canonicalJSON, nil for 6 of 11 types), so the same
	// logical params can appear multiple times with different raw JSON strings.
	paramHistoryFetchLimit = 100
)

// GetParamHistory returns distinct previous transform_params for the given workspace and
// variant type, ordered most-recent first, capped at paramHistoryLimit.
func (s *variantService) GetParamHistory(
	ctx context.Context,
	workspaceID, variantType string,
) ([]ParamHistoryEntry, error) {
	raw, err := s.variants.ListVariantParamHistory(ctx, workspaceID, variantType, paramHistoryFetchLimit)
	if err != nil {
		return nil, fmt.Errorf("param history: %w", err)
	}

	seen := make(map[string]struct{}, len(raw))
	entries := make([]ParamHistoryEntry, 0, paramHistoryLimit)
	for _, jsonStr := range raw {
		var params map[string]any
		if unmarshalErr := json.Unmarshal([]byte(jsonStr), &params); unmarshalErr != nil {
			continue // skip malformed rows — don't fail the whole request
		}
		// encoding/json marshals map keys in sorted order, so this re-marshal gives a
		// canonical form regardless of how the original row's keys were ordered.
		canonical, marshalErr := json.Marshal(params)
		if marshalErr != nil {
			continue
		}
		if _, dup := seen[string(canonical)]; dup {
			continue
		}
		seen[string(canonical)] = struct{}{}
		entries = append(entries, ParamHistoryEntry{Params: params})
		if len(entries) == paramHistoryLimit {
			break
		}
	}
	return entries, nil
}

func toVariantDTO(v repository.Variant, position int) *VariantDTO {
	status := v.Status
	if status == "" {
		status = variantStatusReady
	}
	return &VariantDTO{
		ID:                   v.ID,
		WorkspaceID:          v.WorkspaceID,
		AssetVersionID:       v.AssetVersionID,
		Type:                 v.Type,
		StorageKey:           v.StorageKey,
		TransformParams:      v.TransformParams,
		Size:                 v.Size,
		Status:               status,
		ThumbnailKey:         v.ThumbnailKey,
		ThumbnailContentType: v.ThumbnailContentType,
		Title:                ResolvedTitle(v, position),
		IsShared:             v.IsShared,
		CreatedAt:            v.CreatedAt,
	}
}
