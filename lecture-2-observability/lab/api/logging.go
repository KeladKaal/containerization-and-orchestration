package main

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// traceHandler is a slog.Handler that copies the active span's ids onto every
// log record.
//
// This is the join key of the whole lab. Loki stores the log line, Jaeger
// stores the span; neither knows about the other. The only thing linking them
// is that the log line carries the same trace_id the span was recorded under,
// so you can copy it out of Grafana and paste it into Jaeger (or wire a
// derived field in the Loki datasource to make it a link).
//
// Doing it in a Handler rather than at each call site means you cannot forget
// it: any slog call that carries the request context gets the ids for free.
type traceHandler struct{ slog.Handler }

func (h traceHandler) Handle(ctx context.Context, rec slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		rec.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, rec)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}

// newLogger builds the structured JSON logger written to stdout.
//
// stdout is deliberate: in Kubernetes the container runtime writes stdout to a
// file on the node, and that file is what the collector agent (Alloy /
// Promtail / Fluent Bit) tails and ships to Loki. A service that logs to its
// own file inside the container is invisible to that pipeline.
func newLogger(cfg config) *slog.Logger {
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(traceHandler{base}).With(
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.ServiceVersion),
	)
}
