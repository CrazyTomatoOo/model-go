# Deploy

Helm charts, split by cluster role. See docs/adr/0002 for the hub/member split.

## hub/ — 管理集群 (management plane)

| Chart | Binary (cmd/) | Role |
|---|---|---|
| hub/apiserver | apiserver | REST surface (gin), Postgres only |
| hub/agent-gateway | agent-gateway | gRPC entrypoint for member agents |
| hub/scheduler | scheduler | queueing / scoring / binding |
| hub/controller-manager | controller-manager | orchestration, no workload CRDs |

## member/ — 业务集群 (compute plane)

| Chart | Binary (cmd/) | Role |
|---|---|---|
| member/cluster-agent | cluster-agent | pull tasks, heartbeat, capacity report |
| member/cluster-controller | cluster-controller | reconcile TrainingJob/InferenceService CRDs |
| member/model-gateway | model-gateway | model traffic entrypoint |
| member/addons/ | — | override values for Volcano and accelerator device plugins |

Templates are not written yet; each chart currently ships Chart.yaml + values.yaml only.
