package main

import (
	"context"
	"errors"
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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	interval := durationFromEnv("POLL_INTERVAL", time.Second)
	server := startHealthServer(stop)
	processed := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "shop",
		Subsystem: "worker",
		Name:      "orders_processed_total",
		Help:      "Total number of orders marked as processed by this worker.",
	})
	processingErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "shop",
		Subsystem: "worker",
		Name:      "processing_errors_total",
		Help:      "Total number of errors while processing orders.",
	})
	prometheus.MustRegister(processed, processingErrors)

	slog.Info("worker started", "poll_interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := server.Shutdown(shutdownCtx); err != nil {
				slog.Error("health server shutdown failed", "error", err)
			}
			cancel()
			return
		case <-ticker.C:
			processAvailable(ctx, db, processed, processingErrors)
		}
	}
}

func startHealthServer(stop context.CancelFunc) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health.Handler)
	mux.Handle("GET /metrics", promhttp.Handler())
	address := envOrDefault("HTTP_ADDR", ":8080")
	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.Info("worker health endpoint listening", "address", address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("worker health server failed", "error", err)
			stop()
		}
	}()
	return server
}

func processAvailable(ctx context.Context, db *store.Store, processed prometheus.Counter, processingErrors prometheus.Counter) {
	for {
		order, found, err := db.ProcessNext(ctx)
		if err != nil {
			if ctx.Err() == nil {
				processingErrors.Inc()
				slog.Error("process order failed", "error", err)
			}
			return
		}
		if !found {
			return
		}
		processed.Inc()
		slog.Info("order processed", "order_id", order.ID)
	}
}

func durationFromEnv(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		slog.Warn("invalid duration; using default", "variable", name, "value", strconv.Quote(value), "default", fallback)
		return fallback
	}
	return duration
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
