package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"shop/internal/health"
	"shop/internal/store"
)

const maxOrderSize = 1 << 20 // 1 MiB

type api struct {
	store *store.Store
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	app := &api{store: db}
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "shop",
		Subsystem: "api",
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests handled by the shop API.",
	}, []string{"method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "shop",
		Subsystem: "api",
		Name:      "http_request_duration_seconds",
		Help:      "Duration of HTTP requests handled by the shop API.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route"})
	prometheus.MustRegister(requests, duration)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health.Handler)
	mux.HandleFunc("POST /order", app.createOrder)
	mux.HandleFunc("GET /orders", app.listOrders)
	mux.Handle("GET /metrics", promhttp.Handler())

	address := envOrDefault("HTTP_ADDR", ":8080")
	server := &http.Server{
		Addr:              address,
		Handler:           instrumentHTTP(mux, requests, duration),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("api listening", "address", address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("api server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("api shutdown failed", "error", err)
	}
}

func instrumentHTTP(next http.Handler, requests *prometheus.CounterVec, duration *prometheus.HistogramVec) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		route := metricRoute(r.URL.Path)
		requests.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
		duration.WithLabelValues(r.Method, route).Observe(time.Since(started).Seconds())
	})
}

func metricRoute(path string) string {
	switch path {
	case "/health", "/order", "/orders", "/metrics":
		return path
	default:
		return "unmatched"
	}
}

func (a *api) createOrder(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxOrderSize))
	if err != nil {
		writeError(w, http.StatusBadRequest, "request body is too large or unreadable")
		return
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}

	order, err := a.store.CreateOrder(r.Context(), body)
	if err != nil {
		slog.Error("create order failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not create order")
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (a *api) listOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := a.store.ListOrders(r.Context())
	if err != nil {
		slog.Error("list orders failed", "error", err)
		writeError(w, http.StatusInternalServerError, "could not list orders")
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("encode response failed", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
