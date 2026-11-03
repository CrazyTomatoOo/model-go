# ADR-0005: Heterogeneous accelerator model — vendor passthrough, per-pool ledger, intra-cluster heterogeneity

- Status: Accepted
- Date: 2026-10-27

## Context

The platform targets heterogeneous compute: NVIDIA GPU, Ascend NPU, Cambricon MLU, Hygon DCU, and other domestic accelerators — **including multiple accelerator types mixed within a single member cluster**. Two candidate abstractions were considered: a normalized capability matrix (users request logical tiers, hub translates to vendor resources) versus vendor resource passthrough. Passthrough was chosen for implementation simplicity and natural fit with Volcano; the trade-off is that users must know hardware specifics, and any future logical-tier abstraction must be added at the user-facing API layer, not underneath.

## Decision

### Vendor passthrough — no resource-name wrapping

Tasks request vendor extended resources directly: `nvidia.com/gpu`, `huawei.com/Ascend910`, `cambricon.com/mlu`, etc. The platform introduces **no** synthetic resource names. In-cluster, Volcano and vendor device plugins consume these names natively; wrapping them would break topology awareness.

### Intra-cluster heterogeneity is a first-class scenario

A member cluster may mix accelerator types (e.g. 8× A100 nodes + 4× Ascend910B nodes + 2× DCU nodes). Three consequences:

1. **Per-pool capacity ledger.** The `cluster-agent` reports capacity grouped by accelerator pool, not as a cluster aggregate:

   ```
   accelerator_pools: [
     { vendor, model, nodes, cards_per_node, allocatable/total }
   ]
   ```

   Hub scheduling and tenant quota operate per `(cluster, pool)`, never per cluster as a homogeneous whole.

2. **Node isolation via labels + taints.** Each accelerator node carries `accelerator.vendor/model` labels and a matching taint (e.g. `accelerator=ascend910b:NoSchedule`). Device-plugin DaemonSets select nodes by label.

3. **Translation-layer injection.** Users write only the resource name (`huawei.com/Ascend910: 8`). The member-cluster `controller`, when translating a platform CRD, **automatically injects** the matching nodeAffinity and tolerations. Generated affinity config — not user-supplied — is what keeps workloads from landing on the wrong cards in mixed clusters.

### Data collection: Kubernetes API + vendor exporters (phase 1)

The agent reads node capacity/allocatable from the Kubernetes API and metrics from vendor exporters (dcgm-exporter, Ascend equivalents). **No self-built device daemon** in phase 1. Per-device health data (temperature, ECC errors) is deferred to phase 2 and will be evaluated per vendor.

### Fractional sharing: whole-card only (phase 1)

MIG, vNPU, and time-slicing are out of scope for phase 1 — only whole-device scheduling. Vendor behaviors differ too much to unify early; each vendor's sharing story enters the model individually later.

### Supporting conventions

- **Images**: multi-arch tags with runtime suffixes (`-cu124`, `-cann8`); task templates bind images per accelerator type; the scheduler validates the pairing.
- **Metrics normalization**: vendor metrics are normalized on ingestion to Kafka into a single schema (`accelerator_utilization{cluster, vendor, model, ...}`), so the console and quota logic see one vocabulary regardless of vendor.
- **Vocabulary**: accelerator terms are owned by the compute-context glossary (`docs/glossary/compute.md`, created lazily per `GLOSSARY-MAP.md`).

## Consequences

- The hub scheduler's matching input is the per-pool ledger; a pool with zero allocatable devices is simply invisible to new bindings — no special "full" state.
- Adding a new vendor means: device plugin + labels/taints on nodes, exporter metrics mapping, image suffix convention, ledger pool entry. No hub code changes as long as passthrough holds.
- If a normalized tier model is ever added, it lives in the apiserver/scheduler layer as a request-time mapping onto vendor resources — this ADR's cluster-side design is unaffected.
