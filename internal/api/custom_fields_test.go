//go:build integration

package api_test

import (
	"damask/server/internal/api"
	"damask/server/internal/auth"
	th "damask/server/internal/testhelpers"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// -- helpers -----------------------------------------------------------------

func createFieldDef(t *testing.T, env *th.TestEnv, cookie *http.Cookie, scope, name, key, fieldType string, options *string) api.FieldDefinitionResponse {
	t.Helper()
	req := api.CreateFieldDefinitionRequest{
		Scope:     scope,
		Name:      name,
		Key:       key,
		FieldType: fieldType,
		Options:   options,
	}
	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/field-definitions", th.JSONBody(req), cookie))
	if err != nil {
		t.Fatalf("create field def: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create field def: expected 201, got %d", resp.StatusCode)
	}
	var result api.FieldDefinitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode field def: %v", err)
	}
	return result
}

// -- CF-1: Field definitions CRUD --------------------------------------------

func TestFieldDefinitions_CRUD(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create
	def := createFieldDef(t, env, u.Cookie, "asset", "Client Name", "client_name", "text", nil)
	if def.Key != "client_name" {
		t.Fatalf("expected key=client_name, got %v", def.Key)
	}
	if def.FieldType != "text" {
		t.Fatalf("expected field_type=text, got %v", def.FieldType)
	}
	id := def.ID

	// List
	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", resp.StatusCode)
	}
	var list []api.FieldDefinitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(list))
	}

	// Get single
	resp, _ = env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/field-definitions/"+id, nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp.StatusCode)
	}

	// Update name (allowed)
	nameVal := "Client"
	resp, _ = env.App.Test(th.AuthRequest(http.MethodPut, "/api/v1/field-definitions/"+id,
		th.JSONBody(api.UpdateFieldDefinitionRequest{Name: &nameVal}), u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", resp.StatusCode)
	}
	var updated api.FieldDefinitionResponse
	_ = json.NewDecoder(resp.Body).Decode(&updated)
	if updated.Name != "Client" {
		t.Fatalf("expected name=Client, got %v", updated.Name)
	}

	// Update key (forbidden)
	keyVal := "new_key"
	resp, _ = env.App.Test(th.AuthRequest(http.MethodPut, "/api/v1/field-definitions/"+id,
		th.JSONBody(api.UpdateFieldDefinitionRequest{Key: &keyVal}), u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("update key: expected 422, got %d", resp.StatusCode)
	}

	// Update field_type (forbidden)
	fieldTypeVal := "number"
	resp, _ = env.App.Test(th.AuthRequest(http.MethodPut, "/api/v1/field-definitions/"+id,
		th.JSONBody(api.UpdateFieldDefinitionRequest{FieldType: &fieldTypeVal}), u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("update field_type: expected 422, got %d", resp.StatusCode)
	}

	// Soft delete
	resp, _ = env.App.Test(th.AuthRequest(http.MethodDelete, "/api/v1/field-definitions/"+id, nil, u.Cookie))
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", resp.StatusCode)
	}

	// List after delete — should be empty
	resp, _ = env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, u.Cookie))
	_ = json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Fatalf("expected 0 active definitions after delete, got %d", len(list))
	}
}

func TestFieldDefinitions_SelectValidation(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// select without options → 400
	req := api.CreateFieldDefinitionRequest{
		Scope:     "asset",
		Name:      "Status",
		Key:       "status",
		FieldType: "select",
	}
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/field-definitions",
		th.JSONBody(req), u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("select no options: expected 422, got %d", resp.StatusCode)
	}

	// select with valid options → 201
	opts := `["Draft","Review","Approved"]`
	def := createFieldDef(t, env, u.Cookie, "asset", "Status", "status", "select", &opts)
	if def.Options == nil {
		t.Fatal("expected options to be set")
	}
}

func TestFieldDefinitions_InvalidKey(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := api.CreateFieldDefinitionRequest{
		Scope:     "asset",
		Name:      "Bad Key",
		Key:       "Bad Key!",
		FieldType: "text",
	}
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/field-definitions",
		th.JSONBody(req), u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bad key: expected 422, got %d", resp.StatusCode)
	}
}

func TestFieldDefinitions_DuplicateKey(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)

	req := api.CreateFieldDefinitionRequest{
		Scope:     "asset",
		Name:      "Client2",
		Key:       "client",
		FieldType: "text",
	}
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/field-definitions",
		th.JSONBody(req), u.Cookie))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate key: expected 409, got %d", resp.StatusCode)
	}
}

func TestFieldDefinitions_ScopeIsolation(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Same key allowed in different scopes
	createFieldDef(t, env, u.Cookie, "asset", "Budget", "budget", "number", nil)
	createFieldDef(t, env, u.Cookie, "project", "Budget", "budget", "number", nil)

	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, u.Cookie))
	var list []api.FieldDefinitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode asset scope: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("asset scope: expected 1, got %d", len(list))
	}

	resp, _ = env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/field-definitions?scope=project", nil, u.Cookie))
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode project scope: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("project scope: expected 1, got %d", len(list))
	}
}

func TestFieldDefinitions_Reorder(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	d1 := createFieldDef(t, env, u.Cookie, "asset", "Alpha", "alpha", "text", nil)
	d2 := createFieldDef(t, env, u.Cookie, "asset", "Beta", "beta", "text", nil)

	// Reorder expects a bare JSON array, not an object
	items := []api.ReorderFieldEntry{
		{ID: d1.ID, Position: 10},
		{ID: d2.ID, Position: 5},
	}
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPut, "/api/v1/field-definitions/reorder",
		th.JSONBody(items), u.Cookie))
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("reorder: expected 204, got %d", resp.StatusCode)
	}

	// After reorder, beta (pos=5) should come before alpha (pos=10)
	resp, _ = env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, u.Cookie))
	var list []api.FieldDefinitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode reordered: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(list))
	}
	if list[0].Key != "beta" {
		t.Fatalf("expected beta first after reorder, got %v", list[0].Key)
	}
}

func TestFieldDefinitions_Stats_Asset(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)
	id := def.ID

	// Create an asset and set a field value to have a count
	assetID := "stats-asset-" + t.Name()
	_, err := env.Database.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
		VALUES (?, ?, ?, ?, ?, ?)`, assetID, u.WorkspaceID, "test.jpg", "storage/test.jpg", "image/jpeg", 100)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	req := api.PatchAssetFieldsRequest{
		Values: []api.FieldValueInput{{FieldID: id, Value: "Test Client"}},
	}
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
		th.JSONBody(req), u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch fields: expected 200, got %d", resp.StatusCode)
	}

	// Now check stats
	resp, _ = env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/field-definitions/"+id+"/stats", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stats: expected 200, got %d", resp.StatusCode)
	}
	var stats api.FieldDefinitionStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.AssetCount != 1 {
		t.Fatalf("expected AssetCount=1, got %d", stats.AssetCount)
	}
	if stats.ProjectCount != 0 {
		t.Fatalf("expected ProjectCount=0, got %d", stats.ProjectCount)
	}
}

func TestFieldDefinitions_Stats_Project(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "project", "Budget", "budget", "number", nil)
	id := def.ID

	// Create a project and set a field value to have a count
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JSONBody(api.CreateProjectRequest{Name: "Test Project"}), u.Cookie))
	var proj api.ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&proj); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	projectID := proj.ID

	req := api.PatchProjectFieldsRequest{
		Values: []api.FieldValueInput{{FieldID: id, Value: 50000}},
	}
	resp, _ = env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/projects/"+projectID+"/fields",
		th.JSONBody(req), u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch fields: expected 200, got %d", resp.StatusCode)
	}

	// Now check stats
	resp, _ = env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/field-definitions/"+id+"/stats", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stats: expected 200, got %d", resp.StatusCode)
	}
	var stats api.FieldDefinitionStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.ProjectCount != 1 {
		t.Fatalf("expected ProjectCount=1, got %d", stats.ProjectCount)
	}
	if stats.AssetCount != 0 {
		t.Fatalf("expected AssetCount=0, got %d", stats.AssetCount)
	}
}

func TestFieldDefinitions_Auth(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")
	def := createFieldDef(t, env, u.Cookie, "asset", "X", "x", "text", nil)

	viewerToken := th.MintEditorToken(t, env, u.WorkspaceID, auth.Viewer)

	// Unauthenticated on every mutating endpoint.
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/field-definitions"},
		{http.MethodPost, "/api/v1/field-definitions"},
		{http.MethodGet, "/api/v1/field-definitions/" + def.ID},
		{http.MethodPut, "/api/v1/field-definitions/" + def.ID},
		{http.MethodDelete, "/api/v1/field-definitions/" + def.ID},
	} {
		resp, _ := env.App.Test(th.AuthRequest(tc.method, tc.path, nil, nil))
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unauth %s %s: expected 401, got %d", tc.method, tc.path, resp.StatusCode)
		}
	}

	// Viewer cannot create, update, or delete.
	resp, _ := env.App.Test(th.BearerRequest(http.MethodPost, "/api/v1/field-definitions",
		th.JSONBody(api.CreateFieldDefinitionRequest{Scope: "asset", Name: "Y", Key: "y", FieldType: "text"}), viewerToken))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer create: expected 403, got %d", resp.StatusCode)
	}

	nameVal := "Updated"
	resp, _ = env.App.Test(th.BearerRequest(http.MethodPut, "/api/v1/field-definitions/"+def.ID,
		th.JSONBody(api.UpdateFieldDefinitionRequest{Name: &nameVal}), viewerToken))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer update: expected 403, got %d", resp.StatusCode)
	}

	resp, _ = env.App.Test(th.BearerRequest(http.MethodDelete, "/api/v1/field-definitions/"+def.ID, nil, viewerToken))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer delete: expected 403, got %d", resp.StatusCode)
	}
}

func TestFieldDefinitions_Stats_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/field-definitions/nonexistent-id/stats", nil, u.Cookie))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("stats 404: expected 404, got %d", resp.StatusCode)
	}
}

// -- CF-2: Asset field values ------------------------------------------------

func TestAssetFieldValues_GetPatch(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)
	fieldID := def.ID

	// Create an asset via direct SQL (no file upload needed)
	assetID := "test-asset-fields-01"
	_, err := env.Database.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
		VALUES (?, ?, ?, ?, ?, ?)`, assetID, u.WorkspaceID, "test.jpg", "storage/test.jpg", "image/jpeg", 100)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	// GET — no values yet
	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/fields", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get fields: expected 200, got %d", resp.StatusCode)
	}
	var result api.GetAssetFieldsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode fields: %v", err)
	}
	if len(result.Fields) != 0 {
		t.Fatalf("expected 0 fields, got %d", len(result.Fields))
	}

	// PATCH — set value
	req := api.PatchAssetFieldsRequest{
		Values: []api.FieldValueInput{{FieldID: fieldID, Value: "Nike"}},
	}
	resp, _ = env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
		th.JSONBody(req), u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch fields: expected 200, got %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode patched fields: %v", err)
	}
	if len(result.Fields) != 1 {
		t.Fatalf("expected 1 field after patch, got %d", len(result.Fields))
	}
	if result.Fields[0].Value != "Nike" {
		t.Fatalf("expected value=Nike, got %v", result.Fields[0].Value)
	}

	// PATCH — clear value (null)
	reqClear := api.PatchAssetFieldsRequest{
		Values: []api.FieldValueInput{{FieldID: fieldID, Value: nil}},
	}
	resp, _ = env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
		th.JSONBody(reqClear), u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear field: expected 200, got %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode cleared fields: %v", err)
	}
	if len(result.Fields) != 0 {
		t.Fatalf("expected 0 fields after clear, got %d", len(result.Fields))
	}
}

func TestAssetFieldValues_TypeValidation(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	numDef := createFieldDef(t, env, u.Cookie, "asset", "Price", "price", "number", nil)
	dateDef := createFieldDef(t, env, u.Cookie, "asset", "Expiry", "expiry", "date", nil)
	boolDef := createFieldDef(t, env, u.Cookie, "asset", "Published", "published", "boolean", nil)
	opts := `["Draft","Approved"]`
	selDef := createFieldDef(t, env, u.Cookie, "asset", "Status", "status", "select", &opts)

	assetID := "test-asset-validation"
	_, _ = env.Database.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
		VALUES (?, ?, ?, ?, ?, ?)`, assetID, u.WorkspaceID, "t.jpg", "s/t.jpg", "image/jpeg", 100)

	cases := []struct {
		name     string
		fieldID  string
		value    string
		wantCode int
	}{
		{"number valid", numDef.ID, `42.5`, http.StatusOK},
		{"number string", numDef.ID, `"not a number"`, http.StatusUnprocessableEntity},
		{"date valid", dateDef.ID, `"2026-12-31"`, http.StatusOK},
		{"date bad format", dateDef.ID, `"31-12-2026"`, http.StatusUnprocessableEntity},
		{"boolean valid", boolDef.ID, `true`, http.StatusOK},
		{"boolean string", boolDef.ID, `"yes"`, http.StatusUnprocessableEntity},
		{"select valid", selDef.ID, `"Draft"`, http.StatusOK},
		{"select invalid", selDef.ID, `"Unknown"`, http.StatusUnprocessableEntity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var val interface{}
			_ = json.Unmarshal([]byte(tc.value), &val)
			req := api.PatchAssetFieldsRequest{
				Values: []api.FieldValueInput{{FieldID: tc.fieldID, Value: val}},
			}
			resp, _ := env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
				th.JSONBody(req), u.Cookie))
			if resp.StatusCode != tc.wantCode {
				t.Fatalf("%s: expected %d, got %d", tc.name, tc.wantCode, resp.StatusCode)
			}
		})
	}
}

func TestAssetFieldValues_SoftDeletedField(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Old Field", "old_field", "text", nil)
	fieldID := def.ID

	assetID := "test-asset-deleted-field"
	_, _ = env.Database.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
		VALUES (?, ?, ?, ?, ?, ?)`, assetID, u.WorkspaceID, "t.jpg", "s/t.jpg", "image/jpeg", 100)

	// Set a value
	req := api.PatchAssetFieldsRequest{
		Values: []api.FieldValueInput{{FieldID: fieldID, Value: "some value"}},
	}
	_, _ = env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
		th.JSONBody(req), u.Cookie))

	// Soft delete the definition
	_, _ = env.App.Test(th.AuthRequest(http.MethodDelete, "/api/v1/field-definitions/"+fieldID, nil, u.Cookie))

	// Cannot write to soft-deleted field
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/"+assetID+"/fields",
		th.JSONBody(req), u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("write to deleted field: expected 422, got %d", resp.StatusCode)
	}

	// But GET still shows the orphaned value with definition_deleted=true
	resp, _ = env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/fields", nil, u.Cookie))
	var result api.GetAssetFieldsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode orphaned fields: %v", err)
	}
	if len(result.Fields) != 1 {
		t.Fatalf("expected orphaned value to still appear, got %d fields", len(result.Fields))
	}
	if !result.Fields[0].DefinitionDeleted {
		t.Fatalf("expected definition_deleted=true, got %v", result.Fields[0].DefinitionDeleted)
	}
}

func TestAssetFieldValues_BulkPatch(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)
	fieldID := def.ID

	// Create two assets
	for _, id := range []string{"bulk-asset-1", "bulk-asset-2"} {
		_, _ = env.Database.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
			VALUES (?, ?, ?, ?, ?, ?)`, id, u.WorkspaceID, id+".jpg", "s/"+id, "image/jpeg", 100)
	}

	req := api.BulkPatchAssetFieldsRequest{
		AssetIDs: []string{"bulk-asset-1", "bulk-asset-2"},
		Values:   []api.FieldValueInput{{FieldID: fieldID, Value: "Nike"}},
	}
	resp, err2 := env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/bulk/fields",
		th.JSONBody(req), u.Cookie), fiber.TestConfig{Timeout: 5 * time.Second})
	if err2 != nil {
		t.Fatalf("bulk patch request: %v", err2)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bulk patch: expected 200, got %d", resp.StatusCode)
	}
	var bulkResult api.BulkPatchAssetFieldsResponse
	if err3 := json.NewDecoder(resp.Body).Decode(&bulkResult); err3 != nil {
		t.Fatalf("decode bulk patch response: %v", err3)
	}
	if bulkResult.Updated != 2 {
		t.Fatalf("expected updated=2, got %v", bulkResult.Updated)
	}

	// Verify values were set
	for _, id := range []string{"bulk-asset-1", "bulk-asset-2"} {
		resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/assets/"+id+"/fields", nil, u.Cookie))
		var r api.GetAssetFieldsResponse
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			t.Fatalf("decode asset fields for %s: %v", id, err)
		}
		if len(r.Fields) != 1 || r.Fields[0].Value != "Nike" {
			t.Fatalf("asset %s: expected value=Nike", id)
		}
	}
}

// -- CF-2.4: Asset field filters ---------------------------------------------

func TestAssetFieldFilter(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)
	fieldID := def.ID

	// Create assets
	for _, id := range []string{"filter-asset-nike", "filter-asset-puma"} {
		_, _ = env.Database.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
			VALUES (?, ?, ?, ?, ?, ?)`, id, u.WorkspaceID, id+".jpg", "s/"+id, "image/jpeg", 100)
	}

	// Set client=Nike on first asset
	req := api.PatchAssetFieldsRequest{
		Values: []api.FieldValueInput{{FieldID: fieldID, Value: "Nike"}},
	}
	_, _ = env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/filter-asset-nike/fields",
		th.JSONBody(req), u.Cookie))

	req2 := api.PatchAssetFieldsRequest{
		Values: []api.FieldValueInput{{FieldID: fieldID, Value: "Puma"}},
	}
	_, _ = env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/filter-asset-puma/fields",
		th.JSONBody(req2), u.Cookie))

	// Filter by exact match
	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/assets?field[client]=Nike", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("field filter: expected 200, got %d", resp.StatusCode)
	}
	var result api.AssetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode asset list: %v", err)
	}
	if len(result.Assets) != 1 {
		t.Fatalf("filter by client=Nike: expected 1, got %d", len(result.Assets))
	}

	// Filter contains
	resp, _ = env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/assets?field[client][contains]=ik", nil, u.Cookie))
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode asset list contains: %v", err)
	}
	if len(result.Assets) != 1 {
		t.Fatalf("filter contains ik: expected 1, got %d", len(result.Assets))
	}
}

func TestAssetFieldFilter_TooManyFilters(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet,
		"/api/v1/assets?field[a]=1&field[b]=2&field[c]=3&field[d]=4&field[e]=5&field[f]=6",
		nil, u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("too many filters: expected 422, got %d", resp.StatusCode)
	}
}

func TestAssetFieldFilter_NumericGtLt(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	def := createFieldDef(t, env, u.Cookie, "asset", "Price", "price", "number", nil)
	fieldID := def.ID

	// price=9 and price=10 — lexicographic sort would put "9" > "10", numeric sort must not.
	for _, row := range []struct{ id, price string }{
		{"numfilter-cheap", "9"},
		{"numfilter-mid", "10"},
		{"numfilter-pricey", "20"},
	} {
		_, _ = env.Database.Exec(`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
			VALUES (?, ?, ?, ?, ?, ?)`, row.id, u.WorkspaceID, row.id+".jpg", "s/"+row.id, "image/jpeg", 100)

		var val float64
		switch row.price {
		case "9":
			val = 9
		case "10":
			val = 10
		default:
			val = 20
		}
		req := api.PatchAssetFieldsRequest{
			Values: []api.FieldValueInput{{FieldID: fieldID, Value: val}},
		}
		_, _ = env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/"+row.id+"/fields",
			th.JSONBody(req), u.Cookie))
	}

	// gt=9 should return price=10 and price=20 (2 assets), not just price=20 as lexicographic would give.
	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/assets?field[price][gt]=9", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gt filter: expected 200, got %d", resp.StatusCode)
	}
	var result api.AssetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode gt result: %v", err)
	}
	if len(result.Assets) != 2 {
		t.Fatalf("gt=9: expected 2 assets (price 10 and 20), got %d", len(result.Assets))
	}

	// lt=10 should return only price=9 (1 asset).
	resp, _ = env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/assets?field[price][lt]=10", nil, u.Cookie))
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode lt result: %v", err)
	}
	if len(result.Assets) != 1 {
		t.Fatalf("lt=10: expected 1 asset (price 9), got %d", len(result.Assets))
	}
}

// -- CF-3: Project field values ----------------------------------------------

func TestProjectFieldValues(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create a project field definition
	def := createFieldDef(t, env, u.Cookie, "project", "Budget", "budget", "number", nil)
	fieldID := def.ID

	// Create a project
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JSONBody(api.CreateProjectRequest{Name: "Test Project"}), u.Cookie))
	var proj api.ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&proj); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	projectID := proj.ID

	// GET — empty
	resp, _ = env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/fields", nil, u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get project fields: expected 200, got %d", resp.StatusCode)
	}
	var result api.GetProjectFieldsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode project fields: %v", err)
	}
	if len(result.Fields) != 0 {
		t.Fatalf("expected 0 fields, got %d", len(result.Fields))
	}

	// PATCH — set value
	req := api.PatchProjectFieldsRequest{
		Values: []api.FieldValueInput{{FieldID: fieldID, Value: 50000}},
	}
	resp, _ = env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/projects/"+projectID+"/fields",
		th.JSONBody(req), u.Cookie))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch project fields: expected 200, got %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode patched project fields: %v", err)
	}
	if len(result.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(result.Fields))
	}
	if result.Fields[0].Value != float64(50000) {
		t.Fatalf("expected value=50000, got %v", result.Fields[0].Value)
	}
}

func TestProjectFieldValues_WrongScope(t *testing.T) {
	env := th.SetupTestApp(t)
	u := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create an ASSET-scoped field
	def := createFieldDef(t, env, u.Cookie, "asset", "Client", "client", "text", nil)
	fieldID := def.ID

	// Create a project
	resp, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JSONBody(api.CreateProjectRequest{Name: "Test"}), u.Cookie))
	var proj api.ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&proj); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	projectID := proj.ID

	// Try to set an asset-scoped field on a project → 422
	req := api.PatchProjectFieldsRequest{
		Values: []api.FieldValueInput{{FieldID: fieldID, Value: "Nike"}},
	}
	resp, _ = env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/projects/"+projectID+"/fields",
		th.JSONBody(req), u.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("wrong scope: expected 422, got %d", resp.StatusCode)
	}
}

// -- Workspace isolation -----------------------------------------------------

func TestFieldDefinitions_WorkspaceIsolation(t *testing.T) {
	env := th.SetupTestApp(t)
	u1 := th.Register(t, env, "Alice", "alice@example.com", "password123")
	u2 := th.Register(t, env, "Bob", "bob@example.com", "password123")

	// u1 creates a field definition
	def := createFieldDef(t, env, u1.Cookie, "asset", "Secret", "secret", "text", nil)
	id := def.ID

	// u2 cannot see it
	resp, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, u2.Cookie))
	var list []api.FieldDefinitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode u2 field definitions: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("isolation: u2 should not see u1's field definitions, got %d", len(list))
	}

	// u2 cannot delete it
	resp, _ = env.App.Test(th.AuthRequest(http.MethodDelete, "/api/v1/field-definitions/"+id, nil, u2.Cookie))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("isolation: u2 delete u1's def: expected 404, got %d", resp.StatusCode)
	}
}
