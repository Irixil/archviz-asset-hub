package telemetry

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func newTestMeter(t *testing.T) (metric.Meter, *metricsdk.ManualReader) {
	t.Helper()
	reader := metricsdk.NewManualReader()
	provider := metricsdk.NewMeterProvider(metricsdk.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	return provider.Meter("damask/test"), reader
}

func collectMetricNames(t *testing.T, reader *metricsdk.ManualReader) map[string]struct{} {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	names := make(map[string]struct{})
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names[m.Name] = struct{}{}
		}
	}
	return names
}

func TestCollectMachineResourceMetrics_RegistersExpectedMetrics(t *testing.T) {
	t.Parallel()

	meter, reader := newTestMeter(t)
	ticker := time.NewTicker(time.Millisecond)
	t.Cleanup(ticker.Stop)

	go CollectMachineResourceMetrics(meter, ticker)
	time.Sleep(20 * time.Millisecond)
	ticker.Stop()

	names := collectMetricNames(t, reader)

	expected := []string{
		"mem.used", "mem.total",
		"cpu.used_pcnt",
		"go.go_routines", "go.heap_objects", "go.num_gc", "go.gc_pause",
		"fs.used", "fs.total", "fs.used_pcnt",
		"disk.read_bytes", "disk.write_bytes", "net.open_connections",
	}
	for _, name := range expected {
		if _, ok := names[name]; !ok {
			t.Errorf("metric %q not registered", name)
		}
	}
}

func TestCollectMachineResourceMetrics_MetricValuesAreNonNegative(t *testing.T) {
	t.Parallel()

	meter, reader := newTestMeter(t)
	ticker := time.NewTicker(time.Millisecond)
	t.Cleanup(ticker.Stop)

	go CollectMachineResourceMetrics(meter, ticker)
	time.Sleep(20 * time.Millisecond)
	ticker.Stop()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Gauge[float64]:
				for _, dp := range data.DataPoints {
					if dp.Value < 0 {
						t.Errorf("metric %q has negative value %v", m.Name, dp.Value)
					}
				}
			case metricdata.Gauge[int64]:
				for _, dp := range data.DataPoints {
					if dp.Value < 0 {
						t.Errorf("metric %q has negative value %v", m.Name, dp.Value)
					}
				}
			}
		}
	}
}
