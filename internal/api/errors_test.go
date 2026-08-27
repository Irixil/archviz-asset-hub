package api_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	"damask/server/internal/ai"
	"damask/server/internal/apperr"
	"damask/server/internal/testutil"
)

// captureSlog swaps the default slog logger for one writing to a buffer, and
// restores the previous default on test cleanup. Not safe for parallel tests.
func captureSlog(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestErrorStatusResponse_Logs500WithPathMethodWorkspaceAndError(t *testing.T) {
	buf := captureSlog(t)
	env := testutil.NewTestEnv(t)
	env.Workspace.GetAIProviderKeyStatusFn = func(_ context.Context, _ string, _ string) (*ai.KeyStatus, error) {
		return nil, errors.New("boom: provider registry unavailable")
	}

	req := testutil.BearerRequest(
		http.MethodGet,
		"/api/v1/workspace/settings/aiproviders/openrouter",
		nil,
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	out := buf.String()
	for _, want := range []string{
		"unhandled service error",
		"path=/api/v1/workspace/settings/aiproviders/openrouter",
		"method=GET",
		"workspace_id=ws_1",
		"boom: provider registry unavailable",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected log output to contain %q, got: %s", want, out)
		}
	}
}

func TestErrorStatusResponse_Logs4xxAtWarn(t *testing.T) {
	buf := captureSlog(t)
	env := testutil.NewTestEnv(t)
	env.Workspace.GetAIProviderKeyStatusFn = func(_ context.Context, _ string, _ string) (*ai.KeyStatus, error) {
		return nil, apperr.ErrNotFound
	}

	req := testutil.BearerRequest(
		http.MethodGet,
		"/api/v1/workspace/settings/aiproviders/openrouter",
		nil,
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	out := buf.String()
	if strings.Contains(out, "unhandled service error") {
		t.Errorf("expected no unhandled-error log line for a mapped 4xx error, got: %s", out)
	}
	for _, want := range []string{
		"level=WARN",
		"msg=\"service error\"",
		"status=404",
		"path=/api/v1/workspace/settings/aiproviders/openrouter",
		"method=GET",
		"workspace_id=ws_1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected log output to contain %q, got: %s", want, out)
		}
	}
}
