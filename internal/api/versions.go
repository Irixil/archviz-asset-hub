package api

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

const maxCommentLength = 500

type VersionCreatedByResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type VersionResponse struct {
	ID           string                    `json:"id"`
	VersionNum   int64                     `json:"version_num"`
	MimeType     string                    `json:"mime_type"`
	Size         int64                     `json:"size"`
	Width        *int64                    `json:"width,omitempty"`
	Height       *int64                    `json:"height,omitempty"`
	DurationSec  *float64                  `json:"duration_sec,omitempty"`
	ThumbnailURL *string                   `json:"thumbnail_url,omitempty"`
	Comment      *string                   `json:"comment,omitempty"`
	CreatedBy    *VersionCreatedByResponse `json:"created_by,omitempty"`
	CreatedAt    string                    `json:"created_at"`
	IsCurrent    bool                      `json:"is_current"`
	VariantCount int64                     `json:"variant_count"`
}

// VersionWithAssetResponse is returned by upload and restore endpoints.
type VersionWithAssetResponse struct {
	Version VersionResponse `json:"version"`
	Asset   AssetResponse   `json:"asset"`
}

func (s *Server) resolveCreator(ctx context.Context, userID string) *VersionCreatedByResponse {
	resp := &VersionCreatedByResponse{ID: userID}
	if dto, err := s.users.GetByID(ctx, userID); err == nil {
		resp.Name = dto.Name
	}
	return resp
}

func versionDTOToResponse(v *service.VersionDTO, createdBy *VersionCreatedByResponse) VersionResponse {
	var thumbURL *string
	if v.ThumbnailKey != nil {
		u := versionThumbURL(v.AssetID, v.ID)
		thumbURL = &u
	}
	return VersionResponse{
		ID:           v.ID,
		VersionNum:   v.VersionNum,
		MimeType:     v.MimeType,
		Size:         v.Size,
		Width:        v.Width,
		Height:       v.Height,
		DurationSec:  v.DurationSec,
		ThumbnailURL: thumbURL,
		Comment:      v.Comment,
		CreatedBy:    createdBy,
		CreatedAt:    v.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		IsCurrent:    v.IsCurrent,
	}
}

func versionWithCountDTOToResponse(
	v *service.VersionWithCountDTO,
	createdBy *VersionCreatedByResponse,
) VersionResponse {
	r := versionDTOToResponse(&v.VersionDTO, createdBy)
	r.VariantCount = v.VariantCount
	return r
}

// --- AV-1.3: Upload new version ---

// handleUploadAssetVersion uploads a new version of an asset.
//
// @Summary Upload a new version
// @Description Uploads a new file as the next version of an existing asset. The new version is immediately promoted to current. The previous current version is retained in history.<br><br> After upload, the server: <ul> <li>Enqueues a thumbnail generation job for the new version.</li> <li>Enqueues a variant rebuild job that recreates all variants from the previous current version.</li> <li>Clears the asset's cached thumbnail (pending the new job).</li> </ul> If the uploaded file is byte-for-byte identical to the current version (same content hash), the request is rejected with 409 to prevent no-op versions.<br><br> The response body contains both the new <code>version</code> object and the updated <code>asset</code> object.
// @Tags Versions
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param file formData file true "New version file"
// @Param comment formData string false "Optional change note (max 500 characters)"
// @Success 201 {object} VersionWithAssetResponse
// @Failure 400 {object} ErrorResponse "Missing file field"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 409 {object} ErrorResponse "File is identical to the current version"
// @Failure 422 {object} ErrorResponse "Comment too long"
// @Router /api/v1/assets/{id}/versions [post].
func (s *Server) handleUploadAssetVersion(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	fh, err := c.FormFile("file")
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "file field is required")
	}

	comment := strings.TrimSpace(c.FormValue("comment"))
	if len(comment) > maxCommentLength {
		return errRes(c, fiber.StatusUnprocessableEntity, "comment must be 500 characters or fewer")
	}

	f, err := fh.Open()
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not open uploaded file")
	}
	defer f.Close()

	result, err := s.versions.UploadNewVersion(c.Context(), service.UploadAssetVersionParams{
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		Filename:    fh.Filename,
		ContentType: fh.Header.Get("Content-Type"),
		Comment:     comment,
		UserID:      claims.UserID,
		Reader:      f,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	var createdBy *VersionCreatedByResponse
	if result.Version.CreatedBy != nil {
		createdBy = s.resolveCreator(c.Context(), *result.Version.CreatedBy)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"version":      versionDTOToResponse(result.Version, createdBy),
		apiTargetAsset: assetToResponse(dtoToDBAsset(result.Asset)),
	})
}

// --- AV-1.4: List versions ---

// handleListAssetVersions returns all versions of an asset.
//
// @Summary List asset versions
// @Description Returns all versions of an asset in reverse chronological order. Each version includes its <code>version_num</code>, MIME type, dimensions, file size, optional comment, and the number of variants derived from it (<code>variant_count</code>). The current version has <code>is_current: true</code>.
// @Tags Versions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {array} VersionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/versions [get].
func (s *Server) handleListAssetVersions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	versions, err := s.versions.ListWithVariantCount(c.Context(), assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	// Batch-resolve creator names to avoid N+1 queries.
	userNames := make(map[string]string)
	for _, v := range versions {
		if v.CreatedBy != nil {
			if _, seen := userNames[*v.CreatedBy]; !seen {
				userNames[*v.CreatedBy] = ""
				if u, userErr := s.users.GetByID(c.Context(), *v.CreatedBy); userErr == nil {
					userNames[*v.CreatedBy] = u.Name
				}
			}
		}
	}

	resp := make([]VersionResponse, len(versions))
	for i, v := range versions {
		var createdBy *VersionCreatedByResponse
		if v.CreatedBy != nil {
			createdBy = &VersionCreatedByResponse{
				ID:   *v.CreatedBy,
				Name: userNames[*v.CreatedBy],
			}
		}
		resp[i] = versionWithCountDTOToResponse(v, createdBy)
	}
	return c.JSON(resp)
}

// --- AV-1.5: Restore (rollback) ---

// handleRestoreAssetVersion restores an older version to current.
//
// @Summary Restore a version
// @Description Promotes a non-current version to be the current version of the asset. The asset's thumbnail is also updated to match the restored version. An audit event records the version numbers rolled back from and to.<br><br> Returns 409 if the requested version is already current. Returns 422 if the version has been soft-deleted. The response contains the updated <code>version</code> and <code>asset</code>.
// @Tags Versions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Version ID to restore"
// @Success 200 {object} VersionWithAssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or version not found"
// @Failure 409 {object} ErrorResponse "Version is already current"
// @Failure 422 {object} ErrorResponse "Cannot restore a deleted version"
// @Router /api/v1/assets/{id}/versions/{vid}/restore [post].
func (s *Server) handleRestoreAssetVersion(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	versionID := c.Params("vid")

	assetBeforeRestore, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	target, err := s.versions.Get(c.Context(), claims.WorkspaceID, versionID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if target.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "version not found")
	}
	if target.DeletedAt != nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "cannot restore a deleted version")
	}
	if target.IsCurrent {
		return errRes(c, fiber.StatusConflict, "version is already current")
	}

	if err = s.versions.SetCurrent(c.Context(), assetID, versionID); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not restore version")
	}

	if err = s.versions.SetAssetThumbnail(c.Context(), assetID, target.ThumbnailKey); err != nil {
		slog.ErrorContext(c.Context(), "restore: sync thumbnail", apiErrorKey, err)
	}

	updatedAsset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload asset")
	}

	var fromVersionNum int64
	if assetBeforeRestore.CurrentVersionID != nil {
		if prev, prevErr := s.versions.Get(
			c.Context(),
			claims.WorkspaceID,
			*assetBeforeRestore.CurrentVersionID,
		); prevErr == nil {
			fromVersionNum = prev.VersionNum
		}
	}

	s.versions.WriteVersionRestored(c.Context(), claims.WorkspaceID, assetID, fromVersionNum, target.VersionNum)

	target.IsCurrent = true
	var createdBy *VersionCreatedByResponse
	if target.CreatedBy != nil {
		createdBy = s.resolveCreator(c.Context(), *target.CreatedBy)
	}
	return c.JSON(fiber.Map{
		"version":      versionDTOToResponse(target, createdBy),
		apiTargetAsset: assetToResponse(dtoToDBAsset(updatedAsset)),
	})
}

// --- AV-1.6: Soft-delete version ---

// handleDeleteAssetVersion soft-deletes a non-current version.
//
// @Summary Delete a version
// @Description Soft-deletes an asset version. The current version cannot be deleted — restore a different version first, then delete the old one. A version also cannot be deleted while it is in use as a project cover or workspace icon (returns 409).<br><br> Soft deletion marks the row as deleted; the storage file is not immediately removed (physical cleanup is handled by a background retention job).
// @Tags Versions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Version ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or version not found"
// @Failure 409 {object} ErrorResponse "Version is in use as a cover/icon"
// @Failure 422 {object} ErrorResponse "Cannot delete the current version"
// @Router /api/v1/assets/{id}/versions/{vid} [delete].
func (s *Server) handleDeleteAssetVersion(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	versionID := c.Params("vid")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	target, err := s.versions.Get(c.Context(), claims.WorkspaceID, versionID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if target.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "version not found")
	}

	if err = s.versions.Delete(c.Context(), claims.WorkspaceID, assetID, versionID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// --- AV-1.7: Stream version file and thumbnail ---

// handleGetVersionFile streams the original file for a specific version.
//
// @Summary Get version file
// @Description Streams the raw file for the specified version. The response MIME type and <code>Content-Disposition</code> are set from the stored version metadata.
// @Tags Versions
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Version ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset, version, or file not found"
// @Router /api/v1/assets/{id}/versions/{vid}/file [get].
func (s *Server) handleGetVersionFile(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	versionID := c.Params("vid")

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	target, err := s.versions.Get(c.Context(), claims.WorkspaceID, versionID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if target.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "version not found")
	}

	if setCacheHeaders(c, target.ContentHash, target.CreatedAt, true) {
		return nil
	}

	rc, err := s.storage.Get(c.Context(), target.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "file not found")
	}

	c.Set("Content-Type", target.MimeType)
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, asset.OriginalFilename))
	if target.Size > 0 {
		c.Set("Content-Length", strconv.FormatInt(target.Size, 10))
	}
	return c.SendStream(rc)
}

// handleGetVersionThumb serves the JPEG thumbnail for a specific version.
//
// @Summary Get version thumbnail
// @Description Returns the JPEG thumbnail for the given version. Returns 404 if the thumbnail has not yet been generated (the job runs asynchronously after upload — poll until the <code>thumbnail_url</code> field in the version response is non-null).
// @Tags Versions
// @Produce image/jpeg
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param vid path string true "Version ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset, version, or thumbnail not ready"
// @Router /api/v1/assets/{id}/versions/{vid}/thumb [get].
func (s *Server) handleGetVersionThumb(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	versionID := c.Params("vid")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	target, err := s.versions.Get(c.Context(), claims.WorkspaceID, versionID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if target.AssetID != assetID {
		return errRes(c, fiber.StatusNotFound, "version not found")
	}
	if target.ThumbnailKey == nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not ready")
	}

	thumbETag := target.ID + "_thumb"
	if setCacheHeaders(c, thumbETag, target.CreatedAt, true) {
		return nil
	}

	rc, err := s.storage.Get(c.Context(), *target.ThumbnailKey)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	c.Set("Content-Type", contentTypeImageJPEG)
	return c.SendStream(rc)
}
