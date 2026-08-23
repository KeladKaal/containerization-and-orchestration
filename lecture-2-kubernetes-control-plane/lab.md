# Lab 2 — Cluster guardrails: make it impossible to break the rules

## Situation

In the lecture we saw that Kubernetes is declarative and rests on the reconciliation loop, and that the API server is the single door with a guard on it: authentication, permissions, and **admission**. In this lab you use that door to turn the cluster into a self-defending platform: you define rules declaratively, the cluster **refuses to break them on its own**, and reconciliation heals deviations by itself. Along the way you verify in practice what survives a "brain" outage.

There is almost no application code here: rules are defined by policies (declaratively), and the test service is simple and **generated entirely by AI**. You need a small HTTP service `api`:

- `GET /health` — returns `ok`;
- `POST /order` — writes an order row into PostgreSQL;
- `GET /orders` — reads and returns the list of orders.

PostgreSQL is taken **off the shelf** — you do not write it. The learning part is working with the control plane and policies, not programming.

## Part 0 — Choose a policy engine

You define guardrails declaratively, without code. Pick **one engine** and justify it:

- **Kyverno** — rules are written as ordinary Kubernetes manifests (YAML), lower barrier to entry;
- **OPA / Gatekeeper** — rules in the Rego language, more powerful and flexible, but you must learn the language.

Both work as admission checks at the entrance to the cluster. Note your choice and why in the README.

## Part 1 — Bring up a cluster and feel reconciliation

Bring up a local cluster (**kind** or **k3d** — Kubernetes in containers, installed with a single command) and deploy `shop` in it (the `api` service + PostgreSQL) **via configuration** — manifests or Helm.

Observe the reconciliation loop on a live example (keep `kubectl get pods -w` open):

- delete the `api` pod by hand → show that it came back;
- change the replica count → show that the cluster caught up to the desired state.

In the README, explain in your own words why "I deleted it and it came back" is not a bug but the essence of Kubernetes.

## Part 2 — Kill the "brain" and see what survives

Verify the lecture's claim that the control plane and the workload are decoupled:

1. Stop the **etcd** (or apiserver) container of your kind/k3d.
2. Try to change something (`kubectl apply`, scaling) → you will see the changes do not go through.
3. At the same time, verify that the **already-running `shop` pods keep working** and responding (hit `/health`, `/orders`).
4. Bring the control plane back and confirm the cluster is manageable again.

In the README, record what exactly froze, what survived, and why (who on the node keeps executing pods without the "brain").

## Part 3 — Set the guardrails

Describe with policies **at least four** guardrail rules for the whole cluster. Suggestions (your own are fine):

- no pod can be created **without specified resource limits**;
- images are allowed **only from a trusted registry** (e.g. your local one);
- all workloads must have **mandatory labels** (e.g. `team`, `app`);
- **privileged** containers are forbidden (those that strip away almost all protection — from Lecture 1).

In the README, explain for each rule what real trouble it protects against.

## Part 4 — Try to break your own defenses

Now the interesting part — attacking your own guardrails. **Devise and carry out at least 5 attempts to break the rules**: deploy a pod without limits, a pod with a foreign image, without mandatory labels, a privileged pod, and something of your own. For each attempt, show that the cluster **rejected it with a clear error**.

Then check the other side: confirm that your normal `shop` passes all guardrails without a single edit. A guardrail that also blocks correct deploys is a bad guardrail.

## Part 5 — Monitoring (mandatory)

Set up observation of the cluster's health as an orchestrator: collect metrics (pod state, restarts, requests rejected by policies) and build a dashboard. Decide for yourself what matters to see, then **choose 3 metrics you would build alerts on** and explain the choice.

The tool is your choice.

## What to submit

1. **The `api` service code** with a Dockerfile (generated — not graded as programming).
2. **`README.md`** — the chosen policy engine and why; what reconciliation showed (Part 1); what froze and what survived when the control plane went down (Part 2); the list of guardrails with justification (Part 3); a report on the 5 bypass attempts — what was blocked (Part 4); the dashboard and the justification of the 3 metrics.
3. **Your configs** — manifests/Helm for `shop` and **all policies**.
4. **Screenshots**: the deleted pod coming back; behavior with etcd stopped; rejected attempts to break the rules (with the error text); the dashboard.

## How to start

Open this repository with your AI assistant and ask for help with **Lab 2**. The assistant is set up to guide you **step by step** and check your understanding — it deliberately will not hand over a finished solution. Generate the `api` service (it is simple), and work through the learning part — the cluster, the control plane outage, the policies — step by step starting from Part 0.

> **Using AI?** Make sure your assistant follows the repository rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up automatically; if yours did not — just point it to this file and ask it to act on it.
