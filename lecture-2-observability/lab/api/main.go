package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	log := newLogger(cfg)

	// Signal handling is set up before anything else so a Ctrl-C (or the
	// kubelet's SIGTERM during a rollout) is honoured even while we are still
	// dialling the OTLP endpoint.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := initTracing(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		// Fresh context: ctx is already cancelled by the time we get here, and
		// the exporter needs a live one to flush buffered spans.
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(flushCtx); err != nil {
			log.Error("tracing shutdown", slog.String("error", err.Error()))
		}
	}()

	m := newMetrics()
	srv := &server{
		cfg: cfg,
		log: log,
		client: &http.Client{
			// otelhttp.NewTransport injects the traceparent header into every
			// outbound request, which is what keeps /load's generated traffic
			// inside the same trace.
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   10 * time.Second,
		},
	}

	mux := http.NewServeMux()

	// route registers a handler with the full observability chain around it.
	// otelhttp goes outermost so the span exists before our middleware logs.
	route := func(pattern, name string, h http.HandlerFunc) {
		var handler http.Handler = h
		handler = observability(name, m, log, handler)
		handler = otelhttp.NewHandler(handler, name)
		mux.Handle(pattern, handler)
	}

	route("GET /health", "/health", srv.handleHealth)
	route("GET /fail", "/fail", srv.handleFail)
	route("GET /slow", "/slow", srv.handleSlow)
	route("GET /load", "/load", srv.handleLoad)

	// /metrics is intentionally outside the chain: Prometheus scrapes it every
	// few seconds, and counting those scrapes as application traffic would
	// bury the real signal and generate a trace per scrape.
	mux.Handle("GET /metrics", m.handler())

	httpSrv := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
		// No global WriteTimeout: it would cut /slow off mid-response.
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api listening",
			slog.String("addr", cfg.Addr),
			slog.String("otlp_endpoint", cfg.OTLPEndpoint),
		)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received, draining")
	}

	drainCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return httpSrv.Shutdown(drainCtx)
}
