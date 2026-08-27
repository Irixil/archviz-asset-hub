//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	th "damask/server/internal/testhelpers"

	"github.com/gofiber/fiber/v3"
)

// createTestAsset uploads a small PNG and returns its ID + auth cookie.
func createTestAsset(t *testing.T, env *th.TestEnv) (assetID string, cookie *http.Cookie) {
	t.Helper()
	res := th.Register(t, env, "User", "user@test.com", "password123")
	return uploadTestVariantAsset(t, env, "test.png", res.Cookie), res.Cookie
}

func createTestVideoAsset(t *testing.T, env *th.TestEnv) (assetID string, cookie *http.Cookie) {
	t.Helper()
	res := th.Register(t, env, "Video User", "video@test.com", "password123")
	return uploadTestVideoAsset(t, env, "clip.mp4", res.Cookie), res.Cookie
}

func uploadTestVariantAsset(t *testing.T, env *th.TestEnv, filename string, cookie *http.Cookie) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.Set(5, 5, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}

	req := th.BuildUploadRequest(t, filename, buf.Bytes(), cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 10000})
	if err != nil {
		t.Fatalf("upload asset: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload asset: expected 201, got %d", resp.StatusCode)
	}

	var a api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		t.Fatalf("decode asset: %v", err)
	}
	return a.ID
}

func uploadTestVideoAsset(t *testing.T, env *th.TestEnv, filename string, cookie *http.Cookie) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "transform", "testdata", "sample_video_with_audio.mp4"))
	if err != nil {
		t.Fatalf("read sample video: %v", err)
	}

	req := th.BuildUploadRequest(t, filename, data, cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 10000})
	if err != nil {
		t.Fatalf("upload video asset: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload video asset: expected 201, got %d", resp.StatusCode)
	}

	var a api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		t.Fatalf("decode video asset: %v", err)
	}
	return a.ID
}

func assignSystemTagToAsset(t *testing.T, env *th.TestEnv, cookie *http.Cookie, assetID, tagName string) {
	t.Helper()
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/tags",
		th.JSONBody(map[string]string{"name": tagName}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("assign tag: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("assign tag: expected 201, got %d", resp.StatusCode)
	}
}

// insertVariantDirectly inserts a variant row into the DB bypassing the queue.
// It uses the current version of the given asset as the asset_version_id.
func insertVariantDirectly(t *testing.T, env *th.TestEnv, assetID, workspaceID string) dbgen.Variant {
	t.Helper()
	variantID := "test-variant-id"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Resolve the current version ID for this asset.
	var versionID string
	row := env.Database.QueryRowContext(ctx,
		`SELECT id FROM asset_versions WHERE asset_id = ? AND is_current = 1 LIMIT 1`, assetID)
	if err := row.Scan(&versionID); err != nil {
		t.Fatalf("resolve current version for asset %s: %v", assetID, err)
	}

	storageKey := fmt.Sprintf("%s/%s/variants/%s.jpg", workspaceID, assetID, variantID)
	_ = env.Storage.Put(t.Context(), storageKey, bytes.NewReader([]byte("dummy variant content")))

	_, err := env.Database.ExecContext(ctx, `
		INSERT INTO variants (id, workspace_id, asset_version_id, type, storage_key, transform_params, size)
		VALUES (?, ?, ?, 'image_resize', ?, '{"width":100}', 1024)
	`, variantID, workspaceID, versionID, storageKey)
	if err != nil {
		t.Fatalf("insert variant: %v", err)
	}

	var v dbgen.Variant
	scanRow := env.Database.QueryRowContext(ctx,
		`SELECT id, workspace_id, asset_version_id, type, storage_key, transform_params, size, created_at FROM variants WHERE id = ?`, variantID)
	if err := scanRow.Scan(&v.ID, &v.WorkspaceID, &v.AssetVersionID, &v.Type, &v.StorageKey, &v.TransformParams, &v.Size, &v.CreatedAt); err != nil {
		t.Fatalf("scan variant: %v", err)
	}
	return v
}

// insertVariantWithParams inserts a variant row of the given type+params into the
// asset's current version, bypassing the queue. Unlike insertVariantDirectly, the
// type and transform_params are caller-controlled, for param-history tests.
func insertVariantWithParams(
	t *testing.T,
	env *th.TestEnv,
	assetID, workspaceID, variantID, variantType, paramsJSON string,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var versionID string
	row := env.Database.QueryRowContext(ctx,
		`SELECT id FROM asset_versions WHERE asset_id = ? AND is_current = 1 LIMIT 1`, assetID)
	if err := row.Scan(&versionID); err != nil {
		t.Fatalf("resolve current version for asset %s: %v", assetID, err)
	}

	storageKey := fmt.Sprintf("%s/%s/variants/%s.jpg", workspaceID, assetID, variantID)
	_ = env.Storage.Put(t.Context(), storageKey, bytes.NewReader([]byte("dummy variant content")))

	_, err := env.Database.ExecContext(ctx, `
		INSERT INTO variants (id, workspace_id, asset_version_id, type, storage_key, transform_params, size)
		VALUES (?, ?, ?, ?, ?, ?, 1024)
	`, variantID, workspaceID, versionID, variantType, storageKey, paramsJSON)
	if err != nil {
		t.Fatalf("insert variant: %v", err)
	}
}

// ---- Tests ----

func TestListVariants_Empty(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/variants", nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result api.ListVariantsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Variants) != 0 {
		t.Fatalf("expected empty variants, got %d", len(result.Variants))
	}
	if result.Rebuilding {
		t.Error("expected rebuilding=false")
	}
}

func TestListVariants_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "U", "u@test.com", "pass1234")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/nonexistent/variants", nil, res.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListVariants_WithVariant(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	insertVariantDirectly(t, env, assetID, a.WorkspaceID)

	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/variants", nil, cookie)
	resp2, err := env.App.Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var result api.ListVariantsResponse
	_ = json.NewDecoder(resp2.Body).Decode(&result)
	if len(result.Variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(result.Variants))
	}
	if result.Variants[0].Type != "image_resize" {
		t.Errorf("expected type image_resize, got %s", result.Variants[0].Type)
	}
	if result.Variants[0].DownloadURL == "" {
		t.Error("expected non-empty download URL")
	}
}

func TestCreateVariant_InvalidType(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	params := json.RawMessage(`{}`)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "invalid_type", Params: params}), cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_VideoOnImage(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "video_capture_image", Params: json.RawMessage(`{}`)}), cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_WatermarkQueued(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)
	watermarkID := uploadTestVariantAsset(t, env, "brand-watermark.png", cookie)
	assignSystemTagToAsset(t, env, cookie, watermarkID, "_watermark")

	paramsData := json.RawMessage(`{"opacity":0.5,"format":"jpeg"}`)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "image_watermark", Params: paramsData}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var result api.CreateVariantResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.JobID == "" {
		t.Error("expected job_id in response")
	}
	if result.Status != "pending" {
		t.Errorf("expected status=pending, got %v", result.Status)
	}

	var payload string
	if err := env.Database.QueryRow(`SELECT payload FROM jobs WHERE id = ?`, result.JobID).Scan(&payload); err != nil {
		t.Fatalf("load job payload: %v", err)
	}
	var jobPayload map[string]any
	if err := json.Unmarshal([]byte(payload), &jobPayload); err != nil {
		t.Fatalf("decode job payload: %v", err)
	}
	params, ok := jobPayload["params"].(map[string]any)
	if !ok {
		t.Fatalf("expected params object in payload, got %#v", jobPayload["params"])
	}
	if got := params["watermark_asset_id"]; got != watermarkID {
		t.Fatalf("expected injected watermark_asset_id=%s, got %#v", watermarkID, got)
	}
}

func TestCreateVariant_VideoWatermarkQueued(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestVideoAsset(t, env)
	watermarkID := uploadTestVariantAsset(t, env, "brand-watermark.png", cookie)
	assignSystemTagToAsset(t, env, cookie, watermarkID, "_watermark")

	paramsData := json.RawMessage(`{"opacity":0.35,"format":"webm","strip_audio":true}`)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "video_watermark", Params: paramsData}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var result api.CreateVariantResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.JobID == "" {
		t.Error("expected job_id in response")
	}

	var payload string
	if err := env.Database.QueryRow(`SELECT payload FROM jobs WHERE id = ?`, result.JobID).Scan(&payload); err != nil {
		t.Fatalf("load job payload: %v", err)
	}
	var jobPayload map[string]any
	if err := json.Unmarshal([]byte(payload), &jobPayload); err != nil {
		t.Fatalf("decode job payload: %v", err)
	}
	params, ok := jobPayload["params"].(map[string]any)
	if !ok {
		t.Fatalf("expected params object in payload, got %#v", jobPayload["params"])
	}
	if got := params["watermark_asset_id"]; got != watermarkID {
		t.Fatalf("expected injected watermark_asset_id=%s, got %#v", watermarkID, got)
	}
}

func TestCreateVariant_WatermarkMissingReturns422(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{
			Type:   "image_watermark",
			Params: json.RawMessage(`{"opacity":0.5}`),
		}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestResolveWatermarkAsset(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)
	watermarkID := uploadTestVariantAsset(t, env, "brand-watermark.png", cookie)
	assignSystemTagToAsset(t, env, cookie, watermarkID, "_watermark")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/variants/watermark", nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result api.WatermarkAssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.ID != watermarkID {
		t.Fatalf("expected watermark id %s, got %s", watermarkID, result.ID)
	}
	if result.Scope != "workspace" {
		t.Fatalf("expected workspace scope, got %s", result.Scope)
	}
}

func TestCreateVariant_ResizeQueued(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	paramsData := json.RawMessage(`{"width":200,"height":200,"fit":"contain","quality":80,"format":"jpeg"}`)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "image_resize", Params: paramsData}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var result api.CreateVariantResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.JobID == "" {
		t.Error("expected job_id in response")
	}
	if result.Status != "pending" {
		t.Errorf("expected status=pending, got %v", result.Status)
	}
}

func TestCreateVariant_BgRemoveNoKey(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "image_bg_remove", Params: json.RawMessage(`{}`)}), cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 (no API key), got %d", resp.StatusCode)
	}
}

func TestCreateVariant_ImageBgRemoveQueued(t *testing.T) {
	env := th.SetupTestApp(t, th.WithImageRouterAPIKey("test-key"))
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "image_bg_remove", Params: json.RawMessage(`{}`)}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var result api.CreateVariantResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	var payload string
	if err := env.Database.QueryRow(`SELECT payload FROM jobs WHERE id = ?`, result.JobID).Scan(&payload); err != nil {
		t.Fatalf("load job payload: %v", err)
	}
	if !strings.Contains(payload, `"model":"bria/remove-background"`) {
		t.Fatalf("expected normalized default model in payload, got %s", payload)
	}
}

func TestCreateVariant_ImageWithPromptQueued(t *testing.T) {
	env := th.SetupTestApp(t, th.WithImageRouterAPIKey("test-key"))
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{
			Type:   "image_with_prompt",
			Params: json.RawMessage(`{"prompt":"  add fog  "}`),
		}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var result api.CreateVariantResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	var payload string
	if err := env.Database.QueryRow(`SELECT payload FROM jobs WHERE id = ?`, result.JobID).Scan(&payload); err != nil {
		t.Fatalf("load job payload: %v", err)
	}
	if !strings.Contains(payload, `"prompt":"add fog"`) {
		t.Fatalf("expected trimmed prompt in payload, got %s", payload)
	}
}

func TestCreateVariant_ImageWithPromptMissingPrompt(t *testing.T) {
	env := th.SetupTestApp(t, th.WithImageRouterAPIKey("test-key"))
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{
			Type:   "image_with_prompt",
			Params: json.RawMessage(`{"prompt":"  "}`),
		}), cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_ImageWithPromptRequiresAPIKey(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{
			Type:   "image_with_prompt",
			Params: json.RawMessage(`{"prompt":"add fog"}`),
		}), cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_BgRemoveOldTypeRejected(t *testing.T) {
	env := th.SetupTestApp(t, th.WithImageRouterAPIKey("test-key"))
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "bg_remove", Params: json.RawMessage(`{}`)}), cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_ImageRouterDemoBlocked(t *testing.T) {
	env := th.SetupTestApp(t, th.WithImageRouterAPIKey("test-key"))
	res := th.Register(t, env, "Demo User", "demo-block@test.com", "password123")
	assetID := uploadTestVariantAsset(t, env, "test.png", res.Cookie)
	token, err := env.Maker.CreateDemoToken(res.UserID, res.WorkspaceID, time.Hour)
	if err != nil {
		t.Fatalf("create demo token: %v", err)
	}

	req := th.BearerRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "image_bg_remove", Params: json.RawMessage(`{}`)}), token)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestDeleteVariant(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	v := insertVariantDirectly(t, env, assetID, a.WorkspaceID)

	delReq := th.AuthRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/assets/%s/variants/%s", assetID, v.ID), nil, cookie)
	delResp, err := env.App.Test(delReq)
	if err != nil {
		t.Fatal(err)
	}
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", delResp.StatusCode)
	}

	// Verify deleted
	listReq := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/variants", nil, cookie)
	listResp, _ := env.App.Test(listReq)
	var result api.ListVariantsResponse
	_ = json.NewDecoder(listResp.Body).Decode(&result)
	if len(result.Variants) != 0 {
		t.Fatalf("expected 0 variants after delete, got %d", len(result.Variants))
	}
}

func TestDeleteVariant_NotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/assets/%s/variants/nonexistent", assetID), nil, cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteVariant_PreviousVersionGuard(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	// Get the workspace ID.
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	// Insert a variant on the current version.
	v := insertVariantDirectly(t, env, assetID, a.WorkspaceID)

	// Now upload a second version, making the old version non-current.
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	var buf2 bytes.Buffer
	_ = png.Encode(&buf2, img)
	var body2 bytes.Buffer
	boundary2 := "B2"
	_, _ = fmt.Fprintf(&body2, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"v2.png\"\r\nContent-Type: image/png\r\n\r\n", boundary2)
	_, _ = body2.Write(buf2.Bytes())
	_, _ = fmt.Fprintf(&body2, "\r\n--%s--\r\n", boundary2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/versions", &body2)
	req2.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary2))
	req2.AddCookie(cookie)
	resp2, _ := env.App.Test(req2, fiber.TestConfig{Timeout: 10000})
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("upload second version: expected 201, got %d", resp2.StatusCode)
	}

	// Trying to delete the variant (which now belongs to the old version) should return 422.
	delReq := th.AuthRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/assets/%s/variants/%s", assetID, v.ID), nil, cookie)
	delResp, _ := env.App.Test(delReq)
	if delResp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 (previous version guard), got %d", delResp.StatusCode)
	}
}

func TestGetVariantFile(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	v := insertVariantDirectly(t, env, assetID, a.WorkspaceID)

	fileReq := th.AuthRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/assets/%s/variants/%s/file", assetID, v.ID), nil, cookie)
	fileResp, err := env.App.Test(fileReq)
	if err != nil {
		t.Fatal(err)
	}
	if fileResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", fileResp.StatusCode)
	}
}

func TestPreviewTransform(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/assets/%s/preview?w=50&h=50&fit=contain&format=jpeg&q=80", assetID),
		nil, cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", ct)
	}
}

func TestPreviewTransform_WebP(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/assets/%s/preview?w=50&h=50&fit=contain&format=webp&q=80", assetID),
		nil, cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/webp" {
		t.Errorf("expected image/webp, got %s", ct)
	}
}

func TestPreviewTransform_Cached(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	url := fmt.Sprintf("/api/v1/assets/%s/preview?w=50&h=50", assetID)
	for i := 0; i < 3; i++ {
		req := th.AuthRequest(http.MethodGet, url, nil, cookie)
		resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 10000})
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("iteration %d: expected 200, got %d", i, resp.StatusCode)
		}
	}
}

func TestPreviewTransform_NonImage(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "U", "u2@test.com", "pass1234")

	var body bytes.Buffer
	boundary := "TestBoundaryPDF"
	_, _ = fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"doc.pdf\"\r\nContent-Type: application/pdf\r\n\r\n", boundary)
	_, _ = body.WriteString("%PDF-1.4 fake content")
	_, _ = fmt.Fprintf(&body, "\r\n--%s--\r\n", boundary)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", &body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))
	req.AddCookie(res.Cookie)
	resp, _ := env.App.Test(req, fiber.TestConfig{Timeout: 10000})

	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	previewReq := th.AuthRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/assets/%s/preview?w=100", a.ID), nil, res.Cookie)
	previewResp, _ := env.App.Test(previewReq, fiber.TestConfig{Timeout: 5000})
	if previewResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-image preview, got %d", previewResp.StatusCode)
	}
}

func TestCreateVariant_ViewerForbidden(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, ownerCookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, ownerCookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	viewerToken := th.MintEditorToken(t, env, a.WorkspaceID, auth.Viewer)
	createReq := th.BearerRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "image_resize", Params: json.RawMessage(`{"width":100}`)}), viewerToken)
	createResp, _ := env.App.Test(createReq)
	if createResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer create variant, got %d", createResp.StatusCode)
	}
}

func TestCreateVariant_BlockedDuringRebuild(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	// Get the asset to resolve the workspace ID and current version ID.
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	var versionID string
	_ = env.Database.QueryRow(
		`SELECT id FROM asset_versions WHERE asset_id = ? AND is_current = 1 LIMIT 1`, assetID,
	).Scan(&versionID)
	if versionID == "" {
		t.Fatal("no current version found")
	}

	// Simulate an in-flight rebuild by inserting a pending rebuild_variants job.
	// Use the real workspace_id to satisfy the FK constraint.
	payload := fmt.Sprintf(`{"asset_id":%q,"new_version_id":%q,"source_version_id":"old"}`, assetID, versionID)
	_, err := env.Database.Exec(
		`INSERT INTO jobs (id, workspace_id, type, payload, status, created_at)
		 VALUES ('rebuild-job-1', ?, 'rebuild_variants', ?, 'pending', datetime('now'))`,
		a.WorkspaceID, payload,
	)
	if err != nil {
		t.Fatalf("insert fake rebuild job: %v", err)
	}

	resizeParams := json.RawMessage(`{"width":200,"height":200,"fit":"contain","quality":80,"format":"jpeg"}`)
	createReq := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		th.JSONBody(api.CreateVariantRequest{Type: "image_resize", Params: resizeParams}), cookie)
	createResp, _ := env.App.Test(createReq)
	if createResp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 while rebuild in-flight, got %d", createResp.StatusCode)
	}
}

func TestListVariants_OnlyShowsCurrentVersionVariants(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	// Insert a variant on v1 (currently current).
	insertVariantDirectly(t, env, assetID, a.WorkspaceID)

	// Upload v2 — v1 becomes non-current.
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	var body bytes.Buffer
	boundary := "B2"
	_, _ = fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"v2.png\"\r\nContent-Type: image/png\r\n\r\n", boundary)
	_, _ = body.Write(buf.Bytes())
	_, _ = fmt.Fprintf(&body, "\r\n--%s--\r\n", boundary)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/versions", &body)
	req2.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))
	req2.AddCookie(cookie)
	resp2, _ := env.App.Test(req2, fiber.TestConfig{Timeout: 10000})
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("upload second version: expected 201, got %d", resp2.StatusCode)
	}

	// List variants — should return empty (v2's variants), not v1's variant.
	listReq := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/variants", nil, cookie)
	listResp, _ := env.App.Test(listReq)
	var result api.ListVariantsResponse
	_ = json.NewDecoder(listResp.Body).Decode(&result)
	if len(result.Variants) != 0 {
		t.Fatalf("expected 0 variants for new current version, got %d", len(result.Variants))
	}
}

func TestUploadManualVariant_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	var body bytes.Buffer
	boundary := "ManualUploadBoundary"
	_, _ = fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"manual.png\"\r\nContent-Type: image/png\r\n\r\n", boundary)
	_, _ = body.WriteString("fake manual variant content")
	_, _ = fmt.Fprintf(&body, "\r\n--%s--\r\n", boundary)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants/upload", &body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))
	req.AddCookie(cookie)

	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var variant api.VariantResponse
	if err := json.NewDecoder(resp.Body).Decode(&variant); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if variant.Type != "manual" {
		t.Errorf("expected type=manual, got %q", variant.Type)
	}
	if variant.ID == "" {
		t.Error("expected non-empty variant ID")
	}
	if variant.DownloadURL == "" {
		t.Error("expected non-empty download URL")
	}
}

func TestUploadManualVariant_NoFile(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants/upload", nil, cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing file, got %d", resp.StatusCode)
	}
}

func TestUploadManualVariant_AssetNotFound(t *testing.T) {
	env := th.SetupTestApp(t)
	_, cookie := createTestAsset(t, env)

	var body bytes.Buffer
	boundary := "TestBoundaryManual"
	_, _ = fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"x.png\"\r\nContent-Type: image/png\r\n\r\n", boundary)
	_, _ = body.WriteString("data")
	_, _ = fmt.Fprintf(&body, "\r\n--%s--\r\n", boundary)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets/nonexistent/variants/upload", &body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))
	req.AddCookie(cookie)

	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUploadManualVariant_ViewerForbidden(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, ownerCookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, ownerCookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	viewerToken := th.MintEditorToken(t, env, a.WorkspaceID, auth.Viewer)

	var body bytes.Buffer
	boundary := "ViewerBoundary"
	_, _ = fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"x.png\"\r\nContent-Type: image/png\r\n\r\n", boundary)
	_, _ = body.WriteString("data")
	_, _ = fmt.Fprintf(&body, "\r\n--%s--\r\n", boundary)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants/upload", &body)
	req2.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))
	req2.Header.Set("Authorization", "Bearer "+viewerToken)

	resp2, _ := env.App.Test(req2)
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer manual upload, got %d", resp2.StatusCode)
	}
}

func TestVariant_ViewerCannotDelete(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, ownerCookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, ownerCookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	v := insertVariantDirectly(t, env, assetID, a.WorkspaceID)

	viewerToken := th.MintEditorToken(t, env, a.WorkspaceID, auth.Viewer)
	delReq := th.BearerRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/assets/%s/variants/%s", assetID, v.ID), nil, viewerToken)
	delResp, _ := env.App.Test(delReq)
	if delResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer delete, got %d", delResp.StatusCode)
	}
}

func TestGetVariantParamHistory_HappyPath(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	insertVariantWithParams(t, env, assetID, a.WorkspaceID, "ph-1", "image_with_prompt", `{"prompt":"a cat"}`)
	insertVariantWithParams(t, env, assetID, a.WorkspaceID, "ph-2", "image_with_prompt", `{"prompt":"a dog"}`)

	histReq := th.AuthRequest(http.MethodGet, "/api/v1/variant-param-history?type=image_with_prompt", nil, cookie)
	histResp, err := env.App.Test(histReq)
	if err != nil {
		t.Fatal(err)
	}
	if histResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", histResp.StatusCode)
	}
	var result api.VariantParamHistoryResponse
	if err := json.NewDecoder(histResp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
}

func TestGetVariantParamHistory_MissingType(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "U", "u-missing-type@test.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/variant-param-history", nil, res.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetVariantParamHistory_UnknownType(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "U", "u-unknown-type@test.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/variant-param-history?type=image_bg_remove", nil, res.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result api.VariantParamHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 0 {
		t.Fatalf("expected empty entries for a non-restorable type, got %d", len(result.Entries))
	}
}

func TestGetVariantParamHistory_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/variant-param-history?type=image_resize", nil, nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetVariantParamHistory_ViewerCanRead(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, ownerCookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, ownerCookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	insertVariantWithParams(t, env, assetID, a.WorkspaceID, "ph-viewer", "image_resize", `{"width":800,"height":600}`)

	viewerToken := th.MintEditorToken(t, env, a.WorkspaceID, auth.Viewer)
	histReq := th.BearerRequest(http.MethodGet, "/api/v1/variant-param-history?type=image_resize", nil, viewerToken)
	histResp, _ := env.App.Test(histReq)
	if histResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for viewer read, got %d", histResp.StatusCode)
	}
}

// TestGetVariantParamHistory_CrossWorkspaceIsolation guards against the bug flagged in
// ROADMAP_64: the workspace must come from the JWT claims, never a caller-controlled
// parameter, otherwise one workspace's prompts/commands/watermark settings would leak
// to any authenticated user of another workspace.
func TestGetVariantParamHistory_CrossWorkspaceIsolation(t *testing.T) {
	env := th.SetupTestApp(t)
	assetID, ownerCookie := createTestAsset(t, env)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, ownerCookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	insertVariantWithParams(t, env, assetID, a.WorkspaceID, "ph-secret", "custom_ffmpeg", `{"command":"ffmpeg -i {input} -secret-flag {output}"}`)

	other := th.Register(t, env, "Other", "other-ws@test.com", "password123")
	histReq := th.AuthRequest(http.MethodGet, "/api/v1/variant-param-history?type=custom_ffmpeg", nil, other.Cookie)
	histResp, _ := env.App.Test(histReq)
	if histResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", histResp.StatusCode)
	}
	var result api.VariantParamHistoryResponse
	if err := json.NewDecoder(histResp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 0 {
		t.Fatalf("expected no entries leaked from another workspace, got %d", len(result.Entries))
	}
}

func TestGetVariantParamHistory_EmptyWorkspace(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "U", "u-empty-ws@test.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/variant-param-history?type=image_resize", nil, res.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result api.VariantParamHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 0 {
		t.Fatalf("expected empty entries for a workspace with no variants, got %d", len(result.Entries))
	}
}
