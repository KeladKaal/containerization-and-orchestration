# Lab 1 — Build a container by hand and keep a service within limits

## Situation

All course long we say a container is an ordinary process to which the kernel gives an altered view of the world and trimmed resources. In this lab you prove it to yourself in practice: **you build a container around your own service using bare Linux kernel primitives**, then compare your build with what Docker does and with the hardened gVisor runtime.

There is no foreign or "malicious" code here. The "bad behavior" — eating memory, hogging the CPU — you trigger yourself, through harmless endpoints of your service and the standard `stress-ng` tool. The learning part is not programming but **devops work around a process**: namespaces, cgroups, privileges.

The test service is simple and can be **generated entirely with AI**. You need a tiny HTTP service `api` with three endpoints:

- `GET /health` — returns `ok`;
- `GET /eat?mb=N` — allocates N megabytes of memory and holds them;
- `GET /burn` — loads one CPU core in an infinite loop.

Nothing else is required — it is just a tool to pull resources on command.

## Part 0 — Choose the service language

The language of the test `api` is **your choice**, and it affects what you see in the image part:

- **compiled (e.g. Go)** — the multi-stage build and the `scratch` base image shine: the final image can be reduced to almost a single binary;
- **interpreted (e.g. Python)** — the image is thicker, but the discussion of dependency layers is clearer.

Note in the README what you chose and why.

## Part 1 — Run without isolation (baseline)

Run `api` as an ordinary process directly on the host (no Docker, nothing). Open it in a browser, hit `/health`. Look at the process with `ps` on the host — it is there, an ordinary process among the rest.

In the README, record: this is the starting state; the process has no isolation and no limits — it sees the whole system and can take any amount of resources.

## Part 2 — Give the process its own view of the world (namespaces)

Place the process in its own namespaces via `unshare` (pid, mount, net, uts, ipc and, mandatorily, **user**). Go inside and show:

- from inside, your process is **PID 1**, and it does not see other processes;
- it has its own hostname and its own (empty) network;
- thanks to the user namespace, **root inside is an unprivileged user outside** (check which uid the process runs as on the host).

In the README, describe what changed compared to Part 1 and exactly what each namespace isolated.

## Part 3 — Cap the appetite (cgroups)

Create a cgroup (v2) for the process and attach limits. Test each on your service:

- **memory:** set a small ceiling and hit `/eat?mb=...` past it → catch the kernel killing the process (OOM). Describe what happened and how it maps to `OOMKilled` in Kubernetes.
- **CPU:** set a CPU limit (e.g. half a core), hit `/burn` → measure **throttling** (the process is alive but slowed). Find where this shows up in the cgroup statistics.
- **process count:** set `pids.max` and use `stress-ng --fork` to show that it cannot multiply (protection against a fork bomb).

## Part 4 — Trim privileges (capabilities, seccomp)

Leave the process only the necessary minimum of rights:

- drop excess **capabilities** and show that a privileged action (your choice — e.g. changing the system time) is rejected;
- apply a **seccomp** profile and show that a blocked system call does not go through.

In the README, explain why this is needed and what each mechanism closes off.

## Part 5 — Build an image and compare with Docker

1. Write a **Dockerfile** for `api` and build the image the usual way.
2. Make a **multi-stage** build with a minimal base (for Go — `scratch`/distroless) → compare sizes and layer counts, and see what was reused from cache on a rebuild.
3. Run the service in Docker, write a file inside the container, recreate the container → show the file is gone; repeat with a **volume** → the file survives.
4. Run the same image under **gVisor** (`runsc`). Compare in the README: what your manual build from Parts 2–4 closes off, what Docker gives by default, what gVisor adds, and **where isolation still leaks** (shared kernel).

## Part 6 — Monitoring (mandatory)

Set up observation of your container: collect consumption metrics (memory, CPU, throttling) from the cgroup or via cAdvisor and build a dashboard from them. Decide yourself what matters to see, then **pick 3 metrics you would build alerts on** and explain the choice — what each one catches and what it risks.

The tool is your choice.

## What to submit

1. **Service code** `api` with a Dockerfile (generated — not graded as programming).
2. **`README.md`** — the chosen language and why; what each step of Parts 2–4 showed (namespaces, cgroups, privileges) explained through the lecture's concepts; the image comparison from Part 5; the comparison of your build with Docker and gVisor; the dashboard and the rationale for the 3 alert metrics.
3. **Your scripts and configs** — the `unshare`/cgroups commands, the seccomp profile, the Dockerfile.
4. **Screenshots**: the process as PID 1 from inside and ordinary from outside; the moment of OOM; throttling in the statistics; a seccomp/capabilities rejection; the image size comparison; the dashboard.

## How to start

Open this repository with your AI assistant and ask for help with **Lab 1**. The assistant is set up to guide you **step by step** and to check your understanding — it deliberately does not hand out a ready solution. First generate the `api` service (Part 0), then work through the learning part — building the container by hand — step by step from Part 1.

> **Using AI?** Make sure your assistant follows the repository rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up automatically; if yours did not — just point it at this file and ask it to act accordingly.
