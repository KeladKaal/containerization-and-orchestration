# Lab 4 — Zero-trust network with proof

## Situation

By default a Kubernetes network is flat: any pod can reach any pod. Convenient for development, bad for security: take over one public pod, and from it the whole network is visible, database included. In this lab your mission is to bring `shop` to a **zero-trust** principle — "deny by default, open only what is needed" — and **prove it with an access matrix**, then finish by finding a hidden connectivity break using network observability tools.

There is almost no code; the test services are **generated entirely by AI**. You need two:

- `api` — an HTTP service: `GET /health` → `ok`; `POST /order` → writes an order to PostgreSQL; `GET /orders` → reads them;
- `worker` — a simple loop: once a second it reads new orders from PostgreSQL and marks them processed.

PostgreSQL is a stock image. The learning part is network policies and diagnostics, not programming.

## Part 0 — Choose a CNI (it matters more than it looks)

Network policies are enforced by the **CNI plugin**, and — the key trap from the lecture — **not all plugins can do it**. The default `kindnet` silently ignores policies: you create a rule, kubectl accepts it, and it does nothing. So deliberately pick a CNI that **can** do NetworkPolicy:

- **Cilium** — eBPF-based, also gives a traffic map (Hubble) and policies up to the application layer (L7);
- **Calico** — the classic choice, enforces policies reliably.

Justify your choice in the README. And check it in practice right away: before turning on policies, confirm that **everyone currently reaches everyone** (from an outsider pod, reach PostgreSQL) — that is the original flat network.

## Part 1 — Deploy shop and explore the pod network

Bring up a local cluster (kind/k3d) with your chosen CNI and deploy `shop` **via configuration**. Look under the hood of the flat network:

- inspect the pod's IP and its `eth0` interface from the inside, and find the paired end on the node (veth);
- show that pods on different nodes reach each other directly by IP.

Briefly describe in the README how a pod gets its address and why a pod's IP cannot be relied on (it changes on recreation) — which is why from here on we work through services and labels, not IPs.

## Part 2 — Draw the target access matrix

Before denying anything, design on paper (in the README) a **matrix**: who SHOULD have access to whom and who should NOT. Reference:

- client → `api` (allowed);
- `api` → PostgreSQL (allowed);
- `worker` → PostgreSQL (allowed);
- everything else, including "outsider pod → PostgreSQL" — not allowed.

Do not forget what is easy to miss: **DNS** (without it services cannot find each other by name) and the needed **egress** traffic.

## Part 3 — Turn on zero-trust

Implement the matrix with policies:

1. Enable **default-deny** — deny all traffic to pods (or in the namespace) by default.
2. Open exactly the routes from the matrix, by pod labels (not by IP), with ports specified.
3. Separately allow **DNS** and the necessary egress, or you will break something you did not think about.

## Part 4 — Prove the matrix in practice

Walk the matrix live: from each pod try to reach each target and put the "expected / actual" result into a table. The goal is a **fully green matrix**: what is allowed works, what is denied does not pass. In particular, show that an outsider pod now **cannot** connect to PostgreSQL, while `api` and `worker` can.

## Part 5 — Find the hidden connectivity break

Introduce a **hidden break** into the configuration (for example, an extra or too-broad deny rule that cuts the needed `api → postgres` path — you can ask a teammate to slip it in without telling you which). Then:

- spot the symptom (orders stop going through);
- using the **traffic map** (Hubble with Cilium, or Calico logs/tools), find which connection exactly is blocked and why;
- fix it and confirm against the matrix that everything is green again.

In the README describe the diagnostic path — how exactly you found the cause rather than guessing.

## Part 6 — Monitoring (required)

Set up network observation: connection metrics, policy-denied traffic, DNS operation. Build a dashboard, decide yourself what is important to see, then **pick 3 metrics you would build alerts on** and explain the choice.

The tool is your choice.

## What to submit

1. **Service code** for `api` and `worker` with a Dockerfile (generated — not graded as programming).
2. **`README.md`** — the chosen CNI and why; how a pod gets network (Part 1); the target access matrix (Part 2); the final "expected/actual" matrix — green (Part 4); the analysis of the hidden break — how you found and fixed it (Part 5); the dashboard and the rationale for the 3 metrics.
3. **Your configs** — manifests/Helm and **all network policies**.
4. **Screenshots**: the pod network (IP, veth); an outsider pod that did NOT reach the database; the green access matrix; the traffic map with the located block; the dashboard.

## How to start

Open this repository with your AI assistant and ask for help with **Lab 4**. The assistant is set up to guide you **step by step** and check your understanding — it deliberately does not hand over a finished solution. Generate the `api` and `worker` services, and go through the learning part step by step from Part 0 — start with a deliberate choice of CNI.

> **Using AI?** Make sure your assistant follows the repository rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up automatically; if yours did not — just point it at this file and ask it to act by it.
