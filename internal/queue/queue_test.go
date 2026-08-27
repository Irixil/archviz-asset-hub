package queue

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	_ "modernc.org/sqlite"
)

func TestWorker_FailedJobRecordsSpanError(t *testing.T) {
	q, recorder := newTelemetryQueue(t)
	q.Register("failing", func(context.Context, dbgen.Job) error {
		return errors.New("boom")
	})
	mustEnqueue(t, q, "failing")

	q.processNext(context.Background())

	span := findQueueSpan(t, recorder, "service.queue.job.failing")
	if span.Status().Code != codes.Error {
		t.Fatalf("status = %v, want Error", span.Status().Code)
	}
	if len(span.Events()) == 0 {
		t.Fatal("expected recorded error event")
	}
}

func TestWorker_SuccessfulJobSpan(t *testing.T) {
	q, recorder := newTelemetryQueue(t)
	q.Register("successful", func(context.Context, dbgen.Job) error {
		return nil
	})
	mustEnqueue(t, q, "successful")

	q.processNext(context.Background())

	span := findQueueSpan(t, recorder, "service.queue.job.successful")
	if span.Status().Code != codes.Ok {
		t.Fatalf("status = %v, want Ok", span.Status().Code)
	}
	if span.SpanKind() != trace.SpanKindConsumer {
		t.Fatalf("span kind = %v, want Consumer", span.SpanKind())
	}
	if span.Parent().IsValid() {
		t.Fatal("expected job span to be root")
	}
	for _, attr := range span.Attributes() {
		if string(attr.Key) == "job.type" && attr.Value.AsString() == "successful" {
			return
		}
	}
	t.Fatal("job.type attribute not found")
}

func TestWorker_JobChildSpansParentUnderJobSpan(t *testing.T) {
	q, recorder := newTelemetryQueue(t)
	q.Register("with_child", func(ctx context.Context, _ dbgen.Job) error {
		_, child := telemetry.StartSpan(ctx, "job.child")
		child.End()
		return nil
	})
	mustEnqueue(t, q, "with_child")

	q.processNext(context.Background())

	jobSpan := findQueueSpan(t, recorder, "service.queue.job.with_child")
	childSpan := findQueueSpan(t, recorder, "job.child")
	if childSpan.Parent().SpanID() != jobSpan.SpanContext().SpanID() {
		t.Fatalf("child parent span id = %s, want %s", childSpan.Parent().SpanID(), jobSpan.SpanContext().SpanID())
	}
	if childSpan.SpanContext().TraceID() != jobSpan.SpanContext().TraceID() {
		t.Fatalf("child trace id = %s, want %s", childSpan.SpanContext().TraceID(), jobSpan.SpanContext().TraceID())
	}
}

func newTelemetryQueue(t *testing.T) (*Queue, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	sqlDB, err := sql.Open("sqlite", ":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = dbpkg.RunMigrations(sqlDB); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err = sqlDB.Exec(`INSERT INTO workspaces (id, name) VALUES ('ws_1', 'Test')`); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	return New(dbgen.New(sqlDB), 1), recorder
}

func mustEnqueue(t *testing.T, q *Queue, jobType string) {
	t.Helper()
	if _, err := q.Enqueue(context.Background(), "ws_1", jobType, "{}"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
}

func findQueueSpan(t *testing.T, recorder *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, span := range recorder.Ended() {
		if span.Name() == name {
			return span
		}
	}
	t.Fatalf("span %q not found; ended=%d", name, len(recorder.Ended()))
	return nil
}
