//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	"damask/server/internal/ingress"
	_ "damask/server/internal/ingress/sources/email_api"
	th "damask/server/internal/testhelpers"
)

// createIngressSource is a test helper that POSTs to /api/v1/ingress/sources
// and returns the parsed response. Fails the test if status != 201.
func createIngressSource(t *testing.T, env *th.TestEnv, cookie *http.Cookie, body string) map[string]any {
	t.Helper()
	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources", th.JSONStr(body), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("createIngressSource request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createIngressSource: expected 201, got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode createIngressSource response: %v", err)
	}
	return out
}

// --- Source CRUD

func TestCreateIngressSource_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	body := `{"type":"imap","label":"Test Inbox","config":{"host":"imap.example.com","password":"s3cr3t"},"poll_interval_min":30}`
	src := createIngressSource(t, env, user.Cookie, body)

	if src["id"] == nil || src["id"] == "" {
		t.Fatal("expected non-empty id in response")
	}
	if src["type"] != "imap" {
		t.Fatalf("type: got %v, want imap", src["type"])
	}
	if src["label"] != "Test Inbox" {
		t.Fatalf("label: got %v, want 'Test Inbox'", src["label"])
	}
	// Sensitive config values must be redacted
	config, ok := src["config"].(map[string]any)
	if !ok {
		t.Fatal("config should be an object")
	}
	if config["password"] != "***" {
		t.Fatalf("password should be redacted, got %v", config["password"])
	}
	if config["host"] != "imap.example.com" {
		t.Fatalf("non-sensitive key should not be redacted, got %v", config["host"])
	}
}

func TestCreateIngressSource_DefaultInterval(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// omit poll_interval_min — should default to 15
	body := `{"type":"sftp","label":"My SFTP","config":{}}`
	src := createIngressSource(t, env, user.Cookie, body)

	if src["poll_interval_min"] != float64(15) {
		t.Fatalf("expected default poll_interval_min=15, got %v", src["poll_interval_min"])
	}
}

func TestCreateIngressSource_MissingType(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources",
		th.JSONStr(`{"label":"No type","config":{}}`), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing type, got %d", resp.StatusCode)
	}
}

func TestCreateIngressSource_MissingLabel(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources",
		th.JSONStr(`{"type":"imap","config":{}}`), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing label, got %d", resp.StatusCode)
	}
}

func TestCreateIngressSource_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources",
		th.JSONStr(`{"type":"imap","label":"x","config":{}}`), nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListIngressSources_Empty(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d items", len(list))
	}
}

func TestListIngressSources_ReturnsOwnerSources(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Source A","config":{}}`)
	createIngressSource(t, env, user.Cookie,
		`{"type":"sftp","label":"Source B","config":{}}`)

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(list))
	}
}

func TestListIngressSources_WorkspaceIsolation(t *testing.T) {
	env := th.SetupTestApp(t)
	alice := th.Register(t, env, "Alice", "alice@example.com", "password123")
	bob := th.Register(t, env, "Bob", "bob@example.com", "password456")

	createIngressSource(t, env, alice.Cookie, `{"type":"imap","label":"Alice source","config":{}}`)

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources", nil, bob.Cookie)
	resp, _ := env.App.Test(req)
	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("Bob should not see Alice's sources, got %d", len(list))
	}
}

func TestGetIngressSource_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"My Inbox","config":{"host":"imap.example.com"}}`)
	id := created["id"].(string)

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/"+id, nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var src map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&src); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if src["id"] != id {
		t.Fatalf("id mismatch: got %v, want %v", src["id"], id)
	}
}

func TestGetIngressSource_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/does-not-exist", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetIngressSource_WrongWorkspace(t *testing.T) {
	env := th.SetupTestApp(t)
	alice := th.Register(t, env, "Alice", "alice@example.com", "password123")
	bob := th.Register(t, env, "Bob", "bob@example.com", "password456")

	created := createIngressSource(t, env, alice.Cookie,
		`{"type":"imap","label":"Alice","config":{}}`)
	id := created["id"].(string)

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/"+id, nil, bob.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when accessing another workspace's source, got %d", resp.StatusCode)
	}
}

func TestUpdateIngressSource_Label(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Old Label","config":{}}`)
	id := created["id"].(string)

	req := th.AuthRequest(http.MethodPut, "/api/v1/ingress/sources/"+id,
		th.JSONStr(`{"label":"New Label"}`), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var src map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&src); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if src["label"] != "New Label" {
		t.Fatalf("expected 'New Label', got %v", src["label"])
	}
}

func TestUpdateIngressSource_EnabledFlag(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	id := created["id"].(string)

	// Disable it
	req := th.AuthRequest(http.MethodPut, "/api/v1/ingress/sources/"+id,
		th.JSONStr(`{"enabled":false}`), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var src map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&src); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if src["enabled"] != false {
		t.Fatalf("expected enabled=false, got %v", src["enabled"])
	}
}

func TestUpdateIngressSource_ConfigUpdated(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{"host":"old.example.com","password":"old"}}`)
	id := created["id"].(string)

	req := th.AuthRequest(http.MethodPut, "/api/v1/ingress/sources/"+id,
		th.JSONStr(`{"config":{"host":"new.example.com","password":"newpass"}}`), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var src map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&src); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	config := src["config"].(map[string]any)
	if config["host"] != "new.example.com" {
		t.Fatalf("expected host updated, got %v", config["host"])
	}
	if config["password"] != "***" {
		t.Fatalf("password should be redacted, got %v", config["password"])
	}
}

func TestUpdateIngressSource_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPut, "/api/v1/ingress/sources/nonexistent",
		th.JSONStr(`{"label":"x"}`), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteIngressSource_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"To Delete","config":{}}`)
	id := created["id"].(string)

	req := th.AuthRequest(http.MethodDelete, "/api/v1/ingress/sources/"+id, nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify it's gone
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/"+id, nil, user.Cookie)
	resp2, _ := env.App.Test(req2)
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp2.StatusCode)
	}
}

func TestDeleteIngressSource_RequiresOwner(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	created := createIngressSource(t, env, owner.Cookie,
		`{"type":"imap","label":"Src","config":{}}`)
	id := created["id"].(string)

	editorToken := th.MintEditorToken(t, env, owner.WorkspaceID, auth.Editor)
	req := th.BearerRequest(http.MethodDelete, "/api/v1/ingress/sources/"+id, nil, editorToken)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for editor deleting source, got %d", resp.StatusCode)
	}
}

// --- Poll endpoint

func TestPollIngressSource_EnqueuesJob(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	id := created["id"].(string)

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources/"+id+"/poll", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["job_id"] == nil || body["job_id"] == "" {
		t.Fatal("expected job_id in response")
	}
}

func TestPollIngressSource_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources/nonexistent/poll", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Log API

func TestListIngressSourceLog_Empty(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	id := created["id"].(string)

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/"+id+"/log", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty log, got %d entries", len(list))
	}
}

func TestListIngressSourceLog_SourceNotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/nonexistent/log", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListWorkspaceIngressLog_Empty(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/log", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty log, got %d", len(list))
	}
}

func TestDeleteIngressLogEntry_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodDelete, "/api/v1/ingress/log/nonexistent", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteIngressLogEntry_WrongWorkspace(t *testing.T) {
	env := th.SetupTestApp(t)
	alice := th.Register(t, env, "Alice", "alice@example.com", "password123")
	bob := th.Register(t, env, "Bob", "bob@example.com", "password456")

	created := createIngressSource(t, env, alice.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	sourceID := created["id"].(string)

	// Insert a log entry directly for Alice's source
	entryID := "test-entry-id"
	_, err := env.Database.Exec(
		`INSERT INTO ingress_log (id, source_id, remote_id, filename, status)
		 VALUES (?, ?, ?, ?, 'imported')`,
		entryID, sourceID, "msg-001", "photo.jpg",
	)
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	// Bob tries to delete Alice's log entry. GetLogEntry join-scopes by
	// workspace via the source relation, so a cross-workspace entry ID is
	// indistinguishable from a nonexistent one — 404, not 403, to avoid
	// leaking whether the entry exists in another workspace.
	req := th.AuthRequest(http.MethodDelete, "/api/v1/ingress/log/"+entryID, nil, bob.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRetryIngressLogEntry_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/log/nonexistent/retry", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRetryIngressLogEntry_ImportedEntryNotRetryable(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	sourceID := created["id"].(string)

	entryID := "test-entry-imported"
	_, err := env.Database.Exec(
		`INSERT INTO ingress_log (id, source_id, remote_id, filename, status)
		 VALUES (?, ?, ?, ?, 'imported')`,
		entryID, sourceID, "msg-002", "photo.jpg",
	)
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/log/"+entryID+"/retry", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for retrying 'imported' entry, got %d", resp.StatusCode)
	}
}

func TestRetryIngressLogEntry_ErrorEntryRequeued(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	sourceID := created["id"].(string)

	entryID := "test-entry-error"
	errMsg := "connection refused"
	_, err := env.Database.Exec(
		`INSERT INTO ingress_log (id, source_id, remote_id, filename, status, error)
		 VALUES (?, ?, ?, ?, 'error', ?)`,
		entryID, sourceID, "msg-003", "photo.jpg", errMsg,
	)
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/log/"+entryID+"/retry", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["job_id"] == nil || body["job_id"] == "" {
		t.Fatal("expected job_id in retry response")
	}
}

func TestRetryIngressLogEntry_SkippedEntryRequeued(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	created := createIngressSource(t, env, user.Cookie,
		`{"type":"imap","label":"Inbox","config":{}}`)
	sourceID := created["id"].(string)

	entryID := "test-entry-skipped"
	_, err := env.Database.Exec(
		`INSERT INTO ingress_log (id, source_id, remote_id, filename, status)
		 VALUES (?, ?, ?, ?, 'skipped')`,
		entryID, sourceID, "msg-004", "invoice.pdf",
	)
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/log/"+entryID+"/retry", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 for skipped retry, got %d", resp.StatusCode)
	}
}

// --- Test endpoint (no side-effects, only validates credentials)

func TestTestIngressSource_UnknownSourceType(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create a source with a type that has no th.Registered constructor.
	created := createIngressSource(t, env, user.Cookie,
		`{"type":"unregistered_type","label":"Mystery","config":{}}`)
	id := created["id"].(string)

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources/"+id+"/test", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for unknown source type in /test, got %d", resp.StatusCode)
	}
}

func TestTestIngressSource_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources/nonexistent/test", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Source with initial rules

func TestCreateIngressSource_WithRules(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	body := `{
		"type":"imap",
		"label":"Filtered Inbox",
		"config":{},
		"rules":[
			{"position":0,"field":"filename","operator":"ends_with","value":".pdf","action":"allow"},
			{"position":1,"field":"mime_type","operator":"starts_with","value":"image/","action":"allow"}
		]
	}`
	src := createIngressSource(t, env, user.Cookie, body)
	id := src["id"].(string)

	// Verify source was created
	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/"+id, nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- rawToNullableString helper (unit test, no HTTP)

func TestRawToNullableString(t *testing.T) {
	cases := []struct {
		name        string
		input       *json.RawMessage
		wantValue   *string
		wantPresent bool
	}{
		{
			name:        "nil pointer (field absent)",
			input:       nil,
			wantValue:   nil,
			wantPresent: false,
		},
		{
			name:        "explicit JSON null",
			input:       rawMsg("null"),
			wantValue:   nil,
			wantPresent: true,
		},
		{
			name:        "JSON string value",
			input:       rawMsg(`"folder-abc"`),
			wantValue:   strPtr("folder-abc"),
			wantPresent: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotValue, gotPresent := api.RawToNullableString(tc.input)
			if gotPresent != tc.wantPresent {
				t.Fatalf("present: got %v, want %v", gotPresent, tc.wantPresent)
			}
			if tc.wantValue == nil && gotValue != nil {
				t.Fatalf("value: got %q, want nil", *gotValue)
			}
			if tc.wantValue != nil {
				if gotValue == nil {
					t.Fatal("value: got nil, want non-nil")
				}
				if *gotValue != *tc.wantValue {
					t.Fatalf("value: got %q, want %q", *gotValue, *tc.wantValue)
				}
			}
		})
	}
}

// --- redactConfig helper (unit test, no HTTP)

func TestRedactConfig(t *testing.T) {
	input := map[string]any{
		"host":             "imap.example.com",
		"password":         "s3cr3t",
		"secret_key":       "abc123",
		"access_token":     "tok-xyz",
		"api_key":          "key-999",
		"username":         "alice",
		"private_key_data": "-----BEGIN",
	}
	got := api.RedactConfig(input)

	notRedacted := []string{"host", "username"}
	for _, k := range notRedacted {
		if got[k] != input[k] {
			t.Errorf("key %q should not be redacted, got %v", k, got[k])
		}
	}

	redacted := []string{"password", "secret_key", "access_token", "api_key", "private_key_data"}
	for _, k := range redacted {
		if got[k] != "***" {
			t.Errorf("key %q should be redacted to ***, got %v", k, got[k])
		}
	}
}

// --- Rules CRUD

// createRule is a test helper: POST /api/v1/ingress/sources/:id/rules
func createRule(t *testing.T, env *th.TestEnv, cookie *http.Cookie, sourceID, body string) map[string]any {
	t.Helper()
	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources/"+sourceID+"/rules", th.JSONStr(body), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("createRule request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createRule: expected 201, got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode createRule response: %v", err)
	}
	return out
}

func TestListIngressRules_EmptyInitially(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")
	src := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/"+id+"/rules", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty rules list, got %d", len(list))
	}
}

func TestListIngressRules_SourceNotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/nonexistent/rules", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListIngressRules_WrongWorkspace(t *testing.T) {
	env := th.SetupTestApp(t)
	alice := th.Register(t, env, "Alice", "alice@example.com", "password123")
	bob := th.Register(t, env, "Bob", "bob@example.com", "password456")

	src := createIngressSource(t, env, alice.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/"+id+"/rules", nil, bob.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for wrong workspace, got %d", resp.StatusCode)
	}
}

func TestCreateIngressRule_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")
	src := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	rule := createRule(t, env, user.Cookie, id,
		`{"position":0,"field":"filename","operator":"ends_with","value":".pdf","action":"allow"}`)

	if rule["id"] == nil || rule["id"] == "" {
		t.Fatal("expected non-empty rule id")
	}
	if rule["field"] != "filename" {
		t.Fatalf("field: got %v, want filename", rule["field"])
	}
	if rule["operator"] != "ends_with" {
		t.Fatalf("operator: got %v, want ends_with", rule["operator"])
	}
	if rule["value"] != ".pdf" {
		t.Fatalf("value: got %v, want .pdf", rule["value"])
	}
	if rule["action"] != "allow" {
		t.Fatalf("action: got %v, want allow", rule["action"])
	}
	if rule["source_id"] != id {
		t.Fatalf("source_id: got %v, want %v", rule["source_id"], id)
	}
}

func TestCreateIngressRule_MissingFields(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")
	src := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources/"+id+"/rules",
		th.JSONStr(`{"position":0,"field":"filename"}`), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing fields, got %d", resp.StatusCode)
	}
}

func TestCreateIngressRule_RequiresEditor(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	src := createIngressSource(t, env, owner.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	viewerToken := th.MintEditorToken(t, env, owner.WorkspaceID, auth.Viewer)
	req := th.BearerRequest(http.MethodPost, "/api/v1/ingress/sources/"+id+"/rules",
		th.JSONStr(`{"position":0,"field":"filename","operator":"equals","value":"x","action":"deny"}`),
		viewerToken)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer creating rule, got %d", resp.StatusCode)
	}
}

func TestCreateIngressRule_SourceNotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/ingress/sources/nonexistent/rules",
		th.JSONStr(`{"position":0,"field":"filename","operator":"equals","value":"x","action":"deny"}`),
		user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListIngressRules_OrderedByPosition(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")
	src := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	createRule(t, env, user.Cookie, id,
		`{"position":2,"field":"filename","operator":"contains","value":"b","action":"deny"}`)
	createRule(t, env, user.Cookie, id,
		`{"position":0,"field":"filename","operator":"contains","value":"a","action":"allow"}`)
	createRule(t, env, user.Cookie, id,
		`{"position":1,"field":"mime_type","operator":"equals","value":"image/jpeg","action":"allow"}`)

	req := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/"+id+"/rules", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(list))
	}
	// Should be sorted by position ascending
	for i, want := range []float64{0, 1, 2} {
		if list[i]["position"] != want {
			t.Fatalf("rule[%d] position: got %v, want %v", i, list[i]["position"], want)
		}
	}
}

func TestUpdateIngressRule_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")
	src := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	srcID := src["id"].(string)

	rule := createRule(t, env, user.Cookie, srcID,
		`{"position":0,"field":"filename","operator":"ends_with","value":".pdf","action":"allow"}`)
	rid := rule["id"].(string)

	req := th.AuthRequest(http.MethodPut, "/api/v1/ingress/sources/"+srcID+"/rules/"+rid,
		th.JSONStr(`{"position":5,"action":"deny"}`), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var updated map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated["action"] != "deny" {
		t.Fatalf("action: got %v, want deny", updated["action"])
	}
	if updated["position"] != float64(5) {
		t.Fatalf("position: got %v, want 5", updated["position"])
	}
	// Untouched fields should keep old values
	if updated["field"] != "filename" {
		t.Fatalf("field should be preserved, got %v", updated["field"])
	}
}

func TestUpdateIngressRule_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")
	src := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	req := th.AuthRequest(http.MethodPut, "/api/v1/ingress/sources/"+id+"/rules/nonexistent",
		th.JSONStr(`{"action":"deny"}`), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateIngressRule_WrongSource(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	src1 := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"A","config":{}}`)
	src2 := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"B","config":{}}`)
	id1 := src1["id"].(string)
	id2 := src2["id"].(string)

	rule := createRule(t, env, user.Cookie, id1,
		`{"position":0,"field":"filename","operator":"equals","value":"x","action":"deny"}`)
	rid := rule["id"].(string)

	// Try to update rule belonging to src1 via src2's URL
	req := th.AuthRequest(http.MethodPut, "/api/v1/ingress/sources/"+id2+"/rules/"+rid,
		th.JSONStr(`{"action":"allow"}`), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when rule doesn't belong to source, got %d", resp.StatusCode)
	}
}

func TestDeleteIngressRule_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")
	src := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	rule := createRule(t, env, user.Cookie, id,
		`{"position":0,"field":"filename","operator":"equals","value":"x","action":"deny"}`)
	rid := rule["id"].(string)

	req := th.AuthRequest(http.MethodDelete, "/api/v1/ingress/sources/"+id+"/rules/"+rid, nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify it's gone
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/ingress/sources/"+id+"/rules", nil, user.Cookie)
	resp2, _ := env.App.Test(req2)
	var list []any
	if err := json.NewDecoder(resp2.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list after delete, got %d", len(list))
	}
}

func TestDeleteIngressRule_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")
	src := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	req := th.AuthRequest(http.MethodDelete, "/api/v1/ingress/sources/"+id+"/rules/nonexistent", nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- OnCreate hook

func TestCreateIngressSource_EmailAPI_GeneratesIngestToken(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	src := createIngressSource(t, env, user.Cookie, `{"type":"email_api","label":"Email Inbox","config":{}}`)

	// Response must redact the token ("token" matches the sensitive-key check)
	cfg, ok := src["config"].(map[string]any)
	if !ok {
		t.Fatal("expected config object in response")
	}
	if cfg["ingest_token"] != "***" {
		t.Fatalf("ingest_token should be redacted in response, got %v", cfg["ingest_token"])
	}

	// Verify the token was actually stored (non-empty) by reading from the DB.
	srcID := src["id"].(string)
	var encryptedConfig string
	if err := env.Database.QueryRow("SELECT config FROM ingress_sources WHERE id = ?", srcID).Scan(&encryptedConfig); err != nil {
		t.Fatalf("query ingress_sources: %v", err)
	}
	configJSON, err := ingress.DecryptConfig("test-app-secret-for-tests!!", encryptedConfig)
	if err != nil {
		t.Fatalf("decrypt config: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal(configJSON, &stored); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	token, _ := stored["ingest_token"].(string)
	if token == "" {
		t.Fatal("expected non-empty ingest_token in stored config")
	}
}

func TestCreateIngressSource_EmailAPI_TokenIsServerGenerated(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// User attempts to supply their own ingest_token — it must be overwritten.
	src := createIngressSource(t, env, user.Cookie, `{"type":"email_api","label":"Email Inbox","config":{"ingest_token":"user-supplied"}}`)

	srcID := src["id"].(string)
	var encryptedConfig string
	if err := env.Database.QueryRow("SELECT config FROM ingress_sources WHERE id = ?", srcID).Scan(&encryptedConfig); err != nil {
		t.Fatalf("query ingress_sources: %v", err)
	}
	configJSON, err := ingress.DecryptConfig("test-app-secret-for-tests!!", encryptedConfig)
	if err != nil {
		t.Fatalf("decrypt config: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal(configJSON, &stored); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	token, _ := stored["ingest_token"].(string)
	if token == "" {
		t.Fatal("expected non-empty ingest_token in stored config")
	}
	if token == "user-supplied" {
		t.Fatal("ingest_token must not be user-controlled")
	}
}

func TestDeleteIngressRule_WrongSource(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")

	src1 := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"A","config":{}}`)
	src2 := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"B","config":{}}`)
	id1 := src1["id"].(string)
	id2 := src2["id"].(string)

	rule := createRule(t, env, user.Cookie, id1,
		`{"position":0,"field":"filename","operator":"equals","value":"x","action":"deny"}`)
	rid := rule["id"].(string)

	req := th.AuthRequest(http.MethodDelete, "/api/v1/ingress/sources/"+id2+"/rules/"+rid, nil, user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when rule doesn't belong to source, got %d", resp.StatusCode)
	}
}

func TestReorderIngressRules_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")
	src := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	r0 := createRule(t, env, user.Cookie, id,
		`{"position":0,"field":"filename","operator":"contains","value":"a","action":"allow"}`)
	r1 := createRule(t, env, user.Cookie, id,
		`{"position":1,"field":"filename","operator":"contains","value":"b","action":"deny"}`)
	rid0 := r0["id"].(string)
	rid1 := r1["id"].(string)

	// Swap positions
	bodyBytes, _ := json.Marshal([]map[string]any{
		{"id": rid0, "position": 10},
		{"id": rid1, "position": 0},
	})
	req := th.AuthRequest(http.MethodPut, "/api/v1/ingress/sources/"+id+"/rules/reorder",
		bytes.NewReader(bodyBytes), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 rules in response, got %d", len(list))
	}
	// After reorder, rid1 should be first (position 0)
	if list[0]["id"] != rid1 {
		t.Fatalf("expected rid1 first after reorder, got %v", list[0]["id"])
	}
	if list[1]["id"] != rid0 {
		t.Fatalf("expected rid0 second after reorder, got %v", list[1]["id"])
	}
}

func TestReorderIngressRules_UnknownRule(t *testing.T) {
	env := th.SetupTestApp(t)
	user := th.Register(t, env, "Alice", "alice@example.com", "password123")
	src := createIngressSource(t, env, user.Cookie, `{"type":"imap","label":"Inbox","config":{}}`)
	id := src["id"].(string)

	bodyBytes, _ := json.Marshal([]map[string]any{
		{"id": "nonexistent", "position": 0},
	})
	req := th.AuthRequest(http.MethodPut, "/api/v1/ingress/sources/"+id+"/rules/reorder",
		bytes.NewReader(bodyBytes), user.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown rule in reorder, got %d", resp.StatusCode)
	}
}

// --- helpers

func rawMsg(s string) *json.RawMessage {
	r := json.RawMessage(s)
	return &r
}

func strPtr(s string) *string { return &s }
