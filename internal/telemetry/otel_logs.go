package telemetry

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	logglobal "go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

func SetupLogs(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	logsExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpointURL(cfg.Endpoint+"/logs"),
		otlploghttp.WithHeaders(map[string]string{
			authorizationHeader: "Bearer " + cfg.Token,
		}),
	)
	if err != nil {
		return func(context.Context) error { return nil }, err
	}

	res, _ := getResource(cfg)
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logsExporter,
			sdklog.WithExportInterval(2*time.Second))),
		sdklog.WithResource(res),
	)

	logglobal.SetLoggerProvider(lp)
	slog.SetDefault(otelslog.NewLogger(cfg.ServiceName, otelslog.WithLoggerProvider(lp)))

	return lp.Shutdown, nil
}
