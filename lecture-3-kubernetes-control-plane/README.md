# Lecture 3 — Kubernetes control plane

We move up into orchestration. The declarative model and the reconciliation loop as the central idea, the control-plane components, and the full path of a request from `kubectl apply` to a Pod on a node.

## Blocks

1. **The declarative model** — desired vs actual state; the reconciliation loop as the central idea; objects, specs, statuses.
2. **Control-plane components** — API server, etcd (raft, watch, resourceVersion, optimistic concurrency), scheduler, controller-manager.
3. **The path of a request** — from `kubectl apply` to a Pod on a node; admission; kubelet and its reconcile loop; CRI/CNI/CSI on the node.

> Self-study notes and the lab will be added later.
