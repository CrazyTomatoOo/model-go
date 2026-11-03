# GLOSSARY-MAP

Multi-context domain glossary index for this monorepo. Each context owns one `GLOSSARY.md`; read the ones relevant to your task before naming things.

| Context | Scope | Glossary |
| ------- | ----- | -------- |
| Platform | Cross-cutting terms: cluster registration, quota, tenants | `docs/glossary/platform.md` |
| Training | Training jobs, datasets, checkpointing, distributed strategies | `docs/glossary/training.md` |
| Inference | Inference services, models, routing, autoscaling | `docs/glossary/inference.md` |
| Compute | Heterogeneous accelerators (GPU/NPU/domestic cards), capacity, scheduling | `docs/glossary/compute.md` |

Context-scoped ADRs, when a decision applies to one context only, live in `docs/adr/` alongside the rest and are prefixed with the context name (e.g. `0002-training-job-lifecycle.md`).

Glossaries are created lazily by the `/domain-modeling` skill when terms actually get resolved — do not create empty glossary files.
