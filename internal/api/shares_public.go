package api

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/service"
	apptelemetry "damask/server/internal/telemetry"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/bcrypt"
)

const shareTokenValidityDuration = 24 * time.Hour

// ShareInfoResponse is returned by GET /shared/:id/access.
type ShareInfoResponse struct {
	Label       string `json:"label"`
	HasPassword bool   `json:"has_password"`
}

// ShareAccessResponse is returned by POST /shared/:id/access on success.
type ShareAccessResponse struct {
	Token string `json:"token"`
}

// PublicShareView is the share metadata embedded in share asset list responses.
type PublicShareView struct {
	ID            string  `json:"id"`
	Label         string  `json:"label"`
	AllowComments bool    `json:"allow_comments"`
	AllowDownload bool    `json:"allow_download"`
	ExpiresAt     *string `json:"expires_at,omitempty"`
	HasPassword   bool    `json:"has_password"`
}

// ShareAssetsResponse is returned by GET /shared/:id/assets.
type ShareAssetsResponse struct {
	Share  PublicShareView `json:"share"`
	Assets []AssetResponse `json:"assets"`
}

// CommentResponse is the JSON shape for a share comment.
type CommentResponse struct {
	ID          string  `json:"id"`
	ShareID     string  `json:"share_id"`
	AssetID     string  `json:"asset_id"`
	AuthorName  string  `json:"author_name"`
	AuthorEmail *string `json:"author_email,omitempty"`
	Body        string  `json:"body"`
	CreatedAt   string  `json:"created_at"`
}

func commentDTOToResponse(c *service.ShareCommentDTO) CommentResponse {
	return CommentResponse{
		ID:          c.ID,
		ShareID:     c.ShareID,
		AssetID:     c.AssetID,
		AuthorName:  c.AuthorName,
		AuthorEmail: c.AuthorEmail,
		Body:        c.Body,
		CreatedAt:   c.CreatedAt,
	}
}

func publicAssetDTOToResponse(a *service.PublicAssetDTO) AssetResponse {
	resp := AssetResponse{
		ID:                 a.ID,
		WorkspaceID:        a.WorkspaceID,
		ProjectID:          a.ProjectID,
		FolderID:           a.FolderID,
		DerivedFromAssetID: nil,
		OriginalFilename:   a.OriginalFilename,
		MimeType:           a.MimeType,
		Size:               a.Size,
		Width:              a.Width,
		Height:             a.Height,
		ThumbnailKey:       a.ThumbnailKey,
		Metadata:           a.Metadata,
		Tags:               []string{},
		CreatedAt:          a.CreatedAt,
		UpdatedAt:          a.UpdatedAt,
	}
	return resp
}

// loadActiveShareDTO loads and validates a share via the service layer.
// Returns 404 for missing/revoked and 410 Gone for expired.
func (s *Server) loadActiveShareDTO(c fiber.Ctx, id string) (*service.ShareDTO, error) {
	sh, err := s.sharePublic.GetActive(c.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrGone) {
			return nil, fiber.NewError(fiber.StatusGone, "share has expired")
		}
		return nil, fiber.NewError(fiber.StatusNotFound, "share not found")
	}
	return sh, nil
}

// handleShareInfo returns public metadata about a share.
//
// @Summary Get share info (public)
// @Description Returns the share's label and whether it requires a password. Use this as the first call when a visitor lands on a share link — if <code>has_password</code> is true, show a password gate before calling <code>POST /shared/:id/access</code>. Returns 404 if the share does not exist or has been revoked, and 410 if it has expired.
// @Tags Public Shares
// @Produce json
// @Param id path string true "Share ID"
// @Success 200 {object} ShareInfoResponse
// @Failure 404 {object} ErrorResponse "Share not found"
// @Failure 410 {object} ErrorResponse "Share has expired"
// @Router /shared/{id}/access [get].
func (s *Server) handleShareInfo(c fiber.Ctx) error {
	id := c.Params("id")
	sh, err := s.loadActiveShareDTO(c, id)
	if err != nil {
		return err
	}
	return c.JSON(ShareInfoResponse{
		Label:       sh.Label,
		HasPassword: sh.PasswordHash != nil,
	})
}

// handleShareAccess authenticates access to a share and returns a session token.
//
// @Summary Authenticate share access (public)
// @Description Validates the share (checks password if one is set) and issues a 24-hour share session token. Include this token as <code>Authorization: Bearer &lt;token&gt;</code> in all subsequent requests to <code>/shared/:id/*</code> endpoints. Also increments the share's view count.
// @Tags Public Shares
// @Accept json
// @Produce json
// @Param id path string true "Share ID"
// @Param body body ShareAccessRequest true "Visitor name (required) and password (if required)"
// @Success 200 {object} ShareAccessResponse
// @Failure 400 {object} ErrorResponse "visitor_name missing or too long"
// @Failure 401 {object} ErrorResponse "Password required or incorrect"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Failure 410 {object} ErrorResponse "Share has expired"
// @Router /shared/{id}/access [post].
func (s *Server) handleShareAccess(c fiber.Ctx) error {
	id := c.Params("id")

	sh, err := s.loadActiveShareDTO(c, id)
	if err != nil {
		return err
	}

	body, ok := decodeAndValidate(c, &ShareAccessRequest{})
	if !ok {
		return nil
	}

	if sh.PasswordHash != nil {
		if body.Password == "" {
			return errRes(c, fiber.StatusUnauthorized, "password required")
		}
		if err = bcrypt.CompareHashAndPassword([]byte(*sh.PasswordHash), []byte(body.Password)); err != nil {
			return errRes(c, fiber.StatusUnauthorized, "incorrect password")
		}
	}

	_ = s.sharePublic.IncrementViewCount(c.Context(), id)

	token, err := s.auth.CreateShareToken(
		sh.ID,
		sh.TargetType,
		sh.TargetID,
		sh.AllowComments,
		sh.AllowDownload,
		body.VisitorName,
		shareTokenValidityDuration,
	)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not issue share token")
	}

	return c.JSON(ShareAccessResponse{Token: token})
}

// handleShareListAssets returns the assets accessible via this share.
//
// @Summary List share assets (public)
// @Description Returns all assets accessible through the share, along with share metadata (<code>allow_comments</code>, <code>allow_download</code>, etc.). The asset set depends on the share's target type: a single asset for <code>asset</code> shares, all project assets for <code>project</code> shares. Requires a valid share session token.
// @Tags Public Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Success 200 {object} ShareAssetsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated (share token required)"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Failure 410 {object} ErrorResponse "Share has expired or been revoked"
// @Router /shared/{id}/assets [get].
func (s *Server) handleShareListAssets(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")

	if err := s.assertShareActive(c, shareID); err != nil {
		return err
	}

	dtos, err := s.sharePublic.ListAssets(c.Context(), sc.TargetType, sc.TargetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
	}

	assetIDs := make([]string, len(dtos))
	for i, d := range dtos {
		assetIDs[i] = d.ID
	}
	sharedVariants, err := s.variants.ListSharedByAssets(c.Context(), assetIDs)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list shared variants")
	}
	grouped := make(map[string][]service.SharedVariantDTO, len(assetIDs))
	for _, v := range sharedVariants {
		grouped[v.AssetID] = append(grouped[v.AssetID], v)
	}

	items := make([]AssetResponse, len(dtos))
	for i, d := range dtos {
		item := publicAssetDTOToResponse(d)
		if sv := grouped[d.ID]; len(sv) > 0 {
			item.SharedVariants = make([]SharedVariantResponse, len(sv))
			for j, v := range sv {
				item.SharedVariants[j] = sharedVariantDTOToResponse(shareID, d.ID, v)
			}
		}
		items[i] = item
	}

	sh, err := s.sharePublic.GetActive(c.Context(), shareID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load share")
	}

	sv := PublicShareView{
		ID:            sh.ID,
		Label:         sh.Label,
		AllowComments: sh.AllowComments,
		AllowDownload: sh.AllowDownload,
		HasPassword:   sh.PasswordHash != nil,
		ExpiresAt:     sh.ExpiresAt,
	}

	return c.JSON(ShareAssetsResponse{
		Share:  sv,
		Assets: items,
	})
}

// handleShareGetAsset returns a single asset within a share.
//
// @Summary Get share asset (public)
// @Description Returns a single asset accessible via the share. Returns 404 if the asset is not part of the share's target. Requires a valid share session token.
// @Tags Public Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param aid path string true "Asset ID"
// @Success 200 {object} AssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found in this share"
// @Router /shared/{id}/assets/{aid} [get].
func (s *Server) handleShareGetAsset(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")
	assetID := c.Params("aid")

	if err := s.assertShareActive(c, shareID); err != nil {
		return err
	}
	if err := s.assertAssetInShare(c, sc, assetID); err != nil {
		return err
	}

	a, err := s.sharePublic.GetAsset(c.Context(), assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(publicAssetDTOToResponse(a))
}

func (s *Server) handleShareGetVariantFile(c fiber.Ctx) error {
	ctx, span := apptelemetry.StartSpan(c.Context(), "api.shares.variant_file",
		attribute.String("damask.share_id", c.Params("id")),
		attribute.String("damask.asset_id", c.Params("aid")),
		attribute.String("damask.variant_id", c.Params("vid")),
	)
	defer apptelemetry.EndSpan(span, nil)

	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")
	assetID := c.Params("aid")
	variantID := c.Params("vid")

	if err := s.assertShareActive(c, shareID); err != nil {
		return err
	}

	if !sc.AllowDownload {
		return errRes(c, fiber.StatusForbidden, "download not allowed for this share")
	}

	ok, err := s.sharePublic.IsAssetInTarget(ctx, sc.TargetType, sc.TargetID, assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not validate share scope")
	}
	if !ok {
		return errRes(c, fiber.StatusForbidden, "asset not in share scope")
	}

	v, err := s.variants.GetSharedForShare(ctx, variantID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	rc, err := s.storage.Get(c.Context(), v.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "variant file not found")
	}

	ext := strings.ToLower(filepath.Ext(v.StorageKey))
	c.Set("Content-Type", mime.TypeByExtension(ext))
	name := sanitiseFilename(v.Title)
	if name == "" {
		name = "variant"
	}
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s%s"`, name, ext))
	if v.Size != nil && *v.Size > 0 {
		c.Set("Content-Length", strconv.FormatInt(*v.Size, 10))
	}
	s.variants.WriteVariantDownloadedAsync(v.WorkspaceID, assetID, variantID, v.Type, shareID, sc.VisitorName)
	return c.SendStream(rc)
}

func (s *Server) handleShareGetVariantThumb(c fiber.Ctx) error {
	ctx, span := apptelemetry.StartSpan(c.Context(), "api.shares.variant_thumb",
		attribute.String("damask.share_id", c.Params("id")),
		attribute.String("damask.asset_id", c.Params("aid")),
		attribute.String("damask.variant_id", c.Params("vid")),
	)
	defer apptelemetry.EndSpan(span, nil)

	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")
	assetID := c.Params("aid")
	variantID := c.Params("vid")

	if err := s.assertShareActive(c, shareID); err != nil {
		return err
	}
	if err := s.assertAssetInShare(c, sc, assetID); err != nil {
		return err
	}

	v, err := s.variants.GetSharedForShare(ctx, variantID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if v.ThumbnailKey == nil {
		return c.Redirect().To(sharedAssetThumbURL(shareID, assetID))
	}

	rc, err := s.storage.Get(c.Context(), *v.ThumbnailKey)
	if err != nil {
		return c.Redirect().To(sharedAssetThumbURL(shareID, assetID))
	}

	ct := v.ThumbnailContentType
	if ct == "" {
		ct = contentTypeImageJPEG
	}
	c.Set("Content-Type", ct)
	return c.SendStream(rc)
}

// handleShareGetAssetFile streams the original file for a shared asset.
//
// @Summary Download shared asset file (public)
// @Description Streams the original file for an asset accessible via the share. Returns 403 if the share was created with <code>allow_download: false</code>.
// @Tags Public Shares
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param aid path string true "Asset ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Download not allowed for this share"
// @Failure 404 {object} ErrorResponse "Asset or file not found"
// @Router /shared/{id}/assets/{aid}/file [get].
func (s *Server) handleShareGetAssetFile(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")
	assetID := c.Params("aid")

	if !sc.AllowDownload {
		return errRes(c, fiber.StatusForbidden, "download not allowed for this share")
	}
	if err := s.assertShareActive(c, shareID); err != nil {
		return err
	}
	if err := s.assertAssetInShare(c, sc, assetID); err != nil {
		return err
	}

	f, err := s.sharePublic.GetAssetFile(c.Context(), assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	lastMod := parseVersionTime(c.Context(), f.VersionCreatedAt)
	if setCacheHeaders(c, f.ContentHash, lastMod, false) {
		return nil
	}

	rc, err := s.storage.Get(c.Context(), f.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "file not found")
	}

	c.Set("Content-Type", f.MimeType)
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, f.OriginalFilename))
	if f.Size > 0 {
		c.Set("Content-Length", strconv.FormatInt(f.Size, 10))
	}
	return c.SendStream(rc)
}

// handleShareGetAssetThumb serves the thumbnail for a shared asset.
//
// @Summary Get shared asset thumbnail (public)
// @Description Streams the JPEG thumbnail for an asset accessible via the share. Thumbnails are always accessible regardless of the <code>allow_download</code> setting, as they are needed for review purposes. Returns 404 if the thumbnail has not yet been generated.
// @Tags Public Shares
// @Produce image/jpeg
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param aid path string true "Asset ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found or thumbnail not ready"
// @Router /shared/{id}/assets/{aid}/thumb [get].
func (s *Server) handleShareGetAssetThumb(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")
	assetID := c.Params("aid")

	if err := s.assertShareActive(c, shareID); err != nil {
		return err
	}
	if err := s.assertAssetInShare(c, sc, assetID); err != nil {
		return err
	}

	thumb, err := s.sharePublic.GetAssetThumb(c.Context(), assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	thumbETag := assetID + "_" + strconv.FormatInt(thumb.UpdatedAt.Unix(), 10)
	if setCacheHeaders(c, thumbETag, thumb.UpdatedAt, false) {
		return nil
	}

	rc, err := s.storage.Get(c.Context(), thumb.ThumbnailKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	c.Set("Content-Type", contentTypeImageJPEG)
	return c.SendStream(rc)
}

// handleShareCreateComment posts a comment on a shared asset.
//
// @Summary Post a comment (public)
// @Description Posts a comment on an asset within the share. The share must have <code>allow_comments: true</code>. Commenters identify themselves by name; an optional email can be provided. Returns 403 if comments are disabled for this share.
// @Tags Public Shares
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param body body CreateCommentRequest true "Comment content and author info"
// @Success 201 {object} CommentResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Comments not allowed for this share"
// @Failure 404 {object} ErrorResponse "Asset not found in this share"
// @Router /shared/{id}/comments [post].
func (s *Server) handleShareCreateComment(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")

	if !sc.AllowComments {
		return errRes(c, fiber.StatusForbidden, "comments not allowed for this share")
	}
	if err := s.assertShareActive(c, shareID); err != nil {
		return err
	}

	body, ok := decodeAndValidate(c, &CreateCommentRequest{})
	if !ok {
		return nil
	}
	if err := s.assertAssetInShare(c, sc, body.AssetID); err != nil {
		return err
	}

	var authorEmail *string
	if body.AuthorEmail != nil && *body.AuthorEmail != "" {
		authorEmail = body.AuthorEmail
	}

	comment, err := s.sharePublic.CreateComment(c.Context(), service.CreateShareCommentParams{
		ShareID:     shareID,
		AssetID:     body.AssetID,
		AuthorName:  body.AuthorName,
		AuthorEmail: authorEmail,
		Body:        body.Body,
	})
	if err != nil {
		slog.ErrorContext(c.Context(), "failed to create comment", apiErrorKey, err)
		return errRes(c, fiber.StatusInternalServerError, "could not create comment")
	}

	return c.Status(fiber.StatusCreated).JSON(commentDTOToResponse(comment))
}

// handleShareListComments returns all comments on a share, grouped by asset.
//
// @Summary List share comments (public)
// @Description Returns all comments on the share, grouped by <code>asset_id</code>. Useful for rendering a review dashboard showing all feedback at once.
// @Tags Public Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Success 200 {array} map[string]interface{} "Array of {asset_id, comments[]} groups"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Router /shared/{id}/comments [get].
func (s *Server) handleShareListComments(c fiber.Ctx) error {
	shareID := c.Params("id")

	if err := s.assertShareActive(c, shareID); err != nil {
		return err
	}

	dtos, err := s.sharePublic.ListCommentsByShare(c.Context(), shareID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list comments")
	}

	grouped := make(map[string][]CommentResponse)
	order := []string{}
	for _, cm := range dtos {
		if _, exists := grouped[cm.AssetID]; !exists {
			order = append(order, cm.AssetID)
			grouped[cm.AssetID] = []CommentResponse{}
		}
		grouped[cm.AssetID] = append(grouped[cm.AssetID], commentDTOToResponse(cm))
	}

	type group struct {
		AssetID  string            `json:"asset_id"`
		Comments []CommentResponse `json:"comments"`
	}
	result := make([]group, len(order))
	for i, aid := range order {
		result[i] = group{AssetID: aid, Comments: grouped[aid]}
	}

	return c.JSON(result)
}

// handleShareListAssetComments returns comments for one asset within a share.
//
// @Summary List asset comments (public)
// @Description Returns all comments posted on a specific asset within the share.
// @Tags Public Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param aid path string true "Asset ID"
// @Success 200 {array} CommentResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found in this share"
// @Router /shared/{id}/assets/{aid}/comments [get].
func (s *Server) handleShareListAssetComments(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")
	assetID := c.Params("aid")

	if err := s.assertShareActive(c, shareID); err != nil {
		return err
	}
	if err := s.assertAssetInShare(c, sc, assetID); err != nil {
		return err
	}

	dtos, err := s.sharePublic.ListCommentsByShareAndAsset(c.Context(), shareID, assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list comments")
	}

	items := make([]CommentResponse, len(dtos))
	for i, cm := range dtos {
		items[i] = commentDTOToResponse(cm)
	}
	return c.JSON(items)
}

// handleOwnerListComments returns all comments on a share for the workspace owner.
//
// @Summary List comments on a share (owner)
// @Description Returns all comments posted through the share, visible to workspace members for moderation. The authenticated user must be a member of the workspace that owns the share.
// @Tags Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Success 200 {array} CommentResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Router /api/v1/shares/{id}/comments [get].
func (s *Server) handleOwnerListComments(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	shareID := c.Params("id")

	if _, err := s.sharePublic.GetOwnerShare(c.Context(), claims.WorkspaceID, shareID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	dtos, err := s.sharePublic.ListCommentsByShare(c.Context(), shareID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list comments")
	}

	items := make([]CommentResponse, len(dtos))
	for i, cm := range dtos {
		items[i] = commentDTOToResponse(cm)
	}
	return c.JSON(items)
}

// handleOwnerDeleteComment deletes a public comment (moderation).
//
// @Summary Delete a comment (owner)
// @Description Permanently deletes a comment posted on the share. Only workspace members can use this endpoint. Use it to moderate inappropriate or spam comments.
// @Tags Shares
// @Produce json
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Param cid path string true "Comment ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Share not found"
// @Router /api/v1/shares/{id}/comments/{cid} [delete].
func (s *Server) handleOwnerDeleteComment(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	shareID := c.Params("id")
	commentID := c.Params("cid")

	if _, err := s.sharePublic.GetOwnerShare(c.Context(), claims.WorkspaceID, shareID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	if err := s.sharePublic.DeleteComment(c.Context(), shareID, commentID); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete comment")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Download shared assets as ZIP (public)
// @Description Streams a ZIP archive of all assets accessible via the share. Works for <code>asset</code>, <code>project</code>, and <code>collection</code> target types. Requires the share to have <code>allow_download: true</code> and a valid share session token. The ZIP is scoped strictly to the share's target — it cannot expose assets outside it.
// @Tags Public Shares
// @Produce application/zip
// @Security BearerAuth
// @Param id path string true "Share ID"
// @Success 200 {file} binary "ZIP archive"
// @Failure 401 {object} ErrorResponse "Not authenticated (share token required)"
// @Failure 403 {object} ErrorResponse "Download not allowed for this share"
// @Failure 404 {object} ErrorResponse "Share not found or expired"
// @Router /shared/{id}/export [get].
func (s *Server) handleShareExport(c fiber.Ctx) error {
	sc := auth.GetShareClaims(c)
	shareID := c.Params("id")

	if !sc.AllowDownload {
		return errRes(c, fiber.StatusForbidden, "download not allowed for this share")
	}
	if err := s.assertShareActive(c, shareID); err != nil {
		return err
	}

	dtos, err := s.sharePublic.ListAssets(c.Context(), sc.TargetType, sc.TargetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
	}

	exportAssetIDs := make([]string, len(dtos))
	for i, d := range dtos {
		exportAssetIDs[i] = d.ID
	}
	allSharedVariants, err := s.variants.ListSharedByAssets(c.Context(), exportAssetIDs)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list shared variants")
	}
	sharedByAsset := make(map[string][]service.SharedVariantDTO, len(exportAssetIDs))
	for _, v := range allSharedVariants {
		sharedByAsset[v.AssetID] = append(sharedByAsset[v.AssetID], v)
	}

	var entries []zipExportEntry
	for _, d := range dtos {
		shared := sharedByAsset[d.ID]
		folder := sanitiseFilename(strings.TrimSuffix(d.OriginalFilename, filepath.Ext(d.OriginalFilename)))
		if folder == "" {
			folder = d.ID
		}
		collisions := map[string]int{}
		entries = append(entries, zipExportEntry{
			name: folder + "/" + uniqueZipChildName(
				collisions,
				"original"+strings.ToLower(filepath.Ext(d.OriginalFilename)),
			),
			storageKey: d.StorageKey,
		})
		for _, v := range shared {
			ext := strings.ToLower(filepath.Ext(v.StorageKey))
			child := uniqueZipChildName(collisions, sanitiseFilename(v.Title)+ext)
			entries = append(entries, zipExportEntry{name: folder + "/" + child, storageKey: v.StorageKey})
		}
	}

	sh, err := s.sharePublic.GetActive(c.Context(), shareID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load share")
	}
	filename := sanitiseFilename(sh.Label)
	if filename == "" {
		filename = "collection-export"
	}

	c.Set("Content-Type", "application/zip")
	c.Set(
		"Content-Disposition",
		mime.FormatMediaType("attachment", map[string]string{apiFilenameKey: filename + ".zip"}),
	)

	pr, pw := io.Pipe()
	go writeShareZip( //nolint:gosec // background context is fine for this async operation
		context.Background(),
		pw,
		entries,
		s.storage,
	)

	return c.SendStream(pr)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (s *Server) assertShareActive(c fiber.Ctx, shareID string) error {
	_, err := s.sharePublic.GetActive(c.Context(), shareID)
	if err != nil {
		return fiber.NewError(fiber.StatusGone, "share has been revoked or expired")
	}
	return nil
}

func (s *Server) assertAssetInShare(c fiber.Ctx, sc *auth.ShareClaims, assetID string) error {
	ok, err := s.sharePublic.IsAssetInTarget(c.Context(), sc.TargetType, sc.TargetID, assetID)
	if err != nil || !ok {
		return fiber.NewError(fiber.StatusNotFound, "asset not found in this share")
	}
	return nil
}

func uniqueZipChildName(counts map[string]int, name string) string {
	if name == "" {
		name = "file"
	}
	counts[name]++
	if counts[name] == 1 {
		return name
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s_%d%s", stem, counts[name], ext)
}

type zipExportEntry struct {
	name       string
	storageKey string
}

type shareZipStorage interface {
	Get(ctx context.Context, key string) (io.ReadCloser, error)
}

func writeShareZip(ctx context.Context, pw *io.PipeWriter, entries []zipExportEntry, storage shareZipStorage) {
	zw := zip.NewWriter(pw)
	var missing []string
	for _, e := range entries {
		rc, getErr := storage.Get(ctx, e.storageKey)
		if getErr != nil {
			missing = append(missing, e.name)
			continue
		}
		fw, createErr := zw.Create(e.name)
		if createErr != nil {
			_ = rc.Close()
			missing = append(missing, e.name)
			continue
		}
		if _, copyErr := io.Copy(fw, rc); copyErr != nil {
			slog.WarnContext(ctx, "share zip copy error", "name", e.name, "err", copyErr)
		}
		_ = rc.Close()
	}
	if len(missing) > 0 {
		if fw, createErr := zw.Create("_missing_files.txt"); createErr == nil {
			for _, n := range missing {
				_, _ = fmt.Fprintln(fw, n)
			}
		}
	}
	if closeErr := zw.Close(); closeErr != nil {
		_ = pw.CloseWithError(closeErr)
	} else {
		_ = pw.Close()
	}
}
