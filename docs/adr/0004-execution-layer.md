# ADR-0004: Execution layer — Volcano for training, Deployment + LWS for inference

- Status: Accepted
- Date: 2026-10-27
- Amends: ADR-0001 (self-built controllers on native primitives)

## Context

ADR-0001 decided to self-build controllers reconciling platform CRDs down to native `Job`/`Deployment`. Design review found this insufficient: distributed training needs gang scheduling, multi-role topology, and restart policies that are expensive and error-prone to hand-roll. The requirement became "adopt mature community solutions, but keep them light."

## Decision

### Training: Volcano

Member clusters run **Volcano** (scheduler, admission, controller). Platform `TrainingJob` CRDs are translated by the member-cluster `controller` into Volcano `Job`/`PodGroup` resources. Gang semantics, queueing, priority, and topology-aware placement (NVLink/RDMA domains) come from Volcano plugins; we write none of that logic.

This consciously supersedes ADR-0001's "no Kubeflow/Volcano" stance. The weight is accepted because a training platform eventually needs gang scheduling, queue priority, and topology — adopting early avoids a second migration. (Kubeflow Training Operator and JobSet were considered and rejected in favor of Volcano's single coherent model; Kueue was rejected as overlapping with the hub scheduler's queueing role.)

### Inference: Deployment + LWS + HPA

Platform `InferenceService` CRDs are translated into:
- native `Deployment` for single-node serving,
- **LeaderWorkerSet (LWS)** for multi-node inference (tensor-parallel across machines), adding grouped-Pod semantics with minimal weight,
- native `HPA` for autoscaling.

**KServe is not adopted**: its feature set overlaps heavily with our own `model-gateway` (routing, auth, protocol adaptation, metering), and its abstraction cost is unjustified given the gateway already exists.

### Our CRDs stay; our controller becomes a translation layer

The platform's user-facing API (TrainingJob / InferenceService CRDs) is unchanged. The member-cluster `controller` is demoted from "executor" to "translator": it maps platform CRDs onto community resources and reflects their status back. It owns no scheduling logic and no pod lifecycle logic beyond the translation.

### Member-cluster onboarding chart

The member Helm chart now installs: `cluster-agent`, platform CRDs + `controller`, `model-gateway`, **Volcano**, and **LWS** CRDs/controllers (plus vendor device plugins where applicable, typically pre-installed by the cluster provider).

## Consequences

- In-cluster scheduling for training belongs to Volcano; the hub scheduler only selects cluster and accelerator pool (ADR-0005). The boundary is clean but must be respected in code: no scheduling hints in the translation layer beyond affinity/toleration injection.
- Volcano upgrades become part of member-cluster lifecycle management; the hub must tolerate a supported version window across clusters.
- Reversibility: LWS and Volcano are consumed through the translation layer only, so replacing either (e.g. revisiting KServe) touches one package, not the API.
