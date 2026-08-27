package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/systemtags"
)

func newTagSvc(t *testing.T) (service.TagService, *memory.RealTagRepo) {
	t.Helper()
	repo := memory.NewRealTagRepo()
	return service.NewTagService(repo, audit.NopWriter{}), repo
}

func newTagSvcSpy(t *testing.T) (service.TagService, *spyWriter) {
	t.Helper()
	spy := newSpy()
	repo := memory.NewRealTagRepo()
	return service.NewTagService(repo, spy), spy
}

// --- Create ---

func TestTagService_Create_OK(t *testing.T) {
	svc, _ := newTagSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "Nature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "nature" {
		t.Errorf("Name: got %q, want %q (should be lowercased)", dto.Name, "nature")
	}
}

func TestTagService_Create_EmptyName(t *testing.T) {
	svc, _ := newTagSvc(t)
	_, err := svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "   "})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTagService_Create_Conflict(t *testing.T) {
	svc, _ := newTagSvc(t)
	if _, err := svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "dup"}); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	_, err := svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "dup"})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

// --- List ---

func TestTagService_List_WorkspaceIsolation(t *testing.T) {
	svc, _ := newTagSvc(t)
	svc.Create(context.Background(), "ws_A", service.CreateTagParams{Name: "alpha"})
	svc.Create(context.Background(), "ws_B", service.CreateTagParams{Name: "beta"})
	tags, err := svc.List(context.Background(), "ws_A", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "alpha" {
		t.Errorf("expected [alpha], got %v", tags)
	}
}

// --- Patch (rename) ---

func TestTagService_Patch_Rename_OK(t *testing.T) {
	svc, _ := newTagSvc(t)
	svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "old"})
	newName := "new"
	dto, err := svc.Patch(context.Background(), "ws_1", "old", service.PatchTagParams{Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "new" {
		t.Errorf("Name: got %q, want %q", dto.Name, "new")
	}
}

func TestTagService_Patch_Rename_Conflict(t *testing.T) {
	svc, _ := newTagSvc(t)
	svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "first"})
	svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "second"})
	conflict := "second"
	_, err := svc.Patch(context.Background(), "ws_1", "first", service.PatchTagParams{Name: &conflict})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestTagService_Patch_NotFound(t *testing.T) {
	svc, _ := newTagSvc(t)
	name := "x"
	_, err := svc.Patch(context.Background(), "ws_1", "missing", service.PatchTagParams{Name: &name})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- AddToAsset / RemoveFromAsset ---

func TestTagService_AddToAsset_Idempotent(t *testing.T) {
	svc, _ := newTagSvc(t)
	if _, err := svc.AddToAsset(context.Background(), "ws_1", "ast_1", "photo"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := svc.AddToAsset(context.Background(), "ws_1", "ast_1", "photo"); err != nil {
		t.Fatalf("second add (should be idempotent): %v", err)
	}
	tags, err := svc.ListForAsset(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}
}

func TestTagService_RemoveFromAsset_OK(t *testing.T) {
	svc, _ := newTagSvc(t)
	svc.AddToAsset(context.Background(), "ws_1", "ast_1", "photo")
	if err := svc.RemoveFromAsset(context.Background(), "ws_1", "ast_1", "photo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tags, _ := svc.ListForAsset(context.Background(), "ast_1")
	if len(tags) != 0 {
		t.Errorf("expected 0 tags after remove, got %d", len(tags))
	}
}

func TestTagService_AddToAsset_DispatchesWorkflowTrigger(t *testing.T) {
	repo := memory.NewRealTagRepo()
	assets := memory.NewAssetRepo()
	assets.Seed(repository.Asset{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		OriginalFilename: "photo.jpg",
		StorageKey:       "ws_1/ast_1/original/photo.jpg",
		MimeType:         "image/jpeg",
	})
	triggers := &triggerSpy{}
	svc := service.NewTagService(repo, audit.NopWriter{}, service.TagServiceDeps{
		Assets:   assets,
		Triggers: triggers,
	})

	tag, err := svc.AddToAsset(context.Background(), "ws_1", "ast_1", "photo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForTriggerCount(t, triggers, 1)
	call := triggers.last()
	if call.eventType != "trigger.tag_added" {
		t.Fatalf("eventType: got %q", call.eventType)
	}
	if got := call.data["tag"]; got != tag.Name {
		t.Fatalf("tag: got %v want %s", got, tag.Name)
	}
}

// --- Delete ---

func TestTagService_Delete_OK(t *testing.T) {
	svc, _ := newTagSvc(t)
	svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "gone"})
	if err := svc.Delete(context.Background(), "ws_1", []string{"gone"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tags, _ := svc.List(context.Background(), "ws_1", false)
	if len(tags) != 0 {
		t.Errorf("expected 0 tags after delete, got %d", len(tags))
	}
}

func TestTagService_EnsureSystemTag_CreatesRowOnFirstUse(t *testing.T) {
	svc, repo := newTagSvc(t)
	if err := svc.EnsureSystemTag(context.Background(), "ws_1", systemtags.Watermark); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tag, err := repo.GetByName(context.Background(), "ws_1", systemtags.Watermark)
	if err != nil {
		t.Fatalf("expected tag row: %v", err)
	}
	if tag.GroupName == nil || *tag.GroupName != systemtags.GroupName {
		t.Fatalf("expected group_name=system, got %#v", tag.GroupName)
	}
}

func TestTagService_EnsureSystemTag_IdempotentOnRepeat(t *testing.T) {
	svc, repo := newTagSvc(t)
	if err := svc.EnsureSystemTag(context.Background(), "ws_1", systemtags.Watermark); err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	if err := svc.EnsureSystemTag(context.Background(), "ws_1", systemtags.Watermark); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	tags, err := repo.List(context.Background(), "ws_1", true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag row, got %d", len(tags))
	}
}

func TestTagService_List_DefaultExcludesSystemTags(t *testing.T) {
	svc, _ := newTagSvc(t)
	if err := svc.EnsureSystemTag(context.Background(), "ws_1", systemtags.Watermark); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if _, err := svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "user-tag"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	tags, err := svc.List(context.Background(), "ws_1", false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "user-tag" {
		t.Fatalf("expected only user-tag, got %+v", tags)
	}
}

func TestTagService_Delete_SystemTag_ReturnsProtectedError(t *testing.T) {
	svc, _ := newTagSvc(t)
	if err := svc.EnsureSystemTag(context.Background(), "ws_1", systemtags.Watermark); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	err := svc.Delete(context.Background(), "ws_1", []string{systemtags.Watermark})
	if !errors.Is(err, service.ErrSystemTagProtected) {
		t.Fatalf("expected ErrSystemTagProtected, got %v", err)
	}
}

func TestTagService_BulkDelete_SystemTag_ReturnsProtectedError(t *testing.T) {
	svc, _ := newTagSvc(t)
	if err := svc.EnsureSystemTag(context.Background(), "ws_1", systemtags.Watermark); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	_, err := svc.BulkDelete(context.Background(), "ws_1", []string{systemtags.Watermark})
	if !errors.Is(err, service.ErrSystemTagProtected) {
		t.Fatalf("expected ErrSystemTagProtected, got %v", err)
	}
}

func TestTagService_Patch_RenameSystemTag_ReturnsProtectedError(t *testing.T) {
	svc, _ := newTagSvc(t)
	if err := svc.EnsureSystemTag(context.Background(), "ws_1", systemtags.Watermark); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	name := "renamed"
	_, err := svc.Patch(context.Background(), "ws_1", systemtags.Watermark, service.PatchTagParams{Name: &name})
	if !errors.Is(err, service.ErrSystemTagProtected) {
		t.Fatalf("expected ErrSystemTagProtected, got %v", err)
	}
}

func TestTagService_Merge_SystemTagAsSource_ReturnsProtectedError(t *testing.T) {
	svc, _ := newTagSvc(t)
	if err := svc.EnsureSystemTag(context.Background(), "ws_1", systemtags.Watermark); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if _, err := svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "user-tag"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err := svc.Merge(context.Background(), "ws_1", []string{systemtags.Watermark}, "user-tag")
	if !errors.Is(err, service.ErrSystemTagProtected) {
		t.Fatalf("expected ErrSystemTagProtected, got %v", err)
	}
}

// --- Audit events ---

func TestTagService_AddToAsset_EmitsAuditEvent(t *testing.T) {
	svc, spy := newTagSvcSpy(t)
	if _, err := svc.AddToAsset(context.Background(), "ws_1", "ast_1", "nature"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetTagged {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetTagged)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}

func TestTagService_RemoveFromAsset_EmitsAuditEvent(t *testing.T) {
	svc, spy := newTagSvcSpy(t)
	svc.AddToAsset(context.Background(), "ws_1", "ast_1", "nature")
	spy.asset = nil // reset after add
	if err := svc.RemoveFromAsset(context.Background(), "ws_1", "ast_1", "nature"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetUntagged {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetUntagged)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}

func TestTagService_AddToAsset_AuditOnEveryCall(t *testing.T) {
	svc, spy := newTagSvcSpy(t)
	// Two calls with the same tag: the repo deduplicates the link, but the
	// service emits an audit event on every call regardless.
	svc.AddToAsset(context.Background(), "ws_1", "ast_1", "photo")
	svc.AddToAsset(context.Background(), "ws_1", "ast_1", "photo")
	if spy.assetCount() != 2 {
		t.Errorf("expected 2 audit events (one per call), got %d", spy.assetCount())
	}
}

// --- GetByName ---

func TestTagService_GetByName_OK(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	_, _ = svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "nature"})

	dto, err := svc.GetByName(context.Background(), "ws_1", "nature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "nature" {
		t.Errorf("Name: got %q, want nature", dto.Name)
	}
}

func TestTagService_GetByName_NormalizesCase(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	_, _ = svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "nature"})

	dto, err := svc.GetByName(context.Background(), "ws_1", "  NATURE  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "nature" {
		t.Errorf("Name: got %q, want nature", dto.Name)
	}
}

func TestTagService_GetByName_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	_, err := svc.GetByName(context.Background(), "ws_1", "missing")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- TouchLastUsed ---

func TestTagService_TouchLastUsed_OK(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	err := svc.TouchLastUsed(context.Background(), "ws_1", "nature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- BatchTagsForAssets ---

func TestTagService_BatchTagsForAssets_OK(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	// Seed via AddToAsset so the links are stored correctly
	_, _ = svc.AddToAsset(context.Background(), "ws_1", "ast_1", "alpha")
	_, _ = svc.AddToAsset(context.Background(), "ws_1", "ast_1", "beta")
	_, _ = svc.AddToAsset(context.Background(), "ws_1", "ast_2", "gamma")

	result, err := svc.BatchTagsForAssets(context.Background(), []string{"ast_1", "ast_2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result["ast_1"]) != 2 {
		t.Errorf("ast_1 tags: got %d, want 2", len(result["ast_1"]))
	}
	if len(result["ast_2"]) != 1 {
		t.Errorf("ast_2 tags: got %d, want 1", len(result["ast_2"]))
	}
}

func TestTagService_BatchTagsForAssets_Empty(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	result, err := svc.BatchTagsForAssets(context.Background(), []string{"ast_none"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result["ast_none"]) != 0 {
		t.Errorf("expected empty tags for unknown asset, got %d", len(result["ast_none"]))
	}
}

// --- UpsertForAsset ---

func TestTagService_UpsertForAsset_OK(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	err := svc.UpsertForAsset(context.Background(), "ws_1", "ast_1", "photo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTagService_UpsertForAsset_Idempotent(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	_ = svc.UpsertForAsset(context.Background(), "ws_1", "ast_1", "photo")
	err := svc.UpsertForAsset(context.Background(), "ws_1", "ast_1", "photo")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
}

func TestTagService_UpsertForAsset_SystemTag(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	err := svc.UpsertForAsset(context.Background(), "ws_1", "ast_1", systemtags.Watermark)
	if err != nil {
		t.Fatalf("unexpected error for system tag: %v", err)
	}
}

func TestTagService_UpsertForAsset_EmptyName(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	err := svc.UpsertForAsset(context.Background(), "ws_1", "ast_1", "")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty name, got %v", err)
	}
}

// --- ResolveSystemTag ---

func TestTagService_ResolveSystemTag_InvalidTag(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	_, err := svc.ResolveSystemTag(context.Background(), "ws_1", "not-a-system-tag", service.SystemTagScope{})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for non-system tag, got %v", err)
	}
}

func TestTagService_ResolveSystemTag_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newTagSvc(t)
	// memory repo always returns ErrNotFound for FindAsset* methods
	_, err := svc.ResolveSystemTag(context.Background(), "ws_1", systemtags.Watermark, service.SystemTagScope{})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when no asset tagged, got %v", err)
	}
}
