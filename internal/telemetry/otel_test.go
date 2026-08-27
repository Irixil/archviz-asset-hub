package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSetup_Disabled(t *testing.T) {
	t.Parallel()
	shutdown, err := SetupTraces(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("Setup disabled: %v", err)
	}
	if Tracer("damask/test") == nil {
		t.Fatal("expected non-nil tracer")
	}
	if err = shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestSetup_EnabledUnreachableEndpoint(t *testing.T) {
	t.Parallel()
	shutdown, err := SetupTraces(context.Background(), Config{
		Enabled:     true,
		Endpoint:    "http://127.0.0.1:1/v1/traces",
		Token:       "test-token",
		ServiceName: "damask",
		Env:         "test",
	})
	if err != nil {
		t.Fatalf("Setup enabled: %v", err)
	}
	if Tracer("damask/test") == nil {
		t.Fatal("expected non-nil tracer")
	}
	_ = shutdown(context.Background())
}

func TestSetup_ShutdownIdempotent(t *testing.T) {
	t.Parallel()
	shutdown, err := SetupTraces(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err = shutdown(context.Background()); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}
	if err = shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
}

func TestTracer_Named(t *testing.T) {
	t.Parallel()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	tracerA := Tracer("damask/test/a")
	tracerB := Tracer("damask/test/b")
	if tracerA == nil || tracerB == nil {
		t.Fatal("expected named tracers")
	}
}
