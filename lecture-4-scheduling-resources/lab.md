# Lab 4 — SLA under overload

## Situation

`shop` has a critical path — placing an order (`api → postgres`) — and secondary load that can be pushed aside in a tight moment. You need to set up placement and resources so that under cluster overload the order keeps working and the secondary load is evicted. And prove it with load.

The services are simple, generated. The learning part is the scheduler, resources, and preemption.

## Part 0 — Your services

Write (may be generated, not graded) two services:

- `api` — HTTP: `GET /health` → `ok`; `POST /order` → writes an order to PostgreSQL; `GET /orders` → reads;
- `batch` — background load: in an infinite loop it takes CPU and memory, does nothing useful.

Take PostgreSQL the standard way. Drive order load with `k6` or `hey`.

## Part 1 — Deploy a tight cluster

Bring up kind on 2–3 nodes and deploy `shop` (several `api` replicas + PostgreSQL) and `batch` via manifests or Helm. Keep the cluster small, so it is easy to create a resource shortage.

## Part 2 — Spread the critical service

Choose a spreading mechanism — `podAntiAffinity` or `topologySpreadConstraints` — and justify it. Set it up so that `api` replicas land on different nodes, and prove it (`kubectl get pods -o wide`). Describe why this is needed.

## Part 3 — Set priorities and guarantees

Assign classes: to the critical path (`api`, PostgreSQL) — Guaranteed-level resources, a high priorityClass, and a PodDisruptionBudget; to `batch` — BestEffort and a low priorityClass. Describe what each setting gives under pressure.

## Part 4 — Create a shortage

Inflate `batch` (replica count or requests) until the cluster overflows. Show and explain: pods in Pending — who did not fit and why; preemption — how the order evicts `batch` to get room.

## Part 5 — Apply memory pressure

Make `batch` actively eat memory and observe eviction. Prove the order: `batch` leaves first, the critical path stays. Explain in the README the difference between this case and `OOMKilled` by its own limit.

## Part 6 — Prove the SLA under load

While the fight for resources is on (parts 4–5), run `k6` on the order path and record latency and errors. State your SLA (for example, "95% of orders faster than 300 ms even under overload") and show on the graphs that it holds and that `batch` is what degrades.

## Part 7 — Monitoring

Point the monitoring from lab 2 at pod resources and state. Choose 3 metrics for alerts — the ones that warn first about a risk to the order — and explain the choice.

## What to submit

- Code of the `api` and `batch` services with a Dockerfile (generated code is not graded).
- `README.md`: the chosen spreading mechanism and why; proof of replica placement (part 2); priority/QoS/PDB settings (part 3); what you observed in Pending, preemption, and eviction with an explanation (parts 4–5); the SLA and graphs that it holds (part 6); the dashboard and justification of the three metrics.
- Configs: manifests/Helm, priorityClass, PDB, resource settings.
- Screenshots: replicas on different nodes; pods in Pending; the moment of preemption; the moment of eviction (`batch` evicted, order alive); order load graphs; the dashboard.

## How to start

Open the repository with an AI assistant and ask for help with Lab 4. The assistant guides you step by step and checks understanding, it does not hand out a finished solution. Generate the services (Part 0), then go in order.

If your laptop is short on resources — team up and spread the nodes across several laptops on the local network.

> Using AI? Check that the assistant follows the rules from [`AGENTS.md`](../AGENTS.md). Most tools pick it up on their own; if not — point it to this file.
