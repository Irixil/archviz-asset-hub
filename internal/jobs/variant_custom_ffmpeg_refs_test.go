package jobs

import (
	"context"
	"os"
	"strings"
	"testing"

	dbgen "damask/server/internal/db/gen"
)

func TestResolveCommandRefs_EmptyCommand_ReturnsEmptyMap(t *testing.T) {
	_, _, js, _, _ := newMediaTagsJobTestEnv(t)

	refs, err := js.resolveCommandRefs(
		context.Background(),
		"ffmpeg -i {input} -vf scale {output}",
		"ws_test",
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("resolveCommandRefs: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected empty refs map, got %v", refs)
	}
}

func TestResolveCommandRefs_AssetRef_DownloadsAndReturnsTempPath(t *testing.T) {
	queries, _, js, _, stor := newMediaTagsJobTestEnv(t)

	const storageKey = "ws_test/video/ref-source.mp4"
	if err := stor.Put(t.Context(), storageKey, strings.NewReader("ref bytes")); err != nil {
		t.Fatalf("put asset storage: %v", err)
	}
	if _, err := queries.CreateAsset(context.Background(), dbgen.CreateAssetParams{
		ID:               "asset-ref-1",
		WorkspaceID:      "ws_test",
		OriginalFilename: "ref.mp4",
		StorageKey:       storageKey,
		MimeType:         "video/mp4",
		Size:             9,
	}); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	outDir := t.TempDir()
	cmd := "ffmpeg -i {input} -i {asset:asset-ref-1} {output}"
	refs, err := js.resolveCommandRefs(context.Background(), cmd, "ws_test", outDir)
	if err != nil {
		t.Fatalf("resolveCommandRefs: %v", err)
	}
	path, ok := refs["{asset:asset-ref-1}"]
	if !ok {
		t.Fatalf("expected ref entry for {asset:asset-ref-1}, got %v", refs)
	}
	if !strings.HasSuffix(path, ".tmp") {
		t.Fatalf("expected temp path ending in .tmp, got %q", path)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read downloaded ref file: %v", readErr)
	}
	if string(data) != "ref bytes" {
		t.Fatalf("downloaded ref content = %q, want %q", data, "ref bytes")
	}
}

func TestResolveCommandRefs_AssetRefEmbeddedInFilterArg_DownloadsAndReturnsTempPath(t *testing.T) {
	queries, _, js, _, stor := newMediaTagsJobTestEnv(t)

	const storageKey = "ws_test/video/ref-source.mp4"
	if err := stor.Put(t.Context(), storageKey, strings.NewReader("ref bytes")); err != nil {
		t.Fatalf("put asset storage: %v", err)
	}
	if _, err := queries.CreateAsset(context.Background(), dbgen.CreateAssetParams{
		ID:               "asset-ref-1",
		WorkspaceID:      "ws_test",
		OriginalFilename: "ref.mp4",
		StorageKey:       storageKey,
		MimeType:         "video/mp4",
		Size:             9,
	}); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	outDir := t.TempDir()
	cmd := "ffmpeg -i {input} -vf scale=1280:-1,subtitles={asset:asset-ref-1} {output}"
	refs, err := js.resolveCommandRefs(context.Background(), cmd, "ws_test", outDir)
	if err != nil {
		t.Fatalf("resolveCommandRefs: %v", err)
	}
	path, ok := refs["{asset:asset-ref-1}"]
	if !ok {
		t.Fatalf("expected ref entry for {asset:asset-ref-1}, got %v", refs)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read downloaded ref file: %v", readErr)
	}
	if string(data) != "ref bytes" {
		t.Fatalf("downloaded ref content = %q, want %q", data, "ref bytes")
	}
}

func TestResolveCommandRefs_VariantRef_DownloadsAndReturnsTempPath(t *testing.T) {
	queries, _, js, _, stor := newMediaTagsJobTestEnv(t)
	seedAssetWithVersion(t, queries, "asset-base-1")

	const storageKey = "ws_test/video/variant-ref.mp4"
	if err := stor.Put(t.Context(), storageKey, strings.NewReader("variant bytes")); err != nil {
		t.Fatalf("put variant storage: %v", err)
	}
	if _, err := queries.CreateVariant(context.Background(), dbgen.CreateVariantParams{
		ID:             "variant-ref-1",
		WorkspaceID:    "ws_test",
		AssetVersionID: "asset-base-1-v1",
		Type:           "custom_ffmpeg",
		StorageKey:     storageKey,
		Status:         variantStatusReady,
		ContentHash:    "deadbeef",
	}); err != nil {
		t.Fatalf("create variant: %v", err)
	}

	outDir := t.TempDir()
	cmd := "ffmpeg -i {input} -i {variant:variant-ref-1} {output}"
	refs, err := js.resolveCommandRefs(context.Background(), cmd, "ws_test", outDir)
	if err != nil {
		t.Fatalf("resolveCommandRefs: %v", err)
	}
	path, ok := refs["{variant:variant-ref-1}"]
	if !ok {
		t.Fatalf("expected ref entry for {variant:variant-ref-1}, got %v", refs)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read downloaded ref file: %v", readErr)
	}
	if string(data) != "variant bytes" {
		t.Fatalf("downloaded ref content = %q, want %q", data, "variant bytes")
	}
}

func TestResolveCommandRefs_AssetNotFound_ReturnsHumanReadableError(t *testing.T) {
	_, _, js, _, _ := newMediaTagsJobTestEnv(t)

	cmd := "ffmpeg -i {input} -i {asset:does-not-exist} {output}"
	_, err := js.resolveCommandRefs(context.Background(), cmd, "ws_test", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not found in this workspace") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestResolveCommandRefs_VariantWrongWorkspace_ReturnsError(t *testing.T) {
	queries, sqlDB, js, _, stor := newMediaTagsJobTestEnv(t)
	if _, err := sqlDB.Exec(`INSERT INTO workspaces (id, name) VALUES ('ws_other', 'Other')`); err != nil {
		t.Fatalf("seed other workspace: %v", err)
	}
	seedAssetWithVersionInWorkspace(t, queries, "asset-base-2", "ws_other")

	const storageKey = "ws_other/video/variant-ref.mp4"
	if err := stor.Put(t.Context(), storageKey, strings.NewReader("variant bytes")); err != nil {
		t.Fatalf("put variant storage: %v", err)
	}
	if _, err := queries.CreateVariant(context.Background(), dbgen.CreateVariantParams{
		ID:             "variant-ref-2",
		WorkspaceID:    "ws_other",
		AssetVersionID: "asset-base-2-v1",
		Type:           "custom_ffmpeg",
		StorageKey:     storageKey,
		Status:         variantStatusReady,
		ContentHash:    "deadbeef",
	}); err != nil {
		t.Fatalf("create variant: %v", err)
	}

	cmd := "ffmpeg -i {input} -i {variant:variant-ref-2} {output}"
	_, err := js.resolveCommandRefs(context.Background(), cmd, "ws_test", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not found in this workspace") {
		t.Fatalf("expected cross-workspace not-found error, got %v", err)
	}
}

func TestResolveCommandRefs_VariantNotReady_ReturnsError(t *testing.T) {
	queries, _, js, _, stor := newMediaTagsJobTestEnv(t)
	seedAssetWithVersion(t, queries, "asset-base-3")

	const storageKey = "ws_test/video/pending-ref.mp4"
	if err := stor.Put(t.Context(), storageKey, strings.NewReader("pending bytes")); err != nil {
		t.Fatalf("put variant storage: %v", err)
	}
	if _, err := queries.CreateVariant(context.Background(), dbgen.CreateVariantParams{
		ID:             "variant-ref-3",
		WorkspaceID:    "ws_test",
		AssetVersionID: "asset-base-3-v1",
		Type:           "custom_ffmpeg",
		StorageKey:     storageKey,
		Status:         "processing",
		ContentHash:    "deadbeef",
	}); err != nil {
		t.Fatalf("create variant: %v", err)
	}

	cmd := "ffmpeg -i {input} -i {variant:variant-ref-3} {output}"
	_, err := js.resolveCommandRefs(context.Background(), cmd, "ws_test", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "is not ready (status: processing)") {
		t.Fatalf("expected not-ready error, got %v", err)
	}
}

// seedAssetWithVersion creates an asset + its current asset_version in ws_test,
// using assetID+"-v1" as the version id — needed because variants FK to asset_versions.
func seedAssetWithVersion(t *testing.T, queries *dbgen.Queries, assetID string) {
	t.Helper()
	seedAssetWithVersionInWorkspace(t, queries, assetID, "ws_test")
}

func seedAssetWithVersionInWorkspace(t *testing.T, queries *dbgen.Queries, assetID, workspaceID string) {
	t.Helper()
	if _, err := queries.CreateAsset(context.Background(), dbgen.CreateAssetParams{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		OriginalFilename: "base.mp4",
		StorageKey:       workspaceID + "/video/" + assetID + ".mp4",
		MimeType:         "video/mp4",
		Size:             5,
	}); err != nil {
		t.Fatalf("create base asset: %v", err)
	}
	if _, err := queries.CreateAssetVersion(context.Background(), dbgen.CreateAssetVersionParams{
		ID:          assetID + "-v1",
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		VersionNum:  1,
		StorageKey:  workspaceID + "/video/" + assetID + ".mp4",
		ContentHash: "deadbeef",
		MimeType:    "video/mp4",
		Size:        5,
		IsCurrent:   1,
	}); err != nil {
		t.Fatalf("create base asset version: %v", err)
	}
}
