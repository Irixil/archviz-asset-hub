package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/mail"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"

	"github.com/google/uuid"
)

const testAppSecret = "test-app-secret-for-tests!!"

// ingressTestEnv holds memory repos + a seeded workspace/user for ingress tests.
type ingressTestEnv struct {
	repo        *memory.IngressMemoryRepo
	svc         service.IngressService
	workspaceID string
	userID      string
}

// newIngressEnv wires an IngressService against memory repos with a seeded user.
func newIngressEnv(t *testing.T) *ingressTestEnv {
	t.Helper()
	wsID := uuid.NewString()
	userID := uuid.NewString()

	users := memory.NewRealUserRepo()
	users.Seed(repository.User{ID: userID, Email: userID + "@test.com", Name: "test"})

	repo := memory.NewIngressRepo()
	mailer := mail.NewMailer(&mail.Config{})
	svc := service.NewIngressService(repo, users, testAppSecret, nil, mailer)
	return &ingressTestEnv{repo: repo, svc: svc, workspaceID: wsID, userID: userID}
}

// seedSource creates an ingress source via the service.
func seedSource(t *testing.T, env *ingressTestEnv, label string) *service.IngressSourceDTO {
	t.Helper()
	dto, err := env.svc.CreateSource(
		context.Background(),
		env.workspaceID,
		env.userID,
		service.CreateIngressSourceParams{
			Type:  "sftp",
			Label: label,
		},
	)
	if err != nil {
		t.Fatalf("seed source: %v", err)
	}
	return dto
}

// -- ListSources --

func TestIngressService_ListSources_Empty(t *testing.T) {
	env := newIngressEnv(t)
	out, err := env.svc.ListSources(context.Background(), env.workspaceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty list, got %d", len(out))
	}
}

func TestIngressService_ListSources_WorkspaceIsolation(t *testing.T) {
	env := newIngressEnv(t)
	seedSource(t, env, "my source")

	out, err := env.svc.ListSources(context.Background(), "other-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected 0 for other workspace, got %d", len(out))
	}
}

// -- GetSource --

func TestIngressService_GetSource_NotFound(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.GetSource(context.Background(), env.workspaceID, "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// -- CreateSource --

func TestIngressService_CreateSource_EmptyLabel(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.CreateSource(context.Background(), env.workspaceID, env.userID, service.CreateIngressSourceParams{
		Type:  "sftp",
		Label: "",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty label, got %v", err)
	}
}

func TestIngressService_CreateSource_EmptyType(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.CreateSource(context.Background(), env.workspaceID, env.userID, service.CreateIngressSourceParams{
		Type:  "",
		Label: "my source",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty type, got %v", err)
	}
}

func TestIngressService_CreateSource_DefaultInterval(t *testing.T) {
	env := newIngressEnv(t)
	dto, err := env.svc.CreateSource(
		context.Background(),
		env.workspaceID,
		env.userID,
		service.CreateIngressSourceParams{
			Type:            "sftp",
			Label:           "src",
			PollIntervalMin: 0, // should default to 15
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.PollIntervalMin != 15 {
		t.Errorf("PollIntervalMin: got %d, want 15", dto.PollIntervalMin)
	}
}

func TestIngressService_CreateSource_OK(t *testing.T) {
	env := newIngressEnv(t)
	dto, err := env.svc.CreateSource(
		context.Background(),
		env.workspaceID,
		env.userID,
		service.CreateIngressSourceParams{
			Type:  "sftp",
			Label: "production sftp",
			Config: map[string]any{
				"host":     "sftp.example.com",
				"password": "s3cr3t",
			},
			PollIntervalMin: 30,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Label != "production sftp" {
		t.Errorf("Label: got %q, want %q", dto.Label, "production sftp")
	}
	if dto.PollIntervalMin != 30 {
		t.Errorf("PollIntervalMin: got %d, want 30", dto.PollIntervalMin)
	}
	if !dto.Enabled {
		t.Errorf("expected Enabled=true by default")
	}
	// Sensitive config fields must be redacted
	if pw, ok := dto.Config["password"]; ok && pw != "***" {
		t.Errorf("password should be redacted, got %v", pw)
	}
}

func TestIngressService_CreateSource_WithRules(t *testing.T) {
	env := newIngressEnv(t)
	dto, err := env.svc.CreateSource(
		context.Background(),
		env.workspaceID,
		env.userID,
		service.CreateIngressSourceParams{
			Type:  "sftp",
			Label: "sftp with rules",
			Rules: []service.CreateIngressRuleParams{
				{Position: 1, Field: "filename", Operator: "contains", Value: ".jpg", Action: "include"},
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rules, err := env.svc.ListRules(context.Background(), env.workspaceID, dto.ID)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Field != "filename" {
		t.Errorf("Field: got %q, want %q", rules[0].Field, "filename")
	}
}

// -- UpdateSource --

func TestIngressService_UpdateSource_NotFound(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.UpdateSource(context.Background(), env.workspaceID, "nope", service.UpdateIngressSourceParams{})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngressService_UpdateSource_LabelKeptWhenEmpty(t *testing.T) {
	env := newIngressEnv(t)
	src := seedSource(t, env, "original label")

	updated, err := env.svc.UpdateSource(
		context.Background(),
		env.workspaceID,
		src.ID,
		service.UpdateIngressSourceParams{
			Label: "", // empty = keep original
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Label != "original label" {
		t.Errorf("Label: got %q, want %q", updated.Label, "original label")
	}
}

// -- DeleteSource --

func TestIngressService_DeleteSource_OK(t *testing.T) {
	env := newIngressEnv(t)
	src := seedSource(t, env, "to delete")

	if err := env.svc.DeleteSource(context.Background(), env.workspaceID, src.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := env.svc.GetSource(context.Background(), env.workspaceID, src.ID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

// -- Rules --

func TestIngressService_ListRules_SourceNotFound(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.ListRules(context.Background(), env.workspaceID, "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngressService_CreateRule_OK(t *testing.T) {
	env := newIngressEnv(t)
	src := seedSource(t, env, "src")

	rule, err := env.svc.CreateRule(context.Background(), env.workspaceID, src.ID, service.CreateIngressRuleParams{
		Position: 1,
		Field:    "filename",
		Operator: "ends_with",
		Value:    ".png",
		Action:   "include",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Action != "include" {
		t.Errorf("Action: got %q, want %q", rule.Action, "include")
	}
}

func TestIngressService_DeleteRule_WrongSource(t *testing.T) {
	env := newIngressEnv(t)
	src1 := seedSource(t, env, "src1")
	src2 := seedSource(t, env, "src2")

	rule, err := env.svc.CreateRule(context.Background(), env.workspaceID, src1.ID, service.CreateIngressRuleParams{
		Position: 1, Field: "filename", Operator: "contains", Value: ".jpg", Action: "include",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Attempt to delete rule via wrong source
	err = env.svc.DeleteRule(context.Background(), env.workspaceID, src2.ID, rule.ID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngressService_ReorderRules_OK(t *testing.T) {
	env := newIngressEnv(t)
	src := seedSource(t, env, "src")

	r1, _ := env.svc.CreateRule(context.Background(), env.workspaceID, src.ID, service.CreateIngressRuleParams{
		Position: 1, Field: "filename", Operator: "contains", Value: "a", Action: "include",
	})
	r2, _ := env.svc.CreateRule(context.Background(), env.workspaceID, src.ID, service.CreateIngressRuleParams{
		Position: 2, Field: "filename", Operator: "contains", Value: "b", Action: "include",
	})

	reordered, err := env.svc.ReorderRules(context.Background(), env.workspaceID, src.ID, []service.ReorderRuleEntry{
		{ID: r1.ID, Position: 10},
		{ID: r2.ID, Position: 5},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// List returns in position order; r2 (pos=5) should be first
	if reordered[0].ID != r2.ID {
		t.Errorf("expected r2 first after reorder, got %s", reordered[0].ID)
	}
}

// -- Log --

func TestIngressService_ListLog_Empty(t *testing.T) {
	env := newIngressEnv(t)
	out, err := env.svc.ListLog(context.Background(), env.workspaceID, "", 50, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty log, got %d", len(out))
	}
}

func TestIngressService_DeleteLogEntry_NotFound(t *testing.T) {
	env := newIngressEnv(t)
	err := env.svc.DeleteLogEntry(context.Background(), env.workspaceID, "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngressService_RetryLogEntry_NotFound(t *testing.T) {
	env := newIngressEnv(t)
	_, err := env.svc.RetryLogEntry(context.Background(), env.workspaceID, "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngressService_RetryLogEntry_InvalidStatus(t *testing.T) {
	env := newIngressEnv(t)
	src := seedSource(t, env, "src")

	// Seed a log entry directly on the repo, already marked "imported".
	entryID := uuid.NewString()
	env.repo.SeedLogEntry(repository.IngressLogEntry{
		ID:       entryID,
		SourceID: src.ID,
		RemoteID: "remote_1",
		Filename: "file.jpg",
		Status:   "imported",
	})

	_, err := env.svc.RetryLogEntry(context.Background(), env.workspaceID, entryID)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for imported status, got %v", err)
	}
}

// -- Cross-workspace negative test --
// Every read/update/delete method on IngressRepository must return not-found/no-op
// when called with a different workspace ID than the one that owns the resource.

func TestIngressRepository_CrossWorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewIngressRepo()

	wsA, wsB := "ws-a", "ws-b"
	src, createErr := repo.CreateSource(ctx, repository.CreateIngressSourceParams{
		ID: "src-1", WorkspaceID: wsA, CreatedBy: "user-1", Type: "sftp", Label: "a-source",
	})
	if createErr != nil {
		t.Fatalf("create source: %v", createErr)
	}
	rule, createRuleErr := repo.CreateRule(ctx, repository.CreateIngressRuleParams{
		ID: "rule-1", WorkspaceID: wsA, SourceID: src.ID, Position: 1,
		Field: "filename", Operator: "contains", Value: "x", Action: "include",
	})
	if createRuleErr != nil {
		t.Fatalf("create rule: %v", createRuleErr)
	}
	repo.SeedLogEntry(
		repository.IngressLogEntry{ID: "log-1", SourceID: src.ID, RemoteID: "r1", Filename: "f", Status: "pending"},
	)

	checks := []struct {
		name string
		run  func() error
	}{
		{"GetSource", func() error { _, err := repo.GetSource(ctx, wsB, src.ID); return err }},
		{"DeleteSource", func() error { return repo.DeleteSource(ctx, wsB, src.ID) }},
		{"UpdateSource", func() error {
			_, err := repo.UpdateSource(ctx, repository.UpdateIngressSourceParams{ID: src.ID, WorkspaceID: wsB})
			return err
		}},
		{"ListRules", func() error { _, err := repo.ListRules(ctx, wsB, src.ID); return err }},
		{"GetRule", func() error { _, err := repo.GetRule(ctx, wsB, rule.ID); return err }},
		{"CreateRule", func() error {
			_, err := repo.CreateRule(
				ctx,
				repository.CreateIngressRuleParams{ID: "rule-2", WorkspaceID: wsB, SourceID: src.ID},
			)
			return err
		}},
		{"UpdateRule", func() error {
			_, err := repo.UpdateRule(ctx, wsB, repository.UpdateIngressRuleParams{ID: rule.ID})
			return err
		}},
		{"DeleteRule", func() error { return repo.DeleteRule(ctx, wsB, rule.ID) }},
		{"ListSourceLog", func() error { _, err := repo.ListSourceLog(ctx, wsB, src.ID, 10, 0); return err }},
		{"GetLogEntry", func() error { _, err := repo.GetLogEntry(ctx, wsB, "log-1"); return err }},
		{"UpdateLogEntry", func() error {
			return repo.UpdateLogEntry(
				ctx,
				wsB,
				repository.UpdateIngressLogEntryParams{ID: "log-1", Status: "imported"},
			)
		}},
		{"DeleteLogEntry", func() error { return repo.DeleteLogEntry(ctx, wsB, "log-1") }},
	}
	for _, c := range checks {
		if err := c.run(); !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("%s cross-workspace: expected ErrNotFound, got %v", c.name, err)
		}
	}

	if out, err := repo.ListWorkspaceLog(ctx, wsB, "", 10, 0); err != nil || len(out) != 0 {
		t.Errorf("ListWorkspaceLog cross-workspace: expected empty result, got %v, err=%v", out, err)
	}

	// Sanity: same-workspace access still works after all the negative checks above.
	if _, err := repo.GetSource(ctx, wsA, src.ID); err != nil {
		t.Errorf("GetSource same-workspace: unexpected error %v", err)
	}
}
