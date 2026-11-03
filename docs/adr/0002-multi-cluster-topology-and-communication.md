# ADR-0002: Multi-cluster topology and communication

- Status: Accepted
- Date: 2026-10-27

## Context

ADR-0001 fixed the monorepo skeleton and the hub-spoke split but left the component inventory and wire protocol open. This ADR records both, including two revisions agreed during design review: a dedicated agent gateway, and the addition of a model gateway for inference traffic.

## Decision

### Component inventory

**Hub (management) cluster — control plane:**

| Component | Responsibility |
| --- | --- |
| `apiserver` (gin) | North-south REST API, authn/authz, metadata management. Stateless; reads/writes Postgres only; never talks to member clusters. |
| `scheduler` | Queueing, scoring, and binding of training/inference workloads to member clusters, using Postgres task records and the capacity ledger. |
| `controller-manager` | Orchestration layer only: translates user requests into (target cluster, task spec) records in Postgres. Holds **no workload CRDs**. |
| `agent-gateway` | The single control-plane entrypoint for all member-cluster agents: registration, heartbeat, task dispatch, status/log ingestion. Separate binary from `apiserver` so long-connection management scales and fails independently of user API traffic. |
| Postgres | System of record: clusters, tasks, quotas, users. |
| Redis | Cache, sessions, distributed locks. Not a source of truth. |
| Kafka | Event stream: job status events, log pipeline, metrics pipeline. Hub-internal only; never exposed to member clusters. |

**Member (business) cluster — compute plane, four components onboarded via one Helm chart:**

| Component | Responsibility |
| --- | --- |
| `cluster-agent` | The only component talking to the hub: registration, credential rotation, heartbeat, accelerator-capacity reporting, task pull, status/log push. Pure carrier — no reconcile logic. |
| `controller` (operator) | Reconciles the platform's TrainingJob / InferenceService CRDs **locally**, translating them into execution-plane resources (see ADR-0004). Independent controller-runtime process, crash-restartable and upgradeable on its own. |
| `model-gateway` | Data-plane entrypoint for inference traffic: model-name routing, auth/rate-limit, OpenAI-compatible protocol adaptation, canary/version switching, latency/token metering (reported via Kafka). One per member cluster (distributed, not centralized in the hub). |
| Execution layer | Volcano + vendor device plugins + LWS (see ADR-0004/0005). Not written by us. |

### Communication

- **Hub never initiates connections to member clusters.** All hub→member flow is via agent pull; member clusters need only outbound connectivity, which traverses NAT/firewalls/cloud boundaries naturally.
- **Agent ↔ agent-gateway: a single gRPC bidirectional stream** carrying heartbeat, task dispatch, status and log upload. Proto-defined, type-safe, one connection per cluster.
- **Users/CLI → apiserver: HTTPS REST** (OpenAPI-described).
- **Task state machine lives in Postgres rows** — the single source of truth both `controller-manager` (write) and `scheduler`/`agent-gateway` (read) converge on.
- **Logs/metrics:** agent pushes to agent-gateway, which publishes to Kafka. Kafka stays hub-internal.

### Why CRDs live in member clusters, not the hub

An early draft put the platform CRDs on the hub. That reduces them to a Postgres reskin — a reconcile loop that cannot observe real workloads is meaningless. Member-cluster CRDs give a standard Kubernetes API where the workloads actually run: local controllers observe Pods, handle retries, and report status without cross-cluster reads. The hub keeps its multi-cluster view in Postgres and stays a pure orchestration layer.

## Consequences

- Two gateways with different traffic profiles: `agent-gateway` (few long-lived control streams) vs `model-gateway` (high-volume data plane, per-cluster). They must not be merged.
- Member-cluster onboarding is a single Helm chart installing agent + CRDs + controller + model-gateway; the chart is the unit of version compatibility between hub and member.
- Hub scheduler decisions are coarse ("which cluster, which accelerator pool"); fine placement is delegated to in-cluster scheduling (ADR-0004/0005).
