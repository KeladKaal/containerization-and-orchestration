# Lecture 5 — Scheduling, resources and storage

How the scheduler decides where a Pod runs, what happens to resources under pressure, and how state is handled. This is where cgroups from lecture 1 meet Kubernetes.

## Blocks

1. **The scheduler from the inside** — the filter and scoring cycle; affinity/anti-affinity, taints/tolerations, topology spread; preemption.
2. **Resources and pressure** — requests/limits and node cgroups; QoS classes; CPU throttling; OOM and eviction under memory/disk pressure.
3. **State** — CSI and volumes; PV/PVC/StorageClass; StatefulSet and stable identity; why running a database in Kubernetes is a deliberate decision.

> Self-study notes and the lab will be added later.
