//go:build integration

package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	th "damask/server/internal/testhelpers"

	"github.com/gofiber/fiber/v3"
)

// ── GET /workspace/storage ────────────────────────────────────────────────────

func TestGetWorkspaceStorage_NoLimit(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/workspace/storage", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, ok := body["total_bytes"]; !ok {
		t.Error("response missing total_bytes")
	}
	if v, ok := body["limit_bytes"]; ok && v != nil {
		t.Errorf("expected limit_bytes=null, got %v", v)
	}
	if _, ok := body["by_project"]; !ok {
		t.Error("response missing by_project")
	}
	if _, ok := body["by_type"]; !ok {
		t.Error("response missing by_type")
	}
}

func TestGetWorkspaceStorage_WithLimit(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Set limit directly in DB
	limit := int64(10_737_418_240) // 10 GB
	wsID := owner.WorkspaceID
	if _, err := env.Database.Exec(
		`UPDATE workspaces SET storage_limit_bytes = ? WHERE id = ?`, limit, wsID); err != nil {
		t.Fatalf("set limit: %v", err)
	}

	req := th.AuthRequest(http.MethodGet, "/api/v1/workspace/storage", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	got, ok := body["limit_bytes"]
	if !ok || got == nil {
		t.Fatalf("expected limit_bytes to be set, got %v", got)
	}
	// JSON numbers are float64
	if int64(got.(float64)) != limit {
		t.Errorf("want limit_bytes=%d, got %v", limit, got)
	}
}

// ── GET /workspace/storage/projects/:id/folders ───────────────────────────────

func TestGetProjectFolderStorage_OK(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Create a project first
	projReq := th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JSONBody(map[string]string{"name": "Test Project"}), owner.Cookie)
	projResp, err := env.App.Test(projReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	defer projResp.Body.Close()
	if projResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(projResp.Body)
		t.Fatalf("expected 201, got %d: %s", projResp.StatusCode, b)
	}
	var proj map[string]interface{}
	_ = json.NewDecoder(projResp.Body).Decode(&proj)
	projectID := proj["id"].(string)

	req := th.AuthRequest(http.MethodGet,
		"/api/v1/workspace/storage/projects/"+projectID+"/folders", nil, owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["project_id"] != projectID {
		t.Errorf("want project_id=%s, got %v", projectID, body["project_id"])
	}
	if _, ok := body["folders"]; !ok {
		t.Error("response missing folders")
	}
}

// ── 507 enforcement ───────────────────────────────────────────────────────────

func TestUploadAsset_StorageLimitReached(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Set limit to 0 bytes — any upload exceeds it
	if _, err := env.Database.Exec(
		`UPDATE workspaces SET storage_limit_bytes = 0 WHERE id = ?`, owner.WorkspaceID); err != nil {
		t.Fatalf("set limit: %v", err)
	}

	jpegData := th.MakeJPEG(100, 100)
	req := th.BuildUploadRequest(t, "photo.jpg", jpegData, owner.Cookie)

	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInsufficientStorage {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 507, got %d: %s", resp.StatusCode, b)
	}

	var body map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "storage_limit_reached" {
		t.Errorf("want error=storage_limit_reached, got %v", body["error"])
	}
}

func TestUploadAsset_StorageUnlimited(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	// No limit set — upload should succeed normally
	jpegData := th.MakeJPEG(100, 100)
	req := th.BuildUploadRequest(t, "photo.jpg", jpegData, owner.Cookie)

	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}
}
