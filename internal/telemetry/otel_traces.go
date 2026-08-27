package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func SetupTraces(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if !cfg.Enabled {
		// otel.SetTracerProvider(trace.NewNoopTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	spanExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(cfg.Endpoint+"/traces"),
		otlptracehttp.WithHeaders(map[string]string{
			authorizationHeader: "Bearer " + cfg.Token,
		}),
	)
	if err != nil {
		// otel.SetTracerProvider(trace.NewNoopTracerProvider())
		return func(context.Context) error { return nil }, err
	}

	res, _ := getResource(cfg)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(spanExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
