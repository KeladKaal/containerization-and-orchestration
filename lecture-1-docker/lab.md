# Lab 1 — Your own Docker

## Situation

This lab has you imitate containerization by, in effect, building your own Docker. You'll run an ordinary process with no isolation, then add namespaces, resource limits and reduced privileges to it by hand. You'll collect all of that into a single script — that's your Docker — and compare running it with a real `docker run`.

Writing the service isn't graded — it's a tool, not the goal. The learning is the work with namespaces, cgroups and privileges.

## Part 0 — Your service

Write your service. Hello-world level is fine, generated is fine — writing it is not the goal and isn't graded.

Requirements: an HTTP service `api` with three endpoints:

- `GET /health` — returns `ok`;
- `GET /eat?mb=N` — allocates N megabytes of memory and holds them;
- `GET /burn` — loads one CPU core in an infinite loop.

## Part 1 — Run it directly

Run the service directly on your machine. Open it in a browser, show that `/health` responds. Find the process with `ps` on the host, note its PID.

This is the baseline: no isolation, the process sees the whole system and can take any amount of resources.

## Part 2 — namespaces

Put the process into its own namespaces with `unshare` (pid, mount, net, uts, ipc and user). Enter it and show:

- inside, the process is PID 1 and can't see other processes;
- it has its own hostname and its own empty network;
- root inside is an unprivileged user outside (check which uid the process runs as on the host).

Note in the README what each namespace isolated.

## Part 3 — cgroups

Create a cgroup v2 for the process and add limits. Test each through your service:

- memory: set a small ceiling, hit it via `/eat?mb=...`, catch the OOM. This is the same thing as `OOMKilled` in Kubernetes;
- CPU: set a limit (say, half a core), hit it via `/burn`, find the throttling in the cgroup stats;
- processes: set `pids.max`, launch a fork bomb via `stress-ng --fork`, show that it can't multiply.

## Part 4 — privileges

Leave the process only the minimum it needs:

- drop extra capabilities and show that a privileged action (e.g. changing the system time) no longer works;
- apply a seccomp profile and show that a blocked system call is rejected.

Describe what each mechanism closes off.

## Part 5 — Assemble your Docker

Collect all the commands from parts 2–4 into a single script (e.g. `mydocker.sh`) that starts `api` in its own namespaces, with cgroup limits and reduced privileges, in one command. Check that the service comes up and `/health` responds.

Now run the same service via `docker run` and compare it with your script: what matches, what your script is missing, and what Docker does beyond it. Put the comparison in the README.

## Part 6 — Images

Your script was missing a ready-made filesystem — that's what an image provides.

- Write a Dockerfile for `api` and build the image.
- Do a multi-stage build with a minimal base (for Go, `scratch` or distroless works). Compare the size, the number of layers, and what got reused from cache on a rebuild.
- Write a file inside the container, recreate the container — the file is gone. Repeat with a volume — the file stays.

## Part 7 — When a container isn't enough

Run the image under gVisor (`runsc`) and compare its isolation with ordinary Docker and with your script. Work out how gVisor is built differently and why it's considered more isolated. Separately, answer: what does an ordinary container always share with the host, and why is that the limit of container isolation. Conclusions go in the README.

## Part 8 — Monitoring

Collect container metrics (memory, CPU, throttling) from the cgroup or via cAdvisor and build a dashboard. Decide yourself what matters to see, and pick 3 metrics to alert on — for each, write what it catches and why it matters.

## What to submit

- The `api` service code with a Dockerfile (any implementation, doesn't affect the grade).
- The `mydocker.sh` script — your Docker from parts 2–4.
- `README.md`: the service language; what each step in parts 2–4 showed; the comparison of your script with `docker run` (part 5); the image comparison (part 6); what gVisor adds (part 7); the dashboard and the rationale for the three metrics.
- Configs: seccomp profile, Dockerfile.
- Screenshots: the process as PID 1 inside and ordinary outside; OOM; throttling; a rejection by seccomp or capabilities; a working `mydocker.sh`; the image size comparison; the dashboard.

## How to start

Open the repository with your AI assistant and ask it to help with Lab 1. The assistant works step by step and checks your understanding; it won't hand you a finished solution. Generate the service (Part 0), then go in order from Part 1.

> Using AI? Make sure the assistant follows the rules in [`AGENTS.md`](../AGENTS.md). Most tools pick it up automatically; if not, point it at the file.
