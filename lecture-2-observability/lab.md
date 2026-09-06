# Lab 2 — Service monitoring: metrics, logs, traces

## Situation

You need to set up full monitoring of one service: metrics, logs, and traces. You view metrics and logs in Grafana, traces in Jaeger, alerts in Alertmanager. You build this stack once: from here on monitoring is mandatory in every lab, and you will reuse this same stack.

The service is simple, you can generate it. The learning part is the monitoring around it, not the code.

## Part 0 — Your own service

Write a service (you can generate it, not graded). An HTTP service `api` with endpoints so you can trigger situations yourself:

- `GET /health` — returns `ok`;
- `GET /fail` — returns a 5xx error and increments the error counter;
- `GET /slow` — responds slowly (sleeps 1–3 seconds);
- `GET /load` — makes a burst of requests to itself to spike RPS.

The service must:

- expose Prometheus metrics on `/metrics`: request counter, error counter, response time histogram (RED);
- write structured logs in JSON with the `trace_id` of the current request;
- be instrumented with OpenTelemetry for traces (see part 3);
- have a Dockerfile.

Bring up the whole stack via configuration — docker compose or Helm.

## Part 1 — Metrics (Prometheus + Grafana)

Bring up Prometheus and configure scraping of the service's `/metrics`. Bring up Grafana, add Prometheus as a data source. Build a RED dashboard: request rate, error rate, p95 response time. Hit `/load`, `/fail`, `/slow` and confirm the graphs react.

## Part 2 — Logs (Loki + Grafana)

Bring up Loki and a log collection agent, configure collection of the service's logs. Connect Loki to the same Grafana so metrics and logs are in one window. Hit `/fail` and find that error in the logs.

## Part 3 — Traces (OpenTelemetry + Jaeger)

Instrument the service with OpenTelemetry: a root span for each incoming request. In `/slow` wrap the slow operation in a nested span so the waterfall shows where time went. In `/fail` mark the span as errored. Put `trace_id` into the logs — this links the log and the trace.

Bring up Jaeger (all-in-one) and point the service's OTLP exporter at it. In the Jaeger UI find the `/slow` trace (the waterfall shows a long nested span), find the `/fail` trace (errored span), take a `trace_id` from a log in Grafana and find the same trace by it in Jaeger.

## Part 4 — Alerts (Alertmanager)

Describe alert rules in Prometheus in PromQL and connect Alertmanager. Configure a receiver (webhook, email, Slack — whatever is convenient, as long as firing is visible). Pick 3 metrics and write rules for them. Trigger firing with the buttons (`/fail`, `/load`, `/slow`) and show the alerts in the firing state. On top of Alertmanager bring up Karma — a dashboard for watching alerts — and view the firing alerts there. Explain your choice — what each alert catches and what it threatens.

## What to submit

- The `api` service code with a Dockerfile (generated code is not graded).
- Stack configs: Prometheus with alert rules, Alertmanager, Karma, Grafana, Loki, Jaeger/OTel (compose or Helm).
- `README.md`: what you brought up and how; which metrics are on the dashboard and why; the rationale for the three alerts.
- Screenshots: the RED dashboard in Grafana; logs in Grafana; a trace waterfall in Jaeger (for `/slow` and `/fail`); firing alerts in Alertmanager and Karma.

## How to start

Open the repository with an AI assistant and ask for help with Lab 2. The assistant can generate the service right away — programming is not the goal here. But the monitoring (Prometheus, Grafana, Loki, Jaeger, Alertmanager) it will walk you through step by step and check your understanding, it will not hand over a ready stack solution.

> Using AI? Check that the assistant follows the rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up on their own; if not — point it at this file.
