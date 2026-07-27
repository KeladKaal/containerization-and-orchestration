# Lecture 2 — Images, runtimes and engines

Where the container's filesystem comes from, and the whole stack from engine down to process. The OCI image and runtime specs, and why there are so many layers between `docker run` and a running process.

## Blocks

1. **Images from the inside** — OCI Image Spec, layers, content-addressable storage, overlayfs; manifest and config; cache and reproducibility.
2. **From engine to process** — OCI Runtime Spec and runc; the dockerd → containerd → shim → runc stack; why the shim exists; CRI and where kubelet fits.
3. **Building and shipping** — BuildKit (build graph, cache, parallelism); multi-stage builds; the registry protocol, distribution, pull/push.

> Self-study notes and the lab will be added later.
