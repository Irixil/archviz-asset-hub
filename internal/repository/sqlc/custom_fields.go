package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"damask/server/internal/apperr"
	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

type fieldRepo struct {
	d *db.DB
}

// NewFieldRepo returns a repository.FieldRepository backed by sqlc-generated queries.
func NewFieldRepo(d *db.DB) repository.FieldRepository {
	return &fieldRepo{d: d}
}

func (r *fieldRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.FieldDefinition, error) {
	row, err := r.d.RQ.GetFieldDefinitionByID(ctx, dbgen.GetFieldDefinitionByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.FieldDefinition{}, apperr.ErrNotFound
		}
		return repository.FieldDefinition{}, err
	}
	return toField(row), nil
}

func (r *fieldRepo) List(ctx context.Context, workspaceID, scope string) ([]repository.FieldDefinition, error) {
	rows, err := r.d.RQ.ListFieldDefinitions(ctx, dbgen.ListFieldDefinitionsParams{
		WorkspaceID: workspaceID,
		Scope:       scope,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.FieldDefinition, len(rows))
	for i, row := range rows {
		out[i] = toField(row)
	}
	return out, nil
}

func (r *fieldRepo) Create(ctx context.Context, f repository.FieldDefinition) (repository.FieldDefinition, error) {
	createdBy := ptrIfNonEmpty(f.CreatedBy)
	row, err := r.d.WQ.CreateFieldDefinition(ctx, dbgen.CreateFieldDefinitionParams{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		CreatedBy:          createdBy,
		Scope:              f.Scope,
		Name:               f.Name,
		Key:                f.Key,
		FieldType:          f.FieldType,
		Options:            f.Options,
		Required:           boolToInt64(f.Required),
		Position:           f.Position,
		InheritFromProject: boolToInt64(f.InheritFromProject),
	})
	if err != nil {
		return repository.FieldDefinition{}, err
	}
	return toField(row), nil
}

func (r *fieldRepo) Update(ctx context.Context, f repository.FieldDefinition) (repository.FieldDefinition, error) {
	req := boolToInt64(f.Required)
	ifp := boolToInt64(f.InheritFromProject)
	row, err := r.d.WQ.UpdateFieldDefinition(ctx, dbgen.UpdateFieldDefinitionParams{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		Name:               &f.Name,
		Options:            f.Options,
		Required:           &req,
		Position:           &f.Position,
		InheritFromProject: &ifp,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.FieldDefinition{}, apperr.ErrNotFound
		}
		return repository.FieldDefinition{}, err
	}
	return toField(row), nil
}

func (r *fieldRepo) SoftDelete(ctx context.Context, workspaceID, id string) error {
	return r.d.WQ.SoftDeleteFieldDefinition(ctx, dbgen.SoftDeleteFieldDefinitionParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (r *fieldRepo) CountByWorkspaceAndScope(ctx context.Context, workspaceID, scope string) (int64, error) {
	return r.d.RQ.CountFieldDefinitions(ctx, dbgen.CountFieldDefinitionsParams{
		WorkspaceID: workspaceID,
		Scope:       scope,
	})
}

func toField(f dbgen.FieldDefinition) repository.FieldDefinition {
	return repository.FieldDefinition{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		CreatedBy:          stringValue(f.CreatedBy),
		Source:             f.Source,
		Scope:              f.Scope,
		Name:               f.Name,
		Key:                f.Key,
		FieldType:          f.FieldType,
		Options:            f.Options,
		Required:           f.Required != 0,
		Position:           f.Position,
		InheritFromProject: f.InheritFromProject != 0,
		CreatedAt:          parseCreatedAt(f.CreatedAt),
		UpdatedAt:          parseCreatedAt(f.UpdatedAt),
		DeletedAt:          f.DeletedAt,
	}
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func (r *fieldRepo) CountAssetValues(ctx context.Context, fieldID string) (int64, error) {
	return r.d.RQ.CountFieldDefinitionAssetValues(ctx, fieldID)
}

func (r *fieldRepo) CountProjectValues(ctx context.Context, fieldID string) (int64, error) {
	return r.d.RQ.CountFieldDefinitionProjectValues(ctx, fieldID)
}

func (r *fieldRepo) UpdatePosition(ctx context.Context, workspaceID, id string, position int64) error {
	return r.d.WQ.UpdateFieldDefinitionPosition(ctx, dbgen.UpdateFieldDefinitionPositionParams{
		Position:    position,
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (r *fieldRepo) GetByKey(ctx context.Context, workspaceID, key string) (repository.FieldDefinition, error) {
	row, err := r.d.RQ.GetFieldDefinitionByKey(ctx, dbgen.GetFieldDefinitionByKeyParams{
		WorkspaceID: workspaceID,
		Key:         key,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.FieldDefinition{}, apperr.ErrNotFound
		}
		return repository.FieldDefinition{}, err
	}
	return toField(row), nil
}

func (r *fieldRepo) ListImageAssetIDs(ctx context.Context, workspaceID string) ([]string, error) {
	return r.d.RQ.ListImageAssetIDs(ctx, workspaceID)
}

func (r *fieldRepo) ListMissingExifField(
	ctx context.Context,
	workspaceID, fieldID string,
	limit int64,
) ([]string, error) {
	return r.d.RQ.ListAssetsMissingExifField(ctx, dbgen.ListAssetsMissingExifFieldParams{
		FieldID:     fieldID,
		WorkspaceID: workspaceID,
		Limit:       limit,
	})
}

func (r *fieldRepo) EnsureSystemField(ctx context.Context, params repository.EnsureSystemFieldParams) error {
	return r.d.WQ.InsertSystemFieldDefinition(ctx, dbgen.InsertSystemFieldDefinitionParams{
		ID:          params.ID,
		WorkspaceID: params.WorkspaceID,
		Source:      params.Source,
		Name:        params.Name,
		Key:         params.Key,
		FieldType:   params.FieldType,
		Position:    params.Position,
	})
}

func (r *fieldRepo) ListBySource(
	ctx context.Context,
	workspaceID, source string,
) ([]repository.FieldDefinition, error) {
	rows, err := r.d.RQ.GetSystemFieldsBySource(ctx, dbgen.GetSystemFieldsBySourceParams{
		WorkspaceID: workspaceID,
		Source:      source,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.FieldDefinition, len(rows))
	for i, row := range rows {
		out[i] = toField(row)
	}
	return out, nil
}

func (r *fieldRepo) InheritProjectFields(ctx context.Context, workspaceID, assetID, projectID, userID string) error {
	defs, err := r.d.RQ.ListInheritableAssetFieldDefinitions(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, def := range defs {
		pv, pvErr := r.d.RQ.GetProjectFieldValue(ctx, dbgen.GetProjectFieldValueParams{
			ProjectID: projectID,
			FieldID:   def.ID,
		})
		if pvErr != nil {
			continue // no value set on project for this field — skip
		}
		if _, err = r.d.WQ.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
			ID:           uuid.NewString(),
			AssetID:      assetID,
			FieldID:      def.ID,
			ValueText:    pv.ValueText,
			ValueNumber:  pv.ValueNumber,
			ValueDate:    pv.ValueDate,
			ValueBoolean: pv.ValueBoolean,
			CreatedBy:    ptrIfNonEmpty(userID),
		}); err != nil {
			slog.ErrorContext(ctx, "field inheritance: upsert asset field",
				"workspace_id", workspaceID, "asset_id", assetID,
				"project_id", projectID, "field_id", def.ID, "error", err)
		}
	}
	return nil
}

// -- AssetFieldRepository -----------------------------------------------------

type assetFieldRepo struct {
	d *db.DB
}

// NewAssetFieldRepo returns a repository.AssetFieldRepository backed by sqlc-generated queries.
func NewAssetFieldRepo(d *db.DB) repository.AssetFieldRepository {
	return &assetFieldRepo{d: d}
}

func (r *assetFieldRepo) GetValues(ctx context.Context, assetID string) ([]repository.FieldValue, error) {
	rows, err := r.d.RQ.GetAssetFieldValues(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.FieldValue, len(rows))
	for i, row := range rows {
		out[i] = toFieldValue(
			row.FieldID,
			row.FieldKey,
			row.FieldName,
			row.FieldType,
			row.FieldSource,
			row.FieldOptions,
			row.ValueText,
			row.ValueNumber,
			row.ValueDate,
			row.ValueBoolean,
			row.DefinitionDeleted,
		)
	}
	return out, nil
}

func (r *assetFieldRepo) DeleteValue(ctx context.Context, assetID, fieldID string) error {
	return r.d.WQ.DeleteAssetFieldValue(ctx, dbgen.DeleteAssetFieldValueParams{AssetID: assetID, FieldID: fieldID})
}

func (r *assetFieldRepo) UpsertValue(ctx context.Context, assetID string, p repository.SetFieldValueParams) error {
	_, err := r.d.WQ.UpsertAssetFieldValue(ctx, dbgen.UpsertAssetFieldValueParams{
		ID:           uuid.NewString(),
		AssetID:      assetID,
		FieldID:      p.FieldID,
		ValueText:    p.ValueText,
		ValueNumber:  p.ValueNumber,
		ValueDate:    p.ValueDate,
		ValueBoolean: p.ValueBoolean,
		CreatedBy:    ptrIfNonEmpty(p.CreatedBy),
	})
	return err
}

func ptrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r *assetFieldRepo) RunInTx(ctx context.Context, fn func(tx repository.AssetFieldRepository) error) error {
	tx, err := r.d.Writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after read-only queries or commit.
	txRepo := &assetFieldRepo{d: r.d.WithTx(tx)}
	if err = fn(txRepo); err != nil {
		return err
	}
	return tx.Commit()
}

// -- ProjectFieldRepository ---------------------------------------------------

type projectFieldRepo struct {
	d *db.DB
}

// NewProjectFieldRepo returns a repository.ProjectFieldRepository backed by sqlc-generated queries.
func NewProjectFieldRepo(d *db.DB) repository.ProjectFieldRepository {
	return &projectFieldRepo{d: d}
}

func (r *projectFieldRepo) GetValues(ctx context.Context, projectID string) ([]repository.FieldValue, error) {
	rows, err := r.d.RQ.GetProjectFieldValues(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.FieldValue, len(rows))
	for i, row := range rows {
		out[i] = toFieldValue(row.FieldID, row.FieldKey, row.FieldName, row.FieldType, "", row.FieldOptions,
			row.ValueText, row.ValueNumber, row.ValueDate, row.ValueBoolean, row.DefinitionDeleted)
	}
	return out, nil
}

func (r *projectFieldRepo) DeleteValue(ctx context.Context, projectID, fieldID string) error {
	return r.d.WQ.DeleteProjectFieldValue(
		ctx,
		dbgen.DeleteProjectFieldValueParams{ProjectID: projectID, FieldID: fieldID},
	)
}

func (r *projectFieldRepo) UpsertValue(ctx context.Context, projectID string, p repository.SetFieldValueParams) error {
	_, err := r.d.WQ.UpsertProjectFieldValue(ctx, dbgen.UpsertProjectFieldValueParams{
		ID:           uuid.NewString(),
		ProjectID:    projectID,
		FieldID:      p.FieldID,
		ValueText:    p.ValueText,
		ValueNumber:  p.ValueNumber,
		ValueDate:    p.ValueDate,
		ValueBoolean: p.ValueBoolean,
		CreatedBy:    p.CreatedBy,
	})
	return err
}

func (r *fieldRepo) PurgeExpired(ctx context.Context) (int, error) {
	ids, err := r.d.WQ.HardDeleteExpiredFieldDefinitions(ctx)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := r.d.Writer.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // best-effort

	qtx := r.d.WQ.WithTx(tx)
	for _, id := range ids {
		if err = qtx.DeleteAssetFieldValuesByField(ctx, id); err != nil {
			return 0, err
		}
		if err = qtx.DeleteProjectFieldValuesByField(ctx, id); err != nil {
			return 0, err
		}
		if err = qtx.HardDeleteFieldDefinition(ctx, id); err != nil {
			return 0, err
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func toFieldValue(fieldID, fieldKey, fieldName, fieldType, fieldSource string, fieldOptions *string,
	valueText *string, valueNumber *float64, valueDate *string, valueBoolean *int64, definitionDeleted int64,
) repository.FieldValue {
	return repository.FieldValue{
		FieldID:           fieldID,
		FieldKey:          fieldKey,
		FieldName:         fieldName,
		FieldType:         fieldType,
		FieldSource:       fieldSource,
		FieldOptions:      fieldOptions,
		ValueText:         valueText,
		ValueNumber:       valueNumber,
		ValueDate:         valueDate,
		ValueBoolean:      valueBoolean,
		DefinitionDeleted: definitionDeleted != 0,
	}
}
