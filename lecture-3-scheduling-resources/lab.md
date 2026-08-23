# Lab 3 — Critical-path SLA under overload

## Situation

In `shop` there is a critical path — placing an order (`api → postgres`) — and there is secondary load, like nightly analytics, that can be pushed aside in a tough moment. Your mission in this lab is to design placement and resources so that **under cluster overload the order keeps working within set bounds, while the secondary load is sacrificed**. And not just to configure it, but to **prove it with a stress test**: show that the order of sacrifice is exactly what you intended.

There is almost no code; the test services are **generated entirely by AI**. You need two:

- `api` — an HTTP service: `GET /health` → `ok`; `POST /order` → writes an order to PostgreSQL; `GET /orders` → reads the list;
- `batch` — "background load": a container that in an infinite loop simply burns CPU and memory (analytics imitation) and does nothing useful.

PostgreSQL is a standard image. Load the order path with a standard tool (`k6` or `hey`) — no need to write it.

## Part 0 — Choose the replica-spreading mechanism

The `api` replicas must be spread across different nodes so that one node's failure does not take out the whole service at once. Pick **one approach** and justify it:

- **podAntiAffinity** — a hard/soft rule "do not place these pods together";
- **topologySpreadConstraints** — "keep the balance, distribute evenly across nodes/zones".

Note your choice and why it fits your goal in the README.

## Part 1 — Bring up a tight environment

Bring up a local cluster of **2–3 nodes** (kind can do this — nodes will be containers, all local) and deploy `shop` (`api` in several replicas + PostgreSQL) and `batch` — **via configuration** (manifests/Helm). The environment should be deliberately small so that a resource shortage is easy to create.

## Part 2 — Spread the critical service

Configure the spreading of `api` replicas by the approach chosen in Part 0 and **prove** that the replicas really landed on different nodes (`kubectl get pods -o wide`). Describe how this improves fault tolerance.

## Part 3 — Set priorities and guarantees

Set the classes so the system knows what matters:

- the critical path (`api`, PostgreSQL) — **Guaranteed** resources (requests = limits), a high **priorityClass**, and a **PodDisruptionBudget** (how many `api` replicas must not go down at once);
- `batch` — **BestEffort** (no requests/limits) and a low priorityClass.

In the README explain what each setting gives you under pressure.

## Part 4 — Create a shortage and catch preemption

Inflate the situation so that nothing fits anymore: increase the number of `batch` replicas and/or their requests until the cluster is overfull. Show and describe:

- pods in the **Pending** state — who did not fit and why (the scheduler counts by reserved requests, not by actual usage);
- **preemption** — how the high-priority order evicts the low-priority `batch` to get room.

## Part 5 — Apply memory pressure and record the order of sacrifice

Create real memory pressure on the node (for example, `batch` starts actively eating memory) and observe **eviction** — pods being evicted to save the node. Prove the order is exactly this: `batch` (BestEffort) goes first, and the critical path stays. Describe the difference between this case (node pressure) and `OOMKilled` by a container's own limit — these are different ailments.

## Part 6 — Prove the SLA under load

While all this fighting is going on (Parts 4–5), run `k6`/`hey` along the order path (`POST /order` + `GET /orders`) and capture latency and error graphs. State your SLA (for example, "95% of orders faster than 300 ms even under overload") and show on the graphs that it holds, while it is `batch` that degrades.

## Part 7 — Monitoring (required)

Set up observation of resources and pod state, and build a dashboard. Decide yourself what is important to see, then **pick 3 metrics you would build alerts on** — the ones that would warn first of a risk to the order — and explain the choice.

Tool of your choice.

## What to submit

1. **Service code** for `api` and `batch` with Dockerfiles (generated — not graded as programming).
2. **`README.md`** — the chosen spreading mechanism and why; proof of replica placement (Part 2); the priority/QoS/PDB settings and what they give (Part 3); what you observed in Pending/preemption and eviction (Parts 4–5) explained through the lecture's concepts; your SLA and graphs showing it holds under load (Part 6); the dashboard and the rationale for the 3 metrics.
3. **Your configs** — manifests/Helm, priorityClass, PDB, resource settings.
4. **Screenshots**: replicas on different nodes; pods in Pending; the moment of preemption; the moment of eviction (`batch` evicted, order alive); the order load graphs; the dashboard.

## How to start

Open this repository with your AI assistant and ask for help with **Lab 3**. The assistant is set up to guide you **step by step** and to check your understanding — it deliberately does not hand out a finished solution. Generate the `api` and `batch` services, and work through the learning part step by step starting from Part 0.

**If your laptop is short on resources:** you can **team up** and spread nodes/load across several laptops on the local network — this is also closer to real operations.

> **Using AI?** Make sure your assistant follows the repository rules from [`AGENTS.md`](../AGENTS.md). Most tools pick it up automatically; if yours did not — just point it at this file and ask it to act accordingly.
