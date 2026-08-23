# Lecture 5 — Service mesh and advanced traffic

This lecture answers one question: how do you get encryption, reliability, fine-grained traffic control, and deep visibility between services — without rewriting the code of every service? Lecture 4 gave us two things from the Kubernetes network: connectivity (pods find each other via `Service` and DNS) and coarse "who talks to whom" rules via `NetworkPolicy`, at the level of labels and ports. As a system grows, that stops being enough. We use one running example throughout — an online shop `shop` with services `api` (×3), `worker`, and `postgres` — and look at what a service mesh adds between them, how it works inside, what it really gives you, and what it costs.

---

## Block 1. Why mesh: data plane, control plane, and sidecar

### What the plain network lacks

The network from last lecture gives connectivity and coarse access rules. As the system grows, you want four more things. First, encryption between services: inside the cluster everything travels in plaintext by default. Second, request-level reliability: automatic retries on failure, timeouts, and protection against cascading failures, where one failing service drags down the whole chain that depends on it. Third, fine-grained routing: send 5% of traffic to a new version of `api` and check nothing broke (a canary rollout). Fourth, deep visibility: not "a packet went through," but "which request, to which service, how long it took, with what response code." And ideally all of this without touching each service's code.

### The pain without mesh: the same thing in every service

Without a mesh, all of this lives in each service's code, usually via libraries: one library for retries, one for timeouts, one for metrics, one for encryption. At scale this brings four problems. Duplication — the same infrastructure logic written again and again. Inconsistency — services in different languages (`Go`, `Python`, `Java`), each with its own library, behavior, and bugs; uniformity is nearly impossible. Expensive change — to update, say, the retry policy company-wide you must rebuild and redeploy every service, because the logic is baked into each one. And developers spend time on infrastructure instead of business logic. The natural idea: lift all traffic work out of the code into a separate layer. That is the mesh.

### L4 and L7: which level we work at

To talk about mesh you need two network levels. `L4` (transport) is the level of addresses, ports, and connections: "deliver the packet to IP:port, open a TCP connection." It knows nothing about the request's contents. All the networking from last lecture — `Service`, `kube-proxy`, load balancing — lives at `L4`. `L7` (application) is the level of the request itself: HTTP method, URL (e.g. `/api/orders`), headers, response code (`200`/`500`). Here the request has meaning. The key to the whole lecture: the Kubernetes network works at `L4`, the mesh works at `L7`. That is why the mesh can do what the network fundamentally cannot — retry a failed request, route by URL, count response codes — all of which require understanding the request, not just the address.

### The idea of mesh: move traffic into a proxy layer

A service mesh is an infrastructure layer that takes over all inter-service communication at `L7`: encryption, retries, timeouts, routing, metrics. The core trick: next to each service you place a proxy, and all of that service's traffic — inbound and outbound — goes through it. The service itself talks as usual; it doesn't even know about the proxy. It just sends a request, and the proxy intercepts it and does the smart work: encrypts, retries on failure, measures latency, routes it. The service code never changes. This web of proxies around all services is the service mesh.

### Sidecar and Envoy

The classic way to place a proxy next to a service is the sidecar: an extra container in the same pod as the main service (we covered sidecars in lecture 2). Now the pod has two containers: your service and the proxy. All of the pod's network traffic is redirected through the proxy, and the interception happens automatically via kernel rules — `iptables` or `eBPF`. `iptables` is a Linux kernel mechanism for defining what to do with network packets; the mesh uses such rules to funnel all pod traffic into its proxy, so the service reconfigures nothing. The sidecar is also injected automatically: the mesh uses the mutating webhook from lecture 2 — you create an ordinary pod with your service, and the webhook mixes in the proxy container on the way in. You add nothing by hand. The proxy is almost always `Envoy` — a fast proxy built for `L7`. It's called programmable: an ordinary proxy takes its rules from a config file and must restart to change them, while `Envoy` can receive new rules over the network on the fly, without restarting. That's why the control plane can change routing and policy for thousands of proxies while they run.

### Data plane and control plane

A mesh has two layers — the same separation idea as in lecture 2. The data plane is all the proxies (`Envoy`) next to the services; they actually carry the traffic, encrypt it, retry requests, count metrics. There are many of them — one per pod. The control plane is the mesh's brain; in the popular `Istio` it's called `istiod`. It's a single central component that hands each proxy its configuration, issues certificates for encryption, and tracks which services and pods exist. It all works in the familiar declarative style: you describe a rule (e.g. "retry failed requests to `api` up to three times") as an ordinary Kubernetes object, the control plane turns it into concrete settings and distributes it to the right proxies, and the proxies execute. Hold onto this split: the data plane carries, the control plane configures.

## Block 2. What mesh gives: mTLS, traffic, observability

### What TLS and mTLS are

`TLS` is connection encryption — the padlock in the browser and the s in `https`. It gives two things: the contents can't be read in transit, and the client is sure it's talking to the real server, not an impostor. Plain `TLS` verifies only the server. `mTLS` (mutual TLS) is the same, but both sides are verified: the client is sure of the server, and the server knows exactly who came to it. In a mesh: the control plane issues each service a certificate. A certificate is a cryptographic proof of identity, like a passport: it says "I am service `api`" and is signed by a trust authority shared across the cluster (the mesh control plane). The pair "who I am plus the authority's signature" is the identity — a cryptographic identity; here certificate and identity are essentially the same thing. It can't be forged: to impersonate `api` you'd need a certificate signed by the trust authority, and forging that signature without the authority's secret key is mathematically impossible. The rest is simple: on connecting, proxies show each other their certificates and verify the authority's signature, so each is sure who it's really talking to, and encrypt all traffic over `mTLS` automatically.

### Why mTLS is valued

First: all traffic between services is encrypted, even inside the cluster. Recall the risk from last lecture — an attacker takes over one pod and sniffs the network; with `mTLS` they see only an encrypted stream. Second: services authenticate each other by cryptographic identity, not by IP address. This matters because pod IPs change and can be spoofed, while identity cannot. The policy "only `api` may reach `postgres`" becomes genuinely reliable: not "from such-and-such IP" but "whoever cryptographically proved they are `api`." Third: all of it happens without a single line in the service code and without manual certificate handling — the control plane issues, distributes, and renews them itself. Manual certificate management is a well-known pain, and the mesh removes it. `mTLS` is, honestly, the most common reason people adopt a mesh.

### Traffic management: canaries and more

Because the proxy understands `L7` and sits in the path of every request, it can manage traffic intelligently. The most common use is traffic splitting for a canary rollout: you say "send 95% of requests to `api` v1 and 5% to v2," watch the new version's metrics and errors, and if all is well, gradually raise its share to 100%. If not, you instantly roll back by switching the share, with no rebuild or redeploy. Next, routing by request attributes: by header or URL path, send, say, beta users to the new version and everyone else to the stable one. And mirroring: send a copy of real traffic to the new version idly, without returning its responses to clients, to check how it holds real load with no risk. All of this is set by declarative rules and changes on the fly, without rebuilding services.

### Reliability: retries, timeouts, circuit breaking

The proxy in each request's path can add several safety mechanisms — again with no code. Retries: if a request fails once (a network blip, a pod being recreated), the proxy quietly retries it and the client notices nothing. Timeouts: don't wait forever on a stuck service; cut it off after a set time and return an error fast, so a queue of waiters doesn't pile up. And circuit breaking: if a service consistently returns errors or doesn't respond, the proxy temporarily stops sending it requests altogether. Why: to give the sick service breathing room to recover, and to prevent a cascading failure — where one downed service drags along everyone waiting on it and the whole system falls in a chain. The breaker cuts that chain. All three are set by rules and work the same for services in any language.

### Observability almost for free

Since all inter-service traffic already passes through the proxy, the mesh sees every request and gives observability with no code changes. Metrics per pair of services: how many requests go from `api` to `postgres`, what latencies, what error rate — the so-called golden signals (more in lecture 6). Traces: the path of a single request through the whole chain and where exactly it slows down — invaluable when "something is slow somewhere." And a service map: a visual diagram of who calls whom, highlighting where it's red; provided by tools like `Kiali` and `Jaeger` on top of the mesh. Observability is a big topic — the next lecture is entirely about it — but the mesh hands you a large part of it for free, simply because all traffic flows through it.

## Block 3. Cost and evolution: from sidecar to ambient and eBPF

### The cost of the sidecar model

Nothing is free — the convenience of the sidecar is paid for on several fronts. Resources: a proxy container next to every service eats memory and CPU; at a thousand pods that's a thousand proxies and a noticeable total cost. Latency: a request from `api` to `postgres` used to go directly; now it exits through the `api` proxy and enters through the `postgres` proxy — two extra hops per request, each adding a little to response time; usually a little, but noticed on latency-critical paths. Operational complexity: the proxy is another component to update and debug, and upgrading the mesh itself usually means restarting all pods. And the "magic": traffic is intercepted invisibly to the app, so when something goes wrong, an engineer without mesh knowledge struggles to see where their request went. These costs are exactly what drove the next evolution.

### Ambient / sidecarless

The answer to the sidecar's cost is a new generation of mesh called ambient or sidecarless; in `Istio` it's ambient mode. The idea: stop stuffing a proxy into every pod, and split functions into two layers. The base layer — `mTLS` and `L4` work — moves into one proxy per node, called `ztunnel`, serving all pods on that node at once; instead of a thousand proxies you now have one per node, overhead drops sharply, and encryption between services is nearly free. The complex `L7` logic (routing, canaries, retries) is enabled not everywhere but only for the services that actually need it, via a separate proxy called a `waypoint`. The point: you pay only for what you actually use. Plus a big operational win — enabling this mesh needs no restart of all pods and no sidecar injection.

### eBPF in mesh

The second direction is `eBPF`, from last lecture: it lets you safely run programs directly in the kernel. Why "in the kernel" is cheaper: ordinary programs, including the `Envoy` proxy, run in user space — the ordinary, unprivileged mode, separate from the kernel. For a packet to pass through such a proxy, the kernel hands it up to user space, the proxy processes it, and the packet returns to the kernel — that round trip costs time, on every packet. `eBPF` runs its code inside the kernel itself, without that trip up and down. For a mesh: part of the work — traffic interception, simple `L4` processing — can be done by `eBPF` programs in the kernel instead of routing every packet through a user-space proxy; fewer hops, less overhead. `Cilium` (our CNI from last lecture) builds its mesh on `eBPF`, merging network and mesh into one layer. The overall trend: fewer separate sidecars, closer to the kernel, pay only for what you need.

### When mesh is NOT needed

A mesh is powerful, but also a large, complex component you must understand, update, and be able to fix. Very often it's overkill. You're fine without a mesh if: there are few services and call chains are short; encryption is needed mainly at the edge and `Ingress` with `TLS` covers it while internal traffic isn't yet critical; reliability and metrics from a couple of libraries satisfy you; or the team simply isn't ready to operate another complex layer — a misconfigured mesh can cause more problems than it solves. The rule: adopt a mesh not because it's trendy, but when a concrete pain (`mTLS` everywhere, canary rollouts, visibility at the scale of dozens to hundreds of services) really outweighs its complexity.

### How to decide whether you need a mesh

A practical guide. Three to five services, essentially a monolith with a couple of satellites — you almost certainly don't need a mesh; the overhead and complexity won't pay off. Dozens of services in different languages, `mTLS`/audit requirements, frequent rollouts you want to watch — a mesh is justified. If you essentially need only `mTLS` and not the heavy `L7` logic, look at ambient or `eBPF` first, which give it cheaply, before pulling in a classic sidecar mesh with its full cost. And the general adoption principle — start small: you don't have to enable a mesh everywhere at once; roll it out gradually, one namespace at a time, on non-critical services, and expand as the team gets comfortable.

## Summary

- The mesh moves inter-service traffic work (`L7`) out of service code into a separate proxy layer; the Kubernetes network works at `L4`, the mesh at `L7`, so it understands the request itself.
- Architecture: a sidecar proxy (`Envoy`) next to each service is the data plane; the central `istiod` is the control plane; the proxy is injected automatically via a mutating webhook, and traffic is intercepted via `iptables`/`eBPF`.
- It gives four things: `mTLS` (encryption + mutual authentication by identity, no code and no manual certificates); traffic management (canaries, routing, mirroring); reliability (retries, timeouts, circuit breaking); observability (metrics, traces, service map) almost for free.
- Cost: resources, latency, operational complexity, and interception "magic" → hence the evolution toward ambient/sidecarless (`ztunnel` + `waypoint`) and `eBPF` (`Cilium`).
- Adopt a mesh deliberately — when a concrete pain outweighs its complexity — and roll it out gradually.

> Lab: build an autonomous, safe release and measure its cost — see [lab.md](lab.md).
