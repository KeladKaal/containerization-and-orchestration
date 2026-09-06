# Lab 2 — Service monitoring: metrics, logs, traces

## Situation

You need to set up full monitoring of one service: metrics, logs, and traces. You view metrics and logs in Grafana, traces in Jaeger, alerts in Alertmanager. You build this stack once: from here on monitoring is mandatory in every lab, and you'll reuse the same stack.

The service itself can be anything — your own or generated; it doesn't affect the grade. What's graded is the monitoring around it.

## Part 0 — Your service

Write a service — your own or generated; the implementation doesn't affect the grade. You need an HTTP service `api` with endpoints to trigger situations yourself:

- `GET /health` — returns `ok`;
- `GET /fail` — returns a 5xx error and increments the error counter;
- `GET /slow` — responds slowly (sleeps 1–3 seconds);
- `GET /load` — fires a burst of requests at itself to spike RPS.

The service must:

- expose Prometheus metrics at `/metrics`: request counter, error counter, response-time histogram (RED);
- write structured JSON logs with the current request's `trace_id`;
- be instrumented with OpenTelemetry for traces (see Part 3);
- have a Dockerfile.

Work only in Kubernetes: bring up a local cluster (minikube, kind, or another installation) and deploy the whole stack via Helm. Other options (docker compose and the like) are not used in this lab.

## Part 1 — Metrics (Prometheus + Grafana)

Bring up Prometheus and configure scraping of the service's `/metrics`. Bring up Grafana and add Prometheus as a data source. Build a RED dashboard: request rate, error rate, p95 response time. Hit `/load`, `/fail`, `/slow` and confirm the graphs react.

## Part 2 — Logs (Loki + Grafana)

First understand the separation of roles: Loki is a log store — it does not pull logs from anywhere itself. A separate agent, installed on every node, reads pod output and ships it to Loki — for example Grafana Alloy, Promtail, or Fluent Bit. So you bring up both Loki and such a collector agent.

Bring up Loki and the collector agent, configure log collection for the service, and connect Loki to the same Grafana so metrics and logs are in one place. Hit `/fail` and find that error in the logs.

## Part 3 — Traces (OpenTelemetry + Jaeger)

A trace is the path of a single request through the service (and, in large systems, through the whole chain of services), assembled from segments called spans. Each span has a name, a start, an end, and a duration, and nested spans show what the total time was made of. Tracing answers the question "which step ate the time" when metrics show "something is slow" but not where exactly.

Instrument the service with OpenTelemetry: you wire in a ready library that automatically creates a root span for each incoming request. In the `/slow` handler, wrap the slow operation in a separate nested span (e.g. `slow-op`) so the waterfall shows the time went exactly there. In the `/fail` handler, mark the span as failed (status = error) — Jaeger highlights it red. Put the current span's `trace_id` into every log line: that links the log and the trace, so you can jump from a log line to its trace.

Bring up Jaeger (all-in-one is easiest) and point the service's OTLP exporter at it (the address is usually set via `OTEL_EXPORTER_OTLP_ENDPOINT`). In the Jaeger UI, select your service and find: the trace of a `/slow` request — the waterfall shows a long nested span; the `/fail` trace — with a red, failed span. Take a `trace_id` from a log in Grafana and find the same trace by it in Jaeger.

## Part 4 — Alerts (Alertmanager + Karma)

Describe alert rules in Prometheus in PromQL and connect Alertmanager to it — the component that receives fired alerts, groups them, and routes them to receivers. Configure at least one receiver (webhook, email, Slack — whatever is convenient, as long as firing is visible).

Come up with 3 **genuinely critical** alerts — the kind you'd actually have to react to on a real project, not just "a yellow graph lit up". Start from user pain and service health: e.g. error rate above a threshold for several minutes in a row; p95 response time above the acceptable level; the service not responding at all. For each, explain in the report what it catches, why it matters, and what the on-call should do about it. Trigger them with the buttons (`/fail`, `/load`, `/slow`) and show the alerts in the firing state.

On top of Alertmanager bring up Karma — a web dashboard that shows all active alerts in one place, with grouping and filters. Alertmanager's built-in UI is thin for this, and Karma is handy once there are many alerts to watch live. Look at your fired alerts in Karma.

## Result

What you should end up with — a single monitoring stack in Kubernetes, all deployed via Helm:

- the `api` service runs in the cluster: exposes `/metrics`, writes logs to stdout, is instrumented with traces;
- Prometheus scrapes `/metrics` and stores metrics;
- Loki stores logs, and a collector agent (Alloy / Promtail / Fluent Bit) on each node ships pod output to it;
- Jaeger receives traces over OTLP and shows the request waterfall;
- Grafana is connected to Prometheus and Loki: it has the RED dashboard and log search in one place;
- Alertmanager receives alerts from Prometheus and routes them to a receiver;
- Karma shows the active alerts in a separate dashboard.

Everything is wired up — you check it like this: hit `/load`, `/fail`, `/slow` → the RED dashboard graphs react; the error log shows up in Grafana; Jaeger shows a trace with a slow and a failed span; the three critical rules fire and appear in Alertmanager and Karma. A `trace_id` from a log line opens the same trace in Jaeger.

## What to submit

- The `api` service code with a Dockerfile (any implementation, doesn't affect the grade).
- Stack configs: Prometheus with alert rules, Alertmanager, Karma, Grafana, Loki and the log collector agent, Jaeger/OTel — all as Helm configuration.
- `README.md` — a report on the work done, in general form.
- Screenshots: the RED dashboard in Grafana; logs in Grafana; a trace waterfall in Jaeger (for `/slow` and `/fail`); fired alerts in Alertmanager and Karma.

## How to start

Open the repository with an AI assistant and ask for help with Lab 2. You can write the service yourself or generate it — that doesn't affect the grade; the focus of the lab is monitoring. The monitoring itself (Prometheus, Grafana, Loki, Jaeger, Alertmanager, Karma) the assistant will walk you through step by step and check your understanding; it won't hand over a ready stack solution.

> Using AI? Make sure the assistant follows the rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up automatically; if not, point it at the file.
