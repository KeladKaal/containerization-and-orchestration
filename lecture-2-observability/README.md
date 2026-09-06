# Lecture 2 — Observability and monitoring

You can set up containers, the cluster, and networking perfectly, but if you can't see what happens inside, you learn about problems from angry users instead of your own graphs. This lecture answers how "sight" works in Kubernetes: where metrics come from, how to collect logs from all pods, how to trace one request across many services, and — most importantly — what to watch and what to page on. Monitoring comes early in the course — right after Docker: it's mandatory in every later lab, so it runs as a cross-cutting theme. The running example stays the same — the online shop `shop` with services `api` (×3), `worker`, and `postgres`.

---

## Block 1. Metrics

### Monitoring vs observability

Two words that get confused. Monitoring: we pick known indicators in advance and set alarms ("CPU above 90% — send an alert"). It answers "is everything OK?" and is good for problems we foresaw. Observability: the broader ability to understand what happens inside a system from the signals it emits, even when the problem is new and unforeseen. It answers "why did this particular thing break?". Monitoring says "the service is unhealthy"; good observability lets you find out why without adding new code for every case. A real system needs both.

### The three pillars

- Metrics — numbers over time (requests per second, latency, memory). Answer WHAT happens and when. Cheap and compact — good for trends and alerts.
- Logs — text records of application events ("could not connect to the database"). Answer WHY.
- Traces — the path of one request across many services. Answer WHERE the bottleneck is.

Their power is in the combination. Typical investigation: a metric shows error rate rising in `api` (what) → a trace shows requests stalling on the call to `postgres` (where) → the `postgres` log explains it ran out of connections (why).

### What we want to see in shop

Metrics — load and health of each service and node; logs — what the services write, which errors; cluster events — why a pod didn't start, who got evicted, what restarted; traces — where time is lost when an order is slow.

### What a metric is

A metric is a numeric value measured over time, e.g. "requests per second to `api`": 120 at 10:00, 350 at 13:00. Technically this sequence of "time → value" pairs is a time series. A metric has labels — qualifiers in braces: `http_requests_total{service="api", code="500"}` is the counter of requests to `api` with response code 500. Labels let you slice one metric by service, code, node. Because metrics are cheap and compact (just numbers), you store them for a long time, see week-long trends, and hang alerts on them.

### Prometheus and the pull model

Prometheus is the de facto standard for metrics in Kubernetes. It stores time series and queries them. Its collection model is pull: Prometheus itself periodically (say every 15 s) goes to each target and fetches its metrics. Each target — an app or component — simply exposes an HTTP endpoint (by convention `/metrics`) and serves its current metrics as plain text. Prometheus reads that endpoint — this act is called a scrape. In Kubernetes it also discovers what to scrape via service discovery. The opposite is push (services send metrics themselves). Pull is convenient because Prometheus immediately sees when a target fails to answer (it's down); with push, a silent service is easily confused with a healthy one that simply sends nothing.

### Where metrics come from in Kubernetes

Several sources supply metrics — don't confuse who does what. (`kubelet` is the Kubernetes agent on each node that runs and holds pods.)

- cAdvisor — built into `kubelet`; container metrics: CPU, memory, network per container. This is what `kubectl top pod` shows.
- node-exporter — metrics of the node as a machine: host CPU, free memory, disk usage, network. Health of the hardware under the cluster.
- kube-state-metrics — state of Kubernetes objects: how many pods are Running vs Pending, how many restarts, how many replicas a Deployment wants vs has. The main source for whether the cluster-as-orchestrator is healthy.
- control plane metrics (API server, scheduler, etcd) and `kubelet`.
- application metrics — what your service exposes on `/metrics`, including business metrics like orders placed.

Cheat sheet: consumption of a specific pod — cAdvisor; node health and disk space — node-exporter; "how many pods are Pending, who's restarting" — kube-state-metrics; business metrics — application metrics.

### A bit of PromQL

PromQL is the query language for Prometheus metrics. Three ideas:

- Select a series by name and labels: `http_requests_total{service="api"}`.
- `rate(...[5m])` — rate of growth of a counter. Many metrics are counters that only grow from service start and never reset; the raw "5M requests total" tells you nothing. What matters is the rate — requests per second now. `rate` turns "total accumulated" into "per second", and `[5m]` is a smoothing window over the last 5 minutes.
- Aggregation: `sum by (code) (rate(http_requests_total[5m]))` — sum separately per value of the `code` label, i.e. the rate for 200s and for 500s separately.

Together they give the key health metric — error rate: rate of 5xx requests divided by rate of all requests.

---

## Block 2. Logs and events

### The log's path: why stdout, not a file

Container rule: the app writes logs to standard output (`stdout`/`stderr`), not to files inside the container. Reason — the container filesystem is ephemeral (Lecture 1): write a log to a file inside and the container is recreated, taking the log with it. So: the app writes to `stdout` → the node's runtime (here `containerd`) captures it into a file on the node → `kubectl logs` reads that file. That's why `kubectl logs` works and shows only the current pod. But logs on the node aren't eternal either: they rotate (old ones overwritten so the disk doesn't fill), and if the node dies you lose access. So logs must be collected from nodes into a separate, centralized store.

### Centralized log collection

You need an agent that collects logs from all nodes into one place. On each node runs a collection agent — as a DaemonSet, a useful object type. Where a Deployment runs some number of replicas anywhere, a DaemonSet guarantees exactly one pod per node — ideal for agents that must be on every machine. The agent reads the log files of all pods on its node and ships them to a central store. Common agents: Fluent Bit, Vector, Promtail; common stores: Loki, Elasticsearch. Result: logs of all pods from all nodes flow into one searchable place, readable even after a pod or whole node dies. Two tools: `kubectl logs` for "look at a live pod right now", centralized collection for history, search, and after-the-fact investigations. In production you need the second.

### Structured logs

Two ways to write a log. Plain text: "2026-05-01 db connection error for order 123" — readable to a human, hard for a machine to filter (parsing regexes, brittle). Structured (usually JSON): `{level:"error", msg:"db connection failed", order_id:123}` — same information as fields. The log store can search and aggregate by field: "show all `error` for `order_id=123`", "count errors per endpoint". In large systems structured logs are nearly mandatory — searching by field is far more powerful than by eye. Agree on this with developers early.

### Kubernetes events are not logs

Events are a separate signal: not what your app writes, but messages from the cluster itself about what it does to your objects. Examples: `FailedScheduling` (pod didn't fit — the Pending state), `OOMKilled` (terminated for exceeding memory), `BackOff` (restarting in a loop — the inside of `CrashLoopBackOff`), `Pulling`/`Pulled` (fetching the image), `Evicted` (evicted under pressure). View with `kubectl describe pod <name>` (Events section at the bottom, for that pod — the first place to look when a pod won't start) or `kubectl get events` for the whole namespace. Key detail: events are short-lived — kept about an hour, then deleted. For after-the-fact investigation, collect them centrally too. Keep the distinction: logs = what the APPLICATION says; events = what the CLUSTER does to it. Investigate both.

### Metrics + logs + events together

On a real incident:

1. Metric: `api`'s 5xx error rate spikes — WHAT and when, but not why.
2. Events: pod `postgres` shows `OOMKilled` and `BackOff` — the cluster says the DB is restarting for lack of memory.
3. Logs: `api` logs "timeout connecting to postgres", `postgres` logs "out of memory" — WHY.

Conclusion: `postgres` hit its memory limit → restarts → `api` can't connect → errors for clients. Fix the `postgres` memory (limit or leak). Diagnosis in three steps, each pillar adding its piece.

---

## Block 3. Traces, alerts, eBPF

### Why traces

One client request crosses many services (`api → worker → postgres → ...`). The client says "slow" — but where exactly? Metrics show "slow on average", not for a specific request. Logs of each service show fragments, but reassembling one request's path by hand is nearly impossible at scale. Distributed tracing solves this: it shows one specific request's path across the whole chain and how much time it spent at each step. Literally visible: `api` — 5 ms, `worker` — 10 ms, the `postgres` call — 800 ms — there's the culprit.

### How a trace works: trace and span

- span — one unit of work (e.g. "processing in `api`" or "query to `postgres`"): start, end, duration.
- trace — the whole path of one request, a tree of linked spans.

How they link into one tree across different services and nodes: the first request is assigned a unique trace-id, which is passed down the chain, usually in HTTP headers — this is context propagation. Each service sees the trace-id, adds its spans under the same id, and forwards it; the tracing system assembles all spans with one trace-id into a tree. For this to work, apps must forward the headers — which requires instrumenting the code: adding a library that creates spans around operations and propagates the trace-id. Usually not rewriting logic, just a library plus a few lines of config. A service mesh (covered later in the course) helps partially — it sees inter-service calls and can add basic spans at service boundaries with no code — but detail inside a service (which functions, which DB queries) comes only from instrumenting the code; the mesh doesn't look inside a service.

### OpenTelemetry and Jaeger

- OpenTelemetry (OTel) — an open standard and set of libraries for observability in general: a uniform way to collect and transmit metrics, logs, traces. It ended the zoo of formats where switching monitoring systems meant re-instrumenting every service. Instrument once to the standard, then send to any compatible system.
- Jaeger — a popular system for storing and viewing traces. It shows the span tree with durations: open a slow request and see where the time went.

In practice: instrument with OpenTelemetry, traces go to Jaeger, metrics to Prometheus, logs to Loki — all linked.

### Golden signals: what to watch

You can gather thousands of metrics and drown. What first? The proven answer from Google's SRE (Site Reliability Engineering) team — four golden signals:

- Latency — how long a request takes (separately for successful and failed; fast errors can mask slow success).
- Traffic — requests per second.
- Errors — fraction of failed requests.
- Saturation — how full your resources are (memory, CPU, disk — how close to the limit).

Starting observability for a new service, begin with exactly these four and add specifics later.

### Alerts: page on symptoms, not causes

Main principle: page a human on what actually hurts the user (a symptom), not on every internal cause.

- Good: "`api` error rate > 5% for five minutes", "order latency > 2 s" — the user is hurting now; worth a night call.
- Bad: "node CPU 90%" on its own — maybe the service runs fine at 90%; you wake someone for nothing. Cause metrics like CPU help investigation but are a path to burnout as page triggers.

A good threshold reference is an SLO (service level objective), e.g. "99.9% of requests succeed": alert when you risk breaching it — a real threat to the user, not an abstract number.

### Alert fatigue

Alert fatigue — so many alerts, mostly false, that people stop reacting, mute notifications, and miss the real incident in the noise. Paradox: too much monitoring makes a system less observable because signals lose trust. How to fight it:

- Every alert must require action; if you do nothing on it, delete it.
- Fewer alerts, but meaningful; group related ones so one incident doesn't flood you; fix or remove noisy ones.
- Every alert should carry a clear "what to do", at least a runbook link.

Good observability is not "more alerts", it's "you're paged only when it truly matters".

### eBPF in observability

eBPF — a mechanism that safely runs small programs inside the Linux kernel — gives observability without changing application code.

- Kernel programs see syscalls, network connections, latencies — across all processes on a node at once.
- You can get metrics and often traces automatically, without instrumenting each service by hand.
- Tools: Pixie, Cilium/Hubble (network), Parca (profiling — what a process spends CPU and memory on, down to specific functions).

eBPF doesn't fully replace application observability — business metrics like orders placed are known only to the app; the kernel knows nothing of them. But it drastically lowers the barrier: instead of instrumenting every service for basic visibility, it's largely "turn it on and see".

---

## Summary

- Three pillars: metrics (what), logs (why), traces (where) — strong in combination.
- Metrics: Prometheus pulls (scrapes `/metrics`, discovers targets); sources — cAdvisor (containers), node-exporter (node), kube-state-metrics (k8s objects), control plane, application; PromQL: `rate()` for counters and error rate.
- Logs to `stdout` (ephemeral container FS) → node runtime → centralized collection by a DaemonSet agent into Loki/Elasticsearch; structured (JSON) logs are searchable by field.
- Kubernetes events ≠ logs: what the CLUSTER does (`OOMKilled`, `FailedScheduling`); short-lived, collect them too.
- Traces: spans under a shared trace-id, context propagation; standard — OpenTelemetry, viewer — Jaeger.
- Practice: golden signals (latency, traffic, errors, saturation); alert on symptoms and SLO risk, not causes; fight alert fatigue; eBPF lowers the barrier.

Next we move up into orchestration — how Kubernetes runs all this, starting with the control plane.

> Lab: set up metrics, logs and traces for your service and configure 3 alerts — see [lab.md](lab.md).
