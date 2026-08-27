package api

import (
	"encoding/json"
	"strings"
	"testing"

	dbgen "damask/server/internal/db/gen"
)

func TestAssetResponseStorageKeyVisibility(t *testing.T) {
	internal, err := json.Marshal(assetToResponse(dbgen.Asset{StorageKey: "workspace/asset/model.max"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(internal), `"storage_key":"workspace/asset/model.max"`) {
		t.Fatalf("authenticated response does not contain storage key: %s", internal)
	}

	public, err := json.Marshal(AssetResponse{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(public), "storage_key") {
		t.Fatalf("empty storage key leaked into public response: %s", public)
	}
}

func TestAssetResponseClassificationStatus(t *testing.T) {
	classified := assetToResponseWithCount(
		dbgen.Asset{OriginalFilename: "scene.max", MimeType: "application/octet-stream"},
		[]string{"3D模型"}, 0, 0, false,
	)
	if classified.ClassificationStatus != "classified" {
		t.Fatalf("classified status = %q", classified.ClassificationStatus)
	}

	needsReview := assetToResponseWithCount(
		dbgen.Asset{OriginalFilename: "render.psd", MimeType: "application/octet-stream"},
		nil, 0, 0, false,
	)
	if needsReview.ClassificationStatus != "needs_review" {
		t.Fatalf("missing primary tag status = %q", needsReview.ClassificationStatus)
	}

	manual := assetToResponse(dbgen.Asset{OriginalFilename: "notes.txt", MimeType: "text/plain"})
	if manual.ClassificationStatus != "manual_required" {
		t.Fatalf("unsupported format status = %q", manual.ClassificationStatus)
	}
}
