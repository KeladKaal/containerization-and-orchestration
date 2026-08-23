# Lecture 2 — Kubernetes control plane

Lecture 1 covered a single container on a single machine: what it is, how an image is built, who runs it. But the real `shop` is more than one container: `api` runs several identical replicas under load, plus `worker` and `postgres`, spread across several servers. Servers fail, apps ship ten times a day, load spikes — no one can track this by hand. You need something that constantly decides where to run containers, what to do when a node fails, and how to update without downtime. That is an orchestrator, and the most widespread one is Kubernetes. This lecture answers: how the Kubernetes "brain" is built — the part that makes decisions and keeps the cluster in the desired state. The running example stays the same — the `shop` (`api` ×3, `worker` ×2, `postgres` ×1) spread across nodes.

---

## Block 1. Declarative model and reconciliation

### Two ways to manage: commands or description
Imperative: you issue step-by-step commands ("run a container", "stop this one", "run one more"). Each step is on you; if something breaks, you notice and fix it yourself. That is `docker run` from Lecture 1 — docker does exactly what it was told, and that is the end of it. Declarative: you describe not steps but the desired result ("I want 3 replicas of `api` at a given version, always"). How to get there and how to keep it is the system's job. Kubernetes is declarative: you describe what you want, it figures out how.

### Desired and actual state
Two pictures Kubernetes always holds. Desired state — what you described: 3 replicas of `api` v1.4. Actual state — what exists right now: 2 replicas, because the node with the third failed overnight. The gap between them is what Kubernetes works on: constantly notice the difference and remove it by bringing actual to desired. Not once at startup, but continuously, in a loop.

### Reconciliation loop
The mechanism has a name used throughout the course: the reconciliation loop. An endless four-step cycle: observe actual state → compare with desired → if there is a difference, act to remove it → observe again. It is run not by one central mechanism but by many small ones called controllers, each responsible for its own object type. Hence the familiar effect: you delete a pod by hand (`kubectl delete pod`) and it comes back a second later. Not magic, not a bug: on its next turn the controller sees "desired 3, actual 2", records the difference, and creates a replacement. It does not remember the deletion — it just reconciles continuously.

### Kubernetes object: spec and status
Desired and actual show up in practice as the YAML you write every day. Everything in Kubernetes is an object: `Pod`, `Deployment`, `Service`, and many more, all with the same skeleton. `apiVersion` and `kind` say what it is. `metadata` — name and `labels`, by which objects find each other. Then two key fields. `spec` — desired state, what you want; you write it (e.g. `replicas: 3`). `status` — actual state, what really exists; Kubernetes fills it in (e.g. "available replicas: 2"). A controller's job, in these terms: it constantly pulls `status` toward `spec`.

### Self-healing
Declarative model plus reconciliation loop give self-healing for free, with no code of yours saying "if it dies, bring it back". Node fails → the controller sees missing replicas and starts them on other nodes; someone deletes a pod → it returns; a container crashes → it restarts. The flip side matters: since the system always returns actual to desired, you cannot delete anything by hand for good — it comes back. To really remove a service, change the desired state — delete or edit the `Deployment` object itself, not its pods. In a declarative world you manage by changing the description of what you want, not by fighting the consequences.

## Block 2. Control plane components and the request path

### Map: the brain and the worker nodes
A cluster has two parts. The control plane ("brain") makes decisions and stores state; it has four main components: the `API server` — the single door to everything; `etcd` — the database holding all cluster state; the `scheduler` — picks a node for new pods; the `controller-manager` — the set of controllers running the reconciliation loops. Each worker node runs a `kubelet` — the agent that actually starts and keeps pods (via CRI → containerd → runc from Lecture 1). Nodes also run `kube-proxy` and networking, but that is Lecture 4.

### API server: the single door
An API is an interface, a set of operations the system performs on request: create an object, read, update, delete, and "watch for changes". Technically it is REST over HTTP (an ordinary web request) plus gRPC (a fast binary protocol) between internal components. The key architectural point: no component talks to the database directly — not the `scheduler`, controllers, `kubelet`, or your `kubectl`. All go through the `API server`, the single door. And the only one that reads and writes `etcd` directly is the `API server` itself. One door is convenient because you can put a guard on it: checks of identity and rules.

### What the API server does with each request
Every request passes three gates in order. Authentication — who are you? The system identifies the sender: a person, a service, a component (like showing a badge). Authorization (RBAC, role-based access control) — what are you allowed? You have roles, roles have permissions, and the system checks whether you specifically may do this specific action, e.g. create pods in this namespace. The familiar `403 Forbidden` is a rejection at exactly this second gate: the system knows who you are, but this action is not permitted. Admission — does this specific action violate cluster rules (detailed in Block 3). Pass all three gates, and only then is the change written to `etcd`.

Here `namespace` is a logical partition of the cluster, a way to sort objects into "folders" and grant permissions on them so teams do not collide. Not to be confused with Linux kernel namespaces from Lecture 1: those isolated processes, this just organizes objects.

### etcd: the single source of truth
`etcd` is the cluster's memory, holding all state: every object with its desired and actual state. It is a key-value store ("key → value"): not tables like SQL, but a large, very reliable dictionary. Two key properties. Consistency: everyone who reads sees the same data. Durability: `etcd` usually runs as several copies, and data is not lost when one machine fails — the copies agree among themselves on what is true. Core idea: `etcd` is the single source of truth. No record in `etcd` means it does not exist in the cluster. Useful consequence: if `etcd` is unavailable, you cannot change the cluster (`kubectl apply` fails, no new pods), but already-running pods keep working, because they are executed by the `kubelet` on the node, not by `etcd`. A brain outage does not take down what already runs — it only freezes changes.

### Watch: how everyone learns of changes
There are many controllers and agents, and all need to know when something changed. The naive way — ask the `API server` "any changes?" every second (polling) — would be catastrophic for load on a large cluster. Instead there is `watch` — a subscription to changes. A component tells the `API server` "notify me when these objects change" and gets events immediately. That is why Kubernetes reacts so fast: you change a `spec`, the event lands on the right controller at once, and it runs its reconciliation loop. `Watch` ties all components into a live, reactive system.

### resourceVersion: why there are no races
Many controllers, plus you by hand, plus automation work on the same object at once. Classic race: two processes read an object, each changes it differently, both write — one set of changes is lost. You could lock the object while editing (pessimistic locking) — safe but slow, with queues. Kubernetes chose the other path — optimistic locking via `resourceVersion`, a version number every object has. When you read an object, you get it with its version. When you write, you send the version you read. If it is still current, no one changed the object since your read — the write is accepted and the version increments. If it is stale, someone changed it first — your write is rejected and you are told to re-read and retry. Hence the message `the object has been modified; please apply your changes`.

### Scheduler: who picks the node
A newly created pod has no node yet — it sits in `Pending`. On each tick the `scheduler` takes unscheduled pods and picks a node for each: first filtering out unfit ones (too little memory or CPU, wrong node type), then choosing the best of the rest by a set of rules (e.g. so replicas of one service do not all land on one node). Key detail: the scheduler does not start the pod. It only records the decision "this pod → this node" in the pod object. Scheduler internals are Lecture 3.

### Controller-manager and kubelet: who executes
The `controller-manager` is a single process running many controllers at once: replicas, jobs, nodes, and others. These are exactly the Block 1 controllers running reconciliation loops, pulling `status` toward `spec`. They decide on the desired state but do not start containers themselves. The `kubelet` does — the agent on each worker node. Via `watch` it subscribes to pods assigned to its node; once the scheduler assigns a pod, the `kubelet` sees the event and starts the pod's containers via CRI → containerd → runc. Then it watches the pod: a container crashes — restart; health checks fail — react. Clear split: the brain decides and stores, the `kubelet` on the node executes and reports actual state back.

### The request path: from kubectl apply to a running pod
The whole path in one picture:
1. `kubectl apply` — you describe the desired state (3 replicas of `api`); it goes to the cluster.
2. `API server`: authentication → RBAC → admission → the desired state is written to `etcd`. Nothing runs yet — only the wish is stored.
3. The replica controller, via `watch`, sees "need 3, have 0" and creates 3 `Pod` objects, still without a node — they are `Pending`.
4. The `scheduler`, via `watch`, sees the unscheduled pods and assigns each a node, writing the decision through the `API server` into `etcd`.
5. The `kubelet` on the chosen node, via `watch`, sees the assigned pod and starts its containers via CRI → containerd → runc.
6. The `kubelet` writes the actual status back through the `API server` into `etcd`.

Every step goes through the `API server` and `etcd`; components do not call each other directly — each reacts to state changes.

### Why this architecture is robust
Kubernetes components do not know about each other and do not call each other directly: the scheduler does not call the `kubelet`, a controller does not call the scheduler. All communicate solely through shared state in `etcd`, read and written via the `API server`. Benefits: any component can be killed and restarted — it looks at the current state and continues; there are no fragile call chains "A calls B, B calls C" that break in the middle; what runs on nodes survives a control plane outage. Same principle as reconciliation, but at the architecture level: the system reacts to state, not to commands.

## Block 3. Extending the API: operators, admission, policies

### Kubernetes can be built out to fit you
Kubernetes' main strength is that its API is extensible: you are not locked into built-in types like `Pod` and `Deployment` — you can add your own concepts and behavior, and they live by the same rules as the built-ins. Three ways to plug into the model: add your own object type (`CRD`); add your own controller for it, your own reconciliation loop (an operator); intercept requests at the entrance (admission webhooks) and apply policies. All of this rests on the same declarative model and reconciliation from Block 1 — you extend a familiar paradigm rather than learn a new one.

### CRD: your own object type
A `CRD` (Custom Resource Definition) declares a new object type in the Kubernetes API, e.g. `Database`. After that Kubernetes treats it like a built-in: `kubectl get`, `kubectl apply`, RBAC permissions, storage in `etcd`. In practice: instead of assembling a database from a dozen low-level objects by hand, you declare one clear type and write simple YAML — "I want a postgres database v15, size 100 GB". But a `CRD` by itself is only a new record form, a new kind of object in the store. It does nothing; for "I want a database" to actually produce one, you need a controller.

### Operator: your own controller for your own type
An operator is your `CRD` plus your controller running its reconciliation loop. The idea: since a controller can bring actual to desired, teach it to do so not for abstract pods but for a specific application — with all the expert knowledge of how to operate it correctly. An operator encodes what an experienced engineer knows: how to deploy, upgrade without data loss, back up, restore, and handle failure. You write "I want postgres 15, 100 GB", and the operator itself creates pods, attaches disks (volumes — persistent storage that survives a pod restart, detailed in Lecture 3), brings up database copies with replication, takes backups, and repairs on failure.

Do not confuse the word "replica": pod replicas are identical, interchangeable copies of a stateless service like `api`; database copies are different — they have a primary and followers and are not interchangeable. Real-world operators: `cert-manager` issues TLS certificates and renews them before expiry; Prometheus Operator; database operators. An operator is a way to package operational expertise into code that works for you around the clock.

### Admission webhooks: interception at the entrance
Recall the API server's three gates. The third, admission, is where you can insert your own code and affect every request before it is written to the store. Webhook means: at the right moment the `API server` calls your service and asks its opinion. Two kinds. Validating: your code looks at the request and says yes or no — e.g. "do not admit pods without specified memory limits", such a pod is rejected with a clear error. Mutating: your code does not reject but augments the request — the classic service mesh example (Lecture 5): when a pod is created, a mutating webhook automatically injects a sidecar container. A sidecar is an extra container riding in the same pod next to the main one, handling auxiliary work such as network traffic; you did not write it in your YAML, it appeared at the entrance. All of this happens before the write to `etcd` and is invisible to the sender.

### Policies: rules for the whole cluster
Writing custom admission code for every small rule is expensive. Hence ready-made policy engines that use the same admission mechanism but let you define rules declaratively, without programming: OPA with the Gatekeeper add-on, and Kyverno. You describe a rule, the engine works as an admission check at the entrance. Typical policies: forbid images not from a trusted registry (a registry is an image store the node pulls from, like Docker Hub or your own Harbor); require resource limits and mandatory labels on all pods; forbid privileged containers (from Lecture 1, those that strip away almost all protection). The value: the security team sets cluster-wide boundaries once, and they cannot be broken — not because everyone is disciplined, but because the `API server` will not accept a violating request. And you do not need to review every deploy by hand — the rules are built into the door itself.

## Summary

- Kubernetes is declarative: you set the desired state (`spec`), controllers pull actual (`status`) toward it — the reconciliation loop, an endless check of actual against desired.
- Hence self-healing; you manage by changing the desired state, not by deleting things by hand.
- The brain: `API server` (single door + authentication → RBAC → admission), `etcd` (single source of truth; if it falls, changes freeze but pods live), `scheduler` (decides where), `controller-manager` (reconciles), `kubelet` (runs pods on the node).
- `watch` — subscription instead of polling; `resourceVersion` — optimistic locking against races.
- Everything is tied through state in `etcd`, not through direct calls — hence the robustness.
- The API is extensible: `CRD` (your own type), operators (`CRD` + controller), admission webhooks (validating and mutating), policies (Gatekeeper/Kyverno) — boundaries for the whole cluster.

> Lab: make it impossible to break the cluster's rules — see [lab.md](lab.md).
