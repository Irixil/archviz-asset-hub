//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"damask/server/internal/api"
	th "damask/server/internal/testhelpers"
)

func TestGetAssetFields_IncludesFieldSource(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	if _, err := env.Database.Exec(`
INSERT INTO field_definitions (id, workspace_id, source, scope, name, key, field_type, position, created_at, updated_at)
VALUES
  ('fd_user', ?, 'user', 'asset', 'Client', 'client', 'text', 0, datetime('now'), datetime('now')),
  ('fd_media_title', ?, 'media_tags', 'asset', 'Title', '_media_title', 'text', 1, datetime('now'), datetime('now'))`,
		owner.WorkspaceID, owner.WorkspaceID); err != nil {
		t.Fatalf("insert field defs: %v", err)
	}
	if _, err := env.Database.Exec(`
INSERT INTO asset_field_values (id, asset_id, field_id, value_text, created_at, updated_at)
VALUES
  ('afv_user', ?, 'fd_user', 'Nike', datetime('now'), datetime('now')),
  ('afv_media_title', ?, 'fd_media_title', 'Track title', datetime('now'), datetime('now'))`,
		asset.ID, asset.ID); err != nil {
		t.Fatalf("insert field values: %v", err)
	}

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/fields", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.GetAssetFieldsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(body.Fields))
	}

	gotSources := map[string]string{}
	for _, field := range body.Fields {
		gotSources[field.Key] = field.Source
	}
	if gotSources["client"] != "user" {
		t.Fatalf("expected user source for client, got %q", gotSources["client"])
	}
	if gotSources["_media_title"] != "media_tags" {
		t.Fatalf("expected media_tags source for _media_title, got %q", gotSources["_media_title"])
	}
}

func TestPatchAssetFields_RejectsSystemManagedField(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	if _, err := env.Database.Exec(`
INSERT INTO field_definitions (id, workspace_id, source, scope, name, key, field_type, position, created_at, updated_at)
VALUES ('fd_media_title', ?, 'media_tags', 'asset', 'Title', '_media_title', 'text', 0, datetime('now'), datetime('now'))`,
		owner.WorkspaceID); err != nil {
		t.Fatalf("insert field def: %v", err)
	}

	req := api.PatchAssetFieldsRequest{
		Values: []api.FieldValueInput{{
			FieldID: "fd_media_title",
			Value:   "Changed by user",
		}},
	}
	resp, err := env.App.Test(th.AuthRequest(http.MethodPatch, "/api/v1/assets/"+asset.ID+"/fields", th.JSONBody(req), owner.Cookie))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestFieldDefinitions_ListExcludesSystemSources(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	if _, err := env.Database.Exec(`
INSERT INTO field_definitions (id, workspace_id, source, scope, name, key, field_type, position, created_at, updated_at)
VALUES
  ('fd_user', ?, 'user', 'asset', 'Client', 'client', 'text', 0, datetime('now'), datetime('now')),
  ('fd_media_title', ?, 'media_tags', 'asset', 'Title', '_media_title', 'text', 1, datetime('now'), datetime('now'))`,
		owner.WorkspaceID, owner.WorkspaceID); err != nil {
		t.Fatalf("insert field defs: %v", err)
	}

	req := th.AuthRequest(http.MethodGet, "/api/v1/field-definitions?scope=asset", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var defs []api.FieldDefinitionResponse
	if err := json.NewDecoder(resp.Body).Decode(&defs); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 user field definition, got %d", len(defs))
	}
	if defs[0].Key != "client" {
		t.Fatalf("expected client field, got %q", defs[0].Key)
	}
}
