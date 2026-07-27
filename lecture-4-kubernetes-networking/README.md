# Lecture 4 — Kubernetes networking

How packets actually move in a cluster. The flat pod network and CNI, how Services and load balancing work under the hood, and where the industry is heading with eBPF.

## Blocks

1. **The network model and CNI** — the flat pod network, veth and bridges, what a CNI plugin does when a Pod is created; IPAM.
2. **Services and load balancing** — ClusterIP, EndpointSlices; kube-proxy: iptables vs IPVS; DNS and CoreDNS; headless services.
3. **Policies and eBPF** — NetworkPolicy and who enforces it; Cilium and the eBPF data plane as an alternative to kube-proxy; where things are heading.

> Self-study notes and the lab will be added later.
