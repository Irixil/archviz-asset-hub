package api

import (
	"errors"
	"mime"

	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

const embedThumbRetryAfterSeconds = 5

// resolvePublicEmbed loads the token's current-version file metadata, mapping
// service.ErrGone to 410 (revoked) and anything else to 404 (unknown token).
// 410 must be distinguished from 404 here — ErrorStatusResponse has no Gone
// case, so a revoked token must never reach it.
func (s *Server) resolvePublicEmbed(c fiber.Ctx, tokenParam string) (*service.ResolvedEmbed, error) {
	resolved, err := s.embedTokens.ResolveCurrentFile(c.Context(), service.ExtractTokenID(tokenParam))
	if err != nil {
		if errors.Is(err, service.ErrGone) {
			return nil, fiber.NewError(fiber.StatusGone, "link_revoked")
		}
		return nil, fiber.NewError(fiber.StatusNotFound, "embed token not found")
	}
	return resolved, nil
}

// setEmbedCacheHeaders sets Cache-Control: public, no-cache and an ETag derived
// from the version content hash, then checks If-None-Match. Unlike
// setCacheHeaders (private, max-age), this directive lets CDNs cache the
// response but forces revalidation on every request — required so that an
// embed link always reflects whichever version is current.
// Returns true if the response is a 304 (caller must return nil without a body).
func setEmbedCacheHeaders(c fiber.Ctx, etag string) bool {
	c.Set("Cache-Control", "public, no-cache")
	c.Set("ETag", `"`+etag+`"`)
	if inm := c.Get("If-None-Match"); inm != "" && inm == `"`+etag+`"` {
		c.Status(fiber.StatusNotModified)
		return true
	}
	return false
}

// handlePublicEmbedFile streams the current-version file for a public embed token.
//
// @Summary Stream the current-version file for a public embed token
// @Description Unauthenticated. Always serves whatever version is current at request time. Returns Cache-Control: public, no-cache so CDNs revalidate on every request, but sets ETag from the version's content hash so a CDN can serve a 304 when the content hasn't changed.
// @Tags Public Embed
// @Param token path string true "16-char base62 embed token"
// @Success 200 {file} binary
// @Success 304 "Not Modified (ETag matched)"
// @Failure 404 {object} ErrorResponse "Token not found"
// @Failure 410 {object} ErrorResponse "Token revoked"
// @Router /e/{token} [get].
func (s *Server) handlePublicEmbedFile(c fiber.Ctx) error {
	resolved, err := s.resolvePublicEmbed(c, c.Params("token"))
	if err != nil {
		return err
	}

	if setEmbedCacheHeaders(c, resolved.ContentHash) {
		return nil
	}

	rc, err := s.storage.Get(c.Context(), resolved.StorageKey)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not read file")
	}

	c.Set("Content-Type", resolved.MimeType)
	c.Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{apiFilenameKey: resolved.Filename}))
	return c.SendStream(rc)
}

// handlePublicEmbedThumb streams the current-version thumbnail for a public embed token.
//
// @Summary Stream the current-version thumbnail for a public embed token
// @Description Unauthenticated. Same token resolution and ETag semantics as GET /e/:token. Returns 202 with a Retry-After header when the thumbnail has not finished generating yet — this is a normal, expected state for recently uploaded assets, not an error.
// @Tags Public Embed
// @Param token path string true "16-char base62 embed token"
// @Success 200 {file} binary
// @Success 202 {object} map[string]interface{} "Thumbnail not ready yet"
// @Success 304 "Not Modified (ETag matched)"
// @Failure 404 {object} ErrorResponse "Token not found"
// @Failure 410 {object} ErrorResponse "Token revoked"
// @Router /e/{token}/thumb [get].
func (s *Server) handlePublicEmbedThumb(c fiber.Ctx) error {
	resolved, err := s.resolvePublicEmbed(c, c.Params("token"))
	if err != nil {
		return err
	}

	if resolved.ThumbnailKey == nil {
		c.Set("Retry-After", "5")
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"status":      "pending",
			"retry_after": embedThumbRetryAfterSeconds,
		})
	}

	if setEmbedCacheHeaders(c, resolved.ContentHash) {
		return nil
	}

	rc, err := s.storage.Get(c.Context(), *resolved.ThumbnailKey)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not read thumbnail")
	}

	ct := resolved.ThumbnailContentType
	if ct == "" {
		ct = contentTypeImageJPEG
	}
	c.Set("Content-Type", ct)
	return c.SendStream(rc)
}
