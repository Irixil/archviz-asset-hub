package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/storage"
)

func newStackSvc(t *testing.T) (service.StackService, *memory.AssetRepo) {
	t.Helper()
	assetRepo := memory.NewAssetRepo()
	versionRepo := memory.NewRealVersionRepo()
	variantRepo := memory.NewRealVariantRepo()
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	// nil queue: EnqueueMerge tests that need the queue use their own setup
	svc := service.NewStackService(assetRepo, versionRepo, variantRepo, stor, nil)
	return svc, assetRepo
}

// -- ExportZip --

func TestStackService_ExportZip_EmptyIDs(t *testing.T) {
	svc, _ := newStackSvc(t)
	var buf bytes.Buffer
	err := svc.ExportZip(context.Background(), "ws_1", service.ExportZipParams{}, &buf)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStackService_ExportZip_AssetNotInWorkspace(t *testing.T) {
	svc, assetRepo := newStackSvc(t)
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_A"})

	var buf bytes.Buffer
	err := svc.ExportZip(context.Background(), "ws_B", service.ExportZipParams{
		AssetIDs: []string{"ast_1"},
	}, &buf)
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestStackService_ExportZip_OK(t *testing.T) {
	assetRepo := memory.NewAssetRepo()
	stor, _ := storage.NewAferoMemoryStorage()

	assetRepo.Seed(repository.Asset{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		OriginalFilename: "photo.jpg",
	})

	// Put a file in storage at a key we'll wire via the DB.
	// Since NewStackService needs dbgen.Queries for GetCurrentVersion,
	// and we don't have a real DB here, the export will log a "missing" file
	// (version not found) and produce a zip with _missing_files.txt.
	// That is the expected behavior when no version row exists.
	_ = stor
	svc := service.NewStackService(assetRepo, memory.NewRealVersionRepo(), memory.NewRealVariantRepo(), stor, nil)

	var buf bytes.Buffer
	err := svc.ExportZip(context.Background(), "ws_1", service.ExportZipParams{
		AssetIDs: []string{"ast_1"},
		Filename: "my-export",
	}, &buf)
	// Should succeed (missing file goes into _missing_files.txt, not an error)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty zip output")
	}
}

// -- MergeParams.Validate --

func TestMergeParams_Validate_EmptyIDs(t *testing.T) {
	err := service.MergeParams{OutputType: "gif"}.Validate()
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestMergeParams_Validate_BadOutputType(t *testing.T) {
	err := service.MergeParams{AssetIDs: []string{"x"}, OutputType: "webm"}.Validate()
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad output type, got %v", err)
	}
}

func TestMergeParams_Validate_OK(t *testing.T) {
	for _, ot := range []string{"gif", "pdf"} {
		err := service.MergeParams{AssetIDs: []string{"x"}, OutputType: ot}.Validate()
		if err != nil {
			t.Errorf("output_type %q: unexpected error: %v", ot, err)
		}
	}
}

// -- EnqueueMerge --

func TestStackService_EnqueueMerge_AssetNotInWorkspace(t *testing.T) {
	svc, assetRepo := newStackSvc(t)
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_A"})

	_, err := svc.EnqueueMerge(context.Background(), "ws_B", "user_1", service.MergeParams{
		AssetIDs:   []string{"ast_1"},
		OutputType: "gif",
	})
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestStackService_EnqueueMerge_InvalidOutputType(t *testing.T) {
	svc, assetRepo := newStackSvc(t)
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1"})

	_, err := svc.EnqueueMerge(context.Background(), "ws_1", "user_1", service.MergeParams{
		AssetIDs:   []string{"ast_1"},
		OutputType: "avi",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// -- ExportZip duplicate filename deduplication --

func TestStackService_ExportZip_DuplicateFilenames(t *testing.T) {
	assetRepo := memory.NewAssetRepo()
	versionRepo := memory.NewRealVersionRepo()
	stor, _ := storage.NewAferoMemoryStorage()

	assetRepo.Seed(
		repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", OriginalFilename: "photo.jpg"},
		repository.Asset{ID: "ast_2", WorkspaceID: "ws_1", OriginalFilename: "photo.jpg"},
	)
	versionRepo.Seed(
		repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: true, StorageKey: "key1"},
		repository.AssetVersion{ID: "v2", AssetID: "ast_2", WorkspaceID: "ws_1", IsCurrent: true, StorageKey: "key2"},
	)
	if err := stor.Put(t.Context(), "key1", strings.NewReader("data1")); err != nil {
		t.Fatalf("stor.Put key1: %v", err)
	}
	if err := stor.Put(t.Context(), "key2", strings.NewReader("data2")); err != nil {
		t.Fatalf("stor.Put key2: %v", err)
	}

	svc := service.NewStackService(assetRepo, versionRepo, memory.NewRealVariantRepo(), stor, nil)

	var buf bytes.Buffer
	if err := svc.ExportZip(context.Background(), "ws_1", service.ExportZipParams{
		AssetIDs: []string{"ast_1", "ast_2"},
	}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := buf.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}

	names := make(map[string]struct{}, len(zr.File))
	for _, f := range zr.File {
		if _, dup := names[f.Name]; dup {
			t.Errorf("duplicate zip entry name: %q", f.Name)
		}
		names[f.Name] = struct{}{}
	}
	if len(names) < 2 {
		t.Errorf("expected 2 unique entries, got %d", len(names))
	}
}

// -- sanitiseStackFilename (tested via ExportZip Filename field) --

func TestStackService_ExportZip_FilenameDefault(t *testing.T) {
	// Passing a blank filename should not cause a panic or error (default applied internally).
	assetRepo := memory.NewAssetRepo()
	stor, _ := storage.NewAferoMemoryStorage()
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", OriginalFilename: "f.jpg"})
	svc := service.NewStackService(assetRepo, memory.NewRealVersionRepo(), memory.NewRealVariantRepo(), stor, nil)

	var buf bytes.Buffer
	err := svc.ExportZip(context.Background(), "ws_1", service.ExportZipParams{
		AssetIDs: []string{"ast_1"},
		Filename: strings.Repeat("/", 5), // all stripped → empty → default applied
	}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// cancelAfterNGets wraps a Storage and cancels the given context after the
// Nth call to Get, simulating a client disconnect mid-export.
type cancelAfterNGets struct {
	storage.Storage

	cancel context.CancelFunc
	n      int
	calls  int
}

func (c *cancelAfterNGets) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	c.calls++
	if c.calls == c.n {
		c.cancel()
	}
	return c.Storage.Get(ctx, key)
}

func TestStackService_ExportZip_AbortsOnContextCancellation(t *testing.T) {
	assetRepo := memory.NewAssetRepo()
	versionRepo := memory.NewRealVersionRepo()
	stor, _ := storage.NewAferoMemoryStorage()

	assetRepo.Seed(
		repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", OriginalFilename: "a.jpg"},
		repository.Asset{ID: "ast_2", WorkspaceID: "ws_1", OriginalFilename: "b.jpg"},
		repository.Asset{ID: "ast_3", WorkspaceID: "ws_1", OriginalFilename: "c.jpg"},
	)
	versionRepo.Seed(
		repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: true, StorageKey: "key1"},
		repository.AssetVersion{ID: "v2", AssetID: "ast_2", WorkspaceID: "ws_1", IsCurrent: true, StorageKey: "key2"},
		repository.AssetVersion{ID: "v3", AssetID: "ast_3", WorkspaceID: "ws_1", IsCurrent: true, StorageKey: "key3"},
	)
	for _, k := range []string{"key1", "key2", "key3"} {
		if err := stor.Put(t.Context(), k, strings.NewReader("data")); err != nil {
			t.Fatalf("stor.Put %s: %v", k, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	wrapped := &cancelAfterNGets{Storage: stor, cancel: cancel, n: 1}
	svc := service.NewStackService(assetRepo, versionRepo, memory.NewRealVariantRepo(), wrapped, nil)

	var buf bytes.Buffer
	err := svc.ExportZip(ctx, "ws_1", service.ExportZipParams{
		AssetIDs: []string{"ast_1", "ast_2", "ast_3"},
	}, &buf)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Only the first asset's storage read should have happened before abort.
	if wrapped.calls != 1 {
		t.Errorf("expected export to stop reading storage after cancellation, got %d Get calls", wrapped.calls)
	}
	// The ZIP was never finalized (zw.Close never reached), so this must not be a valid archive.
	data := buf.Bytes()
	if _, zerr := zip.NewReader(bytes.NewReader(data), int64(len(data))); zerr == nil {
		t.Error("expected an incomplete/invalid zip after cancellation, got a valid archive")
	}
}
