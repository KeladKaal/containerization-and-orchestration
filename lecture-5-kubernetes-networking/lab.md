# Lab 5 — Zero-trust network

## Situation

By default the Kubernetes network is flat: any pod reaches any pod. You need to bring `shop` to the principle "denied by default, only what is needed is open", prove it with an access matrix, and at the end find a hidden connectivity fault.

The services are simple, generated. The learning part is network policies and diagnostics.

## Part 0 — Your services

Write (may be generated, not graded):

- `api` — HTTP: `GET /health` → `ok`; `POST /order` → writes an order to PostgreSQL; `GET /orders` → reads;
- `worker` — loop: once a second reads new orders from PostgreSQL and marks them processed.

Take PostgreSQL the standard way.

## Part 0.5 — Choose a CNI

Network policies are applied by the CNI plugin, and not every one can do it — the default `kindnet` silently ignores them. Choose a CNI that can do NetworkPolicy: Cilium (on eBPF, with the Hubble traffic map) or Calico. Justify the choice.

## Part 1 — Deploy and explore the pod network

Bring up a cluster (kind/k3d) with the chosen CNI and deploy `shop`. Look at the pod IP and its `eth0` interface from inside, find the paired end on the node. Check that before policies are enabled a foreign pod reaches PostgreSQL. Briefly describe in the README how a pod gets its address and why the pod IP cannot be relied on.

## Part 2 — Build the access matrix

Before denying anything, write out in the README a matrix: who MUST reach whom and who must NOT. Reference point: client → `api`, `api` → PostgreSQL, `worker` → PostgreSQL, everything else is not allowed. Account for DNS and the required outgoing traffic.

## Part 3 — Enable zero-trust

Enable default-deny and open exactly the required routes from the matrix — by pod labels, with ports. Separately allow DNS and the required egress.

## Part 4 — Prove the matrix

Walk the matrix live: from each pod try to reach each target, put "expected/actual" into a table. Get the allowed to work and the denied to not pass. Show that the foreign pod no longer connects to PostgreSQL.

## Part 5 — Find the hidden fault

Introduce into the configuration a hidden connectivity fault (for example, a too-broad deny rule cutting the `api → postgres` path; you can ask a partner to slip it in without telling you which). Detect the symptom, use the traffic map (Hubble or Calico tools) to find which connection is blocked, and fix it. Describe the diagnosis path in the README.

## Part 6 — Monitoring

Point the monitoring from lab 2 at the network: metrics for connections, dropped traffic, DNS operation. Choose 3 metrics for alerts — for each write what it catches.

## What to submit

- Code of the `api` and `worker` services with a Dockerfile (generated code is not graded).
- `README.md`: the chosen CNI and why; how a pod gets its network (part 1); the target matrix (part 2); the final "expected/actual" matrix (part 4); the breakdown of the hidden fault — how you found and fixed it (part 5); the dashboard and justification of the three metrics.
- Configs: manifests/Helm and all network policies.
- Screenshots: the pod network (IP, veth); the foreign pod did NOT reach the database; a green access matrix; the traffic map with the found block; the dashboard.

## How to start

Open the repository with an AI assistant and ask for help with Lab 5. The assistant leads step by step and checks understanding, it does not hand out a ready solution. Generate the services (Part 0), then go in order — start with choosing a CNI.

> Using AI? Check that the assistant follows the rules from [`AGENTS.md`](../AGENTS.md). Most tools pick it up on their own; if not — point it to this file.
