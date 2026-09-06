# Lecture 1 — Docker: containers and images under the hood

We write `Dockerfile`s, run `docker run`, and ship manifests every day, yet what happens under the hood usually stays a black box. This lecture answers one question: what a container is at the operating-system level, how an image is built, and what path `docker run` travels before a service starts. The point is practical — when something breaks off-script, a list of commands stops helping and a mental model takes over. It turns familiar pains (`OOMKilled`, `CrashLoopBackOff`, "the data vanished", "the service is slow with no errors") into a quick diagnosis, and it guides engineering decisions: which base image, which limits, where the security boundary is.

One example runs through the whole course — an online shop with three services: `api` (handles HTTP requests, stateless, runs as several identical replicas), `worker` (processes orders in the background), and `postgres` (stores data). Today only `api` matters, and only one question: what happens when we package it into a container and run it.

---

## Block 1. A container is a process

### Base terms

- **Host** — the machine everything runs on: a server, a cluster node, or a laptop.
- **Process** — a running program: its own PID, memory, and code.
- **Kernel** — the core of the OS: hands out CPU time and memory, manages files, network, devices. Runs in the privileged "kernel space".
- Normal programs run in "user space" and never touch hardware directly.
- **System call (syscall)** — the only way for a user-space process to ask the kernel for something: open a file, create a connection, allocate memory.
- **Daemon** — a background service program (e.g. `dockerd`).

### The core myth

A container is not a lightweight virtual machine. There is no guest OS and no second kernel inside. A container is an ordinary Linux process on the host's shared kernel, to which the kernel gives an altered view of the system and capped resources. Everything "container-like" about it is settings of one shared kernel.

### VM vs container

In virtualization a hypervisor slices a server into VMs, each running its own full guest kernel — a very strong but heavy boundary (each kernel costs memory and startup time). Containers share one kernel, which is itself the boundary. Hence the upsides: containers start in milliseconds, weigh almost nothing, hundreds fit on one machine. Hence also the caveat for the whole course: with a shared kernel, isolation is thinner than a VM's.

### One process, two views

Run `api` via `docker run` and do `ps` on the host — `api` shows up as an ordinary process next to `systemd` and `sshd`, with a large PID. Inside the container (`docker exec` + `ps`) the same process appears as **PID 1**, as if it were first and foremost. From outside — one process among hundreds; from inside — the sole inhabitant of its world. The lecture is about how the kernel builds that second view.

### Three kernel mechanisms

Container = an ordinary process + three groups of Linux kernel mechanisms:

- **namespaces** — what the process *sees* (its own process list, network, filesystem);
- **cgroups** — how much it *may use* (memory, CPU, disk, process count);
- **privilege tightening** — what it is *allowed to do* (capabilities, seccomp, LSM).

`docker`, `containerd`, `runc` do not invent isolation — they just configure these kernel mechanisms, correctly and in the right order.

### namespaces: what the process sees

A namespace takes a shared system resource and gives the process a private copy:

- **pid** — own process list and numbering (the main process becomes PID 1);
- **net** — own network stack: interfaces, IPs, ports, firewall rules;
- **mnt** — own filesystem and root;
- **uts** — own hostname;
- **user** — own user mapping (root inside can be an unprivileged user outside).

Key point: namespaces isolate *visibility*, not *capacity* — they do not stop the process from eating all memory.

### The PID 1 trap

In Linux, PID 1 must reap terminated child processes (collect zombies). Normally `systemd` does this; in a container PID 1 is your `api`, which knows nothing about the duty. Result: orphaned processes pile up as zombies, and the container may not stop on `docker stop` (PID 1 mishandles the termination signal and is only killed after the timeout). Fix: either the app handles signals properly, or run it under a tiny init — in Docker the `--init` flag, which injects `tini`.

### net namespace: its own network

The container has a full network stack, empty at start. Connectivity comes from a `veth` pair — a virtual patch cable with two ends: one appears inside as `eth0`, the other stays on the host and plugs into a bridge. A consequence of port isolation: two containers can both listen on port 8080 without conflict — they are in different net namespaces. A conflict only arises when a port is published to the host itself.

### mnt namespace: where the filesystem comes from

The container's root is not the host but a minimal environment from the image (unpacked layers merged together; details in Block 2). The root switch uses not old `chroot` (escapable under some privileges) but `pivot_root`: the new root becomes real, the old host root is detached so it cannot be reached. Service filesystems — `/proc`, `/sys`, `/dev` — and data volumes are mounted on top.

### user namespace: root inside ≠ root outside

Linux users are numbers (uid); root is uid 0. A user namespace can say: "uid 0 inside the container is really uid 100000 outside." The process thinks it is root in its own world, while the host kernel sees an unprivileged user. This is the basis of rootless containers and sharply limits damage from an escape: a broken-out attacker lands as a harmless uid 100000. By default the user namespace is often off — then root inside is root outside, and the cost of escape is maximal. The reason is historical compatibility, but the industry is moving toward enabling it.

### cgroups: how much the process may use

Namespaces hid the rest of the system but said nothing about resources — the process can still allocate memory to the last byte and load every core. On a shared node that is the "noisy neighbor". cgroups (control groups) group processes and attach limits and counters: memory, CPU time, disk operations, even the number of processes (process count is a resource too — otherwise one bug forks until the node falls over). Everything you write as memory/cpu limits in Docker or `resources` in Kubernetes becomes cgroup settings.

### Memory limit and OOMKilled

A cgroup has a hard memory ceiling. On overrun the kernel first tries to free memory within the group (drop caches); if there is nothing to give, OOM fires and the kernel forcibly kills a process in that group. Only the offender is hit; the rest of the node is fine. In Kubernetes: the container crosses its limit → status `OOMKilled` → kubelet restarts it. Takeaway: `OOMKilled` almost never means "the node ran out of memory" — it means "this container exceeded its limit". Fix by raising the limit honestly or hunting a leak. If it loops — start, eat, get killed, restart — you get `CrashLoopBackOff`.

### CPU limit and throttling

CPU behaves differently. Memory is all-or-nothing: run short and something gets killed. CPU is divisible in time: on hitting the ceiling the process is not killed but slowed until the next short period — **throttling**. Two cgroup parameters set this: a share under contention (like requests — your fair share when things are tight) and a hard quota (like limits — hitting it causes throttling). A sneaky case: the container looks healthy by all metrics, does not crash, but answers slowly and unevenly — often a too-low CPU limit choking it every period. Visible in the cgroup's throttling metric. Remember: memory hits with a kill, CPU hits with delay.

### Privileges: capabilities, seccomp, LSM

The third group — three layers of defense:

- **capabilities** — root's omnipotence split into ~40 separate rights granted one by one (binding to ports below 1024 is one; managing the network is another). Docker keeps a narrow safe set by default and drops the dangerous ones, so even root inside is heavily trimmed.
- **seccomp** — a syscall filter: the default profile blocks dozens of dangerous and rare syscalls, narrowing the "doors" to the kernel and thus the attack surface.
- **LSM (AppArmor / SELinux)** — Linux security modules. Unlike ordinary file permissions granted by an owner, these are a separate ruleset set centrally by the admin for the whole system; not even root inside can override them. The last line of defense once the first two layers are bypassed.

The `--privileged` flag strips almost all of this at once — effectively "turn off security". Use it only when unavoidable and knowing what you open.

## Block 1 summary

- A container is an ordinary process on a shared kernel; no second OS inside.
- namespaces give a private view: pid, net, mnt, user.
- cgroups cap the appetite: memory hits with a kill (OOMKilled), CPU with delay (throttling).
- Privileges tighten via three layers: capabilities, seccomp, LSM.
- Engines (`docker`, `containerd`, `runc`) only configure the kernel.

---

## Block 2. Image and the runtime stack

### What an image really is

An image is not a zip archive or a whole-disk snapshot. Image = a set of layers (each describing filesystem changes) + metadata (a manifest — the table of contents, and a config — the run parameters). The format is standardized by the OCI (Open Container Initiative), so an image built by Docker runs fine under `containerd` or Podman. An image is immutable: once built it does not change; any edit makes a new image.

### Layers and content addressing

A layer is an archive of filesystem changes relative to the layer below; order matters. The `api` image bottom-up: base environment → dependencies → the `api` binary → metadata. Everything is addressed by a content hash — the digest (`sha256:3f2a...`). Three consequences:

- **deduplication** — an identical layer is stored and pulled exactly once (a base layer is shared across many images; that is why on `pull` half the layers are "already present");
- **immutability by math** — change a byte → different hash → different object;
- **built-in integrity check** — download a layer, hash it, compare: a match means nothing was corrupted or swapped.

### overlayfs: turning layers into a filesystem

Layers are separate file sets, but the process needs one root. `overlayfs` builds it: it stacks directories and presents them as one. Image layers mount below as read-only; a thin writable layer, private to the container, sits on top; the merged result is what the container sees as its root — the one `pivot_root` switches to. Writes use copy-on-write: change a file from a lower read-only layer and it is first copied up into the writable layer, then edited there (so editing a large file from the base layer is costly).

### The writable layer is ephemeral: data goes in volumes

The thin writable layer lives exactly as long as the container. Delete the container, recreate the pod — whatever was written there is gone. So anything that must outlive the container (`postgres` data, uploaded files) goes into volumes, not the writable layer. A volume is separate storage mounted into the container; it is not part of the image and lives its own life. This is why a database must not run without a volume — the first restart wipes the data.

### Tag vs digest

An image has two kinds of reference. A **tag** (`api:1.4`, `api:latest`) is a human-readable label — convenient but treacherous: it can move to another image at any time (someone rebuilds and pushes under the same tag). `latest` is not "the newest version" but simply the default tag. A **digest** (`api@sha256:...`) is an immutable address of specific content that never moves. For production and reproducibility — in Kubernetes manifests — reference by digest (pinning): then you run exactly the image you verified.

### Who starts the container: the runtime stack

Between command and process is a chain with a division of labor:

`docker (CLI) → dockerd → containerd → containerd-shim → runc → process`

- **runc** — takes the unpacked image and container spec, does the Block 1 work (namespaces, cgroups, privileges, `pivot_root`, launch), and exits;
- **shim** — stays to watch the running container;
- **containerd** — a daemon that manages images and container lifecycle, calls runc;
- **dockerd** — a convenient wrapper: CLI, builds, networks, volumes.

### Why runc exits and why the shim exists

runc does the low-level work and exits — it does not linger by the container. If `containerd` were the containers' direct parent, restarting it would sever the link: control, logs, exit codes (the containers would not die — the host's init would re-adopt them — but `containerd` would no longer be their parent). So a `containerd-shim` sits in between, a thin broker per container. It becomes the container process's lasting parent, holds its output streams (stdout/stderr) so logs are not lost, keeps the exit code, and — crucially — survives a `containerd` restart: containers keep running under their shims, and the restarted `containerd` simply reconnects to them and regains control. The point of the layer is to decouple the daemon's fate from the containers'.

### The full docker run path

You type `docker run api`. The `docker` client sends a request to the `dockerd` daemon. `dockerd` asks `containerd` to create a container. `containerd` assembles the root from layers via `overlayfs`, builds the container spec with all namespaces and limits, and starts a shim. The shim calls `runc`. `runc` creates namespaces/cgroups/privileges, does `pivot_root`, launches the `api` process — and exits. `api` keeps running under the shim.

### Where Kubernetes fits: CRI

Kubernetes knows nothing about Docker. Each node runs the kubelet agent, which talks to the runtime through the standard **CRI (Container Runtime Interface)** — a contract of "create a pod, start/stop a container, give status":

`kubelet → CRI → containerd (or CRI-O) → shim → runc → process`

There used to be a dockershim layer to Docker; Kubernetes 1.24 removed it. That is the "Kubernetes drops Docker" news: only the shim to the Docker daemon was removed, while the OCI image format, `containerd`, and `runc` stayed — images work as before. In practice, a modern cluster node runs `containerd`, and the Docker daemon is not needed there.

## Block 2 summary

- Image = layers (filesystem changes) + manifest + config, all addressed by hash (dedup and integrity).
- `overlayfs` builds the root: read-only layers below + a thin writable layer on top, copy-on-write; the writable layer is ephemeral → data goes in volumes.
- Launch runs the `dockerd → containerd → shim → runc → process` stack; at the bottom runc does the Block 1 work.
- Kubernetes uses the same bottom via CRI, with no Docker daemon.

---

## Block 3. Building, delivering, and boundaries

### Building an image: from the old builder to BuildKit

The classic `docker build` ran Dockerfile instructions strictly in order, and almost each produced a layer. The cache was linear: while a line and its context were unchanged the layer came from cache, but from the first changed line the whole tail below was rebuilt. Hence the optimization rule: rarely changing things (installing dependencies) go higher, frequently changing things (copying code) go lower; then the heavy layers come from cache and only a light tail rebuilds. Modern **BuildKit** (now the Docker default) is smarter: it reads the Dockerfile as a dependency graph, runs independent steps in parallel, caches more precisely, and — importantly for security — mounts secrets (e.g. a token for a private dependency) only for the duration of a step so they never end up in image layers.

### Multi-stage and a thin image

A multi-stage build has several stages in one Dockerfile. The build stage holds the full environment (compiler, dependencies) and produces the `api` binary. The final stage starts from a minimal base and copies in only the finished binary — nothing heavy from the build stage reaches it, since the final image takes only what is explicitly copied. Result for `api`: instead of hundreds of MB with a toolchain, single or tens of MB. Smaller image → faster pull → faster pod startup on rollout and scaling; and less clutter → smaller attack surface and fewer foreign vulnerabilities. For the minimal base, distroless (no package manager or shell) or empty scratch is common.

### Delivery: registry and pull

A registry stores and serves images: Docker Hub, GitHub Container Registry, self-hosted Harbor, cloud registries. `docker pull` step by step:

1. fetch the manifest (the image's table of contents);
2. read the list of layers with their hashes;
3. per layer: if the hash is already local, skip; otherwise download;
4. verify each downloaded layer's hash, assemble the image.

So similar images pull fast — only what is missing is downloaded. `push` is the mirror image: the manifest and missing layers are uploaded, while anything already present by hash is not re-uploaded.

### Where isolation leaks

The container boundary is thinner than a VM's:

- **shared kernel** — a host-kernel vulnerability is in principle reachable from any container (unlike VMs with separate kernels);
- **not everything is namespaced** — parts of `/proc`, `/sys`, and some kernel parameters are visible or influential across the boundary (that is why seccomp and LSM are needed — to cover what namespaces do not isolate);
- **root without a user namespace = root on the host** on escape.

The conclusion is not "containers are insecure" but more precise: an ordinary container isolates your own, trusted code excellently. For foreign, untrusted code (building others' artifacts, letting users run their own code) the ordinary boundary may not be enough.

### When an ordinary container is not enough: hardened runtimes

Every solution shares one idea — give the container something like its own kernel so it does not go straight to the host kernel:

- **gVisor** — intercepts the container's syscalls and handles them in its own mini-kernel in user space, keeping dangerous calls away from the host kernel;
- **Kata Containers** — runs the container inside a very lightweight VM (literally its own kernel — real isolation, but overhead closer to a VM);
- **rootless containers** — the whole stack runs without root on the host, resting on the user namespace.

The trade-off is simple: stronger isolation means higher overhead. Choose by task: your own code usually needs only an ordinary container; untrusted code gets a hardened runtime.

### Trusting an image: signature and provenance

A hash (digest) gives integrity (the content is exactly what is claimed) but does not answer trust — who built the image. So on top of the image you add:

- **image signature** (e.g. `cosign`) — cryptographically confirms who built it and that it was not tampered with in transit;
- **provenance** — origin details: which repository, which pipeline, which commit;
- **SBOM** (software bill of materials) — a list of what is inside (packages and versions), so a new high-profile vulnerability can be checked against the image in a minute.

Signatures and the supply chain are covered in the security lectures.

## Block 3 summary

- BuildKit builds by graph with precise caching and secrets kept out of layers; multi-stage + a minimal base give a thin image.
- Delivery goes through a registry; `pull` downloads only what is missing, by hash.
- The isolation boundary is thinner than a VM's because the kernel is shared; for untrusted code strengthen it with gVisor, Kata, or rootless.
- A hash gives integrity; trust comes from a signature and provenance.

---

## Summary

- A container is an ordinary process on a shared kernel: namespaces (what it sees) + cgroups (how much it may use) + privileges (what it may do).
- Memory hits with a kill (`OOMKilled`), CPU with delay (throttling).
- An image is layers + metadata, all by hash; `overlayfs` builds the root; data lives in volumes.
- Launch: `dockerd → containerd → shim → runc → process`; Kubernetes reaches the same bottom via CRI.
- Building with BuildKit + multi-stage; delivery via a registry; isolation is real but thinner than a VM's due to the shared kernel.
- Next — how to observe all this: metrics, logs and traces (the next lecture), then orchestration — Kubernetes and its control plane.

> Lab: build a container by hand and keep a service within limits — see [lab.md](lab.md).
