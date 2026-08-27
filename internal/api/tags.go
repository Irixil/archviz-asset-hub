package api

import (
	"errors"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/agnivade/levenshtein"
	"github.com/gofiber/fiber/v3"
)

const maxSimilarityScore = 0.3

var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// TagResponse is the extended shape returned by all tag endpoints.
type TagResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	AssetCount int64   `json:"asset_count"`
	Color      *string `json:"color,omitempty"`
	GroupName  *string `json:"group_name,omitempty"`
	IsSystem   bool    `json:"is_system"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
}

// BulkDeleteTagsResponse is the result of a bulk tag delete operation.
type BulkDeleteTagsResponse struct {
	Deleted           int   `json:"deleted"`
	RemovedFromAssets int64 `json:"removed_from_assets"`
}

// MergeTagsResponse is the result of a tag merge operation.
type MergeTagsResponse struct {
	MergedAssets int64       `json:"merged_assets"`
	Target       TagResponse `json:"target"`
}

// DuplicateTagPairResponse is a pair of near-duplicate tag names with a similarity score.
type DuplicateTagPairResponse struct {
	A     string  `json:"a"`
	B     string  `json:"b"`
	Score float64 `json:"score"`
}

func tagDTOToResponse(d *service.TagDTO) TagResponse {
	r := TagResponse{
		ID:         d.ID,
		Name:       d.Name,
		AssetCount: d.AssetCount,
		Color:      d.Color,
		GroupName:  d.GroupName,
		IsSystem:   d.GroupName != nil && *d.GroupName == "system",
		CreatedAt:  d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if d.LastUsedAt != nil {
		s := d.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.LastUsedAt = &s
	}
	return r
}

// handleListTags handles GET /api/v1/tags
//
// @Summary List tags
// @Tags Tags
// @Produce json
// @Security BearerAuth
// @Success 200 {array} TagResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/tags [get].
func (s *Server) handleListTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	includeSystem, _ := strconv.ParseBool(c.Query("system", "false"))
	hideEmpty, _ := strconv.ParseBool(c.Query("hide_empty", "false"))

	ws, err := getLocalWorkspace(c, s.workspace)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if ws.LockedTaxonomy {
		member, merr := getLocalMember(c, s.workspace)
		if merr != nil || member.Role != "owner" {
			hideEmpty = true
		}
	}

	dtos, err := s.tags.List(c.Context(), claims.WorkspaceID, includeSystem)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	items := make([]TagResponse, 0, len(dtos))
	for _, d := range dtos {
		if hideEmpty && d.AssetCount == 0 {
			continue
		}
		items = append(items, tagDTOToResponse(d))
	}
	return c.JSON(items)
}

// handleCreateTag handles POST /api/v1/tags
//
// @Summary Create a tag
// @Tags Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createTagRequest true "Tag details"
// @Success 201 {object} TagResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 409 {object} ErrorResponse "Tag already exists"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/tags [post].
func (s *Server) handleCreateTag(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &createTagRequest{})
	if !ok {
		return nil
	}

	ws, err := getLocalWorkspace(c, s.workspace)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if ws.LockedTaxonomy {
		member, merr := getLocalMember(c, s.workspace)
		if merr != nil || member.Role != "owner" {
			return fiber.NewError(fiber.StatusForbidden, "taxonomy_locked")
		}
	}

	dto, err := s.tags.Create(c.Context(), claims.WorkspaceID, service.CreateTagParams{
		Name:      body.Name,
		Color:     body.Color,
		GroupName: body.GroupName,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(tagDTOToResponse(dto))
}

// handlePatchTag handles PATCH /api/v1/tags/:name
// Kept on s.db: the service Patch does not yet update color/group_name.
//
// @Summary Update a tag
// @Tags Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param name path string true "Current tag name"
// @Param body body patchTagRequest true "Fields to update"
// @Success 200 {object} TagResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Tag not found"
// @Failure 409 {object} ErrorResponse "Target name already in use"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/tags/{name} [patch].
func (s *Server) handlePatchTag(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	tagName := strings.ToLower(c.Params("name"))

	body, ok := decodeAndValidate(c, &patchTagRequest{})
	if !ok {
		return nil
	}

	dto, err := s.tags.Patch(c.Context(), claims.WorkspaceID, tagName, service.PatchTagParams{
		Name:      body.Name,
		Color:     body.Color,
		GroupName: body.GroupName,
	})
	if err != nil {
		if errors.Is(err, service.ErrSystemTagProtected) {
			return errRes(c, fiber.StatusUnprocessableEntity, "system_tag_protected")
		}
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(tagDTOToResponse(dto))
}

// handleBulkDeleteTags handles DELETE /api/v1/tags
// Kept on s.db: returns removed_from_assets count not covered by service.Delete.
//
// @Summary Bulk delete tags
// @Tags Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body bulkDeleteTagsRequest true "Tag names to delete"
// @Success 200 {object} BulkDeleteTagsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/tags [delete].
func (s *Server) handleBulkDeleteTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &bulkDeleteTagsRequest{})
	if !ok {
		return nil
	}

	result, err := s.tags.BulkDelete(c.Context(), claims.WorkspaceID, body.Names)
	if err != nil {
		if errors.Is(err, service.ErrSystemTagProtected) {
			return errRes(c, fiber.StatusUnprocessableEntity, "system_tag_protected")
		}
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(BulkDeleteTagsResponse{
		Deleted:           result.Deleted,
		RemovedFromAssets: result.RemovedFromAssets,
	})
}

// handleMergeTags handles POST /api/v1/tags/merge
// Kept on s.db: complex transaction with reassign + delete not yet in service.
//
// @Summary Merge tags
// @Tags Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body mergeTagsRequest true "Source tag names and target tag name"
// @Success 200 {object} MergeTagsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/tags/merge [post].
func (s *Server) handleMergeTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &mergeTagsRequest{})
	if !ok {
		return nil
	}

	result, err := s.tags.Merge(c.Context(), claims.WorkspaceID, body.Sources, body.Target)
	if err != nil {
		if errors.Is(err, service.ErrSystemTagProtected) {
			return errRes(c, fiber.StatusUnprocessableEntity, "system_tag_protected")
		}
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(MergeTagsResponse{
		MergedAssets: result.MergedAssets,
		Target:       tagDTOToResponse(result.Target),
	})
}

// handleTagDuplicateSuggestions handles GET /api/v1/tags/suggestions/duplicates
// Kept on s.db: needs AssetCount from ListTagsWithCount, not yet fully in service.
//
// @Summary Get duplicate tag suggestions
// @Tags Tags
// @Produce json
// @Security BearerAuth
// @Success 200 {array} DuplicateTagPairResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/tags/suggestions/duplicates [get].
func (s *Server) handleTagDuplicateSuggestions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	dtos, err := s.tags.List(c.Context(), claims.WorkspaceID, true)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	active := make([]*service.TagDTO, 0, len(dtos))
	for _, d := range dtos {
		if d.AssetCount > 0 {
			active = append(active, d)
		}
	}

	var pairs []DuplicateTagPairResponse

	for i := 0; i < len(active) && len(pairs) < 20; i++ {
		for j := i + 1; j < len(active) && len(pairs) < 20; j++ {
			a := strings.ToLower(active[i].Name)
			b := strings.ToLower(active[j].Name)
			maxLen := math.Max(float64(utf8.RuneCountInString(a)), float64(utf8.RuneCountInString(b)))
			if maxLen == 0 {
				continue
			}
			dist := levenshtein.ComputeDistance(a, b)
			score := float64(dist) / maxLen
			if score < maxSimilarityScore {
				pairs = append(pairs, DuplicateTagPairResponse{
					A:     active[i].Name,
					B:     active[j].Name,
					Score: math.Round(score*100) / 100, //nolint:mnd // round to 2 decimals
				})
			}
		}
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Score < pairs[j].Score })

	if pairs == nil {
		pairs = []DuplicateTagPairResponse{}
	}
	return c.JSON(pairs)
}

// handleGetAssetTags handles GET /api/v1/assets/:id/tags
//
// @Summary Get tags for an asset
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {array} string
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/tags [get].
func (s *Server) handleGetAssetTags(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	tagDTOs, err := s.tags.ListForAsset(c.Context(), assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	names := make([]string, len(tagDTOs))
	for i, t := range tagDTOs {
		names[i] = t.Name
	}
	return c.JSON(names)
}

// handleAddTagToAsset handles POST /api/v1/assets/:id/tags
//
// @Summary Add tag to an asset
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body AddTagRequest true "Tag name"
// @Success 201 {object} map[string]string
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/{id}/tags [post].
func (s *Server) handleAddTagToAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	body, ok := decodeAndValidate(c, &AddTagRequest{})
	if !ok {
		return nil
	}

	ws, err := getLocalWorkspace(c, s.workspace)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if ws.LockedTaxonomy {
		tagName := strings.ToLower(strings.TrimSpace(body.Name))
		_, tagErr := s.tags.GetByName(c.Context(), claims.WorkspaceID, tagName)
		if tagErr != nil {
			member, merr := getLocalMember(c, s.workspace)
			if merr != nil || member.Role != string(auth.Owner) {
				return fiber.NewError(fiber.StatusUnprocessableEntity, "tag_not_in_vocabulary")
			}
		}
	}

	tag, err := s.tags.AddToAsset(c.Context(), claims.WorkspaceID, assetID, body.Name)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	_ = s.tags.TouchLastUsed(c.Context(), claims.WorkspaceID, tag.Name)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"name": tag.Name})
}

// handleRemoveTagFromAsset handles DELETE /api/v1/assets/:id/tags/:name
//
// @Summary Remove tag from an asset
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param name path string true "Tag name"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/tags/{name} [delete].
func (s *Server) handleRemoveTagFromAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	tagName := strings.ToLower(c.Params("name"))

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	if err := s.tags.RemoveFromAsset(c.Context(), claims.WorkspaceID, assetID, tagName); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}
