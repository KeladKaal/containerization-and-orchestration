# Lab 6 — Release autopilot

## Situation

Build a self-managed canary release of `api`: the new version first gets a small share of traffic, the system watches its health on its own and rolls it out on success or rolls it back if errors grow. At the end, compute the cost of the mesh and decide whether it is needed here.

The lab is sized for a modest laptop: single-node cluster, light mesh, mesh only around `api`. The services are simple and generated.

## Part 0 — Your services

Write (may be generated, not graded) `api` in two versions, v1 and v2, identical except:

- `GET /version` returns `v1` or `v2`;
- in v2 the environment variable `FAIL=true` makes `POST /order` return a 500 error.

The other endpoints are as usual: `GET /health`, `POST /order`, `GET /orders`. You also need a small gate script (also may be generated): about once every 15 seconds it reads the v2 error rate and latency from Prometheus and, by a simple rule, changes the traffic split — grows the v2 share or rolls it back to 0. PostgreSQL is a standard image, outside the mesh.

## Part 0.5 — Choose a mesh

Take a light mesh (do not put heavy sidecar Istio on a laptop): Linkerd or Istio in ambient mode. Justify the choice.

## Part 1 — Measure the baseline

Deploy `api` v1 and PostgreSQL without a mesh. Run load (`k6`/`hey`) over the order path and record the initial latency and resource usage of `api` — you will compare the cost of the mesh against this at the end.

## Part 2 — Enable the mesh and mTLS

Install the chosen mesh and add `api` to it. Enable mTLS between services and prove that traffic is now encrypted. Briefly describe in the README what the mesh gave without code changes.

## Part 3 — Split traffic between two versions

Deploy v1 and v2 at the same time and configure the traffic split through the mesh — start with 90/10. Check via `GET /version` that requests split in this proportion.

## Part 4 — Build the autopilot

Run the gate script. Set a rule in it: if the v2 error rate over the last couple of minutes is below the threshold — raise its share by a step, if above — roll it back to 0. Describe the gate logic in words: which metrics it looks at and how it decides.

## Part 5 — Test both scenarios

Roll out v2 with `FAIL=true` — show that the gate rolled the traffic back to v1 on its own. Roll out a healthy v2 — show that the gate drove it to 100% on its own. Attach graphs: the v2 traffic share and the error rate in both cases.

## Part 6 — Compute the cost

Measure the latency and resources of `api` with the mesh again and compare with the baseline. Compute how much latency and usage grew, and give a verdict: is the mesh justified for a system of this size. Answer with numbers.

## Part 7 — Monitoring

The mesh emits traffic metrics on its own — build a dashboard from them (error rate and latency by version, the traffic split). Pick 3 metrics for alerts and explain the choice.

## What to submit

- The `api` v1/v2 code and the gate script with a Dockerfile (generated code is not graded; the gate logic described in words is graded).
- `README.md`: the chosen mesh and why; proof of mTLS (part 2); the baseline and the cost of the mesh (parts 1 and 6); the gate logic (part 4); the results of both scenarios with graphs (part 5); the verdict "is the mesh needed"; the dashboard and the justification of the three metrics.
- Configs: manifests/Helm, mesh and traffic split settings.
- Screenshots: confirmation of encryption; the 90/10 traffic split by `/version`; the automatic rollback of a bad v2; the automatic rollout of a healthy v2; the comparison of baseline and with mesh; the dashboard.

## How to start

Open the repository with an AI assistant and ask for help with Lab 6. The assistant guides you step by step and checks understanding, it does not hand out a finished solution. Generate `api` v1/v2 and the gate script (Part 0), then go in order.

> Using AI? Check that the assistant follows the rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up on their own; if not — point it at this file.
