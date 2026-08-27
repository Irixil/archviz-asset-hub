package telemetry

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
)

const oneMb = 1_048_576

func SetupMetrics(cfg Config) {
	if !cfg.Enabled {
		otel.SetMeterProvider(metricsdk.NewMeterProvider())
		return
	}

	meter, err := newMeter(context.Background(), cfg)
	if err != nil {
		panic(err)
	}
	go CollectMachineResourceMetrics(meter, time.NewTicker(5*time.Second))
}

// CollectMachineResourceMetrics registers resource usage metrics.
func CollectMachineResourceMetrics(meter metric.Meter, ticker *time.Ticker) {
	var cpuMu sync.Mutex
	var previousCPUTimes *cpuTimes

	for range ticker.C {
		registerMemMetrics(meter)
		registerCPUMetrics(meter, &cpuMu, &previousCPUTimes)
		registerGoRuntimeMetrics(meter)
		registerFilesystemMetrics(meter)
		registerDiskAndNetMetrics(meter)
	}
}

func registerMemMetrics(meter metric.Meter) {
	_, _ = meter.Float64ObservableGauge("mem.used",
		metric.WithDescription("Allocated memory"), metric.WithUnit("MB"),
		metric.WithFloat64Callback(func(_ context.Context, fo metric.Float64Observer) error {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fo.Observe(float64(m.Alloc) / oneMb)
			return nil
		}),
	)
	_, _ = meter.Float64ObservableGauge("mem.total",
		metric.WithDescription("Total system memory"), metric.WithUnit("MB"),
		metric.WithFloat64Callback(func(_ context.Context, fo metric.Float64Observer) error {
			v, err := totalSystemMemoryMB()
			if err != nil {
				return err
			}
			fo.Observe(v)
			return nil
		}),
	)
}

func registerCPUMetrics(meter metric.Meter, cpuMu *sync.Mutex, prev **cpuTimes) {
	_, _ = meter.Float64ObservableGauge("cpu.used_pcnt",
		metric.WithDescription("CPU usage percentage"), metric.WithUnit("%"),
		metric.WithFloat64Callback(func(_ context.Context, fo metric.Float64Observer) error {
			current, err := readCPUTimes()
			if err != nil {
				return err
			}
			cpuMu.Lock()
			defer cpuMu.Unlock()
			if *prev == nil {
				*prev = &current
				fo.Observe(0)
				return nil
			}
			totalDelta := current.total - (*prev).total
			idleDelta := current.idle - (*prev).idle
			*prev = &current
			if totalDelta == 0 {
				fo.Observe(0)
				return nil
			}
			fo.Observe(float64(totalDelta-idleDelta) / float64(totalDelta) * 100)
			return nil
		}),
	)
}

func registerGoRuntimeMetrics(meter metric.Meter) {
	_, _ = meter.Int64ObservableGauge("go.go_routines",
		metric.WithDescription("Active goroutine count"), metric.WithUnit("count"),
		metric.WithInt64Callback(func(_ context.Context, io metric.Int64Observer) error {
			io.Observe(int64(runtime.NumGoroutine()))
			return nil
		}),
	)
	_, _ = meter.Float64ObservableGauge("go.heap_objects",
		metric.WithDescription("Heap object count"), metric.WithUnit("count"),
		metric.WithFloat64Callback(func(_ context.Context, fo metric.Float64Observer) error {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fo.Observe(float64(m.HeapObjects))
			return nil
		}),
	)
	_, _ = meter.Int64ObservableGauge("go.num_gc",
		metric.WithDescription("Total GC cycles"), metric.WithUnit("count"),
		metric.WithInt64Callback(func(_ context.Context, io metric.Int64Observer) error {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			io.Observe(int64(m.NumGC))
			return nil
		}),
	)
	_, _ = meter.Float64ObservableGauge("go.gc_pause",
		metric.WithDescription("Total GC pause time"), metric.WithUnit("ns"),
		metric.WithFloat64Callback(func(_ context.Context, fo metric.Float64Observer) error {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fo.Observe(float64(m.PauseTotalNs))
			return nil
		}),
	)
}

func registerFilesystemMetrics(meter metric.Meter) {
	fsGauge := func(name, desc, unit string, pick func(filesystemStats) float64) {
		_, _ = meter.Float64ObservableGauge(name,
			metric.WithDescription(desc), metric.WithUnit(unit),
			metric.WithFloat64Callback(func(_ context.Context, fo metric.Float64Observer) error {
				stats, err := filesystemUsage("/")
				if err != nil {
					return err
				}
				fo.Observe(pick(stats))
				return nil
			}),
		)
	}
	fsGauge("fs.used", "Used filesystem space", "MB", func(s filesystemStats) float64 { return s.usedMB })
	fsGauge("fs.total", "Total filesystem space", "MB", func(s filesystemStats) float64 { return s.totalMB })
	fsGauge(
		"fs.used_pcnt",
		"Filesystem usage percentage",
		"%",
		func(s filesystemStats) float64 { return s.usedPercent },
	)
}

func registerDiskAndNetMetrics(meter metric.Meter) {
	_, _ = meter.Float64ObservableGauge("disk.read_bytes",
		metric.WithDescription("Disk bytes read"), metric.WithUnit("By"),
		metric.WithFloat64Callback(func(_ context.Context, fo metric.Float64Observer) error {
			stats, err := diskIOStats()
			if err != nil {
				return err
			}
			fo.Observe(float64(stats.readBytes))
			return nil
		}),
	)
	_, _ = meter.Float64ObservableGauge("disk.write_bytes",
		metric.WithDescription("Disk bytes written"), metric.WithUnit("By"),
		metric.WithFloat64Callback(func(_ context.Context, fo metric.Float64Observer) error {
			stats, err := diskIOStats()
			if err != nil {
				return err
			}
			fo.Observe(float64(stats.writeBytes))
			return nil
		}),
	)
	_, _ = meter.Int64ObservableGauge("net.open_connections",
		metric.WithDescription("Open TCP connection count"), metric.WithUnit("count"),
		metric.WithInt64Callback(func(_ context.Context, io metric.Int64Observer) error {
			count, err := openTCPConnectionCount()
			if err != nil {
				return err
			}
			io.Observe(int64(count))
			return nil
		}),
	)
}

type cpuTimes struct {
	idle  uint64
	total uint64
}

type filesystemStats struct {
	usedMB      float64
	totalMB     float64
	usedPercent float64
}

type diskIO struct {
	readBytes  uint64
	writeBytes uint64
}

// newMeter creates a new metric.Meter that can create any metric reporter
// you might want to use in your application.
func newMeter(ctx context.Context, cfg Config) (metric.Meter, error) {
	provider, err := newMeterProvider(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("could not create meter provider: %w", err)
	}

	return provider.Meter(cfg.ServiceName), nil
}

// newMeterProvcider initialize the application resource, connects to the
// OpenTelemetry Collector and configures the metric poller that will be used
// to collect the metrics and send them to the OpenTelemetry Collector.
func newMeterProvider(ctx context.Context, cfg Config) (metric.MeterProvider, error) {
	// Interval which the metrics will be reported to the collector
	interval := 10 * time.Second

	resource, err := getResource(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not get resource: %w", err)
	}

	collectorExporter, err := getOtelMetricsCollectorExporter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("could not get collector exporter: %w", err)
	}

	periodicReader := metricsdk.NewPeriodicReader(collectorExporter,
		metricsdk.WithInterval(interval),
	)

	provider := metricsdk.NewMeterProvider(
		metricsdk.WithResource(resource),
		metricsdk.WithReader(periodicReader),
	)

	return provider, nil
}

// getOtelMetricsCollectorExporter creates a metric exporter that relies on
// an OpenTelemetry Collector running on "localhost:4317".
func getOtelMetricsCollectorExporter(ctx context.Context, cfg Config) (metricsdk.Exporter, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(cfg.Endpoint+"/metrics"),
		otlpmetrichttp.WithHeaders(map[string]string{
			authorizationHeader: "Bearer " + cfg.Token,
		}),
		otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create metric exporter: %w", err)
	}

	return exporter, nil
}
