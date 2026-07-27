# Lecture 7 — Extensibility and runtime security

How Kubernetes is extended, and how far container isolation really goes. Custom APIs and operators, admission control, stronger runtimes, and trust in the supply chain — the payoff of the "where the abstraction leaks" thread running through the course.

## Blocks

1. **Extending the API** — CRDs, controllers and the operator pattern; admission webhooks (validating/mutating); OPA/Gatekeeper, Kyverno.
2. **Stronger isolation** — why a shared kernel is a risk; rootless; gVisor (syscall interception), Kata (micro-VMs); when to use which.
3. **Supply chain and trust boundaries** — image signing, provenance, SBOM; multi-tenancy: where the trust boundaries run; course wrap-up.

> Self-study notes and the lab will be added later.
