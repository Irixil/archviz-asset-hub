package jobs

import (
	"context"
	"testing"
)

// TestStoreAutoTagSuggestions_AllInsertsFail_ReturnsError exercises the case
// where every CreateAutoTagSuggestion insert fails (e.g. a stale/nonexistent
// asset_id violating the foreign key) — the job must surface an error rather
// than silently reporting success with zero suggestions persisted.
func TestStoreAutoTagSuggestions_AllInsertsFail_ReturnsError(t *testing.T) {
	_, _, js, _, _ := newMediaTagsJobTestEnv(t)

	payload := AutoTagPayload{WorkspaceID: "ws_test", AssetID: "nonexistent-asset"}
	err := js.storeAutoTagSuggestions(context.Background(), payload, []string{"hero", "blue"})
	if err == nil {
		t.Fatal("expected an error when every suggestion insert fails")
	}
}

// TestRunAutoTag_IneligibleMime_ReturnsNoTagsNoError exercises the "nothing
// to do" skip path that a workflow continuation must still resume from: an
// ineligible MIME type is not a job failure, so runAutoTag must return a nil
// error alongside empty tags (jobAutoTag then resumes any continuation with
// zero tags instead of leaving the run suspended forever).
func TestRunAutoTag_IneligibleMime_ReturnsNoTagsNoError(t *testing.T) {
	_, sqlDB, js, _, _ := newMediaTagsJobTestEnv(t)
	if _, err := sqlDB.Exec(
		`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size)
		 VALUES ('ast_zip', 'ws_test', 'a.zip', 'k/ast_zip', 'application/zip', 10)`,
	); err != nil {
		t.Fatalf("seed asset: %v", err)
	}

	tags, err := js.runAutoTag(context.Background(), AutoTagPayload{
		WorkspaceID: "ws_test", AssetID: "ast_zip", MimeType: "application/zip",
	})
	if err != nil {
		t.Fatalf("expected nil error for ineligible mime, got %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected no tags for ineligible mime, got %v", tags)
	}
}
