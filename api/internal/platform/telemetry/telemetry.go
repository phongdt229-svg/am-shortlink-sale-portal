// Package telemetry cấu hình OpenTelemetry: traces (OTLP/HTTP → Collector) và metrics (Prometheus /metrics).
// Collector chết → service vẫn chạy (exporter gửi nền, lỗi chỉ log).
package telemetry

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"am-shortlink-portal/api/internal/config"
)

type Telemetry struct {
	shutdown       []func(context.Context) error
	MetricsHandler http.Handler
}

// Version gắn vào service.version (ldflags -X khi build).
var Version = "dev"

func Setup(ctx context.Context, cfg config.OTel, env string) (*Telemetry, error) {
	// NewSchemaless: tránh xung đột schema URL giữa resource.Default() và gói semconv.
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceNamespace(cfg.ServiceNamespace),
		semconv.ServiceVersion(Version),
		semconv.ServiceInstanceID(hostname()),
		semconv.DeploymentEnvironment(env),
	))
	if err != nil {
		return nil, err
	}

	t := &Telemetry{}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	if cfg.Enabled {
		exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.EndpointTraces))
		if err != nil {
			return nil, err
		}
		sampler := sdktrace.AlwaysSample()
		if cfg.Sampler == "ratio" {
			sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplerRatio))
		}
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sampler),
		)
		otel.SetTracerProvider(tp)
		t.shutdown = append(t.shutdown, tp.Shutdown)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	promExp, err := otelprom.New(otelprom.WithRegisterer(reg))
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(promExp), sdkmetric.WithResource(res))
	otel.SetMeterProvider(mp)
	t.shutdown = append(t.shutdown, mp.Shutdown)
	t.MetricsHandler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	return t, nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	for _, f := range t.shutdown {
		errs = append(errs, f(ctx))
	}
	return errors.Join(errs...)
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}
