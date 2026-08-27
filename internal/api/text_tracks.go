package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"

	"damask/server/internal/auth"
	"damask/server/internal/service"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel/attribute"
)

var tesseractAvailable = transform.TesseractAvailable

type TextTrackResponse struct {
	ID               string         `json:"id"`
	AssetID          string         `json:"asset_id"`
	AssetVersionID   *string        `json:"asset_version_id,omitempty"`
	Source           string         `json:"source"`
	Lang             *string        `json:"lang,omitempty"`
	Content          string         `json:"content"`
	ContentTruncated bool           `json:"content_truncated"`
	HasFile          bool           `json:"has_file"`
	DownloadURL      *string        `json:"download_url,omitempty"`
	Meta             map[string]any `json:"meta"`
	Status           string         `json:"status"`
	Error            *string        `json:"error,omitempty"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
}

type ListTextTracksResponse struct {
	TextTracks []TextTrackResponse `json:"text_tracks"`
}

// CreateTextTrackResponse wraps a single text track returned after creation.
type CreateTextTrackResponse struct {
	TextTrack TextTrackResponse `json:"text_track"`
}

func textTrackDTOToResponse(dto service.TextTrackDTO, truncate bool) TextTrackResponse {
	content := dto.Content
	contentTruncated := false
	if truncate && len(content) > 500 {
		content = content[:500]
		contentTruncated = true
	}
	var downloadURL *string
	if dto.StorageKey != nil {
		u := textTrackDownloadURL(dto.AssetID, dto.ID)
		downloadURL = &u
	}
	return TextTrackResponse{
		ID:               dto.ID,
		AssetID:          dto.AssetID,
		AssetVersionID:   dto.AssetVersionID,
		Source:           dto.Source,
		Lang:             dto.Lang,
		Content:          content,
		ContentTruncated: contentTruncated,
		HasFile:          dto.StorageKey != nil,
		DownloadURL:      downloadURL,
		Meta:             dto.Meta,
		Status:           dto.Status,
		Error:            dto.Error,
		CreatedAt:        dto.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        dto.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// handleListTextTracks handles GET /api/v1/assets/:id/text-tracks
//
// @Summary List text tracks for an asset
// @Tags Text Tracks
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} ListTextTracksResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/text-tracks [get].
func (s *Server) handleListTextTracks(c fiber.Ctx) (err error) {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	ctx, span := telemetry.StartSpan(c.Context(), "api.text_tracks.list",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
		attribute.String("damask.asset_id", assetID),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"list text tracks failed",
				"workspace_id",
				claims.WorkspaceID,
				"asset_id",
				assetID,
				apiErrorKey,
				err,
			)
		}
	}()

	assetCtx, assetSpan := telemetry.StartSpan(ctx, "api.text_tracks.list.load_asset")
	if _, loadErr := s.assets.Get(assetCtx, claims.WorkspaceID, assetID); loadErr != nil {
		telemetry.EndSpan(assetSpan, loadErr)
		span.RecordError(loadErr)
		slog.ErrorContext(
			ctx,
			"list text tracks: load asset",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			apiErrorKey,
			loadErr,
		)
		err = ErrorStatusResponse(c, loadErr)
		return err
	}
	telemetry.EndSpan(assetSpan, nil)

	listCtx, listSpan := telemetry.StartSpan(ctx, "api.text_tracks.list.fetch_tracks")
	tracks, err := s.textTracks.List(listCtx, claims.WorkspaceID, assetID)
	telemetry.EndSpan(listSpan, err)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(
			ctx,
			"list text tracks: fetch tracks",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			apiErrorKey,
			err,
		)
		return ErrorStatusResponse(c, err)
	}
	span.SetAttributes(attribute.Int("damask.text_tracks.result_count", len(tracks)))

	out := make([]TextTrackResponse, len(tracks))
	for i, track := range tracks {
		out[i] = textTrackDTOToResponse(track, true)
	}
	return c.JSON(ListTextTracksResponse{TextTracks: out})
}

// handleGetTextTrack handles GET /api/v1/assets/:id/text-tracks/:tid
//
// @Summary Get a text track
// @Tags Text Tracks
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param tid path string true "Text Track ID"
// @Success 200 {object} TextTrackResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Text track not found"
// @Router /api/v1/assets/{id}/text-tracks/{tid} [get].
func (s *Server) handleGetTextTrack(c fiber.Ctx) (err error) {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	trackID := c.Params("tid")
	ctx, span := telemetry.StartSpan(c.Context(), "api.text_tracks.get",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"get text track failed",
				"workspace_id",
				claims.WorkspaceID,
				"asset_id",
				assetID,
				"track_id",
				trackID,
				apiErrorKey,
				err,
			)
		}
	}()

	assetCtx, assetSpan := telemetry.StartSpan(ctx, "api.text_tracks.get.load_asset")
	if _, loadErr := s.assets.Get(assetCtx, claims.WorkspaceID, assetID); loadErr != nil {
		telemetry.EndSpan(assetSpan, loadErr)
		span.RecordError(loadErr)
		slog.ErrorContext(
			ctx,
			"get text track: load asset",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			"track_id",
			trackID,
			apiErrorKey,
			loadErr,
		)
		err = ErrorStatusResponse(c, loadErr)
		return err
	}
	telemetry.EndSpan(assetSpan, nil)

	trackCtx, trackSpan := telemetry.StartSpan(ctx, "api.text_tracks.get.fetch_track")
	track, err := s.textTracks.Get(trackCtx, claims.WorkspaceID, trackID)
	telemetry.EndSpan(trackSpan, err)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(
			ctx,
			"get text track: fetch track",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			"track_id",
			trackID,
			apiErrorKey,
			err,
		)
		return ErrorStatusResponse(c, err)
	}
	span.SetAttributes(
		attribute.String("damask.text_track.source", track.Source),
		attribute.String("damask.text_track.status", track.Status),
		attribute.Bool("damask.text_track.has_file", track.StorageKey != nil),
	)
	if track.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "text track not found")
	}
	return c.JSON(textTrackDTOToResponse(track, false))
}

// handleCreateTextTrack handles POST /api/v1/assets/:id/text-tracks
//
// @Summary Create a text track for an asset
// @Tags Text Tracks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} CreateTextTrackResponse
// @Success 202 {object} CreateTextTrackResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/text-tracks [post].
func (s *Server) handleCreateTextTrack(c fiber.Ctx) (err error) {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	ctx, span := telemetry.StartSpan(c.Context(), "api.text_tracks.create",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
		attribute.String("damask.asset_id", assetID),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"create text track failed",
				"workspace_id",
				claims.WorkspaceID,
				"asset_id",
				assetID,
				apiErrorKey,
				err,
			)
		}
	}()

	assetCtx, assetSpan := telemetry.StartSpan(ctx, "api.text_tracks.create.load_asset")
	asset, assetErr := s.assets.Get(assetCtx, claims.WorkspaceID, assetID)
	telemetry.EndSpan(assetSpan, assetErr)
	if assetErr != nil {
		span.RecordError(assetErr)
		slog.ErrorContext(
			ctx,
			"create text track: load asset",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			apiErrorKey,
			assetErr,
		)
		err = ErrorStatusResponse(c, assetErr)
		return err
	}

	body, ok := decodeAndValidate(c, &CreateTextTrackRequest{})
	if !ok {
		return nil
	}

	params := body.Params
	if params == nil {
		params = map[string]any{}
	}
	span.SetAttributes(attribute.String("damask.text_track.source", body.Source))

	createParams := service.CreateTextTrackParams{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		Source:      body.Source,
		Lang:        body.Lang,
		Params:      params,
		CreatedBy:   claims.UserID,
	}

	if handled, applyErr := s.applyTextTrackSourceParams(
		ctx, c, claims.WorkspaceID, assetID, asset, body, params, &createParams,
	); handled {
		return applyErr
	}

	createCtx, createSpan := telemetry.StartSpan(ctx, "api.text_tracks.create.persist")
	track, err := s.textTracks.Create(createCtx, createParams)
	telemetry.EndSpan(createSpan, err)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(
			ctx,
			"create text track: persist",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			"source",
			body.Source,
			apiErrorKey,
			err,
		)
		return ErrorStatusResponse(c, err)
	}
	span.SetAttributes(
		attribute.String("damask.text_track_id", track.ID),
		attribute.String("damask.text_track.status", track.Status),
	)
	status := fiber.StatusAccepted
	if track.Status == "ready" {
		status = fiber.StatusOK
	}
	return c.Status(status).JSON(CreateTextTrackResponse{
		TextTrack: textTrackDTOToResponse(track, false),
	})
}

// applyTextTrackSourceParams validates and fills in createParams for the
// requested text track source. handled reports whether an HTTP response was
// already written and the caller must return immediately — errRes and
// ErrorStatusResponse write the response themselves and return nil once
// encoding succeeds, so err alone can't be used to signal "stop here".
func (s *Server) applyTextTrackSourceParams(
	ctx context.Context,
	c fiber.Ctx,
	workspaceID, assetID string,
	asset *service.AssetDTO,
	body *CreateTextTrackRequest,
	params map[string]any,
	createParams *service.CreateTextTrackParams,
) (bool, error) {
	switch body.Source {
	case "ocr":
		if ocrErr := s.prepareOCRParams(
			ctx, c, assetID, workspaceID, asset, body.Lang, params, createParams,
		); ocrErr != nil {
			return true, validationErrRes(c, ocrErr)
		}
	case "manual":
		content, _ := params["content"].(string)
		if strings.TrimSpace(content) == "" {
			return true, errRes(c, fiber.StatusUnprocessableEntity, "content is required")
		}
		createParams.InitialContent = content
	case "extract_pdf":
		if extErr := s.prepareExtractParams(ctx, c, assetID, asset.MimeType, asset.CurrentVersionID,
			transform.IsPdfMime, "asset is not a PDF", false, params); extErr != nil {
			return true, extErr
		}
	case "extract_plain":
		if extErr := s.prepareExtractParams(ctx, c, assetID, asset.MimeType, asset.CurrentVersionID,
			transform.IsTextMime, "asset is not a plain text file", false, params); extErr != nil {
			return true, extErr
		}
	case "extract_document":
		if extErr := s.prepareExtractParams(ctx, c, assetID, asset.MimeType, asset.CurrentVersionID,
			transform.IsDocumentMime, "asset is not a supported document type", true, params); extErr != nil {
			return true, extErr
		}
	case "ai_image_description":
		if aiErr := s.prepareAIImageDescriptionParams(
			ctx, c, workspaceID, assetID, asset, body.Lang, params, createParams,
		); aiErr != nil {
			return true, validationErrRes(c, aiErr)
		}
	default:
		return true, errRes(c, fiber.StatusUnprocessableEntity, "unsupported_source")
	}
	return false, nil
}

func (s *Server) prepareExtractParams(
	ctx context.Context,
	c fiber.Ctx,
	assetID, mimeType string,
	currentVersionID *string,
	mimeCheck func(string) bool,
	mimeErrMsg string,
	includeMime bool,
	params map[string]any,
) error {
	if !mimeCheck(mimeType) {
		return errRes(c, fiber.StatusUnprocessableEntity, mimeErrMsg)
	}
	if currentVersionID == nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "asset has no current version")
	}
	currentVersion, err := s.versions.GetCurrentByAsset(ctx, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	params["storage_key"] = currentVersion.StorageKey
	if includeMime {
		params["mime_type"] = currentVersion.MimeType
	}
	return nil
}

type apiValidationError struct {
	status int
	msg    string
}

func (e *apiValidationError) Error() string { return e.msg }

// validationErrRes converts an *apiValidationError into its HTTP response,
// passing through any other error unchanged for the caller to handle.
func validationErrRes(c fiber.Ctx, err error) error {
	var ve *apiValidationError
	if errors.As(err, &ve) {
		return errRes(c, ve.status, ve.msg)
	}
	return err
}

func (s *Server) prepareOCRParams(
	ctx context.Context,
	c fiber.Ctx,
	assetID, workspaceID string,
	asset *service.AssetDTO,
	lang *string,
	params map[string]any,
	createParams *service.CreateTextTrackParams,
) error {
	if !transform.SupportedOCRMIMEs[asset.MimeType] {
		return &apiValidationError{fiber.StatusUnprocessableEntity, "unsupported OCR asset MIME type"}
	}
	if !tesseractAvailable() {
		return &apiValidationError{fiber.StatusServiceUnavailable, "Tesseract is not installed on this server"}
	}
	if asset.CurrentVersionID == nil {
		return &apiValidationError{fiber.StatusUnprocessableEntity, "asset has no current version"}
	}
	outputFormat := "txt"
	if raw, ok := params["output_format"].(string); ok && strings.TrimSpace(raw) != "" {
		outputFormat = raw
	}
	if outputFormat != "txt" && outputFormat != "hocr" {
		return &apiValidationError{fiber.StatusUnprocessableEntity, "unsupported OCR output format"}
	}
	resolvedLang := "eng"
	if lang != nil && strings.TrimSpace(*lang) != "" {
		resolvedLang = strings.TrimSpace(*lang)
	}
	versionCtx, versionSpan := telemetry.StartSpan(ctx, "api.text_tracks.create.load_current_version")
	currentVersion, versionErr := s.versions.GetCurrentByAsset(versionCtx, assetID)
	telemetry.EndSpan(versionSpan, versionErr)
	if versionErr != nil {
		slog.ErrorContext(ctx, "create text track: load current version",
			"workspace_id", workspaceID, "asset_id", assetID, apiErrorKey, versionErr)
		return ErrorStatusResponse(c, versionErr)
	}
	createParams.AssetVersionID = asset.CurrentVersionID
	createParams.Lang = &resolvedLang
	params["lang"] = resolvedLang
	params["output_format"] = outputFormat
	params["storage_key"] = currentVersion.StorageKey
	params["mime_type"] = currentVersion.MimeType
	return nil
}

// handleDeleteTextTrack handles DELETE /api/v1/assets/:id/text-tracks/:tid
//
// @Summary Delete a text track
// @Tags Text Tracks
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param tid path string true "Text Track ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Text track not found"
// @Router /api/v1/assets/{id}/text-tracks/{tid} [delete].
func (s *Server) handleDeleteTextTrack(c fiber.Ctx) (err error) {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	trackID := c.Params("tid")
	ctx, span := telemetry.StartSpan(c.Context(), "api.text_tracks.delete",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"delete text track failed",
				"workspace_id",
				claims.WorkspaceID,
				"asset_id",
				assetID,
				"track_id",
				trackID,
				apiErrorKey,
				err,
			)
		}
	}()

	assetCtx, assetSpan := telemetry.StartSpan(ctx, "api.text_tracks.delete.load_asset")
	if _, loadErr := s.assets.Get(assetCtx, claims.WorkspaceID, assetID); loadErr != nil {
		telemetry.EndSpan(assetSpan, loadErr)
		span.RecordError(loadErr)
		slog.ErrorContext(
			ctx,
			"delete text track: load asset",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			"track_id",
			trackID,
			apiErrorKey,
			loadErr,
		)
		err = ErrorStatusResponse(c, loadErr)
		return err
	}
	telemetry.EndSpan(assetSpan, nil)

	trackCtx, trackSpan := telemetry.StartSpan(ctx, "api.text_tracks.delete.fetch_track")
	track, err := s.textTracks.Get(trackCtx, claims.WorkspaceID, trackID)
	telemetry.EndSpan(trackSpan, err)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(
			ctx,
			"delete text track: fetch track",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			"track_id",
			trackID,
			apiErrorKey,
			err,
		)
		return ErrorStatusResponse(c, err)
	}
	span.SetAttributes(
		attribute.String("damask.text_track.source", track.Source),
		attribute.Bool("damask.text_track.has_file", track.StorageKey != nil),
	)
	if track.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "text track not found")
	}
	deleteCtx, deleteSpan := telemetry.StartSpan(ctx, "api.text_tracks.delete.remove")
	if deleteErr := s.textTracks.Delete(deleteCtx, claims.WorkspaceID, trackID); deleteErr != nil {
		telemetry.EndSpan(deleteSpan, deleteErr)
		span.RecordError(deleteErr)
		slog.ErrorContext(
			ctx,
			"delete text track: remove",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			"track_id",
			trackID,
			apiErrorKey,
			deleteErr,
		)
		err = ErrorStatusResponse(c, deleteErr)
		return err
	}
	telemetry.EndSpan(deleteSpan, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleDownloadTextTrack(c fiber.Ctx) (err error) {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	trackID := c.Params("tid")
	ctx, span := telemetry.StartSpan(c.Context(), "api.text_tracks.download",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"download text track failed",
				"workspace_id",
				claims.WorkspaceID,
				"asset_id",
				assetID,
				"track_id",
				trackID,
				apiErrorKey,
				err,
			)
		}
	}()

	assetCtx, assetSpan := telemetry.StartSpan(ctx, "api.text_tracks.download.load_asset")
	if _, loadErr := s.assets.Get(assetCtx, claims.WorkspaceID, assetID); loadErr != nil {
		telemetry.EndSpan(assetSpan, loadErr)
		span.RecordError(loadErr)
		slog.ErrorContext(
			ctx,
			"download text track: load asset",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			"track_id",
			trackID,
			apiErrorKey,
			loadErr,
		)
		err = ErrorStatusResponse(c, loadErr)
		return err
	}
	telemetry.EndSpan(assetSpan, nil)

	trackCtx, trackSpan := telemetry.StartSpan(ctx, "api.text_tracks.download.fetch_track")
	track, err := s.textTracks.Get(trackCtx, claims.WorkspaceID, trackID)
	telemetry.EndSpan(trackSpan, err)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(
			ctx,
			"download text track: fetch track",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			"track_id",
			trackID,
			apiErrorKey,
			err,
		)
		return ErrorStatusResponse(c, err)
	}
	if track.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "text track not found")
	}
	if track.StorageKey == nil {
		return errRes(c, fiber.StatusNotFound, "This track has no downloadable file.")
	}
	span.SetAttributes(
		attribute.String("damask.text_track.source", track.Source),
		attribute.Bool("damask.text_track.has_file", true),
	)
	_, fileSpan := telemetry.StartSpan(ctx, "api.text_tracks.download.open_storage",
		attribute.String("damask.storage_key", *track.StorageKey),
	)
	rc, err := s.storage.Get(c.Context(), *track.StorageKey)
	telemetry.EndSpan(fileSpan, err)
	if err != nil {
		span.RecordError(err)
		slog.ErrorContext(
			ctx,
			"download text track: open storage",
			"workspace_id",
			claims.WorkspaceID,
			"asset_id",
			assetID,
			"track_id",
			trackID,
			"storage_key",
			*track.StorageKey,
			apiErrorKey,
			err,
		)
		return errRes(c, fiber.StatusNotFound, "file not found")
	}
	contentType := "text/plain; charset=utf-8"
	if track.ContentType != nil && *track.ContentType != "" {
		contentType = *track.ContentType
	}
	span.SetAttributes(attribute.String("damask.content_type", contentType))
	ext := filepath.Ext(*track.StorageKey)
	if ext == "" {
		exts, _ := mime.ExtensionsByType(contentType)
		if len(exts) > 0 {
			ext = exts[0]
		}
	}
	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		apiFilenameKey: fmt.Sprintf("%s-%s%s", assetID, track.Source, ext),
	}))
	return c.SendStream(rc)
}
