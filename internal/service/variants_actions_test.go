package service

import (
	"bytes"
	"context"
	"io"
	"testing"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/storage"
)

type variantAuditSpy struct {
	asset []audit.AssetEvent
}

func (s *variantAuditSpy) WriteAsset(_ context.Context, e audit.AssetEvent) {
	s.asset = append(s.asset, e)
}

func (s *variantAuditSpy) WriteAssetAsync(e audit.AssetEvent) {
	s.asset = append(s.asset, e)
}

func (s *variantAuditSpy) WriteProject(_ context.Context, _ audit.ProjectEvent) {}

type promoteActionsStub struct {
	result promoteVariantDBResult
}

func (s promoteActionsStub) Promote(_ context.Context, _ promoteVariantDBParams) (promoteVariantDBResult, error) {
	return s.result, nil
}

func (promoteActionsStub) SetAsThumbnail(context.Context, setVariantThumbnailDBParams) error {
	return nil
}

func (promoteActionsStub) MarkVariantPending(context.Context, string, string, *string) error {
	return nil
}

func (promoteActionsStub) GetVersion(context.Context, string) (variantSourceVersion, error) {
	return variantSourceVersion{}, nil
}

func (promoteActionsStub) SetVariantStatus(context.Context, string, string, string) error {
	return nil
}

type promoteStorageStub struct {
	content []byte
}

func (promoteStorageStub) Put(context.Context, string, io.Reader) error { return nil }

func (s promoteStorageStub) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.content)), nil
}

func (promoteStorageStub) Delete(context.Context, string) error { return nil }

func (promoteStorageStub) List(context.Context, string) ([]string, error) { return nil, nil }

func (s promoteStorageStub) Stat(context.Context, string) (storage.Info, error) {
	return storage.Info{Size: int64(len(s.content))}, nil
}

type promoteQueueStub struct{}

func (promoteQueueStub) Register(string, queue.HandlerFunc) {}

func (promoteQueueStub) Enqueue(context.Context, string, string, string) (dbgen.Job, error) {
	return dbgen.Job{}, nil
}

func (promoteQueueStub) Start(context.Context) {}

func (promoteQueueStub) Stop() {}

func TestVariantService_Promote_EmitsAuditOnSourceAndDerivedAsset(t *testing.T) {
	varRepo := memory.NewRealVariantRepo()
	assetRepo := memory.NewAssetRepo()
	auditSpy := &variantAuditSpy{}

	currentVersionID := "ver_src"
	width := int64(1200)
	height := int64(800)
	thumbKey := "variants/thumb.jpg"
	transformParams := `{"width":600,"height":400}`
	projectID := "prj_1"
	folderID := "fld_1"

	assetRepo.Seed(repository.Asset{
		ID:               "ast_src",
		WorkspaceID:      "ws_1",
		ProjectID:        &projectID,
		FolderID:         &folderID,
		OriginalFilename: "source.jpg",
		CurrentVersionID: &currentVersionID,
		Width:            &width,
		Height:           &height,
	})
	varRepo.Seed(repository.Variant{
		ID:                   "var_1",
		WorkspaceID:          "ws_1",
		AssetVersionID:       currentVersionID,
		Type:                 queue.JobTypeImageResize,
		StorageKey:           "variants/derived.jpg",
		Size:                 ptrInt64(42),
		ThumbnailKey:         &thumbKey,
		ThumbnailContentType: "image/jpeg",
		TransformParams:      &transformParams,
	})

	svc := NewVariantServiceWithDeps(varRepo, assetRepo, nil, auditSpy, VariantServiceDeps{
		Actions: promoteActionsStub{
			result: promoteVariantDBResult{
				NewAssetID:   "ast_new",
				NewVersionID: "ver_new",
			},
		},
		Queue:   promoteQueueStub{},
		Storage: promoteStorageStub{content: []byte("variant-bytes")},
	})

	userID := "usr_1"
	ctx := auth.WithActor(context.Background(), auth.Actor{Type: audit.ActorTypeUser, UserID: &userID})
	result, err := svc.Promote(ctx, PromoteVariantParams{
		WorkspaceID: "ws_1",
		AssetID:     "ast_src",
		VariantID:   "var_1",
		Name:        "Derived Banner",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NewAssetID != "ast_new" {
		t.Fatalf("NewAssetID: got %q, want %q", result.NewAssetID, "ast_new")
	}

	if len(auditSpy.asset) != 2 {
		t.Fatalf("expected 2 audit events, got %d", len(auditSpy.asset))
	}

	derivedEvent := auditSpy.asset[0]
	if derivedEvent.AssetID != "ast_new" {
		t.Fatalf("derived event asset_id: got %q, want %q", derivedEvent.AssetID, "ast_new")
	}
	if derivedEvent.EventType != audit.EventAssetCreated {
		t.Fatalf("derived event type: got %q, want %q", derivedEvent.EventType, audit.EventAssetCreated)
	}
	payload, ok := derivedEvent.Payload.(audit.AssetCreatedPayload)
	if !ok {
		t.Fatalf("derived payload type: got %T", derivedEvent.Payload)
	}
	if payload.Source != "derived" || payload.SourceID != "ast_src" {
		t.Fatalf("derived payload: got %+v", payload)
	}
	if payload.Filename != "Derived Banner.jpg" {
		t.Fatalf("derived filename: got %q, want %q", payload.Filename, "Derived Banner.jpg")
	}

	sourceEvent := auditSpy.asset[1]
	if sourceEvent.AssetID != "ast_src" {
		t.Fatalf("source event asset_id: got %q, want %q", sourceEvent.AssetID, "ast_src")
	}
	if sourceEvent.EventType != audit.EventAssetVariantPromoted {
		t.Fatalf("source event type: got %q, want %q", sourceEvent.EventType, audit.EventAssetVariantPromoted)
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
