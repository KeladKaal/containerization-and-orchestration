package main

import (
	"os"
	"strconv"
	"time"
)

// config holds everything the service reads from the environment.
//
// Every knob has a working default so the binary runs with no environment at
// all; in the cluster the Helm chart overrides what it needs.
type config struct {
	// ServiceName is reported to Prometheus (as a label) and to Jaeger (as the
	// service name you pick in the UI dropdown).
	ServiceName string
	// ServiceVersion shows up as a resource attribute on every span.
	ServiceVersion string
	// Addr is the listen address for the public API and /metrics.
	Addr string

	// OTLPEndpoint is the collector/Jaeger OTLP gRPC address, e.g.
	// "jaeger-collector:4317". Empty means traces are disabled entirely.
	OTLPEndpoint string
	// OTLPInsecure disables TLS on the OTLP connection (normal in-cluster).
	OTLPInsecure bool

	// SlowMin/SlowMax bound the artificial delay of GET /slow.
	SlowMin time.Duration
	SlowMax time.Duration

	// LoadRequests is how many requests GET /load fires at this service,
	// LoadConcurrency how many of them are in flight at once.
	LoadRequests    int
	LoadConcurrency int
	// SelfBaseURL is the address /load calls back into. Defaults to loopback on
	// the configured port, which is what you want inside a pod.
	SelfBaseURL string
}

func loadConfig() config {
	c := config{
		ServiceName:     env("OTEL_SERVICE_NAME", "api"),
		ServiceVersion:  env("SERVICE_VERSION", "dev"),
		Addr:            env("ADDR", ":8080"),
		OTLPEndpoint:    env("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTLPInsecure:    envBool("OTEL_EXPORTER_OTLP_INSECURE", true),
		SlowMin:         envDuration("SLOW_MIN", time.Second),
		SlowMax:         envDuration("SLOW_MAX", 3*time.Second),
		LoadRequests:    envInt("LOAD_REQUESTS", 200),
		LoadConcurrency: envInt("LOAD_CONCURRENCY", 20),
	}
	c.SelfBaseURL = env("SELF_BASE_URL", "http://127.0.0.1"+portOf(c.Addr))
	return c
}

// portOf turns a listen address into the ":port" suffix used to call ourselves.
func portOf(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[i:]
		}
	}
	return ":8080"
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, err := strconv.ParseBool(env(key, "")); err == nil {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(env(key, "")); err == nil {
		return v
	}
	return def
}
