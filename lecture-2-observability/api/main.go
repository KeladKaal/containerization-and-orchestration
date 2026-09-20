// The API is test infrastructure for Lab 2; the lab focuses on observability.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type app struct {
	logger   *slog.Logger
	requests *prometheus.CounterVec
	errors   *prometheus.CounterVec
	duration *prometheus.HistogramVec
	client   *http.Client
	tracer   trace.Tracer
}

func newApp(logger *slog.Logger) (*app, http.Handler) {
	registry := prometheus.NewRegistry()
	a := &app{
		logger: logger,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total", Help: "Completed API requests.",
		}, []string{"method", "path", "code"}),
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_errors_total", Help: "Completed API requests with a 5xx response.",
		}, []string{"method", "path", "code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "http_request_duration_seconds", Help: "API request latency in seconds.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 10},
		}, []string{"method", "path", "code"}),
		client: &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport), Timeout: 10 * time.Second},
		tracer: otel.Tracer("lab2-api"),
	}
	registry.MustRegister(a.requests, a.errors, a.duration)

	mux := http.NewServeMux()
	mux.Handle("GET /health", a.instrument("/health", http.HandlerFunc(a.health)))
	mux.Handle("GET /fail", a.instrument("/fail", http.HandlerFunc(a.fail)))
	mux.Handle("GET /slow", a.instrument("/slow", http.HandlerFunc(a.slow)))
	mux.Handle("GET /load", a.instrument("/load", http.HandlerFunc(a.load)))
	// Scrapes do not count as user traffic or create traces.
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	return a, mux
}

func (a *app) instrument(path string, next http.Handler) http.Handler {
	measured := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		code := strconv.Itoa(recorder.status)
		labels := []string{r.Method, path, code}
		a.requests.WithLabelValues(labels...).Inc()
		a.duration.WithLabelValues(labels...).Observe(time.Since(start).Seconds())
		level := slog.LevelInfo
		message := "request completed"
		if recorder.status >= http.StatusInternalServerError {
			a.errors.WithLabelValues(labels...).Inc()
			level = slog.LevelError
			message = "request failed"
		}
		a.logger.LogAttrs(r.Context(), level, message,
			slog.String("method", r.Method),
			slog.String("path", path),
			slog.Int("status", recorder.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("trace_id", trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()),
		)
	})
	return otelhttp.NewHandler(measured, "GET "+path, otelhttp.WithSpanNameFormatter(
		func(_ string, r *http.Request) string { return r.Method + " " + path },
	))
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusRecorder) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *statusRecorder) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (a *app) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n")
}

func (a *app) fail(w http.ResponseWriter, r *http.Request) {
	err := errors.New("intentional test failure")
	span := trace.SpanFromContext(r.Context())
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	http.Error(w, err.Error(), http.StatusServiceUnavailable)
}

func (a *app) slow(w http.ResponseWriter, r *http.Request) {
	seconds := 2
	if raw := r.URL.Query().Get("seconds"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 3 {
			http.Error(w, "seconds must be 1, 2 or 3", http.StatusBadRequest)
			return
		}
		seconds = parsed
	}
	ctx, span := a.tracer.Start(r.Context(), "slow-op", trace.WithAttributes(attribute.Int("sleep.seconds", seconds)))
	defer span.End()
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		_, _ = fmt.Fprintf(w, "slept %d seconds\n", seconds)
	case <-ctx.Done():
		span.RecordError(ctx.Err())
		span.SetStatus(codes.Error, ctx.Err().Error())
	}
}

func (a *app) load(w http.ResponseWriter, r *http.Request) {
	count, err := boundedInt(r, "count", 100, 1, 500)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	workers, err := boundedInt(r, "workers", 10, 1, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	target := r.URL.Query().Get("target")
	if target == "" {
		target = "health"
	}
	if target != "health" && target != "fail" && target != "slow" {
		http.Error(w, "target must be health, fail or slow", http.StatusBadRequest)
		return
	}

	var completed atomic.Int64
	var failed atomic.Int64
	jobs := make(chan struct{})
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for range jobs {
				request, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
					"http://127.0.0.1:"+listenPort()+"/"+target, nil)
				if err == nil {
					var response *http.Response
					response, err = a.client.Do(request)
					if err == nil {
						_, _ = io.Copy(io.Discard, response.Body)
						_ = response.Body.Close()
						if response.StatusCode >= http.StatusInternalServerError {
							failed.Add(1)
						} else {
							completed.Add(1)
						}
					}
				}
				if err != nil {
					failed.Add(1)
					a.logger.ErrorContext(r.Context(), "load request error",
						"target", target, "error", err,
						"trace_id", trace.SpanFromContext(r.Context()).SpanContext().TraceID().String())
				}
			}
		}()
	}
	for range count {
		select {
		case jobs <- struct{}{}:
		case <-r.Context().Done():
			close(jobs)
			group.Wait()
			return
		}
	}
	close(jobs)
	group.Wait()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"target": target, "requested": count,
		"successful": completed.Load(), "failed": failed.Load(),
	})
}

func boundedInt(r *http.Request, key string, fallback, min, max int) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < min || parsed > max {
		return 0, fmt.Errorf("%s must be between %d and %d", key, min, max)
	}
	return parsed, nil
}

func listenPort() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return "8080"
}

func initTracing(ctx context.Context) (*sdktrace.TracerProvider, error) {
	options := []otlptracehttp.Option{}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		options = append(options, otlptracehttp.WithEndpointURL("http://localhost:4318/v1/traces"))
	}
	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, err
	}
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "api"
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", serviceName))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return provider, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	provider, err := initTracing(ctx)
	if err != nil {
		logger.Error("tracing initialization failed", "error", err, "trace_id", "")
		os.Exit(1)
	}
	_, handler := newApp(logger)
	server := &http.Server{Addr: ":" + listenPort(), Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	logger.Info("api listening", "port", listenPort(), "trace_id", "")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server failed", "error", err, "trace_id", "")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := provider.Shutdown(shutdownCtx); err != nil {
		logger.Error("tracing shutdown failed", "error", err, "trace_id", "")
	}
}
