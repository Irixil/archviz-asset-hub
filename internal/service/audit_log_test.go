package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

func newAuditSvc(t *testing.T) service.AuditLogService {
	t.Helper()
	return service.NewAuditLogService(memory.NewAuditLogRepo())
}

// -- ListAssetEvents --

func TestAuditLogService_ListAssetEvents_Empty(t *testing.T) {
	t.Parallel()
	svc := newAuditSvc(t)
	result, err := svc.ListAssetEvents(context.Background(), service.ListAssetEventsParams{
		AssetID:     "ast_1",
		WorkspaceID: "ws_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Events) != 0 {
		t.Errorf("expected empty events, got %d", len(result.Events))
	}
	if result.HasMore {
		t.Errorf("expected HasMore=false")
	}
}

// -- ListProjectEvents --

func TestAuditLogService_ListProjectEvents_Empty(t *testing.T) {
	t.Parallel()
	svc := newAuditSvc(t)
	result, err := svc.ListProjectEvents(context.Background(), service.ListProjectEventsParams{
		ProjectID:   "proj_1",
		WorkspaceID: "ws_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Events) != 0 {
		t.Errorf("expected empty events, got %d", len(result.Events))
	}
}

// -- ListWorkspaceActivity --

func TestAuditLogService_ListWorkspaceActivity_Empty(t *testing.T) {
	t.Parallel()
	svc := newAuditSvc(t)
	result, err := svc.ListWorkspaceActivity(context.Background(), service.ListWorkspaceActivityParams{
		WorkspaceID: "ws_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Events) != 0 {
		t.Errorf("expected empty activity, got %d", len(result.Events))
	}
}

// -- ExportActivity --

func TestAuditLogService_ExportActivity_EmptyWorkspace(t *testing.T) {
	t.Parallel()
	svc := newAuditSvc(t)
	csv, err := svc.ExportActivity(context.Background(), service.ExportActivityParams{
		WorkspaceID: "ws_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(csv, "event_id,") {
		t.Errorf("expected CSV header, got: %q", csv[:min(len(csv), 60)])
	}
}

// -- clampLimit / pagination helpers (tested via ListAssetEvents) --

func TestAuditLogService_ListAssetEvents_LimitDefault(t *testing.T) {
	t.Parallel()
	svc := newAuditSvc(t)
	// limit=0 should default to 50 (no panic, no error)
	result, err := svc.ListAssetEvents(context.Background(), service.ListAssetEventsParams{
		AssetID: "x", WorkspaceID: "y", Limit: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// -- ValidateExportDateRange --

func TestValidateExportDateRange_OK(t *testing.T) {
	t.Parallel()
	if err := service.ValidateExportDateRange("2024-01-01", "2024-12-31"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateExportDateRange_EmptyOK(t *testing.T) {
	t.Parallel()
	if err := service.ValidateExportDateRange("", ""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateExportDateRange_BadSince(t *testing.T) {
	t.Parallel()
	err := service.ValidateExportDateRange("not-a-date", "")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestValidateExportDateRange_BadUntil(t *testing.T) {
	t.Parallel()
	err := service.ValidateExportDateRange("", "2024/12/31")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// -- ParseTypesFilter --

func TestParseTypesFilter_Empty(t *testing.T) {
	t.Parallel()
	if got := service.ParseTypesFilter(""); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestParseTypesFilter_Single(t *testing.T) {
	t.Parallel()
	got := service.ParseTypesFilter("asset_renamed")
	if len(got) != 1 || got[0] != "asset_renamed" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestParseTypesFilter_Multiple(t *testing.T) {
	t.Parallel()
	got := service.ParseTypesFilter("asset_renamed, asset_shared")
	if len(got) != 2 {
		t.Errorf("expected 2 types, got %d: %v", len(got), got)
	}
}

// --- ParseLimit ---

func TestParseLimit_Default(t *testing.T) {
	t.Parallel()
	got := service.ParseLimit("", 25, 100)
	if got != 25 {
		t.Errorf("expected 25 for empty input, got %d", got)
	}
}

func TestParseLimit_ValidValue(t *testing.T) {
	t.Parallel()
	got := service.ParseLimit("50", 25, 100)
	if got != 50 {
		t.Errorf("expected 50, got %d", got)
	}
}

func TestParseLimit_ClampedToMax(t *testing.T) {
	t.Parallel()
	got := service.ParseLimit("999", 25, 100)
	if got != 100 {
		t.Errorf("expected 100 (clamped), got %d", got)
	}
}

func TestParseLimit_ZeroReturnsDefault(t *testing.T) {
	t.Parallel()
	// zero is invalid (n <= 0), so returns defaultVal
	got := service.ParseLimit("0", 25, 100)
	if got != 25 {
		t.Errorf("expected default 25 for zero input, got %d", got)
	}
}

func TestParseLimit_NegativeReturnsDefault(t *testing.T) {
	t.Parallel()
	got := service.ParseLimit("-1", 25, 100)
	if got != 25 {
		t.Errorf("expected default 25 for negative input, got %d", got)
	}
}

func TestParseLimit_InvalidString(t *testing.T) {
	t.Parallel()
	got := service.ParseLimit("abc", 25, 100)
	if got != 25 {
		t.Errorf("expected default 25 for invalid string, got %d", got)
	}
}

func TestCSVEscape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"", ""},
		{"with,comma", `"with,comma"`},
		{`with"quote`, `"with""quote"`},
		{"with\nnewline", "\"with\nnewline\""},
		{`both,"`, `"both,"""`},
	}
	for _, c := range cases {
		got := service.CSVEscape(c.in)
		if got != c.want {
			t.Errorf("CSVEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClampLimit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v, def, max, want int64
	}{
		{0, 20, 100, 20},
		{-1, 20, 100, 20},
		{150, 20, 100, 100},
		{50, 20, 100, 50},
		{100, 20, 100, 100},
	}
	for _, c := range cases {
		got := service.ClampLimit(c.v, c.def, c.max)
		if got != c.want {
			t.Errorf("ClampLimit(%d,%d,%d) = %d, want %d", c.v, c.def, c.max, got, c.want)
		}
	}
}

func TestMakeTypesFilter(t *testing.T) {
	t.Parallel()

	if service.MakeTypesFilter(nil) != nil {
		t.Error("nil input should return nil")
	}
	if service.MakeTypesFilter([]string{"  ", ""}) != nil {
		t.Error("all-blank input should return nil")
	}

	m := service.MakeTypesFilter([]string{"a", "b", "  a  "})
	if len(m) != 2 || !m["a"] || !m["b"] {
		t.Errorf("unexpected map: %v", m)
	}
}

func TestPaginateAuditEvents(t *testing.T) {
	t.Parallel()

	makeEvents := func(n int) []service.AuditEventDTO {
		out := make([]service.AuditEventDTO, n)
		for i := range out {
			out[i] = service.AuditEventDTO{CreatedAt: string(rune('A' + i))}
		}
		return out
	}

	// empty slice
	res := service.PaginateAuditEvents(nil, 5)
	if res.HasMore || len(res.Events) != 0 || res.NextCursor != nil {
		t.Errorf("empty: unexpected result %+v", res)
	}

	// under limit — no cursor
	res = service.PaginateAuditEvents(makeEvents(3), 5)
	if res.HasMore || len(res.Events) != 3 || res.NextCursor != nil {
		t.Errorf("under limit: unexpected result %+v", res)
	}

	// over limit — cursor set to last retained item's CreatedAt
	events := makeEvents(5)
	res = service.PaginateAuditEvents(events, 3)
	if !res.HasMore {
		t.Error("expected HasMore=true")
	}
	if len(res.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(res.Events))
	}
	if res.NextCursor == nil || *res.NextCursor != events[2].CreatedAt {
		t.Errorf("expected cursor %q, got %v", events[2].CreatedAt, res.NextCursor)
	}
}

// -- Cross-workspace negative test --
// AuditLogRepository's List* methods must return no rows for a workspace that
// didn't create the events, even when querying by the correct entity ID.

func TestAuditLogRepository_CrossWorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAuditLogRepo()

	wsA, wsB := "ws-a", "ws-b"
	repo.SeedAssetEvent(wsA, repository.AuditEvent{
		ID:        "evt-1",
		EntityID:  "asset-1",
		EventType: "asset_renamed",
		CreatedAt: "2024-01-01 00:00:00",
		ActorType: "user",
	})
	repo.SeedProjectEvent(wsA, repository.AuditEvent{
		ID:        "evt-2",
		EntityID:  "proj-1",
		EventType: "project_renamed",
		CreatedAt: "2024-01-01 00:00:00",
		ActorType: "user",
	})

	assetOut, err := repo.ListAssetEvents(
		ctx,
		repository.ListAuditEventsParams{WorkspaceID: wsB, EntityID: "asset-1", Limit: 50},
	)
	if err != nil || len(assetOut) != 0 {
		t.Errorf("ListAssetEvents cross-workspace: expected empty, got %v, err=%v", assetOut, err)
	}
	projOut, err := repo.ListProjectEvents(
		ctx,
		repository.ListAuditEventsParams{WorkspaceID: wsB, EntityID: "proj-1", Limit: 50},
	)
	if err != nil || len(projOut) != 0 {
		t.Errorf("ListProjectEvents cross-workspace: expected empty, got %v, err=%v", projOut, err)
	}
	wsAssetOut, err := repo.ListWorkspaceAssetEvents(ctx, repository.ListAuditEventsParams{WorkspaceID: wsB, Limit: 50})
	if err != nil || len(wsAssetOut) != 0 {
		t.Errorf("ListWorkspaceAssetEvents cross-workspace: expected empty, got %v, err=%v", wsAssetOut, err)
	}
	wsProjOut, err := repo.ListWorkspaceProjectEvents(
		ctx,
		repository.ListAuditEventsParams{WorkspaceID: wsB, Limit: 50},
	)
	if err != nil || len(wsProjOut) != 0 {
		t.Errorf("ListWorkspaceProjectEvents cross-workspace: expected empty, got %v, err=%v", wsProjOut, err)
	}

	// Sanity: same-workspace access still works after all the negative checks above.
	out, err := repo.ListAssetEvents(
		ctx,
		repository.ListAuditEventsParams{WorkspaceID: wsA, EntityID: "asset-1", Limit: 50},
	)
	if err != nil || len(out) != 1 {
		t.Errorf("ListAssetEvents same-workspace: expected 1 event, got %v, err=%v", out, err)
	}
}
