// Package telemetry provides OpenTelemetry instrumentation helpers.
package telemetry

import (
	"fmt"

	fiberotel "github.com/gofiber/contrib/v3/otel"
	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func FiberMiddleware() fiber.Handler {
	return fiberotel.Middleware(
		fiberotel.WithTracerProvider(otel.GetTracerProvider()),
		fiberotel.WithoutMetrics(true),
		fiberotel.WithSpanNameFormatter(func(ctx fiber.Ctx) string {
			return ctx.Method() + " " + ctx.Route().Path
		}),
	)
}

func FiberStatusMiddleware() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		err := ctx.Next()
		span := trace.SpanFromContext(ctx.Context())
		if span.IsRecording() && ctx.Response().StatusCode() >= fiber.StatusBadRequest {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", ctx.Response().StatusCode()))
		}
		return err
	}
}

func EnrichSpan(ctx fiber.Ctx, workspaceID, userID string) {
	span := trace.SpanFromContext(ctx.Context())
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.user_id", userID),
	)
}
