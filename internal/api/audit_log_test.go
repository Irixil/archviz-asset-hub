//go:build integration

package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"damask/server/internal/api"
	th "damask/server/internal/testhelpers"
)

// --- helpers ---

func uploadTestAsset(t *testing.T, env *th.TestEnv, owner th.AuthResult) string {
	t.Helper()
	return env.UploadTestAsset(t, owner.Cookie)
}

func createProject(t *testing.T, env *th.TestEnv, cookie *http.Cookie, name, color string) api.ProjectResponse {
	t.Helper()
	return th.CreateProject(t, env, cookie, name, color)
}

func getAssetEvents(t *testing.T, env *th.TestEnv, assetID string, cookie *http.Cookie, query string) (int, api.EventListResponse) {
	t.Helper()
	path := fmt.Sprintf("/api/v1/assets/%s/events", assetID)
	if query != "" {
		path += "?" + query
	}
	req := th.AuthRequest(http.MethodGet, path, nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	var body api.EventListResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func getProjectEvents(t *testing.T, env *th.TestEnv, projectID string, cookie *http.Cookie, query string) (int, api.EventListResponse) {
	t.Helper()
	path := fmt.Sprintf("/api/v1/projects/%s/events", projectID)
	if query != "" {
		path += "?" + query
	}
	req := th.AuthRequest(http.MethodGet, path, nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	var body api.EventListResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

// --- asset events ---

func TestListAssetEvents_Empty(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	assetID := uploadTestAsset(t, env, owner)

	// Fresh upload: asset_created event should be present.
	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(body.Events) == 0 {
		t.Fatal("expected at least one event (asset_created)")
	}
	if body.Events[0].EventType != "asset_created" {
		t.Errorf("first event type = %q, want asset_created", body.Events[0].EventType)
	}
	if body.Events[0].HumanReadable == "" {
		t.Error("human_readable should not be empty")
	}
}

func TestListAssetEvents_Unauthenticated(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	code, _ := getAssetEvents(t, env, assetID, nil, "")
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestListAssetEvents_NotFound(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	code, _ := getAssetEvents(t, env, "nonexistent-id", owner.Cookie, "")
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

func TestListAssetEvents_TypeFilter(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	// Add a tag to generate asset_tagged event.
	req := th.AuthRequest(http.MethodPost, fmt.Sprintf("/api/v1/assets/%s/tags", assetID),
		th.JSONBody(api.RenameAssetRequest{Name: "test"}), owner.Cookie)
	if resp, _ := env.App.Test(req); resp.StatusCode != http.StatusCreated {
		t.Fatal("tag: expected 201")
	}

	// Filter to only asset_tagged.
	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_tagged")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	for _, e := range body.Events {
		if e.EventType != "asset_tagged" {
			t.Errorf("unexpected event type %q in filtered result", e.EventType)
		}
	}
}

func TestListAssetEvents_Pagination(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	// Generate more events via tagging.
	for _, tag := range []string{"a", "b", "c"} {
		req := th.AuthRequest(http.MethodPost, fmt.Sprintf("/api/v1/assets/%s/tags", assetID),
			th.JSONBody(api.AddTagRequest{Name: tag}), owner.Cookie)
		resp, _ := env.App.Test(req)
		if resp.StatusCode != http.StatusCreated {
			t.Fatal("tag: expected 201")
		}
	}

	// Fetch with limit=2.
	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "limit=2")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(body.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(body.Events))
	}
	if !body.HasMore {
		t.Error("expected has_more=true")
	}
	if body.NextCursor == nil {
		t.Error("expected next_cursor to be set")
	}

	// Fetch next page using cursor — just verify it returns 200 without panic.
	code2, _ := getAssetEvents(t, env, assetID, owner.Cookie, fmt.Sprintf("limit=2&cursor=%s", url.QueryEscape(*body.NextCursor)))
	if code2 != http.StatusOK {
		t.Fatalf("expected 200 on page 2, got %d", code2)
	}
}

func TestListAssetEvents_WorkspaceIsolation(t *testing.T) {
	env := th.SetupTestApp(t)
	owner1 := th.Register(t, env, "Owner1", "owner1@example.com", "password123")
	owner2 := th.Register(t, env, "Owner2", "owner2@example.com", "password123")

	assetID := uploadTestAsset(t, env, owner1)

	// Owner2 cannot see events for owner1's asset.
	code, _ := getAssetEvents(t, env, assetID, owner2.Cookie, "")
	if code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", code)
	}
}

// --- project events ---

func TestListProjectEvents_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	project := createProject(t, env, owner.Cookie, "My Project", "#123456")

	code, body := getProjectEvents(t, env, project.ID, owner.Cookie, "")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(body.Events) == 0 {
		t.Fatal("expected project_created event")
	}
	if body.Events[0].EventType != "project_created" {
		t.Errorf("first event = %q, want project_created", body.Events[0].EventType)
	}
}

func TestListProjectEvents_NotFound(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	code, _ := getProjectEvents(t, env, "nonexistent", owner.Cookie, "")
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

func TestListProjectEvents_Unauthenticated(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	project := createProject(t, env, owner.Cookie, "My Project", "#123456")

	code, _ := getProjectEvents(t, env, project.ID, nil, "")
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

// --- workspace activity ---

func TestListWorkspaceActivity_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	uploadTestAsset(t, env, owner)
	createProject(t, env, owner.Cookie, "My Project", "#aabbcc")

	req := th.AuthRequest(http.MethodGet, "/api/v1/activity", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Events  []map[string]any `json:"events"`
		HasMore bool             `json:"has_more"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Events) == 0 {
		t.Error("expected at least one activity event")
	}
	for _, e := range body.Events {
		if e["entity_type"] == nil {
			t.Error("expected entity_type in activity event")
		}
	}
}

func TestListWorkspaceActivity_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/activity", nil, nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListWorkspaceActivity_UserFilter(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	uploadTestAsset(t, env, owner)

	req := th.AuthRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/activity?user_id=%s", owner.UserID),
		nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- CSV export ---

func TestExportActivity_CSV(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	uploadTestAsset(t, env, owner)

	req := th.AuthRequest(http.MethodGet, "/api/v1/activity/export?format=csv", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 2 {
		t.Errorf("expected at least header + 1 row, got %d lines", len(lines))
	}
	if !strings.HasPrefix(lines[0], "event_id,") {
		t.Errorf("expected CSV header, got %q", lines[0])
	}
}

func TestExportActivity_InvalidFormat(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/activity/export?format=pdf", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestExportActivity_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/activity/export?format=csv", nil, nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestExportActivity_InvalidDate(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/activity/export?format=csv&since=not-a-date", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestExportActivity_UntilFilter(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	uploadTestAsset(t, env, owner)

	// Use UTC dates to match what SQLite stores via datetime('now').
	todayUTC := time.Now().UTC().Format("2006-01-02")
	tomorrowUTC := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	twoDaysAgoUTC := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")

	// until=tomorrow: should include today's events
	req := th.AuthRequest(http.MethodGet, "/api/v1/activity/export?format=csv&until="+tomorrowUTC, nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 2 {
		t.Errorf("until=%s: expected events in export, got only header", tomorrowUTC)
	}

	// until=today: events created today (before end-of-day UTC) should be included
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/activity/export?format=csv&until="+todayUTC, nil, owner.Cookie)
	resp2, err := env.App.Test(req2)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	body2, _ := io.ReadAll(resp2.Body)
	lines2 := strings.Split(strings.TrimSpace(string(body2)), "\n")
	if len(lines2) < 2 {
		t.Errorf("until=%s: expected today's events in export", todayUTC)
	}

	// until=two days ago: should exclude today's events (only header row)
	req3 := th.AuthRequest(http.MethodGet, "/api/v1/activity/export?format=csv&until="+twoDaysAgoUTC, nil, owner.Cookie)
	resp3, err := env.App.Test(req3)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp3.StatusCode)
	}
	body3, _ := io.ReadAll(resp3.Body)
	lines3 := strings.Split(strings.TrimSpace(string(body3)), "\n")
	if len(lines3) > 1 {
		t.Errorf("until=%s: expected no data rows, got %d lines\ncontent:\n%s", twoDaysAgoUTC, len(lines3), string(body3))
	}
}

// --- asset file download audit ---

func downloadAssetFile(t *testing.T, env *th.TestEnv, assetID string, cookie *http.Cookie, secFetchDest string) int {
	t.Helper()
	req := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/file", assetID), nil, cookie)
	if secFetchDest != "" {
		req.Header.Set("Sec-Fetch-Dest", secFetchDest)
	}
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp.StatusCode
}

func TestGetAssetFile_AuditEvent_Written(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	code := downloadAssetFile(t, env, assetID, owner.Cookie, "")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	// WriteAssetAsync writes in a goroutine; poll briefly for the event to land.
	var body api.EventListResponse
	for range 20 {
		time.Sleep(10 * time.Millisecond)
		var c int
		c, body = getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_downloaded")
		if c != http.StatusOK {
			t.Fatalf("events: expected 200, got %d", c)
		}
		if len(body.Events) > 0 {
			break
		}
	}
	if len(body.Events) == 0 {
		t.Fatal("expected asset_downloaded event to be recorded")
	}
}

func TestGetAssetFile_AuditEvent_SkippedForImageFetch(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	code := downloadAssetFile(t, env, assetID, owner.Cookie, "image")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_downloaded")
	if code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", code)
	}
	if len(body.Events) != 0 {
		t.Fatalf("expected no asset_downloaded event for Sec-Fetch-Dest: image, got %d", len(body.Events))
	}
}

// --- variant file download audit ---

func downloadVariantFile(t *testing.T, env *th.TestEnv, assetID, variantID string, cookie *http.Cookie, secFetchDest string) int {
	t.Helper()
	req := th.AuthRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/variants/%s/file", assetID, variantID), nil, cookie)
	if secFetchDest != "" {
		req.Header.Set("Sec-Fetch-Dest", secFetchDest)
	}
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp.StatusCode
}

func TestGetVariantFile_AuditEvent_Written(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	variant := insertVariantDirectly(t, env, assetID, owner.WorkspaceID)

	code := downloadVariantFile(t, env, assetID, variant.ID, owner.Cookie, "")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	// WriteAssetAsync writes in a goroutine; poll briefly for the event to land.
	var body api.EventListResponse
	for range 20 {
		time.Sleep(10 * time.Millisecond)
		var c int
		c, body = getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_variant_downloaded")
		if c != http.StatusOK {
			t.Fatalf("events: expected 200, got %d", c)
		}
		if len(body.Events) > 0 {
			break
		}
	}
	if len(body.Events) == 0 {
		t.Fatal("expected asset_variant_downloaded event to be recorded")
	}
}

func TestGetVariantFile_AuditEvent_SkippedForImageFetch(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	variant := insertVariantDirectly(t, env, assetID, owner.WorkspaceID)

	code := downloadVariantFile(t, env, assetID, variant.ID, owner.Cookie, "image")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_variant_downloaded")
	if code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", code)
	}
	if len(body.Events) != 0 {
		t.Fatalf("expected no asset_variant_downloaded event for Sec-Fetch-Dest: image, got %d", len(body.Events))
	}
}

func TestGetAssetFile_AuditEvent_SkippedForVideoFetch(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	code := downloadAssetFile(t, env, assetID, owner.Cookie, "video")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_downloaded")
	if code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", code)
	}
	if len(body.Events) != 0 {
		t.Fatalf("expected no asset_downloaded event for Sec-Fetch-Dest: video, got %d", len(body.Events))
	}
}

func TestGetVariantFile_AuditEvent_SkippedForVideoFetch(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	variant := insertVariantDirectly(t, env, assetID, owner.WorkspaceID)

	code := downloadVariantFile(t, env, assetID, variant.ID, owner.Cookie, "video")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_variant_downloaded")
	if code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", code)
	}
	if len(body.Events) != 0 {
		t.Fatalf("expected no asset_variant_downloaded event for Sec-Fetch-Dest: video, got %d", len(body.Events))
	}
}

func TestGetAssetFile_AuditEvent_SkippedForDocumentFetch(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	code := downloadAssetFile(t, env, assetID, owner.Cookie, "document")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_downloaded")
	if code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", code)
	}
	if len(body.Events) != 0 {
		t.Fatalf("expected no asset_downloaded event for Sec-Fetch-Dest: document, got %d", len(body.Events))
	}
}

func TestGetAssetFile_AuditEvent_SkippedForIframeFetch(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	code := downloadAssetFile(t, env, assetID, owner.Cookie, "iframe")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_downloaded")
	if code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", code)
	}
	if len(body.Events) != 0 {
		t.Fatalf("expected no asset_downloaded event for Sec-Fetch-Dest: iframe, got %d", len(body.Events))
	}
}

func TestGetVariantFile_AuditEvent_SkippedForDocumentFetch(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	variant := insertVariantDirectly(t, env, assetID, owner.WorkspaceID)

	code := downloadVariantFile(t, env, assetID, variant.ID, owner.Cookie, "document")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_variant_downloaded")
	if code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", code)
	}
	if len(body.Events) != 0 {
		t.Fatalf("expected no asset_variant_downloaded event for Sec-Fetch-Dest: document, got %d", len(body.Events))
	}
}

func TestGetVariantFile_AuditEvent_SkippedForIframeFetch(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	variant := insertVariantDirectly(t, env, assetID, owner.WorkspaceID)

	code := downloadVariantFile(t, env, assetID, variant.ID, owner.Cookie, "iframe")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_variant_downloaded")
	if code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", code)
	}
	if len(body.Events) != 0 {
		t.Fatalf("expected no asset_variant_downloaded event for Sec-Fetch-Dest: iframe, got %d", len(body.Events))
	}
}

func TestCreateVariant_AuditEvent_Written(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	req := th.AuthRequest(http.MethodPost, fmt.Sprintf("/api/v1/assets/%s/variants", assetID),
		th.JSONBody(api.CreateVariantRequest{Type: "image_resize", Params: json.RawMessage(`{"width":100,"height":100}`)}), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_variant_created")
	if code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", code)
	}
	if len(body.Events) == 0 {
		t.Fatal("expected asset_variant_created event to be recorded")
	}
}

func TestDeleteVariant_AuditEvent_Written(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)
	variant := insertVariantDirectly(t, env, assetID, owner.WorkspaceID)

	req := th.AuthRequest(http.MethodDelete, fmt.Sprintf("/api/v1/assets/%s/variants/%s", assetID, variant.ID), nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	code, body := getAssetEvents(t, env, assetID, owner.Cookie, "types=asset_variant_deleted")
	if code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", code)
	}
	if len(body.Events) == 0 {
		t.Fatal("expected asset_variant_deleted event to be recorded")
	}
}
