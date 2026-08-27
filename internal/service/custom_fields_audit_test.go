package service_test

import (
	"context"
	"testing"

	"damask/server/internal/audit"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"
)

// newAssetFieldSvcSpy returns an AssetFieldService with a spy audit writer.
// All repos are sqlc-backed so FK constraints in asset_field_values are satisfied.
func newAssetFieldSvcSpy(t *testing.T) (service.AssetFieldService, *dbgen.Queries, *spyWriter) {
	t.Helper()
	database, err := dbpkg.Open(t.TempDir() + "/fields_asset.db")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	spy := newSpy()
	svc := service.NewAssetFieldService(
		reposqlc.NewAssetRepo(database),
		reposqlc.NewFieldRepo(database),
		reposqlc.NewAssetFieldRepo(database),
		spy,
	)
	return svc, queries, spy
}

// newProjectFieldSvcSpy returns a ProjectFieldService with a spy audit writer.
func newProjectFieldSvcSpy(t *testing.T) (service.ProjectFieldService, *dbgen.Queries, *spyWriter) {
	t.Helper()
	database, err := dbpkg.Open(t.TempDir() + "/fields_project.db")
	queries := database.WQ
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	spy := newSpy()
	svc := service.NewProjectFieldService(
		reposqlc.NewProjectRepo(database),
		reposqlc.NewFieldRepo(database),
		reposqlc.NewProjectFieldRepo(database),
		spy,
	)
	return svc, queries, spy
}

const fieldTestUserID = "user_fields_1"

// seedWorkspaceAndUser inserts a workspace and a shared test user.
func seedWorkspaceAndUser(t *testing.T, queries *dbgen.Queries, wsID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "t"}); err != nil {
		t.Fatalf("seed workspace %q: %v", wsID, err)
	}
	// Each test has its own DB, so this always succeeds.
	if _, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID: fieldTestUserID, Email: fieldTestUserID + "@t.com", PasswordHash: "x", Name: "t",
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

// seedFieldDef creates a field definition and returns its ID.
func seedFieldDef(t *testing.T, queries *dbgen.Queries, wsID, scope, key, name string) string {
	t.Helper()
	row, err := queries.CreateFieldDefinition(context.Background(), dbgen.CreateFieldDefinitionParams{
		ID:          "fld_" + key,
		WorkspaceID: wsID,
		Scope:       scope,
		Key:         key,
		Name:        name,
		FieldType:   "text",
		CreatedBy:   &[]string{fieldTestUserID}[0],
	})
	if err != nil {
		t.Fatalf("seed field definition %q: %v", key, err)
	}
	return row.ID
}

// seedAsset inserts a minimal asset row into the SQLite DB.
func seedAsset(t *testing.T, queries *dbgen.Queries, wsID, assetID string) {
	t.Helper()
	if _, err := queries.CreateAsset(context.Background(), dbgen.CreateAssetParams{
		ID:               assetID,
		WorkspaceID:      wsID,
		OriginalFilename: "test.txt",
		StorageKey:       "k/" + assetID,
		MimeType:         "text/plain",
	}); err != nil {
		t.Fatalf("seed asset %q: %v", assetID, err)
	}
}

// seedProject inserts a minimal project row into the SQLite DB.
func seedProject(t *testing.T, queries *dbgen.Queries, wsID, projectID string) {
	t.Helper()
	if _, err := queries.CreateProject(context.Background(), dbgen.CreateProjectParams{
		ID:          projectID,
		WorkspaceID: wsID,
		Name:        "t",
	}); err != nil {
		t.Fatalf("seed project %q: %v", projectID, err)
	}
}

// --- AssetFieldService audit ---

func TestAssetFieldService_SetValues_Set_EmitsAuditEvent(t *testing.T) {
	svc, queries, spy := newAssetFieldSvcSpy(t)
	ctx := context.Background()

	wsID := "ws_af_1"
	assetID := "ast_af_1"
	seedWorkspaceAndUser(t, queries, wsID)
	seedAsset(t, queries, wsID, assetID)
	fieldID := seedFieldDef(t, queries, wsID, "asset", "rating", "Rating")

	_, err := svc.SetValues(ctx, wsID, assetID, fieldTestUserID, []service.SetFieldValueInput{
		{FieldID: fieldID, Value: "5"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetFieldSet {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetFieldSet)
	}
	if e.AssetID != assetID {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, assetID)
	}
	if e.WorkspaceID != wsID {
		t.Errorf("WorkspaceID: got %q, want %q", e.WorkspaceID, wsID)
	}
}

func TestAssetFieldService_SetValues_Clear_EmitsAuditEvent(t *testing.T) {
	svc, queries, spy := newAssetFieldSvcSpy(t)
	ctx := context.Background()

	wsID := "ws_af_2"
	assetID := "ast_af_2"
	seedWorkspaceAndUser(t, queries, wsID)
	seedAsset(t, queries, wsID, assetID)
	fieldID := seedFieldDef(t, queries, wsID, "asset", "note", "Note")

	// Set a value first so clearing it produces a "cleared" event.
	if _, err := svc.SetValues(ctx, wsID, assetID, fieldTestUserID, []service.SetFieldValueInput{
		{FieldID: fieldID, Value: "hello"},
	}); err != nil {
		t.Fatalf("set: %v", err)
	}
	spy.asset = nil // reset

	// Clear by passing nil value.
	if _, err := svc.SetValues(ctx, wsID, assetID, fieldTestUserID, []service.SetFieldValueInput{
		{FieldID: fieldID, Value: nil},
	}); err != nil {
		t.Fatalf("clear: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetFieldCleared {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetFieldCleared)
	}
}

// --- ProjectFieldService audit ---

func TestProjectFieldService_SetValues_Set_EmitsAuditEvent(t *testing.T) {
	svc, queries, spy := newProjectFieldSvcSpy(t)
	ctx := context.Background()

	wsID := "ws_pf_1"
	projectID := "proj_pf_1"
	seedWorkspaceAndUser(t, queries, wsID)
	seedProject(t, queries, wsID, projectID)
	fieldID := seedFieldDef(t, queries, wsID, "project", "budget", "Budget")

	_, err := svc.SetValues(ctx, wsID, projectID, fieldTestUserID, []service.SetFieldValueInput{
		{FieldID: fieldID, Value: "1000"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastProject()
	if e.EventType != audit.EventProjectFieldSet {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventProjectFieldSet)
	}
	if e.ProjectID != projectID {
		t.Errorf("ProjectID: got %q, want %q", e.ProjectID, projectID)
	}
	if e.WorkspaceID != wsID {
		t.Errorf("WorkspaceID: got %q, want %q", e.WorkspaceID, wsID)
	}
}

func TestProjectFieldService_SetValues_Clear_EmitsAuditEvent(t *testing.T) {
	svc, queries, spy := newProjectFieldSvcSpy(t)
	ctx := context.Background()

	wsID := "ws_pf_2"
	projectID := "proj_pf_2"
	seedWorkspaceAndUser(t, queries, wsID)
	seedProject(t, queries, wsID, projectID)
	fieldID := seedFieldDef(t, queries, wsID, "project", "owner", "Owner")

	if _, err := svc.SetValues(ctx, wsID, projectID, fieldTestUserID, []service.SetFieldValueInput{
		{FieldID: fieldID, Value: "alice"},
	}); err != nil {
		t.Fatalf("set: %v", err)
	}
	spy.project = nil // reset

	if _, err := svc.SetValues(ctx, wsID, projectID, fieldTestUserID, []service.SetFieldValueInput{
		{FieldID: fieldID, Value: nil},
	}); err != nil {
		t.Fatalf("clear: %v", err)
	}
	e := spy.lastProject()
	if e.EventType != audit.EventProjectFieldCleared {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventProjectFieldCleared)
	}
}
