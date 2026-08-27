package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"damask/server/internal/service"
	"damask/server/internal/testutil"

	"github.com/gofiber/fiber/v3"
)

// TestPatchAssetFields_FTSRefreshContextSurvivesRequestCancellation proves the
// pattern used at both async-FTS-refresh call sites in
// internal/api/custom_fields.go: the goroutine must run with
// [context.WithoutCancel](c.Context()). A test middleware replaces the request
// context with a cancellable one and cancels it right after the handler
// returns, simulating request teardown before the async goroutine runs. With
// WithoutCancel the refresh context stays live; with a plain c.Context() this
// test fails with [context.Canceled].
func TestPatchAssetFields_FTSRefreshContextSurvivesRequestCancellation(t *testing.T) {
	requestDone := make(chan struct{})
	env := testutil.NewTestEnv(t, testutil.WithPreMiddleware(func(c fiber.Ctx) error {
		ctx, cancel := context.WithCancel(c.Context())
		c.SetContext(ctx)
		err := c.Next()
		cancel()
		close(requestDone)
		return err
	}))

	env.AssetFields.SetValuesFn = func(
		_ context.Context, _, _, _ string, _ []service.SetFieldValueInput,
	) ([]*service.FieldValueDTO, error) {
		return nil, nil
	}

	called := make(chan error, 1)
	env.Assets.RefreshFTSFn = func(ctx context.Context, _ string) error {
		<-requestDone // wait until the request context has been cancelled
		called <- ctx.Err()
		return nil
	}

	req := testutil.BearerRequest(
		http.MethodPatch,
		"/api/v1/assets/ast_1/fields",
		testutil.JSONBody(map[string]any{
			"values": []map[string]any{{"field_id": "fld_1", "value": "hello"}},
		}),
		env.MintToken(t, "usr_1", "ws_1"),
	)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case ctxErr := <-called:
		if ctxErr != nil {
			t.Errorf("expected FTS refresh context to survive request cancellation, got: %v", ctxErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async RefreshFTS call")
	}
}
