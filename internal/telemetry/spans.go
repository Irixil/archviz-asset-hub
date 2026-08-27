package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const serviceTracerName = "damask/server/service"

func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, name)
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return ctx, span
}

func StartBackgroundSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	rootCtx := trace.ContextWithSpanContext(ctx, trace.SpanContext{})
	ctx, span := otel.Tracer(serviceTracerName).Start(rootCtx, name, trace.WithSpanKind(trace.SpanKindConsumer))
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return ctx, span
}

func EndSpan(span trace.Span, err error) {
	if err != nil {
		RecordError(span, err)
	}
	span.End()
}

func RecordError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
