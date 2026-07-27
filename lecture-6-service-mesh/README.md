# Lecture 6 — Service mesh and advanced traffic

Why a mesh exists, how it's built, and what it costs. Data plane vs control plane, the sidecar model with Envoy, and the shift toward sidecarless and eBPF.

## Blocks

1. **Why a mesh** — L7 concerns pulled out of application code; data plane vs control plane; Envoy as a sidecar; traffic interception.
2. **What a mesh gives you** — mTLS and workload identity; traffic management (canary, retry, timeout, circuit breaking); observability.
3. **Cost and evolution** — the overhead of the sidecar model; ambient / sidecarless; the eBPF approach; when you don't need a mesh at all.

> Self-study notes and the lab will be added later.
