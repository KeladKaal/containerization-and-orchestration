package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.opentelemetry.io/otel/trace"
)

// tracerName identifies the instrumentation scope of the spans we create by
// hand (as opposed to the ones otelhttp creates for us).
const tracerName = "github.com/KeladKaal/containerization-and-orchestration/lab2/api"

// initTracing wires up the global TracerProvider and returns a shutdown func.
//
// Two things happen here that matter for the lab:
//
//  1. The OTLP exporter ships finished spans to Jaeger's collector over gRPC.
//     Jaeger all-in-one accepts OTLP natively on 4317, so no separate agent or
//     OpenTelemetry Collector is required.
//  2. The propagator is set to W3C tracecontext + baggage. That is what lets a
//     trace continue across a service hop: /load calls this same service over
//     HTTP, and because otelhttp's transport injects the traceparent header,
//     those inner requests show up as children of the /load span rather than as
//     unrelated traces.
//
// If cfg.OTLPEndpoint is empty we install a no-op-ish provider: spans are still
// created (so trace_id exists in logs) but nothing is exported.
func initTracing(ctx context.Context, cfg config) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		// AlwaysSample keeps the lab predictable: every /slow and /fail you
		// click is guaranteed to be in Jaeger. Production would sample.
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}

	if cfg.OTLPEndpoint != "" {
		expOpts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(stripScheme(cfg.OTLPEndpoint)),
		}
		if cfg.OTLPInsecure {
			expOpts = append(expOpts, otlptracegrpc.WithInsecure())
		}
		exp, err := otlptracegrpc.New(ctx, expOpts...)
		if err != nil {
			return nil, fmt.Errorf("create otlp exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// stripScheme lets OTEL_EXPORTER_OTLP_ENDPOINT be given either as a bare
// host:port (what the gRPC exporter wants) or as a URL, which is how the
// variable is usually documented.
func stripScheme(endpoint string) string {
	for _, p := range []string{"http://", "https://"} {
		if len(endpoint) > len(p) && endpoint[:len(p)] == p {
			return endpoint[len(p):]
		}
	}
	return endpoint
}

// tracer returns the tracer used for hand-made spans.
func tracer() trace.Tracer { return otel.Tracer(tracerName) }

// attrString is a tiny helper to keep handler code readable.
func attrString(k, v string) attribute.KeyValue { return attribute.String(k, v) }
