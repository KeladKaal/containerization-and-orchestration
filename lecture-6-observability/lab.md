# Lab 6 — Observability, tested by a blind incident

## Situation

It's easy to build a dashboard that "shows everything" once you already know the answer. This lab tests you honestly: you build observability over `shop`, then run a **blind test**. Someone introduces one of several breakages unknown to you, and — **without knowing which** — you must notice the problem from an alert and follow the chain metric → trace → log down to the cause. The mission is to prove your observability finds the **unknown**, not that it decorates a wall with graphs.

The lab is scaled for a modest laptop: a single-node cluster, `shop` at one replica, a minimal stack.

The test services are **fully AI-generated**. You need:

- `api` — an HTTP service: `GET /health` → `ok`; `POST /order` → writes an order to PostgreSQL; `GET /orders` → reads them; `GET /slow` → responds with a ~1.5 s delay (useful for traces and latency graphs);
- `worker` — a simple loop: reads new orders from PostgreSQL and marks them processed.

PostgreSQL is the standard image. Trace instrumentation uses the OpenTelemetry auto-library, wired in by generating from a description (almost no need to touch the code). The learning part is the observability and the investigation themselves.

## Part 0 — Choose how to assemble the stack

Choose **one approach** and justify it:

- **piece by piece** — Prometheus + Grafana + Loki (logs) + Jaeger all-in-one (traces): more control and understanding of what's made of what;
- **a ready-made bundle** — e.g. `kube-prometheus-stack` plus lightweight log and trace collection: faster to stand up.

Note your choice and why in the README.

## Part 1 — Deploy shop and the observability stack

Bring up a single-node cluster (kind/k3d), deploy the minimal `shop` **via configuration**, and the stack from Part 0. Wire up all three pillars:

- **metrics** — Prometheus scrapes `/metrics` on `api`;
- **logs** — services write to stdout, an agent collects them centrally (into Loki), so logs are available even after a pod is recreated;
- **traces** — `api` and `worker` are instrumented with OpenTelemetry, traces go to Jaeger.

Briefly explain in the README why logs are written to stdout rather than to files inside the container (ephemerality from Lecture 1).

## Part 2 — Set up the golden signals

Build a dashboard for `api` with the four **golden signals**: latency, traffic (requests per second), error rate, saturation (resource consumption). Run some load, hit `/slow`, and confirm the signals on the dashboard are live and meaningful.

## Part 3 — Find one request's trace

Run one order through the chain `api → worker → postgres` and find its **trace** in Jaeger. Show the span tree and how much time each step took (`/slow` should clearly "highlight" the slow segment). Describe how a trace is linked into one whole (shared id, propagation down the chain).

## Part 4 — Define an SLO and a symptom-based alert

Formulate an **SLO** — a reliability target from the user's point of view (e.g. "99% of order placements faster than 500 ms"). Set up an alert that fires **on the risk of breaching the SLO** (a symptom that hurts the user), not on an internal cause like "CPU 90%". In the README explain why you page on symptoms, not on every spike.

## Part 5 — The blind test (climax)

Now the blind check of observability. Ask a partner (or a script with a random choice) to introduce **one of several breakages via configuration** without telling you which. Options (make it 3–4):

- lower PostgreSQL's memory limit so it crashes under load;
- shrink the database connection pool so `api` starts getting rejections;
- slow down the `worker` so orders pile up;
- lower `api`'s CPU limit so throttling kicks in.

Your task — **without knowing what was broken**:

1. notice the problem from your own alert (not from a user complaint);
2. follow the chain: the metric showed "what" → the trace showed "where" → the log/events showed "why";
3. name the cause and fix it.

In the README describe the investigation path in detail — which signal led you to the next, toward the cause.

## Part 6 — Cut the noise (monitoring is mandatory)

Observability is not "more alerts". Go through your alerts and keep **exactly 3 that truly require action**, justify each (what it catches, what to do about it), and explain why the rest are not worth waking a human for (fighting alert fatigue). These 3 alerts are the mandatory monitoring part for this lab.

## What to submit

1. **Service code** for `api` and `worker` with a Dockerfile (generated — not graded as programming).
2. **`README.md`** — the chosen stack assembly approach and why; how the three pillars are wired (Part 1); the golden-signals dashboard (Part 2); the trace analysis (Part 3); your SLO and alert logic (Part 4); the **blind incident analysis** — which breakage was introduced, how you reached the cause via the three pillars (Part 5); the final 3 alerts with justification and a discussion of alert fatigue (Part 6).
3. **Your configs** — manifests/Helm for `shop` and the stack, alert rules.
4. **Screenshots**: the golden-signals dashboard; the trace tree with timings; the fired alert; the investigation steps of the blind incident (metric → trace → log); the final set of 3 alerts.

## How to start

Open this repository with your AI assistant and ask for help with **Lab 6**. The assistant is set up to guide you **step by step** and check your understanding — it deliberately won't hand out a ready solution. Generate the `api` and `worker` services, and work through the learning part step by step from Part 0. For the blind test (Part 5) it's convenient to work in pairs.

**If resources are tight:** keep `shop` at one replica and a minimal stack; optionally use team mode and several laptops.

> **Using AI?** Make sure your assistant follows the repository rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up automatically; if yours didn't — just point it to this file and ask it to follow it.
