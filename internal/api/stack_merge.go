package api

import (
	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

// @Summary Merge stack assets into GIF or PDF
// @Description Enqueues an async job that merges the given assets into a single animated GIF or PDF contact sheet. All assets must belong to the authenticated workspace. Returns 202 immediately with a <code>job_id</code>; listen for event <code>stack_merge_done</code> to check completion. On success the job result contains the new asset ID.
// @Tags Stack
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body stackMergeRequest true "Asset IDs, output type and options"
// @Success 202 {object} map[string]string "job_id"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "One or more assets not in workspace"
// @Failure 422 {object} ValidationErrorResponse "Validation failed (e.g. output_type not gif/pdf)"
// @Router /api/v1/stack/merge [post].
func (s *Server) handleStackMerge(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &stackMergeRequest{})
	if !ok {
		return nil
	}

	jobID, err := s.stack.EnqueueMerge(c.Context(), claims.WorkspaceID, claims.UserID, service.MergeParams{
		AssetIDs:       body.AssetIDs,
		OutputType:     body.OutputType,
		OutputFilename: body.Filename,
		GifFrameMs:     body.GifFrameMs,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{apiJobIDKey: jobID})
}
