package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/ai"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/events"
	"damask/server/internal/jobs"
	"damask/server/internal/jobspec"
	"damask/server/internal/queue"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/systemtags"
	apptelemetry "damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

// ---- Response types ----

type VariantResponse struct {
	ID                   string    `json:"id"`
	AssetVersionID       string    `json:"asset_version_id"`
	Type                 string    `json:"type"`
	TransformParams      *string   `json:"transform_params,omitempty"`
	Size                 *int64    `json:"size,omitempty"`
	Status               string    `json:"status"`
	StorageKey           string    `json:"storage_key"`
	DownloadURL          string    `json:"download_url"`
	ThumbnailURL         *string   `json:"thumbnail_url,omitempty"`
	ThumbnailContentType string    `json:"thumbnail_content_type"`
	Title                string    `json:"title"`
	IsShared             bool      `json:"is_shared"`
	CreatedAt            time.Time `json:"created_at"`
}

type SharedVariantResponse struct {
	ID                   string  `json:"id"`
	Title                string  `json:"title"`
	Type                 string  `json:"type"`
	MimeType             string  `json:"mime_type"`
	Size                 *int64  `json:"size,omitempty"`
	ThumbnailURL         *string `json:"thumbnail_url,omitempty"`
	ThumbnailContentType string  `json:"thumbnail_content_type"`
}

type ListVariantsResponse struct {
	Variants         []VariantResponse            `json:"variants"`
	Rebuilding       bool                         `json:"rebuilding"`
	CoveringWorkflow *service.CoveringWorkflowDTO `json:"covering_workflow,omitempty"`
}

type CreateVariantResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type WatermarkAssetResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	StorageKey   string  `json:"storage_key"`
	MimeType     string  `json:"mime_type"`
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
	Scope        string  `json:"scope"`
}

type PromoteVariantResponse struct {
	AssetID  string `json:"asset_id"`
	AssetURL string `json:"asset_url"`
}

type SetVariantThumbnailResponse struct {
	ThumbnailURL string `json:"thumbnail_url"`
}

type RerunVariantResponse struct {
	VariantID string `json:"variant_id"`
	Status    string `json:"status"`
}

// ParamHistoryEntryResponse is one distinct previous transform_params set.
type ParamHistoryEntryResponse struct {
	Params map[string]any `json:"params"`
}

// VariantParamHistoryResponse is the response for GET /api/v1/variant-param-history.
type VariantParamHistoryResponse struct {
	Entries []ParamHistoryEntryResponse `json:"entries"`
}

// paramHistoryAllowedTypes lists the variant types with user-authored settings worth
// restoring. image_bg_remove and video_capture_image are intentionally excluded — their
// params carry no creative choice worth surfacing as history.
var paramHistoryAllowedTypes = map[string]bool{
	queue.JobTypeImageWithPrompt: true,
	queue.JobTypeImageWatermark:  true,
	queue.JobTypeVideoWatermark:  true,
	queue.JobTypeImageResize:     true,
	queue.JobTypeImageConvert:    true,
	queue.JobTypeImageSmartCrop:  true,
	queue.JobTypeImageCrop:       true,
	queue.JobTypeVideoTranscode:  true,
	queue.JobTypeTranscodeAudio:  true,
	queue.JobTypeNormalizeAudio:  true,
	queue.JobTypeCustomFFmpeg:    true,
}

type automateVariantsRequest struct {
	Scope string `json:"scope"`
}

// AutomateVariantsResponse is returned when a workflow is created from asset variants.
type AutomateVariantsResponse struct {
	WorkflowID  string `json:"workflow_id"`
	WorkflowURL string `json:"workflow_url"`
}

func variantDTOToResponse(assetID string, v *service.VariantDTO) VariantResponse {
	var thumbURL *string
	if v.ThumbnailKey != nil {
		u := variantThumbURL(assetID, v.ID)
		thumbURL = &u
	}
	ct := v.ThumbnailContentType
	if ct == "" {
		ct = contentTypeImageJPEG
	}
	status := v.Status
	if status == "" {
		status = "ready"
	}
	return VariantResponse{
		ID:                   v.ID,
		AssetVersionID:       v.AssetVersionID,
		Type:                 v.Type,
		TransformParams:      v.TransformParams,
		Size:                 v.Size,
		Status:               status,
		StorageKey:           v.StorageKey,
		DownloadURL:          variantFileURL(assetID, v.ID),
		ThumbnailURL:         thumbURL,
		ThumbnailContentType: ct,
		Title:                v.Title,
		IsShared:             v.IsShared,
		CreatedAt:            v.CreatedAt,
	}
}

func sharedVariantDTOToResponse(shareID, assetID string, v service.SharedVariantDTO) SharedVariantResponse {
	var thumbURL *string
	if v.ThumbnailKey != nil {
		u := sharedVariantThumbURL(shareID, assetID, v.ID)
		thumbURL = &u
	}
	ct := v.ThumbnailContentType
	if ct == "" {
		ct = contentTypeImageJPEG
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(v.StorageKey)))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return SharedVariantResponse{
		ID:                   v.ID,
		Title:                v.Title,
		Type:                 v.Type,
		MimeType:             mimeType,
		Size:                 v.Size,
		ThumbnailURL:         thumbURL,
		ThumbnailContentType: ct,
	}
}

func systemTagAssetToWatermarkResponse(v *service.AssetDTO, scope string) WatermarkAssetResponse {
	u := assetThumbURL(v.ID)
	return WatermarkAssetResponse{
		ID:           v.ID,
		Name:         v.OriginalFilename,
		StorageKey:   v.StorageKey,
		MimeType:     v.MimeType,
		ThumbnailURL: &u,
		Scope:        scope,
	}
}

// isRebuildingVariants returns true when a rebuild_variants job for the given
// version is in pending or processing state.
func (s *Server) isRebuildingVariants(c fiber.Ctx, versionID string) bool {
	rebuilding, err := s.assets.IsRebuildingVariants(c.Context(), versionID)
	if err != nil {
		slog.ErrorContext(c, "is_rebuilding_variants", apiErrorKey, err)
	}
	return rebuilding
}

// ---- Handlers ----

// handleListVariants returns all variants for the asset's current version.
//
// @Summary List asset variants
// @Description Returns all generated variants for the asset's current version, plus a <code>rebuilding</code> flag that is true when a variant rebuild job is in progress (e.g. after a new version upload). Each variant includes a <code>download_url</code> for direct file access.
// @Tags Variants
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} ListVariantsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/variants [get]
// handleListVariants handles GET /api/v1/assets/:id/variants
// Returns variants for the current version only, plus a rebuilding flag.
func (s *Server) handleListVariants(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	projectID := ""
	if asset.ProjectID != nil {
		projectID = *asset.ProjectID
	}
	folderID := ""
	if asset.FolderID != nil {
		folderID = *asset.FolderID
	}
	result, err := s.variants.List(c.Context(), service.ListVariantsParams{
		WorkspaceID:    claims.WorkspaceID,
		AssetID:        assetID,
		AssetProjectID: projectID,
		AssetFolderID:  folderID,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	out := make([]VariantResponse, len(result.Variants))
	for i, v := range result.Variants {
		out[i] = variantDTOToResponse(assetID, v)
	}

	rebuilding := false
	if asset.CurrentVersionID != nil {
		rebuilding = s.isRebuildingVariants(c, *asset.CurrentVersionID)
	}

	resp := ListVariantsResponse{
		Variants:   out,
		Rebuilding: rebuilding,
	}
	if result.CoveringWorkflow != nil {
		wf := *result.CoveringWorkflow
		wf.WorkflowURL = "/library/settings/workflows?workflow=" + wf.ID
		resp.CoveringWorkflow = &wf
	}
	return c.JSON(resp)
}

// handleAutomateVariants handles POST /api/v1/assets/:id/variants/automate
//
// @Summary Automate variant creation
// @Tags Variants
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body automateVariantsRequest true "Automation scope"
// @Success 201 {object} AutomateVariantsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/variants/automate [post].
func (s *Server) handleAutomateVariants(c fiber.Ctx) error {
	var req automateVariantsRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{apiErrorKey: "invalid request body"})
	}
	claims := auth.GetClaims(c)
	dto, err := s.workflows.CreateFromVariants(c.Context(), claims.WorkspaceID, service.CreateVariantAutomationParams{
		AssetID:   c.Params("id"),
		CreatedBy: claims.UserID,
		Scope:     service.AutomationScope(req.Scope),
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(AutomateVariantsResponse{
		WorkflowID:  dto.ID,
		WorkflowURL: "/library/settings/workflows?workflow=" + dto.ID,
	})
}

// handleResolveWatermarkAsset handles GET /api/v1/assets/:id/variants/watermark
//
// @Summary Resolve watermark asset for an asset
// @Tags Variants
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} WatermarkAssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ErrorResponse "No watermark asset configured"
// @Router /api/v1/assets/{id}/variants/watermark [get].
func (s *Server) handleResolveWatermarkAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	wm, err := s.tags.ResolveSystemTag(c.Context(), claims.WorkspaceID, systemtags.Watermark, service.SystemTagScope{
		FolderID:  asset.FolderID,
		ProjectID: asset.ProjectID,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if wm == nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "no_watermark_asset")
	}
	scope := "workspace"
	if asset.FolderID != nil && wm.FolderID != nil && *asset.FolderID == *wm.FolderID {
		scope = "folder"
	} else if asset.ProjectID != nil && wm.ProjectID != nil && *asset.ProjectID == *wm.ProjectID {
		scope = "project"
	}
	return c.JSON(systemTagAssetToWatermarkResponse(wm, scope))
}

// ValidateCommandResponse is the response for GET /api/v1/variants/validate-command.
type ValidateCommandResponse struct {
	Valid  bool   `json:"valid"`
	Error  string `json:"error,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// handleValidateCustomFFmpegCommand handles GET /api/v1/variants/validate-command.
//
// @Summary Validate a custom ffmpeg command
// @Description Synchronously validates a custom_ffmpeg command (length, {input}/{output} tokens, blacklisted patterns) without enqueueing a job or touching storage. Always returns 200 — the <code>valid</code> field carries the result.
// @Tags Variants
// @Produce json
// @Security BearerAuth
// @Param q query string true "Command to validate"
// @Success 200 {object} ValidateCommandResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/variants/validate-command [get].
func (s *Server) handleValidateCustomFFmpegCommand(c fiber.Ctx) error {
	cmd := c.Query("q")
	if strings.TrimSpace(cmd) == "" {
		return c.JSON(ValidateCommandResponse{Valid: false, Error: "command_required"})
	}

	err := transform.ValidateCustomCommand(cmd)
	if err == nil {
		return c.JSON(ValidateCommandResponse{Valid: true})
	}

	switch {
	case errors.Is(err, transform.ErrCustomFFmpegNoInputToken):
		return c.JSON(ValidateCommandResponse{
			Valid: false,
			Error: "missing_input_token",
		})
	case errors.Is(err, transform.ErrCustomFFmpegNoOutputToken):
		return c.JSON(ValidateCommandResponse{
			Valid: false,
			Error: "missing_output_token",
		})
	case errors.Is(err, transform.ErrCustomFFmpegTooLong):
		return c.JSON(ValidateCommandResponse{
			Valid: false,
			Error: "command_too_long",
		})
	case errors.Is(err, transform.ErrCustomFFmpegBlacklisted):
		return c.JSON(ValidateCommandResponse{
			Valid:  false,
			Error:  "command_blacklisted",
			Detail: err.Error(),
		})
	case errors.Is(err, transform.ErrCustomFFmpegBadRefToken):
		return c.JSON(ValidateCommandResponse{
			Valid:  false,
			Error:  "bad_ref_token",
			Detail: err.Error(),
		})
	default:
		return c.JSON(ValidateCommandResponse{Valid: false, Error: err.Error()})
	}
}

// handleGetVariantParamHistory returns distinct previous transform_params for a variant type.
//
// @Summary List variant param history
// @Description Returns up to 10 distinct previous <code>transform_params</code> used for the given variant type in the current workspace, ordered most-recently-used first. Lets the UI offer a "reuse previous settings" shortcut for variant tool forms. Unrecognized or non-restorable types (e.g. <code>image_bg_remove</code>) always return an empty list, never an error.
// @Tags Variants
// @Produce json
// @Security BearerAuth
// @Param type query string true "Variant type, e.g. image_resize, custom_ffmpeg"
// @Success 200 {object} VariantParamHistoryResponse
// @Failure 400 {object} ErrorResponse "type query param is required"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/variant-param-history [get]
// handleGetVariantParamHistory handles GET /api/v1/variant-param-history?type=.
func (s *Server) handleGetVariantParamHistory(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	variantType := c.Query("type")
	if variantType == "" {
		return errRes(c, fiber.StatusBadRequest, "type query param is required")
	}

	if !paramHistoryAllowedTypes[variantType] {
		return c.JSON(VariantParamHistoryResponse{Entries: []ParamHistoryEntryResponse{}})
	}

	entries, err := s.variants.GetParamHistory(c.Context(), claims.WorkspaceID, variantType)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	out := make([]ParamHistoryEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = ParamHistoryEntryResponse{Params: e.Params}
	}
	return c.JSON(VariantParamHistoryResponse{Entries: out})
}

// handleCreateVariant enqueues a transform job to produce a new variant.
//
// @Summary Create a variant
// @Description Enqueues a background job to generate a transformed variant of the asset's current version. Supported types and their required params: <ul> <li><strong>image_resize</strong> — <code>{"width": N, "height": N, "fit": "contain|cover|fill"}</code></li> <li><strong>image_convert</strong> — <code>{"format": "jpeg|png|webp|avif"}</code> (WebP output is lossless; quality only affects JPEG)</li> <li><strong>image_crop</strong> — <code>{"x": N, "y": N, "width": N, "height": N}</code></li> <li><strong>image_watermark</strong> — <code>{"opacity": 0.5}</code></li> <li><strong>image_smart_crop</strong> — <code>{"width": N, "height": N}</code> (AI-assisted)</li> <li><strong>image_bg_remove</strong> — <code>{"model": "bria/remove-background"}</code></li> <li><strong>image_with_prompt</strong> — <code>{"prompt": "...", "model": "black-forest-labs/FLUX.1-fill-dev"}</code></li> <li><strong>video_transcode</strong> — <code>{"format": "mp4", "codec": "h264"}</code></li> <li><strong>video_watermark</strong> — <code>{"opacity": 0.5, "format": "mp4"}</code></li> <li><strong>video_capture_image</strong> — <code>{"time_sec": N}</code></li> <li><strong>custom_ffmpeg</strong> — <code>{"command": "ffmpeg -i {input} ... {output}"}</code> (any MIME type; output format auto-detected via ffprobe)</li> </ul> Returns a job ID immediately; poll <code>GET /api/v1/assets/:id/variants</code> to check completion. Returns 409 if a variant rebuild is already in progress.
// @Tags Variants
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body CreateVariantRequest true "Variant type and transform params"
// @Success 202 {object} CreateVariantResponse
// @Failure 400 {object} ErrorResponse "Invalid variant type or wrong asset type"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 409 {object} ErrorResponse "Variant rebuild already in progress"
// @Failure 422 {object} ErrorResponse "Asset has no current version"
// @Router /api/v1/assets/{id}/variants [post]
// handleCreateVariant handles POST /api/v1/assets/:id/variants
// Creates a variant bound to the asset's current version.
func (s *Server) handleCreateVariant(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if asset.CurrentVersionID == nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "asset has no current version")
	}

	if s.isRebuildingVariants(c, *asset.CurrentVersionID) {
		return errRes(
			c,
			fiber.StatusConflict,
			"variants are rebuilding — please wait for the rebuild to complete before creating new variants",
		)
	}

	body, ok := decodeAndValidate(c, &CreateVariantRequest{})
	if !ok {
		return nil
	}

	if isDemoSession(c) && (body.Type == queue.JobTypeImageBgRemove || body.Type == queue.JobTypeImageWithPrompt) {
		return c.Status(fiber.StatusForbidden).JSON(demoRestrictedResponse)
	}

	currentVer, err := s.versions.GetCurrentByAsset(c.Context(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load current version")
	}

	irStatus, err := s.workspace.GetAIProviderKeyStatus(c.Context(), claims.WorkspaceID, string(ai.ProviderImageRouter))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	prepared, err := s.variants.PrepareCreate(c.Context(), service.PrepareCreateVariantParams{
		WorkspaceID:           claims.WorkspaceID,
		AssetID:               assetID,
		Type:                  body.Type,
		Params:                body.Params,
		AssetMimeType:         asset.MimeType,
		ImageRouterConfigured: irStatus.KeySet,
		DefaultImageModel:     s.cfg.ImageRouter.DefaultModel,
		DefaultBgRemoveModel:  s.cfg.ImageRouter.DefaultBgRemoveModel,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidVariantType) {
			return errRes(c, fiber.StatusBadRequest, "invalid variant type")
		}
		if errors.Is(err, service.ErrInvalidVariantReq) {
			return errRes(c, fiber.StatusBadRequest, err.Error())
		}
		return ErrorStatusResponse(c, err)
	}

	payload, _ := json.Marshal(jobspec.VariantJobPayload{
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		VersionID:   currentVer.ID,
		VersionNum:  currentVer.VersionNum,
		StorageKey:  currentVer.StorageKey,
		MimeType:    currentVer.MimeType,
		Type:        prepared.Type,
		Params:      prepared.Params,
	})

	_, enqueueSpan := apptelemetry.StartSpan(c.Context(), "api.variants.enqueue_create",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.job.type", prepared.Type),
	)
	job, err := s.queue.Enqueue(c.Context(), claims.WorkspaceID, prepared.Type, string(payload))
	apptelemetry.EndSpan(enqueueSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not enqueue job")
	}

	s.variants.WriteVariantQueued(c.Context(), claims.WorkspaceID, assetID, prepared.Type)

	return c.Status(fiber.StatusAccepted).JSON(CreateVariantResponse{
		JobID:   job.ID,
		Status:  apiStatusPending,
		Message: "variant creation queued",
	})
}

// handlePromoteVariant handles POST /api/v1/assets/:id/variants/:vid/promote
//
// @Summary Promote a variant to a new asset
// @Tags Variants
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Variant ID"
// @Success 201 {object} PromoteVariantResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or variant not found"
// @Router /api/v1/assets/{id}/variants/{vid}/promote [post].
func (s *Server) handlePromoteVariant(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	variantID := c.Params("vid")

	body, ok := decodeAndValidate(c, &PromoteVariantRequest{})
	if !ok {
		return nil
	}

	result, err := s.variants.Promote(c.Context(), service.PromoteVariantParams{
		AssetID:     assetID,
		VariantID:   variantID,
		WorkspaceID: claims.WorkspaceID,
		Name:        body.Name,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(PromoteVariantResponse{
		AssetID:  result.NewAssetID,
		AssetURL: result.NewAssetURL,
	})
}

// handleSetVariantThumbnail handles POST /api/v1/assets/:id/variants/:vid/set-thumbnail
//
// @Summary Set a variant as the asset thumbnail
// @Tags Variants
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Variant ID"
// @Success 200 {object} SetVariantThumbnailResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or variant not found"
// @Router /api/v1/assets/{id}/variants/{vid}/set-thumbnail [post].
func (s *Server) handleSetVariantThumbnail(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	variantID := c.Params("vid")

	if err := s.variants.SetAsThumbnail(c.Context(), claims.WorkspaceID, assetID, variantID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(SetVariantThumbnailResponse{
		ThumbnailURL: assetThumbURL(assetID),
	})
}

// handleRerunVariant handles POST /api/v1/assets/:id/variants/:vid/rerun
//
// @Summary Rerun a variant job
// @Tags Variants
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Variant ID"
// @Success 202 {object} RerunVariantResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or variant not found"
// @Router /api/v1/assets/{id}/variants/{vid}/rerun [post].
func (s *Server) handleRerunVariant(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	variantID := c.Params("vid")

	body, ok := decodeAndValidate(c, &RerunVariantRequest{})
	if !ok {
		return nil
	}

	if err := s.variants.Rerun(c.Context(), service.RerunVariantParams{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		VariantID:   variantID,
		NewParams:   body.Params,
	}); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusAccepted).JSON(RerunVariantResponse{
		VariantID: variantID,
		Status:    apiStatusPending,
	})
}

func (s *Server) handleUpdateVariantsSharing(c fiber.Ctx) error {
	ctx, span := apptelemetry.StartSpan(c.Context(), "api.variants.update_sharing")
	defer apptelemetry.EndSpan(span, nil)

	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	body, ok := decodeAndValidate(c, &UpdateVariantsSharingRequest{})
	if !ok {
		return nil
	}

	if err := s.variants.UpdateSharing(ctx, service.UpdateVariantsSharingParams{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		Updates:     body.Updates,
	}); err != nil {
		return ErrorStatusResponse(c, err)
	}

	result, err := s.variants.List(ctx, service.ListVariantsParams{WorkspaceID: claims.WorkspaceID, AssetID: assetID})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	out := make([]VariantResponse, len(result.Variants))
	for i, v := range result.Variants {
		out[i] = variantDTOToResponse(assetID, v)
	}

	return c.JSON(ListVariantsResponse{Variants: out})
}

func (s *Server) handlePatchVariant(c fiber.Ctx) error {
	ctx, span := apptelemetry.StartSpan(c.Context(), "api.variants.patch")
	defer apptelemetry.EndSpan(span, nil)

	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	variantID := c.Params("vid")

	body, ok := decodeAndValidate(c, &PatchVariantRequest{})
	if !ok {
		return nil
	}

	if err := s.variants.UpdateTitle(ctx, claims.WorkspaceID, variantID, body.Title); err != nil {
		return ErrorStatusResponse(c, err)
	}
	v, err := s.variants.Get(ctx, claims.WorkspaceID, variantID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(variantDTOToResponse(assetID, v))
}

// handleGetVariantFile streams the variant file bytes.
//
// @Summary Download variant file
// @Description Streams the variant's stored file. Content-Type is derived from the file extension. An <code>asset_variant_downloaded</code> audit event is recorded (browser image prefetch excluded).
// @Tags Variants
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Variant ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset, variant, or file not found"
// @Router /api/v1/assets/{id}/variants/{vid}/file [get]
// handleGetVariantFile handles GET /api/v1/assets/:id/variants/:vid/file.
func (s *Server) handleGetVariantFile(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	variantID := c.Params("vid")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	variant, err := s.variants.Get(c.Context(), claims.WorkspaceID, variantID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if setCacheHeaders(c, variant.ID, variant.CreatedAt, true) {
		return nil
	}

	rc, err := s.storage.Get(c.Context(), variant.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "variant file not found")
	}

	if !audit.IsBrowserPrefetch(c.Get("Sec-Fetch-Dest")) {
		s.variants.WriteVariantDownloadedAsync(claims.WorkspaceID, assetID, variantID, variant.Type, "", "")
	}

	ext := strings.ToLower(filepath.Ext(variant.StorageKey))
	c.Set("Content-Type", mime.TypeByExtension(ext))
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s_%s%s"`, assetID[:8], variantID[:8], ext))
	if variant.Size != nil && *variant.Size > 0 {
		c.Set("Content-Length", strconv.FormatInt(*variant.Size, 10))
	}
	return c.SendStream(rc)
}

// handleDeleteVariant deletes a variant and its stored file.
//
// @Summary Delete a variant
// @Description Permanently removes the variant record and its stored file. Only variants attached to the <em>current</em> version can be deleted manually — variants on older versions are removed automatically by the retention purge job.
// @Tags Variants
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Variant ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or variant not found"
// @Failure 422 {object} ErrorResponse "Variant belongs to a previous version"
// @Router /api/v1/assets/{id}/variants/{vid} [delete]
// handleDeleteVariant handles DELETE /api/v1/assets/:id/variants/:vid
// Guards against deleting variants that belong to non-current versions.
func (s *Server) handleDeleteVariant(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	variantID := c.Params("vid")

	variant, err := s.variants.Get(c.Context(), claims.WorkspaceID, variantID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if err = s.variants.Delete(c.Context(), claims.WorkspaceID, assetID, variantID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	_ = s.storage.Delete(c.Context(), variant.StorageKey)
	if variant.ThumbnailKey != nil {
		_ = s.storage.Delete(c.Context(), *variant.ThumbnailKey)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// handleUploadManualVariant attaches an externally produced file as a manual variant.
//
// @Summary Upload a manual variant
// @Description Accepts a multipart file upload and stores it as a variant of type <code>manual</code> on the asset's current version. Unlike transform-generated variants, manual variants are <em>not</em> automatically rebuilt when a new version is uploaded — they persist across version changes. Use this to attach pre-processed or third-party exports (e.g. color-corrected TIFF, print-ready PDF).
// @Tags Variants
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param file formData file true "File to attach as a manual variant"
// @Success 201 {object} VariantResponse
// @Failure 400 {object} ErrorResponse "file field is required"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ErrorResponse "Asset has no current version"
// @Router /api/v1/assets/{id}/variants/upload [post]
// handleUploadManualVariant handles POST /api/v1/assets/:id/variants/upload
// Accepts a raw file upload and attaches it as a manual variant of type "manual"
// on the current version. Manual variants are NOT rebuilt on new version uploads.
func (s *Server) handleUploadManualVariant(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if asset.CurrentVersionID == nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "asset has no current version")
	}

	currentVer, err := s.versions.GetCurrentByAsset(c.Context(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load current version")
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "file field is required")
	}

	f, err := fh.Open()
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not open uploaded file")
	}
	defer f.Close()

	variantID := uuid.NewString()
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	storageKey := storage.VersionedVariantKey(
		asset.WorkspaceID, assetID, currentVer.VersionNum,
		"manual", variantID[:8], ext,
	)

	_, putSpan := apptelemetry.StartSpan(c.Context(), "api.variants.manual_storage_put",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.storage.key", storageKey),
		attribute.Int64("damask.upload.bytes", fh.Size),
	)
	err = s.storage.Put(c.Context(), storageKey, f)
	apptelemetry.EndSpan(putSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not store file")
	}

	sz := fh.Size
	emptyParams := "{}"
	uploadedMimeType := mime.TypeByExtension(ext)
	if uploadedMimeType == "" {
		uploadedMimeType = "application/octet-stream"
	}

	v, err := s.variants.Create(c.Context(), service.CreateVariantParams{
		ID:              variantID,
		WorkspaceID:     asset.WorkspaceID,
		AssetID:         assetID,
		AssetVersionID:  currentVer.ID,
		Type:            "manual",
		StorageKey:      storageKey,
		TransformParams: &emptyParams,
		Size:            &sz,
	})
	if err != nil {
		_ = s.storage.Delete(c.Context(), storageKey)
		return errRes(c, fiber.StatusInternalServerError, "could not create variant record")
	}

	s.hub.Publish(c.Context(), asset.WorkspaceID, events.VariantReady(assetID, variantID))

	if thumbPayload, thumbErr := json.Marshal(jobs.VariantThumbnailJobPayload{
		VariantID:   variantID,
		WorkspaceID: asset.WorkspaceID,
		AssetID:     assetID,
		StorageKey:  storageKey,
		MimeType:    uploadedMimeType,
	}); thumbErr == nil {
		_, _ = s.queue.Enqueue(c.Context(), asset.WorkspaceID, queue.JobTypeVariantThumbnail, string(thumbPayload))
	}

	return c.Status(fiber.StatusCreated).JSON(variantDTOToResponse(assetID, v))
}

// handleGetVariantThumb handles GET /api/v1/assets/:id/variants/:vid/thumb
// Returns 202 while the thumbnail job is still processing, streams the file once ready.
func (s *Server) handleGetVariantThumb(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	variantID := c.Params("vid")

	variant, err := s.variants.Get(c.Context(), claims.WorkspaceID, variantID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if variant.ThumbnailKey == nil {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{apiStatusKey: "processing"})
	}

	_, getSpan := apptelemetry.StartSpan(c.Context(), "api.variants.thumbnail_storage_get",
		attribute.String("damask.variant_id", variantID),
		attribute.String("damask.storage.key", *variant.ThumbnailKey),
	)
	rc, err := s.storage.Get(c.Context(), *variant.ThumbnailKey)
	apptelemetry.EndSpan(getSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	ct := variant.ThumbnailContentType
	if ct == "" {
		ct = contentTypeImageJPEG
	}
	c.Set("Content-Type", ct)
	return c.SendStream(rc)
}

// handlePreviewTransform applies an in-memory image transform and returns the result.
//
// @Summary Preview image transform
// @Description Applies a resize/format transform to the asset's current version in memory and returns the result directly (never stored). Responses are cached in an LRU cache (100 entries, 5-minute TTL) so repeated identical calls are cheap. <br>Query parameters: <ul> <li><strong>w</strong> — Target width in pixels</li> <li><strong>h</strong> — Target height in pixels</li> <li><strong>fit</strong> — Fit mode: <code>contain</code> (default), <code>cover</code>, <code>fill</code></li> <li><strong>format</strong> — Output format: <code>jpeg</code> (default), <code>png</code>, <code>webp</code> (lossless)</li> <li><strong>q</strong> — JPEG quality 1–100 (default: encoder default; ignored for WebP)</li> </ul> Only supported for image assets. Returns 400 for video, audio, or PDF assets.
// @Tags Variants
// @Produce image/jpeg
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param w query int false "Target width"
// @Param h query int false "Target height"
// @Param fit query string false "Fit mode (contain|cover|fill)"
// @Param format query string false "Output format (jpeg|png|webp)"
// @Param q query int false "JPEG quality (1-100, ignored for WebP)"
// @Success 200 {file} binary
// @Failure 400 {object} ErrorResponse "Preview only supported for images"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/preview [get]
// handlePreviewTransform runs a transform in-memory and returns a small image.
// GET /api/v1/assets/:id/preview?w=&h=&fit=&format=&q=.
func (s *Server) handlePreviewTransform(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if !strings.HasPrefix(asset.MimeType, "image/") {
		return errRes(c, fiber.StatusBadRequest, "preview only supported for images")
	}

	w, _ := strconv.Atoi(c.Query("w"))
	h, _ := strconv.Atoi(c.Query("h"))
	q, _ := strconv.Atoi(c.Query("q"))
	fit := c.Query("fit", "contain")
	format := c.Query("format", "jpeg")

	cacheKey := fmt.Sprintf(
		"%s|%s|w=%d|h=%d|fit=%s|format=%s|q=%d",
		asset.ID,
		*asset.CurrentVersionID,
		w,
		h,
		fit,
		format,
		q,
	)

	// Check conditional request before doing any work (ETag = cacheKey hash).
	if setCacheHeaders(c, cacheKey, asset.UpdatedAt, false) {
		return nil
	}

	if cached, ct := s.previewCache.Get(cacheKey); cached != nil {
		c.Set("Content-Type", ct)
		return c.Send(cached)
	}

	_, getSpan := apptelemetry.StartSpan(c.Context(), "api.variants.preview_storage_get",
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.storage.key", asset.StorageKey),
	)
	rc, err := s.storage.Get(c.Context(), asset.StorageKey)
	apptelemetry.EndSpan(getSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "asset file not found")
	}
	defer rc.Close()

	_, previewSpan := apptelemetry.StartSpan(c.Context(), "api.variants.preview_transform",
		attribute.String("damask.asset_id", assetID),
		attribute.Int("damask.preview.width", w),
		attribute.Int("damask.preview.height", h),
		attribute.String("damask.preview.fit", fit),
		attribute.String("damask.preview.format", format),
	)
	data, ct, err := s.trf.ImagePreview(rc, transform.PreviewParams{
		Width:   w,
		Height:  h,
		Fit:     fit,
		Quality: q,
		Format:  format,
	})
	previewSpan.SetAttributes(attribute.Int("damask.preview.bytes", len(data)))
	apptelemetry.EndSpan(previewSpan, err)
	if err != nil {
		slog.ErrorContext(c.Context(), "preview generation failed", "error", err)
		return errRes(c, fiber.StatusInternalServerError, "preview generation failed")
	}

	s.previewCache.Set(cacheKey, data, ct)
	c.Set("Content-Type", ct)
	return c.Send(data)
}
