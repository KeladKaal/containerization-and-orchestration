# Lecture 4 — Scheduling and resources

In the previous lecture we saw the Kubernetes "brain": you declare the desired state, a controller creates pods, the scheduler assigns each pod to a node, and `kubelet` runs the containers. Two parts of that chain stayed black boxes. By what rules does the scheduler pick a node? And what happens to memory and CPU on the node itself under load — where do `OOMKilled` and mysterious slowdowns come from? Today we answer both, then cover a third question: where does data that must not be lost live, given that a container is ephemeral and everything written inside it disappears when the pod is recreated?

The running example is the online shop `shop` with three services of different needs. Three `api` replicas should be spread across different nodes, so one machine failing does not take out every copy. Two `worker` pods are background processors, less critical, movable if needed. One `postgres` is the demanding one: it holds data that must not be lost and needs a stable name and a stable disk that survives pod recreation. On these three we show how to place, constrain, and store.

---

## Block 1. The scheduler from the inside

### What the scheduler does

Its input is a pod without a node, sitting in `Pending`: created but not yet placed. Its output is a decision — "this pod goes to that node" — written through the API server. The scheduler runs nothing itself; `kubelet` on the chosen node starts the pod.

If no node fits, the pod stays `Pending` — it does not fail, it waits. The most common cause of a stuck `Pending` is not enough resources: the pod requests more memory or CPU than is free on any node. Why "requests" and not "uses" is a key subtlety we reach shortly.

### Two stages: filtering and scoring

The scheduler picks a node in two stages. First, filtering: it walks all nodes and drops those that clearly do not fit — not enough requested resources, wrong node type, pod not allowed there. What remains is the list of feasible nodes. Second, scoring: from the feasible nodes it must pick not any but the best, so it grades each by a set of rules and takes the highest.

Analogy — hiring: first screen out candidates who fail hard requirements, then pick the best of those who pass.

### Filtering: requests as the basis of scheduling

The main filtering criterion is resources, and here is where almost everyone trips. A pod has `requests` — how much memory and CPU it is guaranteed to need. The scheduler looks not at current real usage but at the **sum of `requests`** of all pods already assigned to a node. A node fits only if its free capacity by `requests` (total minus what is already reserved by requests) holds the new pod's `requests`.

Hence a common, puzzling case: a node is really loaded at about 30%, plenty of free memory, yet a new pod still sits `Pending`. The cause is not real load but the reservation: by summed `requests` the node already counts as full, even if pods do not yet use what they asked for. The scheduler thinks in reserved requests, not actual usage — the first thing to check on an unexplained `Pending`.

### Filtering: where a pod is allowed to run

Besides resources, filtering honors placement rules that steer pods to the right places.

`nodeSelector` and its more flexible form `node affinity`: "place this pod only on nodes with a given label." You put `labels` on nodes yourself — for example `disktype=ssd` or zone `eu-west`. A service needing a fast disk gets `disktype=ssd` and lands only on SSD nodes; the scheduler drops the rest during filtering.

`taints` and `tolerations` work the opposite way. A selector attracts a pod to nodes; a `taint` ("repel mark") on a node repels pods: "no ordinary pods here." A `toleration` on a pod is a pass: "I tolerate that taint, let me in." Like a face-control at the door: the node admits no one except pods with the matching `toleration`. This is how you dedicate nodes to `postgres` or GPU: add a `taint` so stray pods stay away, give your own pods a `toleration`. Ordinary pods do not land on master nodes (`control plane`) for exactly this reason — they carry a default `taint`.

### Scoring and spreading: don't put all eggs in one basket

Feasible nodes then get scored. One of the most practical criteria is how to spread related pods.

- `pod anti-affinity` — "do not put these pods on the same node." Exactly what three `api` replicas need: if all three land on one node and it dies, the whole service is lost; anti-affinity spreads them out.
- `pod affinity` — the reverse, "put these pods together" — e.g. a cache close to the service that uses it, for a shorter network hop.
- `topology spread` — a newer, more flexible way: "distribute pods evenly across zones or nodes," not a hard "never together" but a soft "keep the balance."

For `api` we would use `anti-affinity` or `spread` to get one replica per node or zone. A direct gain in fault tolerance, configured declaratively.

### Preemption: eviction by priority

What if an important pod does not fit because every feasible node is full? Priority steps in. Each pod can get a `priorityClass` — essentially a number of how important it is. If a high-priority pod does not fit, the scheduler may perform `preemption`: it evicts one or more lower-priority pods to make room. Evicted pods are not lost — they return to `Pending` and get rescheduled, usually onto other nodes.

Example: a critical `postgres` cannot start because the cluster is packed, while low-priority background jobs like nightly analytics run nearby. With priorities the scheduler moves the background jobs aside for the database. It guarantees the important always finds room even in a full cluster — but use priorities carefully so you do not start evicting what you need.

---

## Block 2. Resources and pressure

### requests and limits: different roles

Everyone writes `requests` and `limits`, and everyone confuses them. `requests` is how much a pod is guaranteed to need, and it has two roles. First, scheduling: the scheduler places the pod by `requests`, reserving room on the node (Block 1). Second, at runtime: when several pods fight over CPU, the kernel splits CPU time in proportion to their `requests`; so `requests` also sets the pod's share of CPU when there is not enough to go around. The same `requests` works in two places — placement and the CPU fight.

`limits` is different: a hard ceiling the pod cannot exceed.

Units. CPU is measured in millicores: `1000m` is one whole core, so `200m` is one fifth of a core. Memory is in mebibytes (`Mi`), practically the same as megabytes. For `api`: `requests` `200m` CPU / `256Mi` for placement and share; `limits` `1` CPU / `512Mi` as the ceiling.

The key link to Lecture 1: `limits` on the node become the very `cgroups` we discussed. In one sentence: a `cgroup` is a Linux kernel mechanism that takes your container's process group and assigns it ceilings on memory and CPU; the limit you write on a pod is the setting of that `cgroup` on the node. Your YAML numbers descend into a concrete kernel mechanism — which is why memory and CPU behave differently.

### Memory kills by termination, CPU by delay

Memory and CPU react to hitting the limit in fundamentally different ways.

Memory: if a container exceeds its memory limit, the kernel forcibly terminates the process — you see `OOMKilled`, and `kubelet` restarts the container. If it repeats in a loop (start, eat, terminate, start again), you get the familiar `CrashLoopBackOff`.

CPU: hitting the CPU limit does not terminate the process; it is slowed until the next period — `throttling`. About the "period": the kernel hands out CPU time in short windows, usually `~100 ms`. In each window the pod has a quota set by its limit. Spend the quota before the window ends and the pod is paused until the next window, even if the CPU is idle. Hence the sneaky symptom: the pod passes every health check, does not crash, yet answers slowly and unevenly because it is periodically frozen for the rest of each window.

These are the same `cgroups` from Lecture 1, set via the pod's `resources` field. Kubernetes adds no new physics. Same diagnosis: `OOMKilled` — look at memory; slowdowns without errors — check CPU throttling.

### QoS classes: who gets sacrificed first

From how you set `requests` and `limits`, Kubernetes assigns the pod one of three quality-of-service (`QoS`) classes.

- `Guaranteed` — `requests` equal `limits` for all resources; the most protected.
- `Burstable` — `requests` are set but the `Guaranteed` condition is not met (limit above request, or not set for everything); the pod may temporarily take more than requested; middle class.
- `BestEffort` — neither `requests` nor `limits` set; the least protected.

Why it matters: when a node runs short on memory, `kubelet` evicts pods to save the node, strictly by class — first `BestEffort`, then `Burstable`, and `Guaranteed` last. A pod with no resources set is first on the block. So: do not leave important services in `BestEffort`.

### Two different "pod got killed": OOM by limit vs eviction by node pressure

These two are constantly confused, yet treated differently.

First — `OOMKilled` by its own limit: a specific container exceeded its own memory limit and the kernel terminated it. The pod is at fault — either the limit is too low or there is a leak. In the pod's events you see `OOMKilled` and the limit overrun.

Second — `eviction` under `node pressure`: memory or disk runs out across the whole node, and `kubelet`, to save the machine, evicts pods by `QoS` class starting with `BestEffort`. Here your pod may be entirely innocent — it behaved well but sat in an unprotected class on an overloaded node. The events show `Evicted` and a node-pressure message. Fix `OOMKilled` by examining the pod; fix `eviction` by looking at the node overall and at `QoS`.

### Example on shop: fixing postgres and api

Case one: `postgres` started with no resources at all, so it is `BestEffort`. An evening spike brings memory pressure on the node, `kubelet` starts evicting — and the first to go is our data-holding `postgres`. Fix: set its `requests` equal to `limits` so it becomes `Guaranteed` and is evicted last, and move it to a dedicated node via a `taint` from Block 1 so neighbors do not interfere.

Case two: `api` runs and does not crash but answers ever slower under load. The cause is a too-low CPU limit, constant `throttling`. Fix: raise the CPU limit or remove it, keeping a sensible `requests`. Why removing the limit helps, counterintuitive as it sounds: `throttling` only happens when you hit the limit; no limit, no ceiling to throttle against — the pod takes as much CPU as is actually free and the uneven delays vanish. But then the pod could grab a lot of CPU on a spike, so keep `requests` anyway for the fair share and scheduling.

Bottom line: `requests` and `limits` are not a formality to please a linter but a direct tool for controlling reliability and speed.

---

## Block 3. State and storage

### The problem: a container is ephemeral

From Lecture 1: a container has a thin writable layer, and it is ephemeral — it lives exactly as long as the container. In Kubernetes pods are recreated constantly: on updates, on moving to another node, on crashes. Each recreation is a new container with a clean writable layer.

For `api` and `worker` this is fine: they are stateless, holding nothing important inside. For `postgres` it is a disaster: its data would sit in that layer and vanish on the first pod recreation. So data must live outside the container — in storage that survives pod recreation and moving to another node.

### Volumes

A `volume` is storage attached to a pod that lives apart from the container's writable layer, by its own rules. Volumes come in many kinds, differing in durability. Simple temporary ones — like `emptyDir`, an empty folder created for the pod's lifetime and deleted with it; good for cache or sharing files between containers of one pod, not for important data. Persistent volumes sit on a real disk or network storage and survive everything: container recreation, pod recreation, and moving to another node. `postgres` needs a persistent volume. But to give a pod one, you need a real disk somewhere — hence the `PV`/`PVC` pair.

### PV, PVC, StorageClass

Three concepts solve one task — decoupling the application from a specific disk.

- `PV` (`PersistentVolume`) — a concrete piece of storage in the cluster: a real disk or network volume of a given size.
- `PVC` (`PersistentVolumeClaim`) — a pod's request: "give me 100 GB of storage of such a class." The pod asks not for a named disk but for an abstract claim — how much and of what quality. Kubernetes finds or creates a matching `PV` and binds them.
- `StorageClass` — describes how to provide storage: disk type, fast or slow, which cloud. Crucially it enables dynamic provisioning: as soon as a pod creates a `PVC`, a new real disk is created for it automatically, no manual pre-slicing.

The application says "give me 100 gigabytes" and the cluster finds or creates a disk and attaches it. A familiar `Pending` on a `PVC` is often a missing `StorageClass` or an inability to provision a disk.

### CSI: how any storage plugs in

There are countless storage systems: AWS and Google Cloud disks, distributed systems like `Ceph`, plain old `NFS`, local SSDs. How does Kubernetes work with all of them without carrying every vendor's code? Through a standard interface — `CSI` (`Container Storage Interface`). It is a contract: any storage vendor writes a `CSI` driver (plugin) that can, on Kubernetes' command, create a volume, attach it to a node, detach it, take a snapshot. Once the driver exists, that vendor's disks work natively in the cluster.

Note the recurring pattern: Lecture 1 had `CRI` for runtimes, the networking lecture will have `CNI`, here `CSI` for storage. Kubernetes standardizes its boundaries through interfaces to stay independent of specific implementations.

### StatefulSet: stable identity

For stateless services (`api`, `worker`) we use a `Deployment`, and it is ideal: pods are identical and interchangeable, with random names, any replaceable by any. For `postgres` that is not enough — it needs identity. Hence `StatefulSet`. What it adds over `Deployment`:

- **stable names** — pods are named in order (`postgres-0`, `postgres-1`, ...), and the names do not change on recreation: recreated, it is `postgres-0` again;
- **its own persistent storage per pod** — each has its own `PVC` and disk, and on recreation the disk moves with the pod, so data is not lost or mixed between pods;
- **ordered start and stop** — pods come up and go down in sequence (`0`, then `1`, then `2`), which matters for replicated databases where the primary must start first.

So a `StatefulSet` gives the database a lasting identity and its own data.

### Run a database in Kubernetes deliberately

`StatefulSet` and `PVC` solve two things — identity and storage — but not the full operation of a database. Backups, restore testing, version upgrades without data loss, replication setup, failover to a replica when the primary dies — a `StatefulSet` does none of that.

So in practice databases in Kubernetes usually run not on a bare `StatefulSet` but through an operator (from the control-plane lecture): the operator encodes all this operational work and does it for you. The grown-up conclusion: running a database in the cluster is a deliberate engineering decision, not "just deploy `postgres` and forget it." Either you take a mature operator that carries backups and failover, or you deliberately keep the database outside the cluster in a managed service. For `shop` a `postgres` operator would be reasonable — it configures `StatefulSet`, `PVC`, and all of today's pieces under the hood.

---

## Summary

- **Scheduler**: filtering (resources by `requests`, selectors, `taints`) → scoring (`anti-affinity`/`spread`) → node.
- `requests` is a reservation — hence `Pending` on a seemingly free node; `taints` repel, selectors attract; `anti-affinity` spreads replicas; `preemption` evicts by priority.
- `requests` works in two places (scheduling and fair CPU share), `limits` is a hard ceiling that descends into the `cgroups` from Lecture 1.
- Memory over the limit kills by termination (`OOMKilled`), CPU by delay (`throttling` in ~100 ms windows).
- `QoS` classes (`Guaranteed`/`Burstable`/`BestEffort`) set the eviction order under pressure; give important services `Guaranteed`.
- Tell apart `OOMKilled` by own limit (pod's fault) from `eviction` by node pressure (node's fault).
- State: a container is ephemeral → data in volumes; `PVC` (claim) → `PV` (real volume), `StorageClass` for dynamic provisioning; `CSI` is the standard for attaching any storage.
- `StatefulSet` gives a database stable names, its own disk, and ordering; full operation is handled by an operator or a managed service.

> The lab for this lecture ("keep the critical path alive under overload") is currently available locally only and is not published in the repository.
