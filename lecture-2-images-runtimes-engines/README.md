# Lecture 2 — Images, runtimes and engines

Lecture 1 left one question open: where does the filesystem that the runtime hands the process as its root come from? This lecture answers it and walks the whole stack — from `docker run` down to a running process.

Three topics: how an image is built, who turns an image into a running container, and how an image is built and shipped between machines. Running example — the image of the `shop` `api` service; for concreteness `api` is a compiled service (say, in Go): essentially one binary plus a little environment.

---

## Block 1. Images from the inside

### What an image is

An image is **not** an archive or a full disk snapshot. It is a set of **layers** plus metadata — a **manifest** and a **config**. All of it is described by the **OCI Image Spec** and addressed by content hashes. An image is **immutable**: built once, never changed; any change yields a different image with a different hash.

The format used to be Docker-internal. When containers became an industry standard, a vendor-neutral spec was needed — the **Open Container Initiative (OCI)** appeared in 2015 with three specs:

- **Image Spec** — how an image is structured (layers, manifest, config);
- **Runtime Spec** — how to start a container from an unpacked image;
- **Distribution Spec** — how images are stored and moved via a registry.

That is why Docker, containerd, Podman and CRI-O all work with the same images.

### Content addressing

Storage is built on **content-addressable storage**: an object's address is the hash of its content (usually sha256). Such a hash is called a **digest**, written `sha256:3f2a...`. Consequences:

- **deduplication** — identical content has one digest, stored/pulled once (a shared base layer is physically one);
- **immutability** — change one byte → different digest → a different object;
- **integrity** — pull a layer, compute sha256, compare with the digest from the manifest.

Storage, distribution, and (later in the course) trust all rest on this.

### Layers, manifest, config

A **layer** is a tar archive of filesystem changes relative to the layer below: added/changed files are stored as-is, and a deletion is encoded with a **whiteout** marker. Layers stack rather than replace, and order matters. The `api` image bottom-up: base environment → dependencies → the `api` binary → run metadata.

The **manifest** is a small JSON "table of contents": a digest reference to the config, an ordered list of layers by digest, media types. The manifest itself is addressed by digest — and **the manifest's digest is the image's real address**.

The **config** is the JSON "passport": run parameters (`ENTRYPOINT`, `CMD`, `ENV`, `WORKDIR`, `USER`, ports), the unpacked layer list (`rootfs`), build history, target platform (arch + OS). The runtime reads the config when deciding what and how to run.

### How layers become a filesystem: overlayfs

Layers are separate file sets; the process needs one root. **overlayfs** assembles it by stacking directories:

- **lowerdir** — image layers, read-only (many, stacked);
- **upperdir** — the container's thin writable layer;
- **merged** — the combined view the container sees as its root.

This is the root from lecture 1 that the runtime `pivot_root`s onto. Writes use **copy-on-write**: a read takes the file from a lower layer without copying; changing a lower-layer file first copies it up (**copy-up**), then edits it there; a deletion writes a whiteout. Crucially, **upperdir is ephemeral** — delete the container, lose what was written. So anything that must outlive the container (`postgres` data, uploads) goes into **volumes**, not the writable layer.

### Tag vs digest

An image has two references. A **tag** (`api:1.4`, `api:latest`) is a human-readable label that **can move** to another image at any time. `latest` is not "newest", just the default tag. A **digest** (`api@sha256:...`) is the immutable address of specific content. For reproducibility and security (prod, Kubernetes manifests, supply chain), reference **by digest** (pinning).

One tag serves multiple architectures via an **image index** (manifest list) — an index over manifests; the client picks the manifest for its platform (`linux/amd64`, `linux/arm64`).

---

## Block 2. From engine to process

Between the command and the process is a chain: `docker` (CLI) → `dockerd` → `containerd` → `containerd-shim` → `runc` → process. Not redundancy, but separation of concerns. Bottom-up:

### runc — the low-level runtime

**runc** is the reference implementation of the **OCI Runtime Spec**. Its input is a **bundle**: `rootfs` (the unpacked image root, a plain directory) and `config.json` (the spec: which namespaces, cgroups, capabilities, seccomp, what to run). runc does **exactly what we assembled by hand in lecture 1** via `unshare`/`clone`, but from a standardized description — creates namespaces, applies cgroups, drops privileges, does `pivot_root`, starts the process — and then **exits**. It does not supervise the container.

### containerd — the container and image manager

**containerd** is the node's daemon workhorse: images (pull/store/registry), the **snapshotter** (prepares `rootfs` via overlayfs from layers), the container lifecycle (create/start/stop/delete). It delegates the actual start to runc. It is self-sufficient: runs under Docker and directly under Kubernetes. Own CLIs — `ctr`, `nerdctl`.

### shim — why the middleman exists

If containerd were the direct parent of containers, restarting it would take them all down (in Linux, a parent's exit hits its children). The fix is **containerd-shim**, a thin per-container middleman. runc starts the process and leaves; the **shim stays as parent**. It holds the I/O streams, keeps the exit code, and — key — lets containerd restart without touching running containers (they live under their shims; containerd re-attaches). When a container exits, the shim reports up.

### dockerd — the convenience wrapper

**dockerd** (Docker Engine) is the developer-facing top layer: a friendly CLI/API (`docker run`, `docker build`, `docker compose`), Docker networks, volumes, builds. To run containers it calls containerd. Takeaway: **Docker is a UX wrapper** over the stack, not the stack; containerd and runc work without it.

### Full path and where Kubernetes fits (CRI)

`docker run api` → CLI → `dockerd` → `containerd` prepares `rootfs` (overlayfs) and `config.json`, starts a `shim` → `shim` calls `runc` → `runc` creates namespaces/cgroups/privileges, does `pivot_root`, starts `api` → `runc` exits, `api` remains under the `shim`.

Kubernetes does not know about Docker. On the node runs **kubelet**, talking to the runtime over the **CRI (Container Runtime Interface)** gRPC contract: `kubelet → CRI → containerd (CRI plugin) or CRI-O → shim → runc`. There used to be a **dockershim** to Docker; Kubernetes 1.24 removed it — dropping a needless layer, while OCI images, containerd and runc stayed. A modern cluster node runs containerd or CRI-O; Docker as a daemon is not needed.

**The point:** underneath it is the same stack as `docker run`. Only who pulls the string on top differs — a human via `dockerd`, or `kubelet` via CRI.

---

## Block 3. Building and shipping

### The classic builder and its limits

The old `docker build` ran Dockerfile instructions strictly in order, almost each one a new layer; the cache was linear (change a line → rebuild it and the whole tail below). Hence the habit: rarely-changing (dependencies) higher, often-changing (code) lower. Limits: no parallelism of independent steps; a coarse cache; **a secret that lands in a build layer stays in the image forever** (deleting it in a later instruction doesn't help — the earlier layer remains); hard to cache intermediates between builds.

### BuildKit

**BuildKit** is the modern engine (default in current Docker):

- the Dockerfile compiles into a **dependency graph** of steps, not a linear list;
- independent branches build **in parallel**;
- a precise, content-based **cache**, exportable/importable (key for CI on a clean machine);
- **mount caches** for packages and compile artifacts, surviving rebuilds;
- **secrets and SSH** are mounted for a step only and **never land in layers**.

### Multi-stage and a thin image

A **multi-stage build** has several stages in one Dockerfile. The `builder` stage carries the full build environment (compiler, dependencies) and produces the `api` binary; the final stage starts from a minimal base into which **only the finished binary** is copied. Nothing heavy from `builder` reaches the final image. Result: instead of hundreds of MB with a toolchain — single/tens of MB.

Good-image practices: minimal base (**distroless** — no package manager or shell; **scratch** — empty, for statically linked binaries); layer order for caching; `.dockerignore` (don't ship `.git`, `node_modules`, secrets); pin the base image by digest. The principle: put nothing in the image "just in case" — smaller attack surface, fewer updates.

### Registry and distribution

A **registry** follows the **OCI Distribution Spec** and is essentially two stores: **blobs** (layers and configs, by digest) and **manifests** (by digest and by tag). It serves an image in parts, not whole.

`docker pull api:1.4` step by step:

1. authenticate — get a token for the repository;
2. request the manifest by tag (or an index → pick the platform);
3. read the layer list and config address;
4. per layer: if its digest is already local — **skip**, else download the blob;
5. verify each layer's sha256, assemble the image.

`push` is the mirror: manifest and missing blobs are uploaded; anything already present by digest is not re-sent. Fast repeat pulls follow directly from content-addressable storage.

### Integrity and trust

**A digest gives integrity** (the exact content), but **not trust** (who built it). On top of the image: a **signature** (e.g. `cosign`) cryptographically attests the author/build; **provenance** — where and how it was built (pipeline, commit); an **SBOM** — what's inside (packages and versions), for vulnerability checks. Covered in depth in lecture 7.

---

## Summary

- An image is layers (FS changes) + manifest (index) + config (how to run); all by digest; overlayfs assembles the live FS (lower RO + upper RW, copy-on-write, upper ephemeral).
- Startup goes through `dockerd → containerd → shim → runc → process`; at the bottom runc does the lecture-1 work. Kubernetes uses the same bottom via CRI, without Docker as a daemon.
- Building — BuildKit (graph, parallelism, precise cache, secrets out of layers) + multi-stage + a thin base.
- Shipping — a registry over the Distribution Spec; pull fetches only what's missing, by digest; integrity is built into addressing, while signatures and provenance add trust.

Next — orchestration: how the Kubernetes control plane decides where and when to run containers (lecture 3).

> The lab will be added later.
