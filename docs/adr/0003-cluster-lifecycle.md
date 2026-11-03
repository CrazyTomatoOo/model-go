# ADR-0003: Member cluster lifecycle — onboarding, draining, removal

- Status: Accepted
- Date: 2026-10-27

## Context

ADR-0002 fixes the hub-spoke topology. The platform needs a safe, automatable lifecycle for member clusters: how a new cluster joins the pool, how it leaves without killing in-flight work, and what happens when one disappears unannounced.

## Decision

### Cluster state machine

```
Pending → Active ⇄ Draining → Removed
              ↘ Lost (heartbeat timeout) ↗ (heartbeat resumes → Active)
```

`Lost` is a side state recoverable to `Active`; `Removed` is terminal and archived (records retained, nothing hard-deleted).

### Onboarding (registration)

1. Admin runs `create cluster` via apiserver/CLI. Hub generates a `cluster-id` and a **one-time, short-TTL bootstrap token**, stored in Postgres.
2. Admin installs the member Helm chart (agent + CRDs + controller + model-gateway) into the new cluster with the bootstrap token and the agent-gateway URL.
3. Agent's first connection presents the bootstrap token; agent-gateway validates it and issues a **long-lived, rotatable credential** (client certificate).
4. Cluster enters `Pending`. On first heartbeat the agent reports its accelerator ledger (per-pool, see ADR-0005).
5. Admin confirms labels (accelerator type, datacenter, tenant isolation); cluster transitions to `Active` and enters the scheduler pool.

The explicit admin confirmation between capacity report and `Active` is deliberate: it prevents accidental onboarding of the wrong cluster and is the checkpoint where taints/labels are validated.

### Removal (draining, never a hard delete)

1. **Cordon**: admin marks the cluster `Draining`. Scheduler immediately stops assigning new work; running work is untouched.
2. **Drain**: running workloads either finish naturally or are handled per policy — training jobs with checkpoints may be migrated, inference services are rebuilt elsewhere, or work is terminated. Policy is per-workload, not per-cluster.
3. **Unregister**: once empty, credentials are revoked. On its next pull the agent receives a `deregistered` directive and self-cleans (deletes its namespace and platform resources); alternatively the admin uninstalls the Helm release manually.
4. **Archive**: the hub marks the cluster `Removed`. History (jobs, metrics, ledger) is retained for audit.

### Failure path

Missing heartbeats (3 consecutive periods) → cluster marked `Lost` → scheduler removes it from the pool → its workloads are marked `Unknown` and rescheduling policy is applied per workload type. Recovery (heartbeat resumes) returns it to `Active` after a fresh capacity report; workloads still running are reconciled with reported state.

### Credential management

Bootstrap tokens are single-use with short TTL. Long-lived credentials are per-cluster client certificates, rotatable without re-onboarding, revocable at the hub. An agent's credential scopes it to its own cluster's tasks only.

## Consequences

- Draining is the only removal path; there is no "force delete" that orphans running work.
- The `Lost`/`Active` round trip reuses the same state machine as planned maintenance, so monitoring and runbooks have one vocabulary.
- Agent self-cleanup on deregistration means offboarding does not require the admin to still have cluster access at removal time — but the Helm uninstall fallback remains documented for broken agents.
