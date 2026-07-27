# Lecture 1 — Container fundamentals

What a container actually is: an ordinary Linux process placed under kernel restrictions, not a lightweight VM. We take `docker run` and go down to the kernel mechanisms that make a process "a container".

## Blocks

1. **A container is a process** — the "lightweight VM" myth; VM vs container; `docker run` seen from the host; the three mechanisms a container is built from.
2. **namespaces — what a process sees** — pid, net, mnt, uts, ipc, user, cgroup; the root filesystem and `pivot_root`; how it's created (`clone` / `unshare` / `setns`).
3. **cgroups and hardening — what a process is allowed** — cgroups v2, memory/CPU limits and OOM vs throttling; capabilities; seccomp; LSM (AppArmor/SELinux); where the isolation leaks.

> Self-study notes and the lab will be added later.
