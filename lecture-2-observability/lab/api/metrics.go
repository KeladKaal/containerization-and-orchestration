package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metrics is the RED instrumentation: Rate, Errors, Duration.
//
// Note there is no separate "rate" metric. In Prometheus you do not store a
// rate, you store a monotonically increasing counter and take rate() of it at
// query time. requestsTotal is that counter; the request rate and the error
// rate are both derived from it in PromQL, which is why the status code has to
// be a label rather than a second counter.
type metrics struct {
	registry *prometheus.Registry

	// requestsTotal counts every finished request, labelled so that both
	// throughput and error ratio can be computed from this one series family.
	requestsTotal *prometheus.CounterVec
	// errorsTotal counts 5xx responses only. Strictly redundant with
	// requestsTotal{status=~"5.."}, but kept because the lab asks for an
	// explicit error counter and it makes the alert rules easier to read.
	errorsTotal *prometheus.CounterVec
	// duration is the histogram behind the p95 panel and the latency alert.
	duration *prometheus.HistogramVec
	// inFlight shows concurrency, which is what makes /load visually obvious.
	inFlight prometheus.Gauge
}

func newMetrics() *metrics {
	reg := prometheus.NewRegistry()

	m := &metrics{
		registry: reg,
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests handled, by route and status.",
		}, []string{"method", "route", "status"}),
		errorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_request_errors_total",
			Help: "Total number of HTTP requests that resulted in a 5xx response.",
		}, []string{"method", "route"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request latency in seconds.",
			// Buckets must straddle the values you care about, because
			// histogram_quantile interpolates *within* a bucket. /slow sleeps
			// 1-3s, so there are explicit boundaries across that range --
			// with the client_golang defaults (top bucket 10s) every slow
			// request would land in the same bucket and p95 would be a
			// meaningless straight line.
			Buckets: []float64{
				0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5,
				1, 1.5, 2, 2.5, 3, 4, 5, 10,
			},
		}, []string{"method", "route", "status"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being served.",
		}),
	}

	reg.MustRegister(
		m.requestsTotal,
		m.errorsTotal,
		m.duration,
		m.inFlight,
		// Go runtime and process collectors: useful on the dashboard and free.
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// handler serves the /metrics endpoint from our own registry.
//
// A private registry rather than prometheus.DefaultRegisterer keeps the
// exposition surface exactly what we registered above -- no stray metrics from
// a transitively imported library.
func (m *metrics) handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		Registry: m.registry,
	})
}

// observe records one finished request.
func (m *metrics) observe(method, route string, status int, d time.Duration) {
	code := strconv.Itoa(status)
	m.requestsTotal.WithLabelValues(method, route, code).Inc()
	m.duration.WithLabelValues(method, route, code).Observe(d.Seconds())
	if status >= 500 {
		m.errorsTotal.WithLabelValues(method, route).Inc()
	}
}

// statusRecorder captures the status code so the middleware can label metrics
// with it. net/http gives the middleware no way to read back what the handler
// wrote, so we have to intercept WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	// A handler that writes without calling WriteHeader implies 200.
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer, so
// streaming and deadline control keep working through the middleware.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
