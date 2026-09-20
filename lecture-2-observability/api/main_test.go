package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func testHandler(t *testing.T) (*app, http.Handler, *tracetest.SpanRecorder, *bytes.Buffer) {
	t.Helper()
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans), sdktrace.WithSampler(sdktrace.AlwaysSample()))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(t.Context())
	})
	var logs bytes.Buffer
	a, handler := newApp(slog.New(slog.NewJSONHandler(&logs, nil)))
	return a, handler, spans, &logs
}

func TestFailCorrelatesMetricLogAndTrace(t *testing.T) {
	_, handler, spans, logs := testHandler(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/fail", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &entry); err != nil {
		t.Fatal(err)
	}
	ended := spans.Ended()
	if len(ended) != 1 || ended[0].Status().Code != codes.Error {
		t.Fatalf("ended spans = %d, want one failed server span", len(ended))
	}
	if entry["trace_id"] != ended[0].SpanContext().TraceID().String() {
		t.Fatalf("log trace_id %v does not match span", entry["trace_id"])
	}
	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(metrics.Body.String(), `http_errors_total{code="503",method="GET",path="/fail"} 1`) {
		t.Fatalf("missing /fail error counter in metrics: %s", metrics.Body.String())
	}
}

func TestSlowHasChildSpan(t *testing.T) {
	_, handler, spans, _ := testHandler(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/slow?seconds=1", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	ended := spans.Ended()
	if len(ended) != 2 {
		t.Fatalf("ended spans = %d, want server and slow-op spans", len(ended))
	}
	var child, parent sdktrace.ReadOnlySpan
	for _, span := range ended {
		if span.Name() == "slow-op" {
			child = span
		} else {
			parent = span
		}
	}
	if child == nil || parent == nil || child.Parent().SpanID() != parent.SpanContext().SpanID() {
		t.Fatal("slow-op is not nested under the request span")
	}
}

func TestLoadGeneratesFailingRequests(t *testing.T) {
	a, handler, _, logs := testHandler(t)
	a.client.Transport = otelhttp.NewTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Result(), nil
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/load?count=5&workers=2&target=fail", nil))
	var result struct {
		Requested int   `json:"requested"`
		Failed    int64 `json:"failed"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Requested != 5 || result.Failed != 5 {
		t.Fatalf("load result = %+v, want 5 failed requests", result)
	}
	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(metrics.Body.String(), `http_errors_total{code="503",method="GET",path="/fail"} 5`) {
		t.Fatal("five /fail requests were not counted")
	}
	lineCount := 0
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		lineCount++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if lineCount != 6 {
		t.Fatalf("got %d request logs, want 6", lineCount)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
