package main

import (
	"log/slog"
	"net/http"
	"time"
)

// observability is the middleware that turns one request into all three
// signals: a metric observation, a log line, and (via otelhttp, which wraps
// this) a span.
//
// Ordering matters. otelhttp wraps this middleware from the outside, so by the
// time we run, the request context already carries the server span -- which is
// what lets the log line below pick up a trace_id.
//
// The route label is passed in explicitly rather than taken from r.URL.Path.
// Using the raw path would make every distinct URL its own metric series; with
// a path like /orders/{id} that is unbounded cardinality, the classic way to
// melt a Prometheus instance.
func observability(route string, m *metrics, log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.inFlight.Inc()
		defer m.inFlight.Dec()

		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		elapsed := time.Since(start)
		m.observe(r.Method, route, rec.status, elapsed)

		// Log at error level for 5xx so the Loki query in Part 2 can filter on
		// level rather than on string matching the message.
		level := slog.LevelInfo
		if rec.status >= 500 {
			level = slog.LevelError
		}
		log.LogAttrs(r.Context(), level, "http request",
			slog.String("method", r.Method),
			slog.String("route", route),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000),
			slog.String("user_agent", r.UserAgent()),
		)
	})
}
