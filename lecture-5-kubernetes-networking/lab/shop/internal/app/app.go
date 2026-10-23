// Package app is the small amount of plumbing that api, worker and batch share:
// connecting to postgres and serving the endpoints every pod must expose.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Open connects to DATABASE_URL and waits until postgres actually answers.
// A pod can easily start before an operator-managed database is accepting
// connections, so the retry loop is what keeps the container from crash-looping.
func Open() (*sql.DB, error) {
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, err
	}
	for i := 0; ; i++ {
		if err = db.Ping(); err == nil {
			return db, nil
		}
		if i == 30 {
			return nil, err
		}
		log.Printf("waiting for postgres: %v", err)
		time.Sleep(2 * time.Second)
	}
}

// requestDuration is the one metric the SLA in Part 8 is measured on: its
// _bucket series give the latency quantiles, its _count series split by code
// give the request rate and the error ratio.
var requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "Time to serve an HTTP request, by route and status code.",
	Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.5, 1, 2.5, 5},
}, []string{"method", "route", "code"})

// Counter returns a counter exposed on /metrics.
func Counter(name, help string) prometheus.Counter {
	return promauto.NewCounter(prometheus.CounterOpts{Name: name, Help: help})
}

// Gauge returns a gauge exposed on /metrics.
func Gauge(name, help string) prometheus.Gauge {
	return promauto.NewGauge(prometheus.GaugeOpts{Name: name, Help: help})
}

// Serve registers /health and /metrics on mux and blocks serving it. Every
// other route is timed into http_request_duration_seconds.
//
// /health is what the kubelet probes. HEALTH_FAIL=true makes it answer 503 on
// purpose: the switch for shipping a deliberately broken release.
func Serve(mux *http.ServeMux) error {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("HEALTH_FAIL") == "true" {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprintln(w, "ok")
	})
	mux.Handle("GET /metrics", promhttp.Handler())

	addr := ":" + Env("PORT", "8080")
	log.Printf("listening on %s", addr)
	srv := &http.Server{Addr: addr, ReadHeaderTimeout: 5 * time.Second, Handler: instrument(mux)}

	shutdownErr := make(chan error, 1)
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, os.Interrupt)
		<-sig
		log.Print("shutting down: draining in-flight requests")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr <- srv.Shutdown(ctx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return <-shutdownErr
}

// instrument times every request except the probe and scrape endpoints, which
// would otherwise drown the order path in the latency histogram.
func instrument(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := mux.Handler(r)
		if pattern == "GET /health" || pattern == "GET /metrics" {
			mux.ServeHTTP(w, r)
			return
		}
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		start := time.Now()
		mux.ServeHTTP(rec, r)
		// The mux pattern ("POST /order"), not the raw path: a scanner hitting
		// random URLs must not mint a new time series per URL.
		route := "unmatched"
		if _, path, ok := strings.Cut(pattern, " "); ok {
			route = path
		}
		requestDuration.WithLabelValues(r.Method, route, strconv.Itoa(rec.code)).Observe(time.Since(start).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

// Env reads an environment variable, falling back to def when it is unset.
func Env(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// EnvInt reads an integer environment variable, falling back to def when it
// is unset or not a number.
func EnvInt(name string, def int) int {
	v, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return def
	}
	return v
}
