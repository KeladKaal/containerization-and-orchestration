package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type server struct {
	cfg    config
	log    *slog.Logger
	client *http.Client
}

// writeJSON is the single place that serialises a response body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// handleHealth is the liveness/readiness endpoint.
//
// Deliberately trivial and side-effect free: a health check that talks to a
// database turns a slow dependency into a mass pod restart, because the
// kubelet will kill every replica at once.
func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": s.cfg.ServiceName,
		"version": s.cfg.ServiceVersion,
	})
}

// handleFail always returns 500 and marks the span as failed.
//
// Two separate things happen for the error to be visible everywhere:
//   - RecordError attaches an exception event to the span (Jaeger shows it in
//     the span detail);
//   - SetStatus(codes.Error) sets the span status, which is what actually
//     paints the span red in the Jaeger UI.
//
// Recording the error without setting the status is a common mistake: the
// trace contains the error but the waterfall still looks green.
func (s *server) handleFail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	err := errors.New("synthetic failure triggered via /fail")
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.SetAttributes(attrString("failure.kind", "synthetic"))

	s.log.LogAttrs(ctx, slog.LevelError, "request failed deliberately",
		slog.String("route", "/fail"),
		slog.String("error", err.Error()),
	)

	writeJSON(w, http.StatusInternalServerError, map[string]string{
		"error":   "internal server error",
		"trigger": "/fail",
	})
}

// handleSlow sleeps for a random duration inside a dedicated child span.
//
// The nested span is the point of the exercise. Without it the trace shows a
// single 2-second bar and tells you nothing. With it, the waterfall shows the
// root span, and inside it a "slow-op" span covering nearly the whole
// duration, which is the shape you look for when answering "where did the time
// actually go".
func (s *server) handleSlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	spread := s.cfg.SlowMax - s.cfg.SlowMin
	delay := s.cfg.SlowMin
	if spread > 0 {
		delay += time.Duration(rand.Int63n(int64(spread)))
	}

	ctx, span := tracer().Start(ctx, "slow-op")
	span.SetAttributes(
		attrString("slow.reason", "artificial delay"),
		attrString("slow.duration", delay.String()),
	)

	select {
	case <-time.After(delay):
	case <-ctx.Done():
		// The client hung up or the server is shutting down. Mark the span so
		// the trace explains the truncation instead of just ending early.
		span.SetStatus(codes.Error, "request cancelled during slow operation")
		span.End()
		return
	}
	span.End()

	s.log.LogAttrs(ctx, slog.LevelInfo, "slow request completed",
		slog.String("route", "/slow"),
		slog.Duration("slept", delay),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"slept_ms": delay.Milliseconds(),
	})
}

// handleLoad fires a burst of requests at this same service to spike RPS.
//
// The burst runs in a background goroutine with its own context, and the
// handler returns immediately. If it ran inline, the /load request itself
// would be held open for the whole burst, and -- worse -- cancelling the curl
// would cancel the load.
//
// Because the outbound client uses otelhttp's transport, the traceparent
// header is propagated, so the generated requests appear as children of the
// burst span rather than as hundreds of orphan traces.
func (s *server) handleLoad(w http.ResponseWriter, r *http.Request) {
	total := s.cfg.LoadRequests
	concurrency := min(s.cfg.LoadConcurrency, total)

	// Link the background work to the triggering trace, then detach from the
	// request context so the burst survives the handler returning.
	burstCtx := trace.ContextWithSpanContext(
		context.Background(),
		trace.SpanContextFromContext(r.Context()),
	)

	go s.runBurst(burstCtx, total, concurrency)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":      "load started",
		"requests":    total,
		"concurrency": concurrency,
	})
}

// runBurst issues `total` requests with at most `concurrency` in flight.
func (s *server) runBurst(ctx context.Context, total, concurrency int) {
	ctx, span := tracer().Start(ctx, "load-burst")
	defer span.End()

	start := time.Now()
	var ok, failed atomic.Int64

	// Mix of targets so the burst moves every RED panel at once: mostly cheap
	// /health for raw RPS, a slice of /fail to lift the error rate, and a few
	// /slow to drag p95 up.
	targets := make([]string, 0, total)
	for i := range total {
		switch {
		case i%10 == 0:
			targets = append(targets, "/fail")
		case i%25 == 0:
			targets = append(targets, "/slow")
		default:
			targets = append(targets, "/health")
		}
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, path := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(path string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := s.fire(ctx, path); err != nil {
				failed.Add(1)
				return
			}
			ok.Add(1)
		}(path)
	}
	wg.Wait()

	span.SetAttributes(
		attrString("load.duration", time.Since(start).String()),
	)
	s.log.LogAttrs(ctx, slog.LevelInfo, "load burst finished",
		slog.Int("requests", total),
		slog.Int64("delivered", ok.Load()),
		slog.Int64("undelivered", failed.Load()),
		slog.Duration("took", time.Since(start)),
	)
}

// fire performs one request of the burst. A 5xx is not an error here -- it is
// the point of hitting /fail -- so only transport failures count.
func (s *server) fire(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.SelfBaseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Drain the body so the connection can be reused instead of being torn
	// down and re-dialled for every single request in the burst.
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
