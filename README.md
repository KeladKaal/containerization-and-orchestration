# Containerization and Orchestration: How It Works Under the Hood

> 🌐 **Language:** English (branch `main`) · [Русская версия → branch `main-rus`](https://github.com/KeladKaal/containerization-and-orchestration/tree/main-rus)

An in-depth course for engineers who already run Docker and Kubernetes every day.

This course is not about *how to use* containers — you already do. It's about **how they work underneath**: which Linux kernel mechanisms, which components and protocols sit behind the familiar commands, where the abstractions leak, and why. We take something familiar — `docker run`, an image, a Pod, a Service — and go one layer down.

Assumes you're comfortable with containers and Kubernetes in daily work. Prior knowledge of Linux internals is helpful but not required.

## Lectures

Seven lectures, 1.5 hours each. Every lecture has three logical blocks (~25–30 min). Click a lecture to see what's inside.

1. [Container fundamentals](lecture-1-container-fundamentals/) — namespaces, cgroups, capabilities, seccomp: what a container really is
2. [Images, runtimes and engines](lecture-2-images-runtimes-engines/) — OCI images, overlayfs, runc / containerd / shim, BuildKit, registries
3. [Kubernetes control plane](lecture-3-kubernetes-control-plane/) — the declarative model, API server, etcd, scheduler, reconciliation
4. [Kubernetes networking](lecture-4-kubernetes-networking/) — CNI, Services, kube-proxy, DNS, NetworkPolicy, eBPF / Cilium
5. [Scheduling, resources and storage](lecture-5-scheduling-resources-storage/) — scheduler internals, requests/limits, QoS, OOM/eviction, CSI, StatefulSet
6. [Service mesh and advanced traffic](lecture-6-service-mesh/) — data plane vs control plane, Envoy, mTLS, traffic management, ambient / eBPF
7. [Extensibility and runtime security](lecture-7-extensibility-runtime-security/) — CRDs and operators, admission webhooks, gVisor / Kata, supply chain

## Running example

The whole course follows one small online shop, `shop`, made of three services:

- `api` — a stateless HTTP service, several replicas;
- `worker` — a background order processor;
- `postgres` — holds the state.

Each lecture drops one layer deeper into this same example: how `api` becomes a process in a container, how its image is built, how it lands on a node, how replicas find each other and `postgres`, how the scheduler places them, how mTLS and traffic management appear between them, and how it all gets isolated and signed.

## Labs

Hands-on labs will be added later. They'll be designed to run **locally and for free** (kind / k3d / minikube) — no access to a real cluster required.

## Using AI in this repo

Using an AI assistant is allowed and encouraged — but this repo is set up so it **helps you learn instead of handing you finished answers**: it works step by step, explains the reasoning, and checks your understanding before moving on.

The rules live in [`AGENTS.md`](AGENTS.md) (and `CLAUDE.md` for Claude Code). Most AI tools pick these up automatically. **If yours doesn't, just point it at [`AGENTS.md`](AGENTS.md)** and ask it to follow the file.

## How this repo is organized

- One folder per lecture, each with a short README describing its blocks.
- **`main`** — everything in **English**. **`main-rus`** — the same in **Russian**.
- Anything pushed here is mirrored in both languages: English → `main`, Russian → `main-rus`.

## License

© 2026 KeladKaal. Licensed under [CC BY-NC 4.0](LICENSE). Reuse and adapt freely for **non-commercial** purposes, with **credit** to the author.
