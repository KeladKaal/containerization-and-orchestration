# Lab 3 — Cluster Guardrails

## Situation

Turn the cluster into a platform that stops itself from breaking its own rules. You will deploy `shop`, watch the cluster restore what you delete (reconciliation), check what survives a control plane failure, set guardrail rules with policies, and try to break them.

There is almost no code here: rules are declared declaratively, the service is simple and generated. The learning part is the control plane and policies.

## Part 0 — Your service

Write a service — your own or generated; the implementation doesn't affect the grade. HTTP service `api`:

- `GET /health` — returns `ok`;
- `POST /order` — writes an order row to PostgreSQL;
- `GET /orders` — reads the list of orders.

Use PostgreSQL the standard way, you do not need to write it.

## Part 1 — Deploy and watch reconciliation

Bring up a cluster (kind or k3d) and deploy `shop` with manifests or Helm. Open `kubectl get pods -w`. Delete the `api` pod — see what happens. Change the number of replicas — see how the cluster reacts. Describe in the README why deleted things come back.

## Part 2 — Kill the control plane

Stop the `etcd` container (or apiserver). Try to change something with `kubectl`. In parallel, hit `/health` and `/orders` on the already-running `shop`. Bring the control plane back. Answer in the README: what stopped working, what kept working, and why.

## Part 3 — Set guardrails

Pick a policy engine — Kyverno (rules as YAML) or OPA/Gatekeeper (rules in Rego) — and justify the choice. Describe at least 4 rules for the whole cluster. Reference (you may use your own):

- do not admit pods without the specified resource limits;
- allow images only from a trusted registry;
- require mandatory labels (for example, `team`, `app`);
- forbid privileged containers.

For each rule, write what it protects against.

## Part 4 — Break your own defense

Design and run at least 5 attempts to violate the rules: a pod without limits, a foreign image, without labels, privileged, and something of your own. Show that the cluster rejects each attempt with an error.

Then check the reverse: a normal `shop` passes all guardrails without a single edit. A guardrail that blocks correct deploys too is broken.

## Part 5 — Monitoring

Bring up monitoring as in lab 2 and point it at the cluster: collect control plane metrics and pod state (restarts, requests rejected by policies) onto a dashboard. Pick 3 metrics for alerts — for each, write what it catches.

## What to submit

- The `api` service code with a Dockerfile (any implementation, doesn't affect the grade).
- `README.md`: the chosen policy engine and why; why deleted things come back (part 1); what stopped and what kept working during the control plane failure (part 2); the list of rules and what each protects against (part 3); the report on the 5 bypass attempts (part 4); the dashboard and the justification of the three metrics.
- Configs: manifests/Helm for `shop` and all policies.
- Screenshots: a deleted pod coming back; behavior with `etcd` stopped; rejected attempts to violate the rules with the error text; the dashboard.

## How to start

Open the repository with an AI assistant and ask for help with Lab 3. The assistant guides you step by step and checks understanding, it does not hand out a finished solution. Generate the service (Part 0), then go in order.

> Using AI? Check that the assistant follows the rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up on their own; if not — point it to this file.
