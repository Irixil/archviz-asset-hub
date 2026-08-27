package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"damask/server/internal/assetio"
	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

const maxBulkFieldSize = 20

// hasFieldFilters returns true if the request contains any field[...] query params.
func hasFieldFilters(c fiber.Ctx) bool {
	for k := range c.Queries() {
		if strings.HasPrefix(k, "field[") {
			return true
		}
	}
	return false
}

// fieldFilterDef represents a parsed field[key][op]=value query param.
type fieldFilterDef struct {
	key      string
	operator string
	value    string
}

// newInheritProjectFieldsFunc returns an assetio.FieldInheritanceFunc that uses
// s.fields (FieldService) instead of *dbgen.Queries.
func (s *Server) newInheritProjectFieldsFunc() assetio.FieldInheritanceFunc {
	return func(ctx context.Context, workspaceID, assetID, projectID, userID string) {
		if err := s.fields.InheritProjectFields(ctx, workspaceID, assetID, projectID, userID); err != nil {
			slog.ErrorContext(ctx, "field inheritance", "workspace_id", workspaceID, "asset_id", assetID,
				"project_id", projectID, apiErrorKey, err)
		}
	}
}

var fieldParamRe = regexp.MustCompile(`^field\[([a-z0-9_]+)\](?:\[([a-z_]+)\])?$`)

func parseFieldFilters(c fiber.Ctx) []fieldFilterDef {
	var filters []fieldFilterDef
	seen := map[string]bool{}

	for k, v := range c.Queries() {
		m := fieldParamRe.FindStringSubmatch(k)
		if m == nil {
			continue
		}
		key := m[1]
		op := "eq"
		if len(m[2]) > 0 {
			op = m[2]
		}
		switch op {
		case "eq", "lt", "lte", "gt", "gte", "contains", "starts_with":
		default:
			continue
		}
		dedup := key + ":" + op
		if seen[dedup] {
			continue
		}
		seen[dedup] = true
		filters = append(filters, fieldFilterDef{key: key, operator: op, value: v})
	}
	return filters
}

const maxFieldFilters = 5

func (s *Server) handleListAssetsByFields(c fiber.Ctx, workspaceID string, limit int64) error {
	defs := parseFieldFilters(c)
	if len(defs) > maxFieldFilters {
		return errRes(
			c,
			fiber.StatusUnprocessableEntity,
			fmt.Sprintf("maximum of %d field filters allowed", maxFieldFilters),
		)
	}
	if len(defs) == 0 {
		return errRes(c, fiber.StatusBadRequest, "no valid field filters provided")
	}

	svcFilters := make([]service.FieldFilter, len(defs))
	for i, f := range defs {
		svcFilters[i] = service.FieldFilter{Key: f.key, Operator: f.operator, Value: f.value}
	}

	var cursorAt *string
	var cursorID *string
	if cursor := c.Query("cursor"); cursor != "" {
		cv, err := decodeCursor(cursor)
		if err == nil {
			cursorAt = &cv.Value
			cursorID = &cv.ID
		}
	}

	assets, err := s.assets.ListByFields(c.Context(), service.ListAssetsByFieldsParams{
		WorkspaceID:  workspaceID,
		FieldFilters: svcFilters,
		CursorAt:     cursorAt,
		CursorID:     cursorID,
		Limit:        limit,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	// Convert service DTOs to the existing response type via dbgen shim.
	// batchVersionCounts / batchVariantCounts still use s.db; that is handled
	// by the assets handler layer which calls those helpers directly.
	// For now return the slim asset list without counts (consistent with other list paths).
	return c.JSON(buildAssetListResponseFromDTOs(assets, limit, apiCreatedAtField, nil, nil, nil))
}

var keyRegexp = regexp.MustCompile(`^[a-z0-9_]+$`)

type FieldDefinitionResponse struct {
	ID                 string  `json:"id"`
	WorkspaceID        string  `json:"workspace_id"`
	Scope              string  `json:"scope"`
	Name               string  `json:"name"`
	Key                string  `json:"key"`
	FieldType          string  `json:"field_type"`
	Options            *string `json:"options,omitempty"`
	Required           bool    `json:"required"`
	Position           int64   `json:"position"`
	InheritFromProject bool    `json:"inherit_from_project"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	DeletedAt          *string `json:"deleted_at,omitempty"`
}

type FieldDefinitionStatsResponse struct {
	AssetCount   int64 `json:"asset_count"`
	ProjectCount int64 `json:"project_count"`
}

func fieldDTOToResponse(f *service.FieldDefinitionDTO) FieldDefinitionResponse {
	return FieldDefinitionResponse{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		Scope:              f.Scope,
		Name:               f.Name,
		Key:                f.Key,
		FieldType:          f.FieldType,
		Options:            f.Options,
		Required:           f.Required,
		Position:           f.Position,
		InheritFromProject: f.InheritFromProject,
		CreatedAt:          f.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          f.UpdatedAt.Format(time.RFC3339),
		DeletedAt:          f.DeletedAt,
	}
}

// @Summary List field definitions
// @Description Returns all custom field definitions for the workspace filtered by scope.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param scope query string false "Scope filter: asset (default) or project"
// @Success 200 {array} FieldDefinitionResponse
// @Failure 400 {object} ErrorResponse "Invalid scope value"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/field-definitions [get].
func (s *Server) handleListFieldDefinitions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	scope := c.Query("scope", apiTargetAsset)
	if scope != apiTargetAsset && scope != apiTargetProject {
		return errRes(c, fiber.StatusBadRequest, "scope must be 'asset' or 'project'")
	}

	defs, err := s.fields.List(c.Context(), claims.WorkspaceID, scope)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	items := make([]FieldDefinitionResponse, len(defs))
	for i, d := range defs {
		items[i] = fieldDTOToResponse(d)
	}
	return c.JSON(items)
}

// @Summary Create a field definition
// @Description Creates a new custom field definition for assets or projects.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateFieldDefinitionRequest true "Field definition"
// @Success 201 {object} FieldDefinitionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 409 {object} ErrorResponse "Key already exists in this scope"
// @Failure 422 {object} ValidationErrorResponse "Validation failed or 50-field limit reached"
// @Router /api/v1/field-definitions [post].
func (s *Server) handleCreateFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &CreateFieldDefinitionRequest{})
	if !ok {
		return nil
	}

	def, err := s.fields.Create(c.Context(), claims.WorkspaceID, service.CreateFieldDefinitionParams{
		CreatedBy:          claims.UserID,
		Scope:              body.Scope,
		Name:               body.Name,
		Key:                body.Key,
		FieldType:          body.FieldType,
		Options:            body.Options,
		Required:           body.Required,
		Position:           body.Position,
		InheritFromProject: body.InheritFromProject,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fieldDTOToResponse(def))
}

// @Summary Get a field definition
// @Description Returns a single custom field definition.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param id path string true "Field definition ID"
// @Success 200 {object} FieldDefinitionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Field definition not found"
// @Router /api/v1/field-definitions/{id} [get].
func (s *Server) handleGetFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	def, err := s.fields.Get(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(fieldDTOToResponse(def))
}

// @Summary Get field definition stats
// @Description Returns the number of assets and projects that have a value set for this field definition.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param id path string true "Field definition ID"
// @Success 200 {object} FieldDefinitionStatsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Field definition not found"
// @Router /api/v1/field-definitions/{id}/stats [get].
func (s *Server) handleGetFieldDefinitionStats(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	stats, err := s.fields.GetStats(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(FieldDefinitionStatsResponse{
		AssetCount:   stats.AssetCount,
		ProjectCount: stats.ProjectCount,
	})
}

// @Summary Update a field definition
// @Description Updates a custom field definition. The key and field_type are immutable after creation.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Field definition ID"
// @Param body body UpdateFieldDefinitionRequest true "Fields to update"
// @Success 200 {object} FieldDefinitionResponse
// @Failure 400 {object} ErrorResponse "Invalid options format or empty name"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Field definition not found"
// @Failure 422 {object} ErrorResponse "Attempt to change immutable key or field_type"
// @Router /api/v1/field-definitions/{id} [put].
func (s *Server) handleUpdateFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &UpdateFieldDefinitionRequest{})
	if !ok {
		return nil
	}

	// Validate options format if provided
	if body.Options != nil {
		err := s.validateFieldOptions(c, claims.WorkspaceID, id, body)
		if err != nil {
			return err
		}
	}

	if body.Name != nil {
		trimmed := strings.TrimSpace(*body.Name)
		if trimmed == "" {
			return errRes(c, fiber.StatusBadRequest, "name cannot be empty")
		}
		body.Name = &trimmed
	}

	updated, err := s.fields.Update(c.Context(), claims.WorkspaceID, id, service.UpdateFieldDefinitionParams{
		Name:               body.Name,
		Key:                body.Key,
		FieldType:          body.FieldType,
		Options:            body.Options,
		Required:           body.Required,
		Position:           body.Position,
		InheritFromProject: body.InheritFromProject,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(fieldDTOToResponse(updated))
}

func (s *Server) validateFieldOptions(
	c fiber.Ctx,
	workspaceID string,
	id string,
	body *UpdateFieldDefinitionRequest,
) error {
	existing, err := s.fields.Get(c.Context(), workspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if existing.FieldType == "select" {
		var opts []string
		if unmarshalErr := json.Unmarshal([]byte(*body.Options), &opts); unmarshalErr != nil || len(opts) == 0 {
			return errRes(c, fiber.StatusBadRequest, "options must be a non-empty JSON array of strings")
		}
	} else {
		return errRes(c, fiber.StatusBadRequest, "options can only be set for select fields")
	}
	return nil
}

// @Summary Delete a field definition
// @Description Soft-deletes the field definition.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param id path string true "Field definition ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Field definition not found"
// @Router /api/v1/field-definitions/{id} [delete].
func (s *Server) handleDeleteFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if err := s.fields.Delete(c.Context(), claims.WorkspaceID, id); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Reorder field definitions
// @Description Updates the position of multiple field definitions in a single request.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ReorderFieldDefinitionsRequest true "Reorder items"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/field-definitions/reorder [put].
func (s *Server) handleReorderFieldDefinitions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &ReorderFieldDefinitionsRequest{})
	if !ok {
		return nil
	}

	items := make([]service.ReorderFieldItem, len(body.Items))
	for i, item := range body.Items {
		items[i] = service.ReorderFieldItem{ID: item.ID, Position: item.Position}
	}
	_ = s.fields.Reorder(c.Context(), claims.WorkspaceID, items)

	return c.SendStatus(fiber.StatusNoContent)
}

type projectFieldValueResponse struct {
	FieldID           string `json:"field_id"`
	Key               string `json:"key"`
	Name              string `json:"name"`
	FieldType         string `json:"field_type"`
	Value             any    `json:"value"`
	DefinitionDeleted bool   `json:"definition_deleted"`
}

type GetProjectFieldsResponse struct {
	Fields []projectFieldValueResponse `json:"fields"`
}

func projectFieldValueDTOToResponse(dto *service.FieldValueDTO) projectFieldValueResponse {
	return projectFieldValueResponse{
		FieldID:           dto.FieldID,
		Key:               dto.FieldKey,
		Name:              dto.FieldName,
		FieldType:         dto.FieldType,
		Value:             dto.Value,
		DefinitionDeleted: dto.DefinitionDeleted,
	}
}

// handleGetProjectFields returns all custom field values for a project.
//
// @Summary Get project field values
// @Description Returns all custom field values set on the project. Only fields with <code>scope=project</code> are included. The response structure mirrors the asset fields endpoint.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 200 {object} GetProjectFieldsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Router /api/v1/projects/{id}/fields [get].
func (s *Server) handleGetProjectFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	dtos, err := s.projectFields.GetValues(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	items := make([]projectFieldValueResponse, len(dtos))
	for i, d := range dtos {
		items[i] = projectFieldValueDTOToResponse(d)
	}
	return c.JSON(GetProjectFieldsResponse{Fields: items})
}

// handlePatchProjectFields sets or clears custom field values on a project.
//
// @Summary Update project field values
// @Description Sets or clears one or more custom field values on the project. Only fields with <code>scope=project</code> are accepted — submitting an asset-scoped field returns 422.<br><br> Pass <code>null</code> as the value to clear a field. Returns the full updated field values for the project.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Param body body PatchProjectFieldsRequest true "Field values to set"
// @Success 200 {object} GetProjectFieldsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project or field not found"
// @Failure 422 {object} ErrorResponse "Value validation failed or wrong scope"
// @Router /api/v1/projects/{id}/fields [patch].
func (s *Server) handlePatchProjectFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &PatchProjectFieldsRequest{})
	if !ok {
		return nil
	}

	inputs := make([]service.SetFieldValueInput, len(body.Values))
	for i, v := range body.Values {
		inputs[i] = service.SetFieldValueInput{FieldID: v.FieldID, Value: v.Value}
	}

	dtos, err := s.projectFields.SetValues(c.Context(), claims.WorkspaceID, id, claims.UserID, inputs)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	items := make([]projectFieldValueResponse, len(dtos))
	for i, d := range dtos {
		items[i] = projectFieldValueDTOToResponse(d)
	}
	return c.JSON(GetProjectFieldsResponse{Fields: items})
}

// -- Asset field values response ----------------------------------------------

type assetFieldValueResponse struct {
	FieldID           string `json:"field_id"`
	Key               string `json:"key"`
	Name              string `json:"name"`
	FieldType         string `json:"field_type"`
	Source            string `json:"source"`
	Value             any    `json:"value"`
	DefinitionDeleted bool   `json:"definition_deleted"`
}

type GetAssetFieldsResponse struct {
	Fields []assetFieldValueResponse `json:"fields"`
}

type BulkPatchAssetFieldsResponse struct {
	Updated int64 `json:"updated"`
	Cleared int64 `json:"cleared"`
}

func fieldValueDTOToResponse(dto *service.FieldValueDTO) assetFieldValueResponse {
	return assetFieldValueResponse{
		FieldID:           dto.FieldID,
		Key:               dto.FieldKey,
		Name:              dto.FieldName,
		FieldType:         dto.FieldType,
		Source:            dto.FieldSource,
		Value:             dto.Value,
		DefinitionDeleted: dto.DefinitionDeleted,
	}
}

func fieldValueDTOsToResponse(dtos []*service.FieldValueDTO) []assetFieldValueResponse {
	items := make([]assetFieldValueResponse, len(dtos))
	for i, d := range dtos {
		items[i] = fieldValueDTOToResponse(d)
	}
	return items
}

// handleGetAssetFields returns all custom field values for an asset.
//
// @Summary Get asset field values
// @Description Returns all custom field values set on the asset, including fields for which no value has been set yet (those have <code>value: null</code>). Each entry includes the field definition metadata (<code>key</code>, <code>name</code>, <code>field_type</code>) alongside the typed value. The <code>definition_deleted</code> flag indicates the field definition was soft-deleted — the value is still stored but the field is no longer active.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} GetAssetFieldsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/fields [get].
func (s *Server) handleGetAssetFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	dtos, err := s.assetFields.GetValues(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(GetAssetFieldsResponse{Fields: fieldValueDTOsToResponse(dtos)})
}

// handlePatchAssetFields sets or clears custom field values on an asset.
//
// @Summary Update asset field values
// @Description Sets or clears one or more custom field values on the asset. Each entry in the <code>values</code> array targets a field by <code>field_id</code>. To clear a value, pass <code>null</code> as the value. All values are validated before any writes.<br><br> Value types must match the field definition's <code>field_type</code>: <ul> <li><strong>text / url</strong> — string</li> <li><strong>number</strong> — number</li> <li><strong>date</strong> — string in <code>YYYY-MM-DD</code> format</li> <li><strong>boolean</strong> — boolean</li> <li><strong>select</strong> — string matching one of the field's defined options</li> </ul> Returns the full updated field values for the asset.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body PatchAssetFieldsRequest true "Field values to set"
// @Success 200 {object} GetAssetFieldsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ErrorResponse "Value validation failed"
// @Router /api/v1/assets/{id}/fields [patch].
func (s *Server) handlePatchAssetFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &PatchAssetFieldsRequest{})
	if !ok {
		return nil
	}

	inputs := make([]service.SetFieldValueInput, len(body.Values))
	for i, v := range body.Values {
		inputs[i] = service.SetFieldValueInput{FieldID: v.FieldID, Value: v.Value}
	}

	dtos, err := s.assetFields.SetValues(c.Context(), claims.WorkspaceID, id, claims.UserID, inputs)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	ftsCtx := context.WithoutCancel(c.Context())
	go func() { _ = s.assets.RefreshFTS(ftsCtx, id) }()

	return c.JSON(GetAssetFieldsResponse{Fields: fieldValueDTOsToResponse(dtos)})
}

// handleBulkPatchAssetFields applies the same field values to multiple assets.
//
// @Summary Bulk update asset field values
// @Description Applies the same set of field value changes to multiple assets in a single transaction. Assets not belonging to the workspace are silently skipped. Returns the count of assets successfully updated.<br><br> Supports the same value format and null-for-clear semantics as the single-asset PATCH endpoint.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BulkPatchAssetFieldsRequest true "Asset IDs and field values"
// @Success 200 {object} BulkPatchAssetFieldsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ErrorResponse "Value validation failed"
// @Router /api/v1/assets/bulk/fields [patch].
func (s *Server) handleBulkPatchAssetFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &BulkPatchAssetFieldsRequest{})
	if !ok {
		return nil
	}

	inputs := make([]service.SetFieldValueInput, len(body.Values))
	for i, v := range body.Values {
		inputs[i] = service.SetFieldValueInput{FieldID: v.FieldID, Value: v.Value}
	}

	result, err := s.assetFields.BulkSetValues(c.Context(), claims.WorkspaceID, claims.UserID, body.AssetIDs, inputs)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	assetIDsCopy := make([]string, len(body.AssetIDs))
	copy(assetIDsCopy, body.AssetIDs)
	ftsCtx := context.WithoutCancel(c.Context())
	go func() {
		for _, assetID := range assetIDsCopy {
			_ = s.assets.RefreshFTS(ftsCtx, assetID)
		}
	}()

	return c.JSON(BulkPatchAssetFieldsResponse{Updated: result.Updated, Cleared: result.Cleared})
}

// FieldValueInput is the unexported alias for backward compatibility.
type FieldValueInput struct {
	FieldID string `json:"field_id"`
	Value   any    `json:"value"`
}

// -- Bulk fields preview -------------------------------------------------------

type BulkFieldsPreviewRequest struct {
	AssetIDs []string `json:"asset_ids"`
	FieldIDs []string `json:"field_ids"` // optional; empty = all active fields
}

func (r *BulkFieldsPreviewRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.AssetIDs) == 0 {
		p["asset_ids"] = "required"
	}
	if len(r.FieldIDs) > maxBulkFieldSize {
		p["field_ids"] = fmt.Sprintf("must not exceed %d", maxBulkFieldSize)
	}
	return p
}

type BulkFieldPreviewEntry struct {
	FieldID         string   `json:"field_id"`
	FieldName       string   `json:"field_name"`
	FieldType       string   `json:"field_type"`
	AssetsWithValue int      `json:"assets_with_value"`
	DistinctValues  []string `json:"distinct_values"`
}

type BulkFieldsPreviewResponse struct {
	Fields []BulkFieldPreviewEntry `json:"fields"`
}

// handleBulkFieldsPreview returns per-field overwrite impact for a set of assets.
//
// @Summary Preview bulk field overwrite impact
// @Description Returns how many assets already have a value for each field, and the top distinct values. Use before committing a bulk field update.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BulkFieldsPreviewRequest true "Asset IDs and optional field IDs"
// @Success 200 {object} BulkFieldsPreviewResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ErrorResponse "Validation failed"
// @Router /api/v1/assets/bulk/fields/preview [post].
func (s *Server) handleBulkFieldsPreview(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &BulkFieldsPreviewRequest{})
	if !ok {
		return nil
	}

	entries, err := s.assetFields.BulkPreview(c.Context(), claims.WorkspaceID, body.AssetIDs, body.FieldIDs)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	resp := BulkFieldsPreviewResponse{Fields: make([]BulkFieldPreviewEntry, len(entries))}
	for i, e := range entries {
		resp.Fields[i] = BulkFieldPreviewEntry{
			FieldID:         e.FieldID,
			FieldName:       e.FieldName,
			FieldType:       e.FieldType,
			AssetsWithValue: e.AssetsWithValue,
			DistinctValues:  e.DistinctValues,
		}
	}
	return c.JSON(resp)
}
