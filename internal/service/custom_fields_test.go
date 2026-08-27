package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

func newFieldSvc(t *testing.T) service.FieldService {
	t.Helper()
	repo := memory.NewRealFieldRepo()
	return service.NewFieldService(repo)
}

func baseFieldParams() service.CreateFieldDefinitionParams {
	return service.CreateFieldDefinitionParams{
		CreatedBy: "user_1",
		Scope:     "asset",
		Name:      "Rating",
		Key:       "rating",
		FieldType: "number",
	}
}

// --- Create ---

func TestFieldService_Create_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", baseFieldParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Rating" || dto.Key != "rating" {
		t.Errorf("unexpected dto: %+v", dto)
	}
	if dto.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestFieldService_Create_EmptyName(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	p := baseFieldParams()
	p.Name = "   "
	_, err := svc.Create(context.Background(), "ws_1", p)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestFieldService_Create_InvalidScope(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	p := baseFieldParams()
	p.Scope = "invalid"
	_, err := svc.Create(context.Background(), "ws_1", p)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestFieldService_Create_DuplicateKey(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	if _, err := svc.Create(context.Background(), "ws_1", baseFieldParams()); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(context.Background(), "ws_1", baseFieldParams())
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate key, got %v", err)
	}
}

func TestFieldService_Create_MaxFields(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	for i := range 50 {
		p := baseFieldParams()
		p.Key = "field_" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		p.Name = "Field " + p.Key
		if _, err := svc.Create(context.Background(), "ws_1", p); err != nil {
			t.Fatalf("create #%d: %v", i, err)
		}
	}
	// 51st should fail
	p := baseFieldParams()
	p.Key = "overflow"
	p.Name = "Overflow"
	_, err := svc.Create(context.Background(), "ws_1", p)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (max fields), got %v", err)
	}
}

// --- Get ---

func TestFieldService_Get_NotFound(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Update ---

func TestFieldService_Update_ImmutableKey(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseFieldParams())
	newKey := "new_key"
	_, err := svc.Update(context.Background(), "ws_1", dto.ID, service.UpdateFieldDefinitionParams{Key: &newKey})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (immutable key), got %v", err)
	}
}

func TestFieldService_Update_ImmutableFieldType(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseFieldParams())
	newType := "text"
	_, err := svc.Update(context.Background(), "ws_1", dto.ID, service.UpdateFieldDefinitionParams{FieldType: &newType})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (immutable field_type), got %v", err)
	}
}

func TestFieldService_Update_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseFieldParams())
	newName := "Score"
	updated, err := svc.Update(
		context.Background(),
		"ws_1",
		dto.ID,
		service.UpdateFieldDefinitionParams{Name: &newName},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "Score" {
		t.Errorf("Name: got %q, want %q", updated.Name, "Score")
	}
}

func TestFieldService_Update_EmptyName(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseFieldParams())
	empty := ""
	_, err := svc.Update(context.Background(), "ws_1", dto.ID, service.UpdateFieldDefinitionParams{Name: &empty})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// --- Delete ---

func TestFieldService_Delete_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	dto, _ := svc.Create(context.Background(), "ws_1", baseFieldParams())
	if err := svc.Delete(context.Background(), "ws_1", dto.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Soft-deleted: should return not found
	_, err := svc.Get(context.Background(), "ws_1", dto.ID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after soft-delete, got %v", err)
	}
}

func TestFieldService_Delete_NotFound(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- List ---

func TestFieldService_List_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	svc.Create(
		context.Background(),
		"ws_1",
		service.CreateFieldDefinitionParams{
			CreatedBy: "u1",
			Scope:     "asset",
			Name:      "Rating",
			Key:       "rating",
			FieldType: "number",
		},
	)
	svc.Create(
		context.Background(),
		"ws_1",
		service.CreateFieldDefinitionParams{
			CreatedBy: "u1",
			Scope:     "asset",
			Name:      "Color",
			Key:       "color",
			FieldType: "text",
		},
	)
	svc.Create(
		context.Background(),
		"ws_2",
		service.CreateFieldDefinitionParams{
			CreatedBy: "u1",
			Scope:     "asset",
			Name:      "Size",
			Key:       "size",
			FieldType: "number",
		},
	)

	fields, err := svc.List(context.Background(), "ws_1", "asset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 2 {
		t.Errorf("expected 2 fields for ws_1, got %d", len(fields))
	}
}

func TestFieldService_List_Empty(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	fields, err := svc.List(context.Background(), "ws_none", "asset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected empty, got %d", len(fields))
	}
}

// --- GetStats ---

func TestFieldService_GetStats_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	f, _ := svc.Create(
		context.Background(),
		"ws_1",
		service.CreateFieldDefinitionParams{
			CreatedBy: "u1",
			Scope:     "asset",
			Name:      "Rating",
			Key:       "rating",
			FieldType: "number",
		},
	)

	stats, err := svc.GetStats(context.Background(), "ws_1", f.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.AssetCount < 0 {
		t.Errorf("expected non-negative AssetCount, got %d", stats.AssetCount)
	}
}

func TestFieldService_GetStats_NotFound(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	_, err := svc.GetStats(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Reorder ---

func TestFieldService_Reorder_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	f1, _ := svc.Create(
		context.Background(),
		"ws_1",
		service.CreateFieldDefinitionParams{
			CreatedBy: "u1",
			Scope:     "asset",
			Name:      "Alpha",
			Key:       "alpha",
			FieldType: "text",
		},
	)
	f2, _ := svc.Create(
		context.Background(),
		"ws_1",
		service.CreateFieldDefinitionParams{
			CreatedBy: "u1",
			Scope:     "asset",
			Name:      "Beta",
			Key:       "beta",
			FieldType: "text",
		},
	)

	err := svc.Reorder(context.Background(), "ws_1", []service.ReorderFieldItem{
		{ID: f1.ID, Position: 2},
		{ID: f2.ID, Position: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- InheritProjectFields ---

func TestFieldService_InheritProjectFields_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	err := svc.InheritProjectFields(context.Background(), "ws_1", "ast_1", "proj_1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- PurgeExpiredFields ---

func TestFieldService_PurgeExpiredFields_OK(t *testing.T) {
	t.Parallel()
	svc := newFieldSvc(t)
	n, err := svc.PurgeExpiredFields(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n < 0 {
		t.Errorf("expected non-negative purge count, got %d", n)
	}
}
