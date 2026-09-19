# Lab 3 — Build a platform for shop

> **New to Kubernetes?** Work through [Lens](https://k8slens.dev/) — a graphical interface for the cluster that makes pod state, reconciliation, and failures much easier to see than plain `kubectl`.

## Situation

In a real cluster, anyone can deploy a service with no resource limits, an untrusted image, or missing labels — and it quietly reaches production, because nobody reviews every YAML by hand. A database needs manual operational routine — provisioning, recovering after a pod failure — work that falls on an engineer instead of the system. And when the control plane goes down, nobody knows in advance what actually stops and what keeps running — that gets discovered during the incident.

This lab closes all three problems at once. First you set guardrails with policy — the cluster physically refuses deploys that break the rules, so nobody has to review every YAML by hand. Then you write a Helm chart for `shop` (`api`, `worker`, `postgres`) that complies with those guardrails, and you bring up `postgres` through an operator that takes over operating the database. And on a live cluster you check by hand what survives a control plane failure.

Everything runs locally and for free: kind, k3d, or minikube — no cloud. Writing `api` and `worker` isn't graded.

## Part 0 — Services

Generate `api` and `worker`:

- `api` — an HTTP service with endpoints:
  - `GET /health` — returns `ok`;
  - `POST /order` — writes an order to `postgres`;
  - `GET /orders` — reads and returns the list of orders;
- `worker` — reads orders from `postgres` and marks them processed.

## Part 1 — Guardrails for the cluster

Pick one policy engine and justify it in the README:

- **Kyverno** — rules as ordinary Kubernetes manifests, a lower entry bar, lighter for a local cluster;
- **OPA / Gatekeeper** — rules in Rego, more powerful and flexible, but requires learning it.

Describe at least five cluster-wide rules: for example, mandatory resource limits, images only from a trusted registry, mandatory labels, no privileged containers, and so on. For each, explain in the README what it's good for.

Prepare 5 raw manifests, each breaking one rule (no limits, an untrusted image, no labels, privileged, and one of your own), and show that they're rejected.

## Part 2 — Chart for api and worker

Write `Chart.yaml`, `values.yaml`, and `Deployment`/`Service` templates for `api` (replicas from `values`, 3 by default) and `worker` (2 by default). Bake in from the start what the rules from Part 1 require.

Install the chart. If a template doesn't comply with a rule, the install fails with a policy error — fix it and get a clean install.

Check reconciliation on your chart:

- delete the `api` pod by hand → it comes back;
- change `replicas` in `values.yaml`, run `helm upgrade` → the pod count catches up to the new value.

## Part 3 — Postgres through an operator

Deploy a database operator (CloudNativePG or the Zalando postgres-operator) once, for the whole cluster. Add a template to your chart that creates the operator's object (e.g. a `Cluster` for CloudNativePG).

Delete the `postgres` pod by hand → the operator brings it back. Look at the object (`kubectl get cluster -o yaml`), find `spec` and `status` in it. In the README, compare how an operator differs from `controller-manager`.

## Part 4 — Control plane failure

Stop `etcd` (or `apiserver`) on your kind/k3d/minikube:

- try to change something (`helm upgrade`, `kubectl apply`) → the change doesn't go through;
- check that `api`/`worker`/`postgres` keep responding (`/health`, `/orders`).

Bring the control plane back and confirm the cluster is manageable again.

## Part 5 — Monitoring

Add `ServiceMonitor`/`PrometheusRule` templates to the chart, reusing the stack from Lab 2 — don't stand up a new one. Bake 3 alert rules into the chart as `PrometheusRule` templates, and justify each in the README. Trigger at least one and show it in the firing state.

## Result

What you should end up with:

- cluster-wide guardrails (Kyverno/Gatekeeper) are in place, set up before the chart;
- the `shop` Helm chart (`api`, `worker`, `postgres` through an operator) installs clean from scratch, with zero rejections;
- the `api` and `postgres` pods come back on their own after a manual delete;
- when the control plane goes down, `shop` keeps responding, and once it's back the cluster is manageable again;
- a dashboard and 3 alerts on top of the Lab 2 stack.

## What to submit

- The `api`/`worker` code with a Dockerfile (any implementation, doesn't affect the grade).
- The cluster guardrail bundle (Kyverno/Gatekeeper policies).
- The `shop` Helm chart: `Chart.yaml`, `values.yaml`, all templates, including the database's CRD object and `PrometheusRule`/`ServiceMonitor`.
- `README.md`: the policy engine choice and rule rationale (Part 1); what reconciliation showed on the chart (Part 2) and on the operator (Part 3); how an operator differs from `controller-manager`; what survived the control plane failure (Part 4); the rationale for the 3 alerts (Part 5).
- Screenshots: of the whole process.

## How to start

Open the repository with your AI assistant and ask for help with Lab 3. Generate `api` and `worker` (Part 0), then go in order from Part 1. The assistant works step by step and checks your understanding; it won't hand you finished policy rules or chart templates.

> Using AI? Make sure the assistant follows the rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up automatically; if not, point it at the file.
