package api

import (
	"context"
	"io"
	"mime"
	"strings"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

// @Summary Export stack as ZIP
// @Description Streams a ZIP archive containing the original files for the given asset IDs. All assets must belong to the authenticated workspace. Files are named after their original filenames; duplicates are suffixed with <code>_N</code>. If any file is missing from storage it is skipped and listed in a <code>_missing_files.txt</code> manifest inside the ZIP. The response is streamed — do not buffer the entire body before processing.
// @Tags Stack
// @Accept json
// @Produce application/zip
// @Security BearerAuth
// @Param body body stackExportRequest true "Asset IDs and optional filename"
// @Success 200 {file} binary "ZIP archive"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "One or more assets not in workspace"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/stack/export [post].
func (s *Server) handleStackExport(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &stackExportRequest{})
	if !ok {
		return nil
	}

	// Verify all assets belong to this workspace before sending any headers.
	if allOk, err := s.allAssetsInWorkspace(c.Context(), claims.WorkspaceID, body.AssetIDs); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not verify assets")
	} else if !allOk {
		return errRes(c, fiber.StatusForbidden, "one or more assets not found in workspace")
	}

	filename := sanitiseFilename(body.Filename)
	if filename == "" {
		filename = "stack-export"
	}

	c.Set("Content-Type", "application/zip")
	c.Set(
		"Content-Disposition",
		mime.FormatMediaType("attachment", map[string]string{apiFilenameKey: filename + ".zip"}),
	)

	workspaceID := claims.WorkspaceID
	params := service.ExportZipParams{AssetIDs: body.AssetIDs, Filename: body.Filename, VariantMode: body.VariantMode}

	// c.Context() is never cancelled by fiber on client disconnect; derive a
	// cancellable context and cancel it when fasthttp closes the stream body
	// (disconnect or completion) so ExportZip stops writing doomed entries.
	ctx, cancel := context.WithCancel(c.Context())

	pr, pw := io.Pipe()
	go func() {
		err := s.stack.ExportZip(ctx, workspaceID, params, pw)
		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()

	return c.SendStream(&cancelReadCloser{Reader: pr, close: func() error {
		cancel()
		return pr.Close()
	}})
}

// cancelReadCloser cancels the export context when the response stream is closed.
type cancelReadCloser struct {
	io.Reader

	close func() error
}

func (c *cancelReadCloser) Close() error { return c.close() }

// sanitiseFilename strips path separators and ASCII control characters from a
// filename so it is safe to use in a Content-Disposition header.
func sanitiseFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '/' || r == '\\' || r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
