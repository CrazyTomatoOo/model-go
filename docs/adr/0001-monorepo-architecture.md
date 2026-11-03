# ADR-0001: Monorepo architecture — modular monolith hub with self-built controllers and pull-based cluster agents

- Status: Accepted
- Date: 2026-10-27

## Context

`model-go` is a monorepo for a multi-cluster, heterogeneous-compute (GPU/NPU/domestic accelerators) AI training & inference platform built on Kubernetes. Tech stack: React + Ant Design frontend, Go (gin) backend, Postgres for metadata, Redis for cache/locks, Kafka for the event stream. The repo started greenfield and needs a directory layout and component split that can grow without a big-bang restructure.

## Decision

### Hub-spoke control plane

- **Hub cluster (management plane)** runs:
  - `apiserver` (gin): the only north-south REST surface. Stateless, reads/writes Postgres only; never talks to member clusters directly.
  - `scheduler`: queueing, scoring, and binding of training/inference workloads to member clusters.
  - `controller-manager`: reconciles the platform's own CRDs (training job, inference service, cluster registration).
- **Member clusters (compute plane)** each run a `cluster-agent` that:
  - **pulls** work from the hub (no hub→member inbound network requirement),
  - reports heartbeat and heterogeneous compute capacity (extended resources such as `nvidia.com/gpu`, Ascend NPU, domestic cards),
  - executes assigned workloads and streams status/logs back.

### Task orchestration: self-built controllers

We do **not** adopt Kubeflow Training Operator or Volcano at this stage. The platform defines its own CRDs and reconcile loops (`internal/controller`), starting from native `Job`/`Deployment` primitives. Rationale: full control over the lifecycle and multi-cluster semantics; the team accepts the higher upfront cost. Revisit if gang scheduling or topology-aware batching becomes a hard requirement — that would be a new ADR.

### Service split: modular monolith

One `apiserver` binary, internally modularized under `internal/apiserver/` by domain module. `scheduler`, `controller-manager`, and `cluster-agent` are separate binaries because they scale and deploy independently. We deliberately do not split into microservices with separate data boundaries yet.

### Data layer responsibilities

- **Postgres**: system of record for metadata (users, jobs, clusters, quota).
- **Redis**: cache, sessions, distributed locks. Not a source of truth.
- **Kafka**: event stream — job status events, log pipeline, metrics pipeline. Postgres state and Kafka events are kept separate; no dual-write without an outbox if we ever need both for one fact (future ADR).

### Repository layout

```
cmd/<binary>/        one directory per deployable (hub: apiserver, scheduler,
                     controller-manager, agent-gateway; member: cluster-agent,
                     cluster-controller, model-gateway; plus cli)
pkg/                 shared libraries (apis, store, cache, mq, k8s)
internal/<comp>/     per-component private implementation, one package per
                     server component, using the cmd/ name without dashes
                     (cmd/agent-gateway -> internal/agentgateway)
web/                 pnpm workspace; apps/console = React + AntD console
deploy/              Helm charts and member-cluster manifests
build/               Dockerfiles and CI config
hack/                local development scripts
test/e2e/            cross-component end-to-end tests
docs/adr/            architecture decision records
```

`internal/` is split by component, `pkg/` holds only genuinely shared code — nothing imports across components except through `pkg/`.

### Domain docs: multi-context

The repo is a monorepo with distinct domain contexts, so domain documentation uses the multi-context layout: a root `GLOSSARY-MAP.md` pointing at per-context `GLOSSARY.md` files, created lazily. See `docs/agents/domain.md`.

## Consequences

- Apiserver stays horizontally scalable; all cluster mutation flows through the controller/agent path, so failures are retryable rather than user-facing timeouts.
- Self-built controllers mean we own the CRD versioning discipline from day one (v1alpha1 → v1beta1 promotion policy to be defined when the first CRD lands).
- The agent-pull model simplifies member-cluster onboarding (outbound-only network) at the cost of polling latency for task dispatch.
- New binaries go under `cmd/`; new shared code must justify itself as genuinely cross-component before entering `pkg/`.
