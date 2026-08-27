package service_test

import (
	"context"
	"testing"
	"time"

	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"

	"golang.org/x/oauth2"
)

type integrationTestEnv struct {
	svc         service.IntegrationService
	workspaceID string
	userID      string
}

func newIntegrationEnv(t *testing.T) *integrationTestEnv {
	t.Helper()
	env := newSQLiteEnv(t)
	repo := reposqlc.NewOAuthRepo(env.database)
	return &integrationTestEnv{
		svc:         service.NewIntegrationService(repo),
		workspaceID: env.workspaceID,
		userID:      env.userID,
	}
}

func passthroughEncrypt(s string) (string, error) { return s, nil }

func makeOAuthToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-xyz",
		Expiry:       time.Now().Add(time.Hour),
	}
}

func TestIntegrationService_ListConnections_Empty(t *testing.T) {
	t.Parallel()
	env := newIntegrationEnv(t)

	conns, err := env.svc.ListConnections(context.Background(), env.workspaceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 0 {
		t.Errorf("expected empty list, got %d", len(conns))
	}
}

func TestIntegrationService_UpsertThenList(t *testing.T) {
	t.Parallel()
	env := newIntegrationEnv(t)

	err := env.svc.UpsertConnection(context.Background(), service.UpsertConnectionParams{
		WorkspaceID:    env.workspaceID,
		UserID:         env.userID,
		Provider:       "google",
		ProviderUserID: "goog_123",
		ProviderEmail:  "user@gmail.com",
		Token:          makeOAuthToken(),
		Scopes:         []string{"drive.readonly"},
		EncryptToken:   passthroughEncrypt,
	})
	if err != nil {
		t.Fatalf("UpsertConnection: %v", err)
	}

	conns, err := env.svc.ListConnections(context.Background(), env.workspaceID)
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].Provider != "google" {
		t.Errorf("Provider: got %q, want google", conns[0].Provider)
	}
	if conns[0].ProviderEmail != "user@gmail.com" {
		t.Errorf("ProviderEmail: got %q, want user@gmail.com", conns[0].ProviderEmail)
	}
}

func TestIntegrationService_UpsertIdempotent(t *testing.T) {
	t.Parallel()
	env := newIntegrationEnv(t)

	p := service.UpsertConnectionParams{
		WorkspaceID:    env.workspaceID,
		UserID:         env.userID,
		Provider:       "google",
		ProviderUserID: "goog_456",
		ProviderEmail:  "dup@gmail.com",
		Token:          makeOAuthToken(),
		Scopes:         []string{"drive"},
		EncryptToken:   passthroughEncrypt,
	}
	if err := env.svc.UpsertConnection(context.Background(), p); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	p.Token.AccessToken = "new-access"
	if err := env.svc.UpsertConnection(context.Background(), p); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	conns, _ := env.svc.ListConnections(context.Background(), env.workspaceID)
	if len(conns) != 1 {
		t.Errorf("expected 1 connection after two upserts for same provider user, got %d", len(conns))
	}
}

func TestIntegrationService_DeleteConnection_NotFound(t *testing.T) {
	t.Parallel()
	env := newIntegrationEnv(t)

	err := env.svc.DeleteConnection(context.Background(), env.workspaceID, "nonexistent-id")
	if err == nil {
		t.Error("expected error deleting non-existent connection")
	}
}

func TestIntegrationService_DeleteThenList(t *testing.T) {
	t.Parallel()
	env := newIntegrationEnv(t)

	if err := env.svc.UpsertConnection(context.Background(), service.UpsertConnectionParams{
		WorkspaceID:    env.workspaceID,
		UserID:         env.userID,
		Provider:       "canva",
		ProviderUserID: "canva_1",
		Token:          makeOAuthToken(),
		Scopes:         []string{},
		EncryptToken:   passthroughEncrypt,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	conns, _ := env.svc.ListConnections(context.Background(), env.workspaceID)
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection after insert, got %d", len(conns))
	}

	if err := env.svc.DeleteConnection(context.Background(), env.workspaceID, conns[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	remaining, _ := env.svc.ListConnections(context.Background(), env.workspaceID)
	if len(remaining) != 0 {
		t.Errorf("expected 0 connections after delete, got %d", len(remaining))
	}
}
