package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/repository"
	apptelemetry "damask/server/internal/telemetry"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

const (
	maxMissingExifAssets        = 1000
	maxFieldDefinitionsPerScope = 50
	fieldTypeBoolean            = "boolean"
	fieldTypeDate               = "date"
	fieldTypeNumber             = "number"
	fieldTypeSelect             = "select"
	fieldTypeText               = "text"
	fieldTypeURL                = "url"
)

// FieldDefinitionDTO is the output of FieldService methods.
type FieldDefinitionDTO struct {
	ID                 string
	WorkspaceID        string
	CreatedBy          string
	Scope              string
	Name               string
	Key                string
	FieldType          string
	Options            *string
	Required           bool
	Position           int64
	InheritFromProject bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *string
}

// CreateFieldDefinitionParams is the input for FieldService.Create.
type CreateFieldDefinitionParams struct {
	CreatedBy          string
	Scope              string
	Name               string
	Key                string
	FieldType          string
	Options            *string
	Required           bool
	Position           int64
	InheritFromProject bool
}

func (p *CreateFieldDefinitionParams) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("name is required: %w", apperr.ErrInvalidInput)
	}
	p.Key = strings.TrimSpace(p.Key)
	if p.Key == "" {
		return fmt.Errorf("key is required: %w", apperr.ErrInvalidInput)
	}
	if p.Scope != string(AutomationScopeAsset) && p.Scope != string(AutomationScopeProject) {
		return fmt.Errorf("scope must be 'asset' or 'project': %w", apperr.ErrInvalidInput)
	}
	validTypes := map[string]bool{
		fieldTypeText: true, fieldTypeNumber: true, fieldTypeDate: true,
		fieldTypeBoolean: true, fieldTypeSelect: true, fieldTypeURL: true,
	}
	if !validTypes[p.FieldType] {
		return fmt.Errorf("invalid field_type %q: %w", p.FieldType, apperr.ErrInvalidInput)
	}
	if p.FieldType != fieldTypeSelect {
		p.Options = nil
	}
	return nil
}

// UpdateFieldDefinitionParams is the input for FieldService.Update.
// Key and FieldType are immutable -- passing a different value returns ErrInvalidInput.
type UpdateFieldDefinitionParams struct {
	Name               *string
	Key                *string
	FieldType          *string
	Options            *string
	Required           *bool
	Position           *int64
	InheritFromProject *bool
}

type fieldService struct {
	fields repository.FieldRepository
}

// NewFieldService returns a FieldService.
func NewFieldService(fields repository.FieldRepository) FieldService {
	return &fieldService{fields: fields}
}

func (s *fieldService) List(ctx context.Context, workspaceID, scope string) ([]*FieldDefinitionDTO, error) {
	rows, err := s.fields.List(ctx, workspaceID, scope)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toFieldDTO)
	return out, nil
}

func (s *fieldService) Get(ctx context.Context, workspaceID, id string) (*FieldDefinitionDTO, error) {
	f, err := s.fields.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if f.DeletedAt != nil {
		return nil, fmt.Errorf("field definition %q: %w", id, apperr.ErrNotFound)
	}
	return toFieldDTO(f), nil
}

func (s *fieldService) Create(
	ctx context.Context,
	workspaceID string,
	p CreateFieldDefinitionParams,
) (*FieldDefinitionDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	count, err := s.fields.CountByWorkspaceAndScope(ctx, workspaceID, p.Scope)
	if err != nil {
		return nil, err
	}
	if count >= maxFieldDefinitionsPerScope {
		return nil, fmt.Errorf(
			"maximum of %d field definitions per scope reached: %w",
			maxFieldDefinitionsPerScope,
			apperr.ErrInvalidInput,
		)
	}
	f, err := s.fields.Create(ctx, repository.FieldDefinition{
		ID:                 uuid.NewString(),
		WorkspaceID:        workspaceID,
		CreatedBy:          p.CreatedBy,
		Scope:              p.Scope,
		Name:               p.Name,
		Key:                p.Key,
		FieldType:          p.FieldType,
		Options:            p.Options,
		Required:           p.Required,
		Position:           p.Position,
		InheritFromProject: p.InheritFromProject,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("a field with key %q already exists in this scope: %w", p.Key, apperr.ErrConflict)
		}
		return nil, err
	}
	return toFieldDTO(f), nil
}

func (s *fieldService) Update(
	ctx context.Context,
	workspaceID, id string,
	p UpdateFieldDefinitionParams,
) (*FieldDefinitionDTO, error) {
	existing, err := s.fields.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if existing.DeletedAt != nil {
		return nil, fmt.Errorf("field definition %q: %w", id, apperr.ErrNotFound)
	}

	// key and field_type are immutable
	if p.Key != nil && *p.Key != existing.Key {
		return nil, fmt.Errorf("key cannot be changed after creation: %w", apperr.ErrInvalidInput)
	}
	if p.FieldType != nil && *p.FieldType != existing.FieldType {
		return nil, fmt.Errorf("field_type cannot be changed after creation: %w", apperr.ErrInvalidInput)
	}

	if p.Name != nil {
		*p.Name = strings.TrimSpace(*p.Name)
		if *p.Name == "" {
			return nil, fmt.Errorf("name cannot be empty: %w", apperr.ErrInvalidInput)
		}
		existing.Name = *p.Name
	}
	if p.Options != nil {
		existing.Options = p.Options
	}
	if p.Required != nil {
		existing.Required = *p.Required
	}
	if p.Position != nil {
		existing.Position = *p.Position
	}
	if p.InheritFromProject != nil {
		existing.InheritFromProject = *p.InheritFromProject
	}

	updated, err := s.fields.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return toFieldDTO(updated), nil
}

func (s *fieldService) Delete(ctx context.Context, workspaceID, id string) error {
	existing, err := s.fields.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if existing.DeletedAt != nil {
		return fmt.Errorf("field definition %q: %w", id, apperr.ErrNotFound)
	}
	return s.fields.SoftDelete(ctx, workspaceID, id)
}

func (s *fieldService) GetStats(ctx context.Context, workspaceID, id string) (FieldStatsDTO, error) {
	if _, err := s.fields.GetByID(ctx, workspaceID, id); err != nil {
		return FieldStatsDTO{}, err
	}
	assetCount, err := s.fields.CountAssetValues(ctx, id)
	if err != nil {
		return FieldStatsDTO{}, err
	}
	projectCount, err := s.fields.CountProjectValues(ctx, id)
	if err != nil {
		return FieldStatsDTO{}, err
	}
	return FieldStatsDTO{AssetCount: assetCount, ProjectCount: projectCount}, nil
}

func (s *fieldService) Reorder(ctx context.Context, workspaceID string, items []ReorderFieldItem) error {
	for _, item := range items {
		_ = s.fields.UpdatePosition(ctx, workspaceID, item.ID, item.Position)
	}
	return nil
}

func (s *fieldService) InheritProjectFields(
	ctx context.Context,
	workspaceID, assetID, projectID, userID string,
) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.fields.inherit_project_fields",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.project_id", projectID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"field inheritance failed",
				"workspace_id",
				workspaceID,
				"asset_id",
				assetID,
				"project_id",
				projectID,
				"error",
				err,
			)
		}
	}()
	return s.fields.InheritProjectFields(ctx, workspaceID, assetID, projectID, userID)
}

// ListAssetsMissingExif returns asset IDs that need EXIF extraction jobs.
// If the tombstone field "_exif_make" doesn't exist yet, all image asset IDs are returned.
// Otherwise, only assets missing the tombstone field value are returned (up to 10 000).
func (s *fieldService) ListAssetsMissingExif(ctx context.Context, workspaceID string) ([]string, error) {
	tombstone, err := s.fields.GetByKey(ctx, workspaceID, "_exif_make")
	if err != nil {
		// No tombstone field yet — return all image asset IDs.
		return s.fields.ListImageAssetIDs(ctx, workspaceID)
	}
	return s.fields.ListMissingExifField(ctx, workspaceID, tombstone.ID, maxMissingExifAssets)
}

func (s *fieldService) PurgeExpiredFields(ctx context.Context) (int, error) {
	n, err := s.fields.PurgeExpired(ctx)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		slog.InfoContext(ctx, "field cleanup: purged expired field definitions", "count", n)
	}
	return n, nil
}

func toFieldDTO(f repository.FieldDefinition) *FieldDefinitionDTO {
	return &FieldDefinitionDTO{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		CreatedBy:          f.CreatedBy,
		Scope:              f.Scope,
		Name:               f.Name,
		Key:                f.Key,
		FieldType:          f.FieldType,
		Options:            f.Options,
		Required:           f.Required,
		Position:           f.Position,
		InheritFromProject: f.InheritFromProject,
		CreatedAt:          f.CreatedAt,
		UpdatedAt:          f.UpdatedAt,
		DeletedAt:          f.DeletedAt,
	}
}

// -- AssetFieldService --------------------------------------------------------

type assetFieldService struct {
	assets      repository.AssetRepository
	fields      repository.FieldRepository
	assetFields repository.AssetFieldRepository
	audit       audit.Writer
}

// NewAssetFieldService returns an AssetFieldService.
func NewAssetFieldService(
	assets repository.AssetRepository,
	fields repository.FieldRepository,
	assetFields repository.AssetFieldRepository,
	aw audit.Writer,
) AssetFieldService {
	return &assetFieldService{assets: assets, fields: fields, assetFields: assetFields, audit: aw}
}

func (s *assetFieldService) GetValues(ctx context.Context, workspaceID, assetID string) ([]*FieldValueDTO, error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.asset_fields.get_values",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
	)
	var err error
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"asset fields get values failed",
				"workspace_id",
				workspaceID,
				"asset_id",
				assetID,
				"error",
				err,
			)
		}
	}()

	if _, getErr := s.assets.GetByID(ctx, workspaceID, assetID); getErr != nil {
		err = getErr
		return nil, err
	}
	rows, err := s.assetFields.GetValues(ctx, assetID)
	if err != nil {
		return nil, err
	}
	dtos := toFieldValueDTOs(rows)
	span.SetAttributes(attribute.Int("damask.asset_fields.count", len(dtos)))
	return dtos, nil
}

func (s *assetFieldService) SetValues(
	ctx context.Context,
	workspaceID, assetID, userID string,
	inputs []SetFieldValueInput,
) ([]*FieldValueDTO, error) {
	if _, err := s.assets.GetByID(ctx, workspaceID, assetID); err != nil {
		return nil, err
	}
	// Snapshot before-state for audit diff.
	existingRows, _ := s.assetFields.GetValues(ctx, assetID)
	existingByFieldID := make(map[string]*FieldValueDTO, len(existingRows))
	for _, v := range toFieldValueDTOs(existingRows) {
		existingByFieldID[v.FieldID] = v
	}

	for _, input := range inputs {
		def, err := s.fields.GetByID(ctx, workspaceID, input.FieldID)
		if err != nil {
			return nil, err
		}
		if def.DeletedAt != nil {
			return nil, fmt.Errorf("field %s has been deleted: %w", input.FieldID, apperr.ErrInvalidInput)
		}
		if def.Source != "" && def.Source != "user" {
			return nil, fmt.Errorf("field %s is system-managed: %w", input.FieldID, apperr.ErrInvalidInput)
		}
		if input.Value == nil {
			if err = s.assetFields.DeleteValue(ctx, assetID, input.FieldID); err != nil {
				return nil, err
			}
			continue
		}
		p, err := resolveFieldValue(input.FieldID, def.FieldType, def.Options, input.Value)
		if err != nil {
			return nil, fmt.Errorf("%w", apperr.ErrInvalidInput)
		}
		p.CreatedBy = userID
		if err = s.assetFields.UpsertValue(ctx, assetID, p); err != nil {
			return nil, err
		}
	}
	rows, err := s.assetFields.GetValues(ctx, assetID)
	if err != nil {
		return nil, err
	}
	dtos := toFieldValueDTOs(rows)

	// Emit per-field audit events.
	actor := auth.ActorFromCtx(ctx)
	afterByFieldID := make(map[string]*FieldValueDTO, len(dtos))
	for _, v := range dtos {
		afterByFieldID[v.FieldID] = v
	}

	emitFieldValueAuditEvents(inputs, existingByFieldID, afterByFieldID, func(
		before, after *FieldValueDTO,
		beforeVal any,
	) {
		s.audit.WriteAsset(ctx, audit.AssetEvent{
			WorkspaceID: workspaceID,
			AssetID:     assetID,
			UserID:      actor.UserID,
			ActorType:   actor.Type,
			EventType:   audit.EventAssetFieldCleared,
			Payload: audit.AssetFieldClearedPayload{
				V:         1,
				FieldKey:  fieldKeyOf(before, after),
				FieldName: fieldNameOf(before, after),
				Before:    beforeVal,
			},
		})
	}, func(before, after *FieldValueDTO, beforeVal, afterVal any) {
		s.audit.WriteAsset(ctx, audit.AssetEvent{
			WorkspaceID: workspaceID,
			AssetID:     assetID,
			UserID:      actor.UserID,
			ActorType:   actor.Type,
			EventType:   audit.EventAssetFieldSet,
			Payload: audit.AssetFieldSetPayload{
				V:         1,
				FieldKey:  fieldKeyOf(before, after),
				FieldName: fieldNameOf(before, after),
				Before:    beforeVal,
				After:     afterVal,
			},
		})
	})

	return dtos, nil
}

func (s *assetFieldService) BulkSetValues(
	ctx context.Context,
	workspaceID, userID string,
	assetIDs []string,
	inputs []SetFieldValueInput,
) (result BulkSetValuesResult, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.asset_fields.bulk_set_values",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Int("damask.assets.requested_count", len(assetIDs)),
		attribute.Int("damask.fields.input_count", len(inputs)),
	)
	defer func() {
		span.SetAttributes(
			attribute.Int64("damask.assets.updated_count", result.Updated),
			attribute.Int64("damask.assets.cleared_count", result.Cleared),
		)
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"asset fields bulk set failed",
				"workspace_id",
				workspaceID,
				"asset_count",
				len(assetIDs),
				"input_count",
				len(inputs),
				"error",
				err,
			)
		}
	}()

	// Validate values once (same fields for all assets).
	resolved := make([]bulkFieldInput, len(inputs))
	for i, input := range inputs {
		if input.Value == nil {
			resolved[i] = bulkFieldInput{fieldID: input.FieldID}
			continue
		}
		def, defErr := s.fields.GetByID(ctx, workspaceID, input.FieldID)
		if defErr != nil {
			return result, defErr
		}
		if def.DeletedAt != nil {
			return result, fmt.Errorf("field %s has been deleted: %w", input.FieldID, apperr.ErrInvalidInput)
		}
		if def.Source != "" && def.Source != "user" {
			return result, fmt.Errorf("field %s is system-managed: %w", input.FieldID, apperr.ErrInvalidInput)
		}
		p, resolveErr := resolveFieldValue(input.FieldID, def.FieldType, def.Options, input.Value)
		if resolveErr != nil {
			return result, fmt.Errorf("%w", apperr.ErrInvalidInput)
		}
		p.CreatedBy = userID
		resolved[i] = bulkFieldInput{fieldID: input.FieldID, p: &p}
	}

	// Pre-filter: collect only asset IDs that belong to this workspace (read before tx).
	validIDs := make([]string, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		if _, getErr := s.assets.GetByID(ctx, workspaceID, assetID); getErr == nil {
			validIDs = append(validIDs, assetID)
		}
	}

	err = s.assetFields.RunInTx(ctx, func(tx repository.AssetFieldRepository) error {
		return applyBulkFieldValues(ctx, tx, validIDs, resolved, &result)
	})
	if err != nil {
		return BulkSetValuesResult{}, err
	}
	return result, nil
}

func (s *assetFieldService) BulkPreview(
	ctx context.Context,
	workspaceID string,
	assetIDs, fieldIDs []string,
) ([]BulkPreviewEntry, error) {
	defs, err := resolveFieldDefs(ctx, s.fields, workspaceID, fieldIDs)
	if err != nil {
		return nil, err
	}
	if len(defs) == 0 {
		return []BulkPreviewEntry{}, nil
	}

	// Pre-filter assets to workspace membership.
	validIDs := make([]string, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		if _, validErr := s.assets.GetByID(ctx, workspaceID, assetID); validErr == nil {
			validIDs = append(validIDs, assetID)
		}
	}

	// Collect values per fieldID across all valid assets.
	type fieldAccum struct {
		withValue   int
		valueCounts map[string]int
	}
	accums := make(map[string]*fieldAccum, len(defs))
	for _, d := range defs {
		accums[d.ID] = &fieldAccum{valueCounts: make(map[string]int)}
	}

	for _, assetID := range validIDs {
		rows, validErr := s.assetFields.GetValues(ctx, assetID)
		if validErr != nil {
			continue
		}
		for _, row := range rows {
			acc, ok := accums[row.FieldID]
			if !ok {
				continue
			}
			strVal := fieldValueString(row)
			if strVal != "" {
				acc.withValue++
				acc.valueCounts[strVal]++
			}
		}
	}

	const maxDistinct = 5
	entries := make([]BulkPreviewEntry, 0, len(defs))
	for _, d := range defs {
		acc := accums[d.ID]
		// Sort distinct values by frequency descending.
		type kv struct {
			val   string
			count int
		}
		sorted := make([]kv, 0, len(acc.valueCounts))
		for v, c := range acc.valueCounts {
			sorted = append(sorted, kv{v, c})
		}
		slices.SortFunc(sorted, func(a, b kv) int { return b.count - a.count })
		distinct := make([]string, 0, maxDistinct+1)
		for i, kv := range sorted {
			if i >= maxDistinct {
				distinct = append(distinct, fmt.Sprintf("+%d more", len(sorted)-maxDistinct))
				break
			}
			distinct = append(distinct, kv.val)
		}
		entries = append(entries, BulkPreviewEntry{
			FieldID:         d.ID,
			FieldName:       d.Name,
			FieldType:       d.FieldType,
			AssetsWithValue: acc.withValue,
			DistinctValues:  distinct,
		})
	}
	return entries, nil
}

type bulkFieldInput struct {
	fieldID string
	p       *repository.SetFieldValueParams // nil = delete (clear)
}

func applyBulkFieldValues(
	ctx context.Context,
	tx repository.AssetFieldRepository,
	validIDs []string,
	resolved []bulkFieldInput,
	result *BulkSetValuesResult,
) error {
	for _, assetID := range validIDs {
		assetOK := true
		assetCleared := int64(0)
		for _, r := range resolved {
			if r.p == nil {
				if delErr := tx.DeleteValue(ctx, assetID, r.fieldID); delErr != nil {
					assetOK = false
					break
				}
				assetCleared++
				continue
			}
			if upsertErr := tx.UpsertValue(ctx, assetID, *r.p); upsertErr != nil {
				return upsertErr
			}
		}
		if assetOK {
			result.Updated++
			result.Cleared += assetCleared
		}
	}
	return nil
}

func resolveFieldDefs(
	ctx context.Context,
	fields repository.FieldRepository,
	workspaceID string,
	fieldIDs []string,
) ([]repository.FieldDefinition, error) {
	if len(fieldIDs) == 0 {
		all, err := fields.List(ctx, workspaceID, string(AutomationScopeAsset))
		if err != nil {
			return nil, err
		}
		var defs []repository.FieldDefinition
		for _, d := range all {
			if d.DeletedAt == nil {
				defs = append(defs, d)
			}
		}
		return defs, nil
	}
	var defs []repository.FieldDefinition
	for _, fid := range fieldIDs {
		d, err := fields.GetByID(ctx, workspaceID, fid)
		if err != nil || d.DeletedAt != nil {
			continue
		}
		defs = append(defs, d)
	}
	return defs, nil
}

func fieldValueString(row repository.FieldValue) string {
	switch {
	case row.ValueText != nil:
		return *row.ValueText
	case row.ValueNumber != nil:
		return fmt.Sprintf("%v", *row.ValueNumber)
	case row.ValueDate != nil:
		return *row.ValueDate
	case row.ValueBoolean != nil:
		if *row.ValueBoolean != 0 {
			return "true"
		}
		return "false"
	}
	return ""
}

// -- ProjectFieldService -------------------------------------------------------

type projectFieldService struct {
	projects      repository.ProjectRepository
	fields        repository.FieldRepository
	projectFields repository.ProjectFieldRepository
	audit         audit.Writer
}

// NewProjectFieldService returns a ProjectFieldService.
func NewProjectFieldService(
	projects repository.ProjectRepository,
	fields repository.FieldRepository,
	projectFields repository.ProjectFieldRepository,
	aw audit.Writer,
) ProjectFieldService {
	return &projectFieldService{projects: projects, fields: fields, projectFields: projectFields, audit: aw}
}

func (s *projectFieldService) GetValues(ctx context.Context, workspaceID, projectID string) ([]*FieldValueDTO, error) {
	if _, err := s.projects.GetByID(ctx, workspaceID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.projectFields.GetValues(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return toFieldValueDTOs(rows), nil
}

func (s *projectFieldService) SetValues(
	ctx context.Context,
	workspaceID, projectID, userID string,
	inputs []SetFieldValueInput,
) ([]*FieldValueDTO, error) {
	if _, err := s.projects.GetByID(ctx, workspaceID, projectID); err != nil {
		return nil, err
	}
	// Snapshot before-state for audit diff.
	existingRows, _ := s.projectFields.GetValues(ctx, projectID)
	existingByFieldID := make(map[string]*FieldValueDTO, len(existingRows))
	for _, v := range toFieldValueDTOs(existingRows) {
		existingByFieldID[v.FieldID] = v
	}

	for _, input := range inputs {
		def, err := s.fields.GetByID(ctx, workspaceID, input.FieldID)
		if err != nil {
			return nil, err
		}
		if def.Scope != string(AutomationScopeProject) {
			return nil, fmt.Errorf("field %s is not a project field: %w", def.Key, apperr.ErrInvalidInput)
		}
		if input.Value == nil {
			if err = s.projectFields.DeleteValue(ctx, projectID, input.FieldID); err != nil {
				return nil, err
			}
			continue
		}
		p, resolveErr := resolveFieldValue(input.FieldID, def.FieldType, def.Options, input.Value)
		if resolveErr != nil {
			return nil, fmt.Errorf("%w", apperr.ErrInvalidInput)
		}
		p.CreatedBy = userID
		if err = s.projectFields.UpsertValue(ctx, projectID, p); err != nil {
			return nil, err
		}
	}
	rows, err := s.projectFields.GetValues(ctx, projectID)
	if err != nil {
		return nil, err
	}
	dtos := toFieldValueDTOs(rows)

	// Emit per-field audit events.
	actor := auth.ActorFromCtx(ctx)
	afterByFieldID := make(map[string]*FieldValueDTO, len(dtos))
	for _, v := range dtos {
		afterByFieldID[v.FieldID] = v
	}

	emitFieldValueAuditEvents(inputs, existingByFieldID, afterByFieldID, func(
		before, after *FieldValueDTO,
		beforeVal any,
	) {
		s.audit.WriteProject(ctx, audit.ProjectEvent{
			WorkspaceID: workspaceID,
			ProjectID:   projectID,
			UserID:      actor.UserID,
			ActorType:   actor.Type,
			EventType:   audit.EventProjectFieldCleared,
			Payload: audit.ProjectFieldClearedPayload{
				V:         1,
				FieldKey:  fieldKeyOf(before, after),
				FieldName: fieldNameOf(before, after),
				Before:    beforeVal,
			},
		})
	}, func(before, after *FieldValueDTO, beforeVal, afterVal any) {
		s.audit.WriteProject(ctx, audit.ProjectEvent{
			WorkspaceID: workspaceID,
			ProjectID:   projectID,
			UserID:      actor.UserID,
			ActorType:   actor.Type,
			EventType:   audit.EventProjectFieldSet,
			Payload: audit.ProjectFieldSetPayload{
				V:         1,
				FieldKey:  fieldKeyOf(before, after),
				FieldName: fieldNameOf(before, after),
				Before:    beforeVal,
				After:     afterVal,
			},
		})
	})

	return dtos, nil
}

// -- Shared helpers -----------------------------------------------------------

func emitFieldValueAuditEvents(
	inputs []SetFieldValueInput,
	existingByFieldID map[string]*FieldValueDTO,
	afterByFieldID map[string]*FieldValueDTO,
	writeCleared func(*FieldValueDTO, *FieldValueDTO, any),
	writeSet func(*FieldValueDTO, *FieldValueDTO, any, any),
) {
	for _, input := range inputs {
		before := existingByFieldID[input.FieldID]
		after := afterByFieldID[input.FieldID]
		beforeVal := fieldValueOrNil(before)
		if input.Value == nil {
			writeCleared(before, after, beforeVal)
			continue
		}
		writeSet(before, after, beforeVal, fieldValueOrNil(after))
	}
}

func fieldValueOrNil(value *FieldValueDTO) any {
	if value == nil {
		return nil
	}
	return value.Value
}

func resolveFieldValue(fieldID, fieldType string, options *string, value any) (repository.SetFieldValueParams, error) {
	p := repository.SetFieldValueParams{FieldID: fieldID}
	var err error
	switch fieldType {
	case fieldTypeText, fieldTypeURL:
		err = resolveTextFieldValue(&p, fieldID, value)
	case fieldTypeSelect:
		err = resolveSelectFieldValue(&p, fieldID, options, value)
	case fieldTypeNumber:
		err = resolveNumberFieldValue(&p, fieldID, value)
	case fieldTypeDate:
		err = resolveDateFieldValue(&p, fieldID, value)
	case fieldTypeBoolean:
		err = resolveBooleanFieldValue(&p, fieldID, value)
	}
	return p, err
}

func resolveTextFieldValue(p *repository.SetFieldValueParams, fieldID string, value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("field %s expects a string value", fieldID)
	}
	p.ValueText = &s
	return nil
}

func resolveSelectFieldValue(p *repository.SetFieldValueParams, fieldID string, options *string, value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("field %s expects a string value", fieldID)
	}
	if options != nil {
		var opts []string
		if err := json.Unmarshal([]byte(*options), &opts); err == nil && !slices.Contains(opts, s) {
			return fmt.Errorf("value '%s' is not a valid option for field %s", s, fieldID)
		}
	}
	p.ValueText = &s
	return nil
}

func resolveNumberFieldValue(p *repository.SetFieldValueParams, fieldID string, value any) error {
	switch v := value.(type) {
	case float64:
		p.ValueNumber = &v
	case int64:
		f := float64(v)
		p.ValueNumber = &f
	default:
		return fmt.Errorf("field %s expects a numeric value", fieldID)
	}
	return nil
}

func resolveDateFieldValue(p *repository.SetFieldValueParams, fieldID string, value any) error {
	s, ok := value.(string)
	if !ok || !dateRe.MatchString(s) {
		return fmt.Errorf("field %s expects a date in YYYY-MM-DD format", fieldID)
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return fmt.Errorf("field %s: invalid date '%s'", fieldID, s)
	}
	p.ValueDate = &s
	return nil
}

func resolveBooleanFieldValue(p *repository.SetFieldValueParams, fieldID string, value any) error {
	b, ok := value.(bool)
	if !ok {
		return fmt.Errorf("field %s expects a boolean value", fieldID)
	}
	var v int64
	if b {
		v = 1
	}
	p.ValueBoolean = &v
	return nil
}

func toFieldValueDTOs(rows []repository.FieldValue) []*FieldValueDTO {
	return mapDTOs(rows, toFieldValueDTO)
}

func toFieldValueDTO(row repository.FieldValue) *FieldValueDTO {
	dto := &FieldValueDTO{
		FieldID:           row.FieldID,
		FieldKey:          row.FieldKey,
		FieldName:         row.FieldName,
		FieldType:         row.FieldType,
		FieldSource:       row.FieldSource,
		FieldOptions:      row.FieldOptions,
		DefinitionDeleted: row.DefinitionDeleted,
	}
	switch row.FieldType {
	case fieldTypeText, fieldTypeURL, fieldTypeSelect:
		if row.ValueText != nil {
			dto.Value = *row.ValueText
		}
	case fieldTypeNumber:
		if row.ValueNumber != nil {
			dto.Value = *row.ValueNumber
		}
	case fieldTypeDate:
		if row.ValueDate != nil {
			dto.Value = *row.ValueDate
		}
	case fieldTypeBoolean:
		if row.ValueBoolean != nil {
			dto.Value = *row.ValueBoolean != 0
		}
	}
	return dto
}

func fieldKeyOf(before, after *FieldValueDTO) string {
	if after != nil {
		return after.FieldKey
	}
	if before != nil {
		return before.FieldKey
	}
	return ""
}

func fieldNameOf(before, after *FieldValueDTO) string {
	if after != nil {
		return after.FieldName
	}
	if before != nil {
		return before.FieldName
	}
	return ""
}
