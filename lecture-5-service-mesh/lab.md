# Lab 5 — Release autopilot: ship the good, roll back the bad

## Situation

A service mesh promises encryption and smart traffic management with no code changes. In this lab you put that to the test: on top of `api` you build a **self-driving canary release** — a new version first gets a small share of traffic, the system watches its health on its own, and **automatically promotes it on success or rolls it back when errors rise, with no human involved**. At the end you honestly **measure the cost**: how much (latency, resources) all this magic cost, and you deliver a verdict — does this system need a mesh?

The lab is trimmed for a modest laptop: a single-node cluster, a lightweight mesh, mesh only around `api`.

The test services are **generated entirely by AI**. You need:

- `api` in two versions, **v1** and **v2**, identical except: `GET /version` returns `v1`/`v2`; v2 has a "bad behavior" switch — the environment variable `FAIL=true` makes `POST /order` return a 500 error (so you have something to trigger a rollback with);
- the usual endpoints: `GET /health` → `ok`, `POST /order`, `GET /orders`;
- a small **gate script** (also generated from a description): about every ~15 seconds it reads the error rate and latency of v2 from Prometheus and, by a simple rule, changes the traffic split — raising v2's share if it's healthy, or rolling it back to 0 if errors rose.

PostgreSQL is a standard image, outside the mesh.

## Part 0 — Choose a mesh

Choose **one lightweight mesh** (no heavy sidecar Istio on a laptop) and justify it:

- **Linkerd** — very lightweight, simple to install, an excellent start;
- **Istio ambient** — Istio's sidecarless mode, also without heavy proxies in every pod.

Note your choice and why in the README.

## Part 1 — Measure the baseline (no mesh)

Deploy `api` v1 and PostgreSQL **through configuration**, without a mesh. Run a load (`k6`/`hey`) over the order path and record the **initial** numbers: latency and the resource usage of the `api` pod. This is the reference point you'll compare the mesh's cost against at the end.

## Part 2 — Enable the mesh and mTLS

Install the chosen mesh and bring `api` into it. Enable **mTLS** between services and **prove** the traffic is now encrypted (for example, show that intercepted traffic between `api` pods is no longer readable in plaintext, and the service identity is confirmed by a certificate). Briefly describe in the README what the mesh gave "for free" to the code.

## Part 3 — Deploy two versions and split traffic

Deploy `api` v1 and v2 at the same time and configure **traffic split** through the mesh — start with 90% on v1 and 10% on v2. Verify via `GET /version` that requests are actually split in this proportion.

## Part 4 — Build the autopilot

Run your **gate script**. Give it a simple but sensible rule (for example: "if v2's error rate over the last 2 minutes is below the threshold — raise its share by one step; if above — roll it back to 0"). In the README, describe the gate's logic in words: which metrics it looks at and how it makes decisions.

## Part 5 — Test both scenarios

1. **Bad release:** roll out v2 with `FAIL=true`. Show that the gate noticed the rise in errors and **rolled traffic back** to v1 on its own — users were barely affected.
2. **Good release:** roll out a healthy v2 (`FAIL=false`). Show that the gate gradually **drove** its share up to 100% on its own.

Attach graphs: how v2's traffic share and the error rate changed in both scenarios.

## Part 6 — Compute the cost and deliver a verdict

Measure `api`'s latency and resource usage again — now with the mesh — and compare with the baseline from Part 1. In the README compute **how much latency and resources grew** because of the proxy, and deliver a reasoned verdict: is the mesh justified for a system of this size, or does its cost outweigh the benefit? Answer with numbers, not "gut feeling."

## Part 7 — Monitoring (required)

The mesh itself exposes rich traffic metrics — build a dashboard from them (error rate and latency by version, traffic split). Decide for yourself what matters to see, then **choose 3 metrics you would build alerts on** and explain your choice.

## What to submit

1. **Service code** for `api` v1/v2 and the **gate script** with a Dockerfile (generated code isn't graded as programming; what's graded is the gate logic described in words).
2. **`README.md`** — the chosen mesh and why; the mTLS proof (Part 2); the baseline and the mesh's cost with a comparison (Parts 1 and 6); the gate logic (Part 4); the results of both scenarios with graphs (Part 5); the final "do we need a mesh" verdict; the dashboard and the justification of the 3 metrics.
3. **Your configs** — manifests/Helm, mesh and traffic-split settings.
4. **Screenshots**: confirmation of encryption; the 90/10 traffic split via `/version`; the automatic rollback of the bad v2; the automatic promotion of the healthy v2; the baseline vs. with-mesh comparison; the dashboard.

## How to start

Open this repository with your AI assistant and ask for help with **Lab 5**. The assistant is set up to guide you **step by step** and check your understanding — it deliberately won't hand out a ready solution. Generate `api` v1/v2 and the gate script from the description, and go through the learning part step by step starting from Part 0.

**If resources are tight:** keep only what's necessary in the cluster (mesh only on `api`), don't run extra services; if you like — team mode and several laptops.

> **Using AI?** Make sure your assistant follows the repository rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up automatically; if yours didn't — just point it to this file and ask it to follow it.
