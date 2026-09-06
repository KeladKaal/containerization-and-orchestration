# Lecture 5 — Kubernetes networking

This lecture answers a question left open after the resources lecture: pods are constantly recreated and change their addresses — so how does anything keep working? From the earlier lectures we know that each container has its own network stack (net-namespace) with its own interfaces, IPs, and ports, and that pods are ephemeral: they are recreated on updates, moved to other nodes on failure, and get a new IP each time. Hence two problems: how pods on different servers see each other at all, and how to rely on an address that keeps changing. We work through it on a running example — the `shop` online store: an `api` service in three replicas across different nodes, a `worker`, and a `postgres` database. The store needs three things from the network: the three `api` replicas reachable by clients under one stable address; `api` and `worker` reliably finding `postgres` despite recreations; and only `api` and `worker` allowed to connect to `postgres`, not every pod in the cluster. We go bottom-up, in three blocks.

First, three basic words. An `IP address` is the numeric address of a machine or pod on the network (e.g. `10.1.2.3`); packets use it to find the recipient. A `port` is a "door" number at that address: many programs share one IP, each listening on its own port (`api` on `8080`, `postgres` on `5432`); the pair "IP plus port" points to one specific service. `NAT` (network address translation) is when someone rewrites the address in a packet along the way; it is a common source of "why does the recipient see a different address than I sent from" confusion, and one goal of the Kubernetes network model is to avoid NAT between pods.

---

## Block 1. The network model and CNI

### The flat network: the core rule

All of Kubernetes networking rests on a surprisingly simple rule, called the flat network. First: every pod has its own unique IP address across the whole cluster, not per node. Second: any pod can reach any other pod directly by its IP, with no address translation, even if they are on different physical nodes. Third: a pod sees its own address exactly as others see it — no NAT. It is as if all pods were on one big shared network where anyone can call anyone. Important caveat: Kubernetes does not build this network itself — it only states the rule "it must be so"; a separate component, the network plugin, actually connects the pods.

### How a pod gets network on a node

A pod has its own net-namespace — an isolated network stack, empty at start. To connect it, a `veth` pair is created — a virtual patch cord with two ends. One end goes inside the pod, where it appears as `eth0`, the pod's network card. The other end stays on the node and plugs into a bridge — a virtual switch inside the node that all its pods connect to, just as a real switch connects office computers. The pod gets an IP from the range assigned to that node. So pods on one node see each other through the bridge.

### How pods on different nodes see each other

Each node is assigned its own non-overlapping address range for its pods (e.g. node A — `10.1.1.0/24`, node B — `10.1.2.0/24`). The slash notation is the range size: `/24` means 256 consecutive addresses. Since ranges do not overlap, a pod's address tells you which node it is on. To deliver a packet from node A to a pod on node B there are two main approaches. Routing: nodes know in advance which range lives on which node, and the packet goes straight over the ordinary network between servers, no repackaging — fast, but the inter-node network must allow it (example — Calico). Overlay: a pod's packet is wrapped whole inside an ordinary inter-node packet, like a letter in an envelope, sent to the right node and unwrapped there — works almost everywhere, but you pay small overhead for the wrap/unwrap (example — Flannel in VXLAN mode). Do not confuse the terms: a bridge (switch) connects pods within one node; a router passes traffic between networks, i.e. between nodes.

### CNI: who configures the pod's network

`CNI` (Container Network Interface) is the standard socket between Kubernetes and whatever actually configures the network. When `kubelet` on a node creates a pod, it calls the CNI plugin at the right moment, and the plugin does all the work: creates the `veth` pair, connects the pod to the network, assigns an IP from the node's range, and sets up routes to other nodes. So Kubernetes only states the rule "produce a flat network"; how to build it is decided by the chosen plugin — Calico, Cilium, Flannel, and others — which differ in exactly the inter-node delivery method and extra features. This is the same recurring pattern as `CRI` for runtimes (Lecture 1) and `CSI` for storage (the resources lecture): a standard interface with pluggable implementations.

### IPAM: handing out addresses

`IPAM` (IP Address Management) is the part of CNI's job that hands out addresses: which pod gets which IP and how to avoid two pods getting the same one. The mechanics are simple: each node has its own range; a pod is created — IPAM gives it a free address from the node's range; a pod is deleted — the address is freed and later reused by another pod. A direct consequence, which becomes Block 2's main problem: a pod's IP is temporary, alive only as long as the pod. You cannot rely on a specific pod IP — today it is `postgres`, tomorrow it is a different pod.

---

## Block 2. Service, load balancing, DNS

### The problem: you cannot rely on a pod's IP

We want `api` to reach `postgres`, and clients to reach the three `api` replicas. What gets in the way: a pod's IP changes on every recreation; there are several replicas, each with its own IP; pods come and go during scaling and updates. We need a stable address that stands in front of a group of pods, never changes, and always knows the current list of live pods behind it. In Kubernetes that address is the `Service` object.

### Service: a stable address in front of pods

A `Service` gives a group of pods one shared stable address — a `ClusterIP`. It is virtual: no specific machine sits behind it, it is just a permanent entry point that does not change for the service's lifetime. Which pods belong to the group is decided by labels: you put a label on the pods (e.g. `app=api`) and give the Service a `selector` — "my pods are the ones with this label" — and the Service automatically catches all matching pods. The beauty: a pod is recreated with a new IP — the Service updates its own list, while its own address, the `ClusterIP`, stays the same. The client always talks to the stable address and never knows the pods behind it come and go.

### EndpointSlice: the current list of pods

How does the Service know its live pods' real addresses at any moment? A separate object handles that — the `EndpointSlice`. A controller watches pods matching the `selector` and keeps in the EndpointSlice a current list — the IP and port of each live, ready pod. "Ready" is the key word: a pod enters the list not when it merely started but when it passed its readiness probe; as soon as a pod fails or is recreated, it is removed and traffic stops going to it. Keep two things apart: a `Service` is a stable address plus a label-based selection rule; an `EndpointSlice` is the continuously updated list of concrete addresses that this address maps to right now.

### kube-proxy: turning a ClusterIP into real pods

A `ClusterIP` is virtual — literally nobody listens on it. So something must intercept traffic going to the ClusterIP and redirect it to the real IP of one of the live pods from the EndpointSlice. That is `kube-proxy`, a component on every node. The name is a bit misleading: "proxy" suggests it passes all traffic through itself, but usually it does not. In fact kube-proxy configures rules in the node's kernel, and then the kernel itself catches packets addressed to the ClusterIP on the fly and rewrites the destination to one of the pods. A "kernel rule" in plain words is an entry like "a packet going to this ClusterIP, send it to this real pod IP." kube-proxy constantly watches the EndpointSlice: the list changed — it immediately rewrites the rules so traffic only goes to current pods. The kernel moves the traffic, fast and with no extra hops. Since there are several pods, the kernel picks now one, now another — round-robin or random; that gives you load balancing for free. It is simple, not "smart": the kernel does not look at pod load, it just spreads evenly. Smart, request-level balancing is a job for Ingress or a mesh, in the next lecture.

### iptables vs IPVS

kube-proxy has two main modes — what it uses to write kernel rules. `iptables` is the classic Linux mechanism for filtering and rewriting packets: everywhere and reliable, but rules are checked as a list, one by one. With dozens of services it is unnoticeable; with thousands the list grows and the kernel walks a long chain per packet — this slows things down and updates slowly. `IPVS` (IP Virtual Server) is a kernel mechanism built for load balancing: it keeps services in a hash table, finds the right rule at once without scanning, and on large clusters is noticeably faster and more stable. On a small cluster you will not notice the difference and iptables is fine; on a large one (thousands of services) IPVS is markedly better.

### DNS: finding a service by name

Even a stable `ClusterIP` is a number, awkward and fragile to hardcode. So every Service has a name, and the cluster runs its own DNS. DNS is the network's phone book: a service that turns a human-readable name into an IP address. In Kubernetes that role is played by `CoreDNS`. Thanks to it, `api` connects to the database just by the name `postgres`: it asks CoreDNS for `postgres`'s IP, gets the service's ClusterIP, and goes there — no hardcoded addresses. The full name looks like `service-name.namespace.svc.cluster.local`, but if `api` and `postgres` are in the same namespace, the short name `postgres` is enough. This solves the lecture's second task: `api` finds `postgres` by a stable name, whatever happens to the pods.

### Service types: outward and inward

`ClusterIP` (the default) — the address is visible only inside the cluster, for service-to-service traffic (`api` → `postgres`). `NodePort` — opens the same port on every node, so from outside you can reach any node's address on that port; simple but crude. `LoadBalancer` — the production way to let traffic in from outside: Kubernetes asks the cloud for a real external load balancer that directs external traffic into the service; this is how `api` is usually published outward. `headless` (no ClusterIP) — DNS returns not one virtual address but the list of all the pods' IPs; needed when the client cares about individual pods rather than "any of the group." It almost always pairs with a `StatefulSet`: that gives pods stable names (`postgres-0`, `postgres-1`), and the headless service gives each its own stable DNS name so you can address a specific replica — e.g. write to the primary, read from a replica.

---

## Block 3. Policies and eBPF

### By default — everyone talks to everyone

From a security angle, the flat network from Block 1 means: by default any pod can reach any pod, no restrictions. Convenient for development, bad for security. Imagine an attacker finds a vulnerability in the public `api` and takes over the pod. Since the network is flat, from that pod they see everything in the cluster over the network — including reaching `postgres` directly to try to steal data. We would prefer that only `api` and `worker` could connect to `postgres`, with other pods blocked at the network level. That is exactly what network policies do.

### NetworkPolicy: who may talk to whom

A `NetworkPolicy` is a rule object describing which pod may talk to which: by labels, ports, and traffic direction (ingress or egress). In words, the store's policy reads: "to pods labeled `app=postgres`, incoming connections are allowed only from pods labeled `app=api` and `app=worker`, and only on port `5432`; deny everything else." Then a compromised public pod without the right label simply cannot open a connection to the database. Two important subtleties. First: policies operate on labels, not IP addresses — sensibly so, since IPs change but labels are stable, and you describe rules in terms of pod roles. Second, the one everyone trips on: while no policy applies to a pod, it is open to everyone (the flat network); but as soon as a pod falls under even one policy, the principle "whatever is not explicitly allowed is denied" kicks in — everything the policy does not permit is blocked for that pod. So policies are usually added in pairs: first "deny all," then "allow what is needed."

### Who actually enforces policies

A `NetworkPolicy`, like everything in Kubernetes, is only a description of the desired state in the API: you create the rule object and it is stored. By itself it blocks nothing — the rule must actually be enforced, written into the nodes' kernels, by the CNI plugin from Block 1. And here is the trap: not all plugins can enforce NetworkPolicy. Advanced ones like Calico and Cilium can; some simple ones, e.g. plain Flannel, cannot at all. With such a plugin you can create a policy, `kubectl` accepts it, it sits in the cluster — but it does nothing, silently, without a single error: you think you closed access to the database while it stays open. Conclusion: the very ability to restrict traffic depends on the chosen CNI, and this must be checked in advance.

### Limits of the classic stack

The classic combination — iptables rules from kube-proxy plus policy rules — starts to hurt at scale. First: long rule chains cause latency and slow updates at thousands of services. Second: everything is described in terms of IPs and ports, though we think in terms of pods, services, and labels — a large gap between our model and what is in the kernel. Open the iptables rule list on a node and you get a wall of bare IP addresses and ports, not a single human name; to tell which rule is about which pod you must map addresses by hand. Hence the third pain: weak visibility — it is hard to tell who really talks to whom and why something does not connect. We want something that runs close to the kernel and is therefore fast, yet understands the Kubernetes context and gives visibility. That came with `eBPF`.

### eBPF in plain words

`eBPF` is a mechanism that lets you safely run small programs right inside the Linux kernel, without changing its source or loading modules. Previously, to change kernel behavior you had to either patch the kernel source or load a module — both risky, since a bug takes down the whole kernel. eBPF is different: the kernel verifies the program for safety before running it, so it cannot crash the kernel. Such programs hook onto events — a packet arrives, a syscall happens — and handle them on the spot, in the kernel, and therefore very fast. For networking this is a game changer: a decision about a packet (where to send it, whether to pass it) can be made right in the kernel, flexibly and with Kubernetes context, instead of walking long static chains. eBPF is not one tool but a platform on which new networking, observability, and security solutions are built.

### Cilium: networking on eBPF

`Cilium` is a CNI plugin built on eBPF that brings together everything above. First, it can fully replace `kube-proxy`: it does service balancing via eBPF programs in the kernel instead of long iptables chains, and on a large cluster this is noticeably faster — the third path after iptables and IPVS. Second, network policies on eBPF, not only at the "port open or closed" level but at the application level. Here the term `L7` is useful: networks are split into layers, from the lowest (wires and signals) to the highest; the seventh, top layer L7 is the application layer, i.e. the request content (which HTTP method, which URL path, which headers). IPs and ports are low layers where you know "where to deliver" but not "what is inside." Working at L7 lets you decide based on the request itself — e.g. "this pod may only `GET /health`, everything else to this service is denied" — far finer than the crude "port open or closed." Third, observability: Cilium has a component, `Hubble`, that shows who really talks to whom, which connections pass, and which are blocked by policies — directly solving the blindness of the classic stack. Because of all this, Cilium has largely become the de facto standard for new clusters.

---

## Summary

- Flat network: every pod has its own unique IP, everyone sees everyone directly, no NAT.
- Within a node pods are connected via `veth` pairs and a bridge; between nodes traffic is delivered by routing or overlay.
- All of this is done by the CNI plugin — a standard socket with pluggable implementations, like `CRI` and `CSI`.
- A pod's IP is temporary, so a stable layer is built on top: `Service` (`ClusterIP`) selects pods by labels, `EndpointSlice` keeps the list of live ready pods, `kube-proxy` configures kernel rules (`iptables`/`IPVS`), and the kernel balances.
- `CoreDNS` lets services be found by name, so `api` reaches `postgres` by name, not address.
- Service types for different needs: `ClusterIP`, `NodePort`, `LoadBalancer`, `headless`.
- Traffic is restricted via `NetworkPolicy` (by labels, "whatever is not allowed is denied"), but the CNI enforces it, and not every one can — check.
- The classic iptables stack slows and goes blind at scale; the industry's answer is `eBPF` and Cilium built on it: a fast data plane, fine-grained policies up to L7, and observability (`Hubble`).

> Lab: build a zero-trust network and prove it holds — see [lab.md](lab.md).
